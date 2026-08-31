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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockResponseWriter implements http.ResponseWriter and http.Flusher for testing.
type mockResponseWriter struct {
	headers    http.Header
	body       *bytes.Buffer
	statusCode int
	flushed    bool
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		headers: make(http.Header),
		body:    &bytes.Buffer{},
	}
}

func (m *mockResponseWriter) Header() http.Header {
	return m.headers
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	return m.body.Write(data)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}

func (m *mockResponseWriter) Flush() {
	m.flushed = true
}

// mockNonFlushingWriter implements http.ResponseWriter but not http.Flusher.
type mockNonFlushingWriter struct {
	headers    http.Header
	body       *bytes.Buffer
	statusCode int
}

type mockControlledWriter struct {
	*mockResponseWriter
	writeErr    error
	flushErr    error
	deadlineErr error
	deadline    time.Time
}

func newMockControlledWriter() *mockControlledWriter {
	return &mockControlledWriter{mockResponseWriter: newMockResponseWriter()}
}

func (m *mockControlledWriter) Write(data []byte) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return m.mockResponseWriter.Write(data)
}

func (m *mockControlledWriter) FlushError() error {
	if m.flushErr != nil {
		return m.flushErr
	}
	m.flushed = true
	return nil
}

func (m *mockControlledWriter) SetWriteDeadline(deadline time.Time) error {
	if m.deadlineErr != nil {
		return m.deadlineErr
	}
	m.deadline = deadline
	return nil
}

type mockWrappedWriter struct{ http.ResponseWriter }

func (m mockWrappedWriter) Unwrap() http.ResponseWriter { return m.ResponseWriter }

func newMockNonFlushingWriter() *mockNonFlushingWriter {
	return &mockNonFlushingWriter{
		headers: make(http.Header),
		body:    &bytes.Buffer{},
	}
}

func (m *mockNonFlushingWriter) Header() http.Header {
	return m.headers
}

func (m *mockNonFlushingWriter) Write(data []byte) (int, error) {
	return m.body.Write(data)
}

func (m *mockNonFlushingWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}

func TestNewSSE(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		w := newMockResponseWriter()
		sender, err := NewSSE(w)
		if err != nil {
			t.Fatalf("NewSSE failed: %v", err)
		}
		if sender == nil {
			t.Fatal("NewSSE returned nil sender")
		}

		// Verify headers
		if ct := w.headers.Get("Content-Type"); ct != "text/event-stream; charset=utf-8" {
			t.Errorf("Content-Type header = %q, want %q", ct, "text/event-stream; charset=utf-8")
		}
		if cc := w.headers.Get("Cache-Control"); cc != "no-cache, no-transform" {
			t.Errorf("Cache-Control header = %q, want %q", cc, "no-cache, no-transform")
		}
		if conn := w.headers.Get("Connection"); conn != "" {
			t.Errorf("Connection header = %q, want empty", conn)
		}
		if xab := w.headers.Get("X-Accel-Buffering"); xab != "no" {
			t.Errorf("X-Accel-Buffering header = %q, want %q", xab, "no")
		}

		// Verify status code and flush
		if w.statusCode != http.StatusOK {
			t.Errorf("status code = %d, want %d", w.statusCode, http.StatusOK)
		}
		if !w.flushed {
			t.Error("Flush was not called")
		}
	})

	t.Run("non-flushing writer", func(t *testing.T) {
		w := newMockNonFlushingWriter()
		sender, err := NewSSE(w)
		if err == nil {
			t.Fatal("NewSSE should fail with non-flushing writer")
		}
		if !strings.Contains(err.Error(), "streaming not supported") {
			t.Errorf("error message = %q, want to contain %q", err.Error(), "streaming not supported")
		}
		if sender != nil {
			t.Error("NewSSE should return nil sender on error")
		}
		if !errors.Is(err, ErrSSEStreamingUnsupported) {
			t.Fatalf("error = %v, want ErrSSEStreamingUnsupported", err)
		}
	})

	t.Run("wrapped flushing writer", func(t *testing.T) {
		base := newMockResponseWriter()
		sender, err := NewSSE(mockWrappedWriter{ResponseWriter: base})
		if err != nil || sender == nil {
			t.Fatalf("NewSSE with wrapped writer = %v, %v", sender, err)
		}
	})

	t.Run("invalid option", func(t *testing.T) {
		sender, err := NewSSE(newMockResponseWriter(), WithSSEWriteTimeout(-time.Second))
		if sender != nil || !errors.Is(err, ErrSSEInvalidOption) {
			t.Fatalf("NewSSE invalid option = %v, %v", sender, err)
		}
		sender, err = NewSSE(newMockResponseWriter(), WithSSEHeartbeat(0, "keep-alive"))
		if sender != nil || !errors.Is(err, ErrSSEInvalidOption) {
			t.Fatalf("NewSSE invalid heartbeat = %v, %v", sender, err)
		}
	})

	t.Run("initial flush error", func(t *testing.T) {
		w := newMockControlledWriter()
		flushErr := errors.New("initial flush failed")
		w.flushErr = flushErr
		sender, err := NewSSE(w)
		if sender != nil || !errors.Is(err, flushErr) {
			t.Fatalf("NewSSE flush error = %v, %v", sender, err)
		}
	})
}

