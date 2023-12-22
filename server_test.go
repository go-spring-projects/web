package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewServer(t *testing.T) {
	svr := NewServer(Options{})
	assert.NotNil(t, svr)
	assert.Equal(t, ":8080", svr.Addr())
	assert.Equal(t, false, svr.options.IsTls())
	assert.Nil(t, svr.options.TlsConfig())
}
