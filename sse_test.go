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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		if ct := w.headers.Get("Content-Type"); ct != "text/event-stream" {
			t.Errorf("Content-Type header = %q, want %q", ct, "text/event-stream")
		}
		if cc := w.headers.Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("Cache-Control header = %q, want %q", cc, "no-cache")
		}
		if conn := w.headers.Get("Connection"); conn != "keep-alive" {
			t.Errorf("Connection header = %q, want %q", conn, "keep-alive")
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
	})
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
}

func TestSSESender_Concurrent(t *testing.T) {
	w := newMockResponseWriter()
	sender, err := NewSSE(w)
	if err != nil {
		t.Fatalf("NewSSE failed: %v", err)
	}

	// Test concurrent sends (should be thread-safe)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			sender.Send("message", "test")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have some data
	if w.body.Len() == 0 {
		t.Error("No data written from concurrent sends")
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
		if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
			t.Errorf("Content-Type header = %q, want %q", ct, "text/event-stream")
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