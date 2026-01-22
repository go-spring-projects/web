package web

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewServer(t *testing.T) {
	svr := NewServer(Options{})
	assert.NotNil(t, svr)
	assert.Equal(t, ":8080", svr.Addr())
	assert.Equal(t, false, svr.options.IsTls())
	assert.Nil(t, svr.options.TlsConfig())
}

func TestServer_Run(t *testing.T) {
	// Create a server with a random port to avoid conflicts
	svr := NewServer(Options{Addr: ":0"})

	// Run server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- svr.Run()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Server should be running, try to shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := svr.Shutdown(ctx)
	assert.NoError(t, err)

	// Wait for Run to return
	select {
	case err := <-errCh:
		// Run returns "http: Server closed" when shutdown gracefully
		assert.ErrorContains(t, err, "Server closed")
	case <-time.After(2 * time.Second):
		t.Fatal("server didn't stop in time")
	}
}

func TestServer_Shutdown(t *testing.T) {
	svr := NewServer(Options{Addr: ":0"})

	// Shutdown on a non-running server should not error
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := svr.Shutdown(ctx)
	assert.NoError(t, err)
}