func TestSSESender_Send(t *testing.T) {
	w := newMockResponseWriter()
	sender, err := NewSSE(w)
	if err != nil {
		t.Fatalf("NewSSE failed: %v", err)
	}

	t.Run("send event with data", func(t *testing.T) {
		w.body.Reset()
		w.flushed = false

		err := sender.Send("message", "Hello, World!")
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		expected := "event: message\ndata: Hello, World!\n\n"
		if w.body.String() != expected {
			t.Errorf("body = %q, want %q", w.body.String(), expected)
		}
		if !w.flushed {
			t.Error("Flush was not called")
		}
	})

	t.Run("send data without event", func(t *testing.T) {
		w.body.Reset()
		w.flushed = false

		err := sender.Send("", "Just data")
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		expected := "data: Just data\n\n"
		if w.body.String() != expected {
			t.Errorf("body = %q, want %q", w.body.String(), expected)
		}
		if !w.flushed {
			t.Error("Flush was not called")
		}
	})

	t.Run("send multiline data", func(t *testing.T) {
		w.body.Reset()
		w.flushed = false

		multilineData := "Line 1\nLine 2\nLine 3"
		err := sender.Send("log", multilineData)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		expected := "event: log\ndata: Line 1\ndata: Line 2\ndata: Line 3\n\n"
		if w.body.String() != expected {
			t.Errorf("body = %q, want %q", w.body.String(), expected)
		}
	})

	t.Run("send multiline with CRLF", func(t *testing.T) {
		w.body.Reset()
		w.flushed = false

		// Test with Windows line endings
		multilineData := "Line 1\r\nLine 2\r\nLine 3"
		err := sender.Send("log", multilineData)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Should be normalized to \n
		expected := "event: log\ndata: Line 1\ndata: Line 2\ndata: Line 3\n\n"
		if w.body.String() != expected {
			t.Errorf("body = %q, want %q", w.body.String(), expected)
		}
	})

	t.Run("send after close", func(t *testing.T) {
		w.body.Reset()
		sender.Close()

		err := sender.Send("test", "data")
		if err == nil {
			t.Error("Send should fail after Close")
		}
		if !strings.Contains(err.Error(), "closed") {
			t.Errorf("error message = %q, want to contain %q", err.Error(), "closed")
		}
		if !errors.Is(err, ErrSSEClosed) || !errors.Is(sender.Err(), ErrSSEClosed) {
			t.Fatalf("closed errors = %v, %v", err, sender.Err())
		}
	})
}

