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
	assert.Equal(t, "{\"code\":0,\"data\":\"0987654321\"}\n", response.Body.String())
}

func TestBindWithParams(t *testing.T) {
	var handler = func(ctx context.Context, req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}) string {
		webCtx := FromContext(ctx)
		assert.NotNil(t, webCtx)
		assert.Equal(t, "aaa", req.Username)
		assert.Equal(t, "88888888", req.Password)
		return "success"
	}

	request := httptest.NewRequest(http.MethodPost, "/post", strings.NewReader(`{"username": "aaa", "password": "88888888"}`))
	request.Header.Add("Content-Type", "application/json")
	response := httptest.NewRecorder()
	Bind(handler, JsonRender())(response, request)
	assert.Equal(t, "{\"code\":0,\"data\":\"success\"}\n", response.Body.String())
}

func TestBindWithParamsAndWebError(t *testing.T) {
	var handler = func(ctx context.Context, req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}) (string, error) {
		webCtx := FromContext(ctx)
		assert.NotNil(t, webCtx)
		assert.Equal(t, "aaa", req.Username)
		assert.Equal(t, "88888888", req.Password)
		return "requestid: 9999999", Error(403, "user locked")
	}

	request := httptest.NewRequest(http.MethodPost, "/post", strings.NewReader(`{"username": "aaa", "password": "88888888"}`))
	request.Header.Add("Content-Type", "application/json")
	response := httptest.NewRecorder()
	Bind(handler, JsonRender())(response, request)
	assert.Equal(t, "{\"code\":403,\"message\":\"user locked\",\"data\":\"requestid: 9999999\"}\n", response.Body.String())
}

func TestBindWithParamsAndError(t *testing.T) {
	var handler = func(ctx context.Context, req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}) (string, error) {
		webCtx := FromContext(ctx)
		assert.NotNil(t, webCtx)
		assert.Equal(t, "aaa", req.Username)
		assert.Equal(t, "88888888", req.Password)
		return "requestid: 9999999", fmt.Errorf("user locked")
	}

	request := httptest.NewRequest(http.MethodPost, "/post", strings.NewReader(`{"username": "aaa", "password": "88888888"}`))
	request.Header.Add("Content-Type", "application/json")
	response := httptest.NewRecorder()
	Bind(handler, JsonRender())(response, request)
	assert.Equal(t, "{\"code\":500,\"message\":\"user locked\",\"data\":\"requestid: 9999999\"}\n", response.Body.String())
}

func TestBind(t *testing.T) {

	var testBind = func(h interface{}, expected string) {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"username": "aaa", "password": "88888888"}`))
		request.Header.Add("Content-Type", "application/json")

		response := httptest.NewRecorder()

		handler := Bind(h, JsonRender())
		assert.NotNil(t, handler)

		handler.ServeHTTP(response, request)
		assert.Equal(t, expected, response.Body.String())
	}

	var cases = []struct {
		Fn       interface{}
		Expected string
	}{
		{Fn: func(ctx context.Context) {}, Expected: ""},
		{Fn: func(ctx context.Context) error { return nil }, Expected: "{\"code\":0,\"data\":null}\n"},
		{Fn: func(ctx context.Context) string { return "ok" }, Expected: "{\"code\":0,\"data\":\"ok\"}\n"},
		{Fn: func(ctx context.Context) (string, error) { return "ok", nil }, Expected: "{\"code\":0,\"data\":\"ok\"}\n"},

		{Fn: func(ctx context.Context, req struct{}) {}, Expected: ""},
		{Fn: func(ctx context.Context, req struct{}) error { return nil }, Expected: "{\"code\":0,\"data\":null}\n"},
		{Fn: func(ctx context.Context, req struct{}) string { return "ok" }, Expected: "{\"code\":0,\"data\":\"ok\"}\n"},
		{Fn: func(ctx context.Context, req *struct{}) (string, error) { return "ok", nil }, Expected: "{\"code\":0,\"data\":\"ok\"}\n"},

		{Fn: func(w http.ResponseWriter, r *http.Request) {}, Expected: ""},
		{Fn: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), Expected: ""},
		{Fn: http.NewServeMux(), Expected: "404 page not found\n"},
	}

	for _, c := range cases {
		testBind(c.Fn, c.Expected)
	}
}

func TestValidMappingFunc(t *testing.T) {

	var cases = []struct {
		Fn       interface{}
		Expected interface{}
	}{
		{Fn: func(ctx context.Context) {}, Expected: nil},
		{Fn: func(ctx context.Context) error { return nil }, Expected: nil},
		{Fn: func(ctx context.Context) string { return "ok" }, Expected: nil},
		{Fn: func(ctx context.Context) (string, error) { return "ok", nil }, Expected: nil},

		{Fn: func(ctx context.Context, req struct{}) error { return nil }, Expected: nil},
		{Fn: func(ctx context.Context, req struct{}) string { return "ok" }, Expected: nil},
		{Fn: func(ctx context.Context, req *struct{}) (string, error) { return "ok", nil }, Expected: nil},

		{Fn: func() {}, Expected: "func(): expect func(ctx context.Context, [T]) [R, error]"},
		{Fn: func(ctx context.Context) (error, string) { return nil, "" }, Expected: "func(context.Context) (error, string): expect func(...) (R, error)"},
		{Fn: func(ctx context.Context) (string, int32, error) { return "", 0, nil }, Expected: "func(context.Context) (string, int32, error): expect func(ctx context.Context, [T]) [(R, error)]"},
		{Fn: func(ctx context.Context, name string) {}, Expected: "func(context.Context, string): input param type (string) must be struct/*struct"},
		{Fn: func(ctx context.Context, name *string) error { return nil }, Expected: "func(context.Context, *string) error: input param type (*string) must be struct/*struct"},
		{Fn: func(ctx context.Context, name string, age int32) {}, Expected: "func(context.Context, string, int32): expect func(ctx context.Context, [T]) [R, error]"},
		{Fn: func(w http.ResponseWriter, r *http.Request) {}, Expected: "func(http.ResponseWriter, *http.Request): expect func(ctx context.Context, [T]) [(R, error)"},
		{Fn: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), Expected: "http.HandlerFunc: expect func(ctx context.Context, [T]) [(R, error)"},
		{Fn: http.NewServeMux(), Expected: "*http.ServeMux: not a func"},
	}

	for _, c := range cases {
		if nil == c.Expected {
			assert.NoError(t, validMappingFunc(reflect.TypeOf(c.Fn)))
		} else {
			assert.ErrorContains(t, validMappingFunc(reflect.TypeOf(c.Fn)), c.Expected.(string))
		}
	}

}
