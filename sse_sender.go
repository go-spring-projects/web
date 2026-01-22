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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// SSESender defines the interface for sending Server-Sent Events.
type SSESender interface {
	// Send sends an event with the given event name and data.
	// If event is empty, the event field will be omitted.
	Send(event, data string) error

	// SendJSON sends an event with JSON-encoded data.
	SendJSON(event string, v interface{}) error

	// SendRetry sets the retry interval in milliseconds for the client.
	SendRetry(retryMS int) error

	// SendComment sends a comment line (useful for keep-alive).
	SendComment(comment string) error

	// Close closes the SSE connection (optional cleanup).
	Close() error
}

// sseSender implements the SSESender interface.
type sseSender struct {
	writer  http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
	closed  bool
}

// NewSSE creates a new SSESender for the given http.ResponseWriter.
// It sets the required HTTP headers for SSE and checks if the writer supports flushing.
func NewSSE(w http.ResponseWriter) (SSESender, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable buffering for Nginx

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported: http.ResponseWriter does not implement http.Flusher")
	}

	// Write initial headers
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	return &sseSender{
		writer:  w,
		flusher: flusher,
		closed:  false,
	}, nil
}

// Send sends an event with the given event name and data.
func (s *sseSender) Send(event, data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("SSE connection closed")
	}

	if event != "" {
		if _, err := fmt.Fprintf(s.writer, "event: %s\n", event); err != nil {
			return err
		}
	}

	// Write data line by line (SSE spec: data field can be multiline)
	lines := splitLines(data)
	for _, line := range lines {
		if _, err := fmt.Fprintf(s.writer, "data: %s\n", line); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(s.writer, "\n"); err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// SendJSON sends an event with JSON-encoded data.
func (s *sseSender) SendJSON(event string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.Send(event, string(data))
}

// SendRetry sets the retry interval in milliseconds for the client.
func (s *sseSender) SendRetry(retryMS int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("SSE connection closed")
	}

	if _, err := fmt.Fprintf(s.writer, "retry: %d\n\n", retryMS); err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// SendComment sends a comment line (useful for keep-alive).
func (s *sseSender) SendComment(comment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("SSE connection closed")
	}

	if _, err := fmt.Fprintf(s.writer, ": %s\n\n", comment); err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// Close marks the SSE connection as closed.
func (s *sseSender) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// splitLines splits data into lines for proper SSE formatting.
// It handles different line endings: \n, \r\n, and \r.
func splitLines(data string) []string {
	// Normalize line endings to \n
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")
	return strings.Split(data, "\n")
}