func TestSSESender_SendEvent(t *testing.T) {
	w := newMockResponseWriter()
	sender, err := NewSSE(w)
	if err != nil {
		t.Fatal(err)
	}
	w.body.Reset()
	if err := sender.SendEvent(SSEEvent{ID: "event-42", Event: "update", Data: "line one\nline two"}); err != nil {
		t.Fatal(err)
	}
	expected := "id: event-42\nevent: update\ndata: line one\ndata: line two\n\n"
	if body := w.body.String(); body != expected {
		t.Fatalf("body = %q, want %q", body, expected)
	}
}

func TestSSESender_RejectsInvalidFields(t *testing.T) {
	w := newMockResponseWriter()
	sender, err := NewSSE(w)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []SSEEvent{
		{Event: "bad\nevent", Data: "data"},
		{ID: "bad\x00id", Data: "data"},
		{ID: "bad\rid", Data: "data"},
	} {
		if err := sender.SendEvent(event); !errors.Is(err, ErrSSEInvalidEvent) {
			t.Fatalf("SendEvent(%+v) error = %v", event, err)
		}
	}
	if err := sender.SendRetry(-1); !errors.Is(err, ErrSSEInvalidRetry) {
		t.Fatalf("negative retry error = %v", err)
	}
	if err := sender.Send("valid", "still open"); err != nil {
		t.Fatalf("validation error closed sender: %v", err)
	}
}

func TestSSESender_SendJSON(t *testing.T) {
	w := newMockResponseWriter()
	sender, err := NewSSE(w)
	if err != nil {
		t.Fatalf("NewSSE failed: %v", err)
	}

	t.Run("send JSON data", func(t *testing.T) {
		w.body.Reset()
		w.flushed = false

		data := map[string]interface{}{
			"id":   123,
			"name": "Test User",
		}
		err := sender.SendJSON("user", data)
		if err != nil {
			t.Fatalf("SendJSON failed: %v", err)
		}

		// JSON should be marshaled
		expectedJSON := `{"id":123,"name":"Test User"}`
		expected := "event: user\ndata: " + expectedJSON + "\n\n"
		if w.body.String() != expected {
			t.Errorf("body = %q, want %q", w.body.String(), expected)
		}
		if !w.flushed {
			t.Error("Flush was not called")
		}
	})

	t.Run("send JSON without event", func(t *testing.T) {
		w.body.Reset()
		w.flushed = false

		data := []string{"a", "b", "c"}
		err := sender.SendJSON("", data)
		if err != nil {
			t.Fatalf("SendJSON failed: %v", err)
		}

		expectedJSON := `["a","b","c"]`
		expected := "data: " + expectedJSON + "\n\n"
		if w.body.String() != expected {
			t.Errorf("body = %q, want %q", w.body.String(), expected)
		}
	})
}

func TestSSESender_SendRetry(t *testing.T) {
	w := newMockResponseWriter()
	sender, err := NewSSE(w)
	if err != nil {
		t.Fatalf("NewSSE failed: %v", err)
	}

	t.Run("send retry", func(t *testing.T) {
		w.body.Reset()
		w.flushed = false

		err := sender.SendRetry(5000)
		if err != nil {
			t.Fatalf("SendRetry failed: %v", err)
		}

		expected := "retry: 5000\n\n"
		if w.body.String() != expected {
			t.Errorf("body = %q, want %q", w.body.String(), expected)
		}
		if !w.flushed {
			t.Error("Flush was not called")
		}
	})
}

func TestSSESender_SendComment(t *testing.T) {
	w := newMockResponseWriter()
	sender, err := NewSSE(w)
	if err != nil {
		t.Fatalf("NewSSE failed: %v", err)
	}

	t.Run("send comment", func(t *testing.T) {
		w.body.Reset()
		w.flushed = false

		err := sender.SendComment("This is a comment")
		if err != nil {
			t.Fatalf("SendComment failed: %v", err)
		}

		expected := ": This is a comment\n\n"
		if w.body.String() != expected {
			t.Errorf("body = %q, want %q", w.body.String(), expected)
		}
		if !w.flushed {
			t.Error("Flush was not called")
		}
	})

	t.Run("send multiline comment", func(t *testing.T) {
		w.body.Reset()
		if err := sender.SendComment("first\r\nsecond"); err != nil {
			t.Fatal(err)
		}
		expected := ": first\n: second\n\n"
		if body := w.body.String(); body != expected {
			t.Fatalf("body = %q, want %q", body, expected)
		}
	})
}

