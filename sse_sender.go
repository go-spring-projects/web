/*
 * Copyright 2026 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrSSEClosed               = errors.New("SSE connection closed")
	ErrSSEStreamingUnsupported = errors.New("SSE streaming not supported")
	ErrSSEInvalidEvent         = errors.New("invalid SSE event")
	ErrSSEInvalidRetry         = errors.New("invalid SSE retry interval")
	ErrSSEInvalidOption        = errors.New("invalid SSE option")
)

// SSEEvent is one Server-Sent Event frame. ID is optional and, when present,
// is sent as the event ID used by EventSource reconnection.
type SSEEvent struct {
	ID    string
	Event string
	Data  string
}

// SSESender sends Server-Sent Event frames to one HTTP connection.
type SSESender interface {
	// Send sends an event with the given event name and data.
	// If event is empty, the event field is omitted.
	Send(event, data string) error

	// SendEvent sends a structured event with optional ID and event name.
	SendEvent(event SSEEvent) error

	// SendJSON sends an event with JSON-encoded data.
	SendJSON(event string, v interface{}) error

	// SendRetry sets the retry interval in milliseconds for the client.
	SendRetry(retryMS int) error

	// SendComment sends one or more comment lines (useful for keep-alive).
	SendComment(comment string) error

	// Err returns the terminal sender error, or nil while the sender is open.
	Err() error

	// Close marks the sender closed. The HTTP transport is owned by net/http
	// and closes when the request handler returns.
	Close() error
}

type SSEOption func(*sseOptions) error

type sseOptions struct {
	writeTimeout      time.Duration
	heartbeatInterval time.Duration
	heartbeatComment  string
}

// WithSSEHeartbeat sends a comment at the given interval until the sender is
// closed or a write fails.
func WithSSEHeartbeat(interval time.Duration, comment string) SSEOption {
	return func(options *sseOptions) error {
		if interval <= 0 {
			return fmt.Errorf("%w: heartbeat interval must be positive", ErrSSEInvalidOption)
		}
		options.heartbeatInterval = interval
		options.heartbeatComment = comment
		return nil
	}
}

// WithSSEWriteTimeout bounds each event write and flush. A zero timeout keeps
// the server's existing write deadline behavior.
func WithSSEWriteTimeout(timeout time.Duration) SSEOption {
	return func(options *sseOptions) error {
		if timeout < 0 {
			return fmt.Errorf("%w: write timeout cannot be negative", ErrSSEInvalidOption)
		}
		options.writeTimeout = timeout
		return nil
	}
}

// SSEStream is invoked by SSEHandler after the event stream is established.
// Returning stops the stream; errors cannot be rendered after headers commit.
type SSEStream func(context.Context, SSESender) error

// SSEHandler adapts an SSE stream to an http.HandlerFunc. Because the returned
// value is a standard handler, Router does not invoke its default Renderer when
// the stream ends.
func SSEHandler(stream SSEStream, options ...SSEOption) http.HandlerFunc {
	if stream == nil {
		panic("SSE stream is nil")
	}
	return func(writer http.ResponseWriter, request *http.Request) {
		sender, err := NewSSE(writer, options...)
		if err != nil {
			if errors.Is(err, ErrSSEStreamingUnsupported) || errors.Is(err, ErrSSEInvalidOption) {
				http.Error(writer, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		defer sender.Close()
		_ = stream(request.Context(), sender)
	}
}

// SSELastEventID returns the event ID supplied by a reconnecting EventSource.
func SSELastEventID(request *http.Request) string {
	if request == nil {
		return ""
	}
	return request.Header.Get("Last-Event-ID")
}

type sseSender struct {
	writer            http.ResponseWriter
	controller        *http.ResponseController
	writeTimeout      time.Duration
	heartbeatInterval time.Duration
	heartbeatComment  string
	done              chan struct{}

	mu          sync.Mutex
	closed      bool
	doneClosed  bool
	terminalErr error
}

// NewSSE creates an SSE sender, commits status 200, and flushes the headers.
func NewSSE(writer http.ResponseWriter, optionList ...SSEOption) (SSESender, error) {
	options := sseOptions{}
	for _, option := range optionList {
		if option == nil {
			return nil, fmt.Errorf("%w: option is nil", ErrSSEInvalidOption)
		}
		if err := option(&options); err != nil {
			return nil, err
		}
	}
	if !supportsSSEFlush(writer) {
		return nil, fmt.Errorf("%w: http.ResponseWriter cannot flush", ErrSSEStreamingUnsupported)
	}

	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache, no-transform")
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.Header().Del("Connection")
	writer.Header().Del("Content-Length")

	controller := http.NewResponseController(writer)
	writer.WriteHeader(http.StatusOK)
	if err := controller.Flush(); err != nil {
		return nil, fmt.Errorf("flush SSE headers: %w", err)
	}
	sender := &sseSender{
		writer:            writer,
		controller:        controller,
		writeTimeout:      options.writeTimeout,
		heartbeatInterval: options.heartbeatInterval,
		heartbeatComment:  options.heartbeatComment,
		done:              make(chan struct{}),
	}
	if sender.heartbeatInterval > 0 {
		go sender.runHeartbeat()
	}
	return sender, nil
}

func (s *sseSender) Send(event, data string) error {
	return s.SendEvent(SSEEvent{Event: event, Data: data})
}

func (s *sseSender) SendEvent(event SSEEvent) error {
	frame, err := encodeSSEEvent(event)
	if err != nil {
		return err
	}
	return s.writeFrame(frame)
}

func (s *sseSender) SendJSON(event string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.Send(event, string(data))
}

func (s *sseSender) SendRetry(retryMS int) error {
	if retryMS < 0 {
		return fmt.Errorf("%w: milliseconds cannot be negative", ErrSSEInvalidRetry)
	}
	return s.writeFrame([]byte(fmt.Sprintf("retry: %d\n\n", retryMS)))
}

func (s *sseSender) SendComment(comment string) error {
	var frame bytes.Buffer
	for _, line := range splitLines(strings.ToValidUTF8(comment, "\uFFFD")) {
		_, _ = fmt.Fprintf(&frame, ": %s\n", line)
	}
	frame.WriteByte('\n')
	return s.writeFrame(frame.Bytes())
}

func (s *sseSender) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.terminalErr
}

func (s *sseSender) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.closeDoneLocked()
		if s.terminalErr == nil {
			s.terminalErr = ErrSSEClosed
		}
	}
	return nil
}

func (s *sseSender) writeFrame(frame []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		if s.terminalErr != nil {
			return s.terminalErr
		}
		return ErrSSEClosed
	}
	if s.writeTimeout > 0 {
		if err := s.controller.SetWriteDeadline(time.Now().Add(s.writeTimeout)); err != nil {
			return s.failLocked(fmt.Errorf("set SSE write deadline: %w", err))
		}
	}
	written, err := s.writer.Write(frame)
	if err != nil {
		return s.failLocked(err)
	}
	if written != len(frame) {
		return s.failLocked(io.ErrShortWrite)
	}
	if err := s.controller.Flush(); err != nil {
		return s.failLocked(err)
	}
	return nil
}

func (s *sseSender) failLocked(err error) error {
	s.closed = true
	s.terminalErr = err
	s.closeDoneLocked()
	return err
}

func (s *sseSender) closeDoneLocked() {
	if !s.doneClosed {
		close(s.done)
		s.doneClosed = true
	}
}

func (s *sseSender) runHeartbeat() {
	ticker := time.NewTicker(s.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			if err := s.SendComment(s.heartbeatComment); err != nil {
				return
			}
		}
	}
}

func encodeSSEEvent(event SSEEvent) ([]byte, error) {
	if containsSSEFieldBreak(event.Event) {
		return nil, fmt.Errorf("%w: event name contains a line break", ErrSSEInvalidEvent)
	}
	if containsSSEFieldBreak(event.ID) || strings.ContainsRune(event.ID, '\x00') {
		return nil, fmt.Errorf("%w: event ID contains a prohibited character", ErrSSEInvalidEvent)
	}
	var frame bytes.Buffer
	if event.ID != "" {
		_, _ = fmt.Fprintf(&frame, "id: %s\n", strings.ToValidUTF8(event.ID, "\uFFFD"))
	}
	if event.Event != "" {
		_, _ = fmt.Fprintf(&frame, "event: %s\n", strings.ToValidUTF8(event.Event, "\uFFFD"))
	}
	for _, line := range splitLines(strings.ToValidUTF8(event.Data, "\uFFFD")) {
		_, _ = fmt.Fprintf(&frame, "data: %s\n", line)
	}
	frame.WriteByte('\n')
	return frame.Bytes(), nil
}

func containsSSEFieldBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

func supportsSSEFlush(writer http.ResponseWriter) bool {
	switch value := writer.(type) {
	case interface{ FlushError() error }:
		return true
	case http.Flusher:
		return true
	case interface{ Unwrap() http.ResponseWriter }:
		return supportsSSEFlush(value.Unwrap())
	default:
		return false
	}
}

func splitLines(data string) []string {
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")
	return strings.Split(data, "\n")
}
