/*
 * Copyright 2023 the original author or authors.
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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMiddlewares_Handler(t *testing.T) {
	// Create a simple middleware that adds a header
	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware1", "processed")
			next.ServeHTTP(w, r)
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware2", "processed")
			next.ServeHTTP(w, r)
		})
	}

	mws := Middlewares{middleware1, middleware2}

	// Create a final handler
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("final"))
	})

	// Build handler chain
	handler := mws.Handler(finalHandler)

	// Test the chain
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "processed", response.Header().Get("X-Middleware1"))
	assert.Equal(t, "processed", response.Header().Get("X-Middleware2"))
	assert.Equal(t, "final", response.Body.String())
}

func TestMiddlewares_HandlerFunc(t *testing.T) {
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "value")
			next.ServeHTTP(w, r)
		})
	}

	mws := Middlewares{middleware}
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("done"))
	})

	handler := mws.HandlerFunc(finalHandler)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	assert.Equal(t, "value", response.Header().Get("X-Test"))
	assert.Equal(t, "done", response.Body.String())
}

func TestMiddlewares_chain_Empty(t *testing.T) {
	mws := Middlewares{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
	})

	chained := mws.chain(handler)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	chained.ServeHTTP(response, request)

	assert.Equal(t, "test", response.Body.String())
}

func TestChainHandler_Unwrap(t *testing.T) {
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	chain := &ChainHandler{
		Endpoint: finalHandler,
		chain:    finalHandler,
		Middlewares: Middlewares{},
	}

	// Unwrap should return the endpoint handler
	assert.NotNil(t, chain.Unwrap())
}

func TestChainHandler_ServeHTTP(t *testing.T) {
	called := false
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte("ok"))
	})

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware", "yes")
			next.ServeHTTP(w, r)
		})
	}

	mws := Middlewares{middleware}
	chain := mws.chain(finalHandler)
	handler := &ChainHandler{
		Endpoint:    finalHandler,
		chain:       chain,
		Middlewares: mws,
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	assert.True(t, called)
	assert.Equal(t, "yes", response.Header().Get("X-Middleware"))
	assert.Equal(t, "ok", response.Body.String())
}

func TestNoCache(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("content"))
	})

	wrapped := NoCache(handler)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("ETag", "\"123\"")
	request.Header.Set("If-Modified-Since", "Wed, 21 Oct 2015 07:28:00 GMT")

	response := httptest.NewRecorder()
	wrapped.ServeHTTP(response, request)

	// Check that ETag headers are removed from request
	assert.Equal(t, "", request.Header.Get("ETag"))
	assert.Equal(t, "", request.Header.Get("If-Modified-Since"))

	// Check that no-cache headers are set in response
	assert.Equal(t, epoch, response.Header().Get("Expires"))
	assert.Equal(t, "no-cache, no-store, no-transform, must-revalidate, private, max-age=0", response.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", response.Header().Get("Pragma"))
	assert.Equal(t, "0", response.Header().Get("X-Accel-Expires"))
	assert.Equal(t, "content", response.Body.String())
}

func TestRecovery(t *testing.T) {
	// Create a handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Use Recovery middleware
	handler := Recovery()(panicHandler)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	// This should not panic, but recover and return 500
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestRecoveryTo(t *testing.T) {
	// Create a buffer to capture panic output
	var output strings.Builder

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("custom panic")
	})

	handler := RecoveryTo(&output)(panicHandler)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	// Check that panic was logged to our buffer
	assert.Contains(t, output.String(), "custom panic")
	assert.Contains(t, output.String(), "[recovered]")
}

func TestProfiler(t *testing.T) {
	handler := Profiler()

	// Test that profiler returns a router
	assert.NotNil(t, handler)

	// Test a few profiler endpoints
	request := httptest.NewRequest(http.MethodGet, "/pprof/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	// pprof index should return 200 OK
	assert.Equal(t, http.StatusOK, response.Code)

	// Test vars endpoint
	request2 := httptest.NewRequest(http.MethodGet, "/vars", nil)
	response2 := httptest.NewRecorder()
	handler.ServeHTTP(response2, request2)

	// expvar handler should return 200 OK
	assert.Equal(t, http.StatusOK, response2.Code)
}