func TestSSESender_TerminalWriteAndFlushErrors(t *testing.T) {
	t.Run("write error is sticky", func(t *testing.T) {
		w := newMockControlledWriter()
		sender, err := NewSSE(w)
		if err != nil {
			t.Fatal(err)
		}
		writeErr := errors.New("write failed")
		w.writeErr = writeErr
		if err := sender.Send("test", "data"); !errors.Is(err, writeErr) {
			t.Fatalf("send error = %v", err)
		}
		if err := sender.SendRetry(1000); !errors.Is(err, writeErr) {
			t.Fatalf("subsequent error = %v", err)
		}
		if !errors.Is(sender.Err(), writeErr) {
			t.Fatalf("terminal error = %v", sender.Err())
		}
	})

	t.Run("flush error is sticky", func(t *testing.T) {
		w := newMockControlledWriter()
		sender, err := NewSSE(w)
		if err != nil {
			t.Fatal(err)
		}
		flushErr := errors.New("flush failed")
		w.flushErr = flushErr
		if err := sender.Send("test", "data"); !errors.Is(err, flushErr) {
			t.Fatalf("send error = %v", err)
		}
		if !errors.Is(sender.Err(), flushErr) {
			t.Fatalf("terminal error = %v", sender.Err())
		}
	})

	t.Run("short write is terminal", func(t *testing.T) {
		w := newMockControlledWriter()
		sender, err := NewSSE(w)
		if err != nil {
			t.Fatal(err)
		}
		w.writeErr = io.ErrShortWrite
		if err := sender.Send("test", "data"); !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("send error = %v", err)
		}
	})
}

func TestSSESender_WriteDeadline(t *testing.T) {
	w := newMockControlledWriter()
	sender, err := NewSSE(w, WithSSEWriteTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if err := sender.Send("test", "data"); err != nil {
		t.Fatal(err)
	}
	if w.deadline.Before(before.Add(900 * time.Millisecond)) {
		t.Fatalf("write deadline = %v", w.deadline)
	}

	t.Run("deadline error is terminal", func(t *testing.T) {
		w := newMockControlledWriter()
		sender, err := NewSSE(w, WithSSEWriteTimeout(time.Second))
		if err != nil {
			t.Fatal(err)
		}
		deadlineErr := errors.New("deadline failed")
		w.deadlineErr = deadlineErr
		if err := sender.Send("test", "data"); !errors.Is(err, deadlineErr) {
			t.Fatalf("send error = %v", err)
		}
		if !errors.Is(sender.Err(), deadlineErr) {
			t.Fatalf("terminal error = %v", sender.Err())
		}
	})
}

func TestSSESender_Heartbeat(t *testing.T) {
	w := newMockResponseWriter()
	sender, err := NewSSE(w, WithSSEHeartbeat(10*time.Millisecond, "keep-alive"))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(35 * time.Millisecond)
	if err := sender.Close(); err != nil {
		t.Fatal(err)
	}
	body := w.body.String()
	if count := strings.Count(body, ": keep-alive\n\n"); count < 2 {
		t.Fatalf("heartbeat count = %d, want at least 2; body=%q", count, body)
	}
	length := w.body.Len()
	time.Sleep(20 * time.Millisecond)
	if w.body.Len() != length {
		t.Fatalf("heartbeat continued after close: before=%d after=%d", length, w.body.Len())
	}
}

func TestSSESender_Concurrent(t *testing.T) {
	w := newMockResponseWriter()
	sender, err := NewSSE(w)
	if err != nil {
		t.Fatalf("NewSSE failed: %v", err)
	}

	// Test concurrent sends (should be thread-safe)
	errorsCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			errorsCh <- sender.Send("message", "test")
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
	}
	if count := strings.Count(w.body.String(), "event: message\ndata: test\n\n"); count != 10 {
		t.Fatalf("complete event frames = %d, want 10; body=%q", count, w.body.String())
	}
}

