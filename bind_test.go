package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsErrorType(t *testing.T) {
	err := fmt.Errorf("error")
	assert.True(t, isErrorType(reflect.TypeOf(err)))
	err = os.ErrClosed
	assert.True(t, isErrorType(reflect.TypeOf(err)))
}

func TestIsContextType(t *testing.T) {
	ctx := context.TODO()
	assert.True(t, isContextType(reflect.TypeOf(ctx)))
	ctx = context.WithValue(context.TODO(), "a", "3")
	assert.True(t, isContextType(reflect.TypeOf(ctx)))
}

func TestBindWithoutParams(t *testing.T) {

	var handler = func(ctx context.Context) string {
		webCtx := FromContext(ctx)
		assert.NotNil(t, webCtx)
		return "0987654321"
	}

	request := httptest.NewRequest(http.MethodGet, "/get", strings.NewReader("{}"))
	response := httptest.NewRecorder()
	Bind(handler, JsonRender())(response, request)
	assert.Equal(t, response.Body.String(), "{\"code\":0,\"data\":\"0987654321\"}\n")
}

func TestBindWithParams(t *testing.T) {
	var handler = func(ctx context.Context, req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}) string {
		webCtx := FromContext(ctx)
		assert.NotNil(t, webCtx)
		assert.Equal(t, req.Username, "aaa")
		assert.Equal(t, req.Password, "88888888")
		return "success"
	}

	request := httptest.NewRequest(http.MethodPost, "/post", strings.NewReader(`{"username": "aaa", "password": "88888888"}`))
	request.Header.Add("Content-Type", "application/json")
	response := httptest.NewRecorder()
	Bind(handler, JsonRender())(response, request)
	assert.Equal(t, response.Body.String(), "{\"code\":0,\"data\":\"success\"}\n")
}

func TestBindWithParamsAndError(t *testing.T) {
	var handler = func(ctx context.Context, req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}) (string, error) {
		webCtx := FromContext(ctx)
		assert.NotNil(t, webCtx)
		assert.Equal(t, req.Username, "aaa")
		assert.Equal(t, req.Password, "88888888")
		return "requestid: 9999999", Error(403, "user locked")
	}

	request := httptest.NewRequest(http.MethodPost, "/post", strings.NewReader(`{"username": "aaa", "password": "88888888"}`))
	request.Header.Add("Content-Type", "application/json")
	response := httptest.NewRecorder()
	Bind(handler, JsonRender())(response, request)
	assert.Equal(t, response.Body.String(), "{\"code\":403,\"message\":\"user locked\",\"data\":\"requestid: 9999999\"}\n")
}