func TestSSEHandlerSkipsDefaultRenderer(t *testing.T) {
	router := NewRouter()
	contextSeen := false
	router.Get("/events", SSEHandler(func(ctx context.Context, sender SSESender) error {
		contextSeen = FromContext(ctx) != nil
		return sender.SendEvent(SSEEvent{ID: "1", Event: "ready", Data: "ok"})
	}))
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/events", nil)
	request.Header.Set("Last-Event-ID", "previous")
	router.ServeHTTP(response, request)
	if !contextSeen {
		t.Fatal("SSE stream did not receive web.Context")
	}
	if body := response.Body.String(); body != "id: 1\nevent: ready\ndata: ok\n\n" {
		t.Fatalf("body = %q", body)
	}
	if strings.Contains(response.Body.String(), `"code"`) {
		t.Fatalf("default renderer appended JSON: %q", response.Body.String())
	}
}

func TestSSEHandlerClientDisconnectCancelsContext(t *testing.T) {
	started := make(chan struct{})
	done := make(chan struct{})
	router := NewRouter()
	router.Get("/events", SSEHandler(func(ctx context.Context, sender SSESender) error {
		close(started)
		<-ctx.Done()
		close(done)
		return ctx.Err()
	}))
	server := httptest.NewServer(router)
	defer server.Close()

	response, err := http.Get(server.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE stream context was not canceled after client disconnect")
	}
}

func TestSSELastEventID(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/events", nil)
	request.Header.Set("Last-Event-ID", "event-41")
	if value := SSELastEventID(request); value != "event-41" {
		t.Fatalf("Last-Event-ID = %q", value)
	}
	ctx := &Context{Request: request}
	if value := ctx.SSELastEventID(); value != "event-41" {
		t.Fatalf("Context Last-Event-ID = %q", value)
	}
}

func TestContext_SSE(t *testing.T) {
	t.Run("context SSE method", func(t *testing.T) {
		// Create a test request and recorder
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		// Create Context
		ctx := &Context{
			Writer:  rec,
			Request: req,
		}

		// Get SSE sender
		sender, err := ctx.SSE()
		if err != nil {
			t.Fatalf("Context.SSE() failed: %v", err)
		}
		if sender == nil {
			t.Fatal("Context.SSE() returned nil sender")
		}

		// Verify headers were set
		if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream; charset=utf-8" {
			t.Errorf("Content-Type header = %q, want %q", ct, "text/event-stream; charset=utf-8")
		}

		// Test sending through the sender
		err = sender.Send("test", "data")
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		expected := "event: test\ndata: data\n\n"
		if rec.Body.String() != expected {
			t.Errorf("body = %q, want %q", rec.Body.String(), expected)
		}
	})

	t.Run("context SSE with non-flushing writer", func(t *testing.T) {
		// Use mockNonFlushingWriter which doesn't implement Flusher
		bw := newMockNonFlushingWriter()

		req := httptest.NewRequest("GET", "/", nil)
		ctx := &Context{
			Writer:  bw,
			Request: req,
		}

		// Should fail because bw doesn't implement Flusher
		sender, err := ctx.SSE()
		if err == nil {
			t.Fatal("Context.SSE() should fail with non-flushing writer")
		}
		if !strings.Contains(err.Error(), "streaming not supported") {
			t.Errorf("error message = %q, want to contain %q", err.Error(), "streaming not supported")
		}
		if sender != nil {
			t.Error("Context.SSE() should return nil sender on error")
		}
	})
}
