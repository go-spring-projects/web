package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go-spring.dev/web/render"
)

func TestContextBodyAllowedForStatus(t *testing.T) {
	assert.False(t, false, bodyAllowedForStatus(http.StatusProcessing))
	assert.False(t, false, bodyAllowedForStatus(http.StatusNoContent))
	assert.False(t, false, bodyAllowedForStatus(http.StatusNotModified))
	assert.True(t, true, bodyAllowedForStatus(http.StatusInternalServerError))
}

func TestContext_Context(t *testing.T) {

	request := httptest.NewRequest(http.MethodGet, "/", nil)

	webCtx := &Context{
		Request: request.WithContext(context.WithValue(request.Context(), "test-key001", "val001")),
		Writer:  httptest.NewRecorder(),
	}

	assert.Equal(t, "val001", webCtx.Context().Value("test-key001"))
}

func TestContext_ContentType(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Content-Type", "application/pdf")

	webCtx := &Context{
		Request: request,
		Writer:  httptest.NewRecorder(),
	}

	assert.Equal(t, "application/pdf", webCtx.ContentType())
}

func TestContext_Header(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Token", "9191928928")

	webCtx := &Context{
		Request: request,
		Writer:  httptest.NewRecorder(),
	}

	value, ok := webCtx.Header("X-Token")
	assert.True(t, ok)
	assert.Equal(t, "9191928928", value)

	value, ok = webCtx.Header("X-NotFound")
	assert.False(t, ok)
	assert.Equal(t, "", value)
}

func TestContext_Cookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Cookie", "user=web")

	webCtx := &Context{
		Request: request,
		Writer:  httptest.NewRecorder(),
	}

	value, ok := webCtx.Cookie("user")
	assert.True(t, ok)
	assert.Equal(t, "web", value)

	value, ok = webCtx.Cookie("user-notfound")
	assert.False(t, ok)
	assert.Equal(t, "", value)
}

func TestContext_PathParam(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/endpoint/web888/add", nil)

	var router = NewRouter()
	router.Get("/endpoint/{user}/{action}", func(ctx context.Context) {
		webCtx := FromContext(ctx)
		assert.NotNil(t, webCtx)

		value, ok := webCtx.PathParam("user")
		assert.True(t, ok)
		assert.Equal(t, "web888", value)

		value, ok = webCtx.PathParam("action")
		assert.True(t, ok)
		assert.Equal(t, "add", value)

		value, ok = webCtx.PathParam("user-notfound")
		assert.False(t, ok)
		assert.Equal(t, "", value)
	})

	router.ServeHTTP(httptest.NewRecorder(), request)
}

func TestContext_QueryParam(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/endpoint?user=web123", nil)

	var router = NewRouter().Renderer(JsonRender())
	router.Get("/endpoint", func(ctx context.Context) {
		webCtx := FromContext(ctx)
		assert.NotNil(t, webCtx)

		value, ok := webCtx.QueryParam("user")
		assert.True(t, ok)
		assert.Equal(t, "web123", value)

		value, ok = webCtx.QueryParam("user-notfound")
		assert.False(t, ok)
		assert.Equal(t, "", value)
	})

	router.ServeHTTP(httptest.NewRecorder(), request)
}

func TestContext_FormParams(t *testing.T) {

	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "admin888")

	request := httptest.NewRequest(http.MethodPost, "/endpoint?user=web123", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	webCtx := &Context{Request: request, Writer: httptest.NewRecorder()}
	values, err := webCtx.FormParams()
	assert.NoError(t, err)
	assert.Equal(t, "web123", values.Get("user"))
	assert.Equal(t, "admin", values.Get("username"))
	assert.Equal(t, "admin888", values.Get("password"))
	assert.Equal(t, "", values.Get("notfound-key"))

}

func TestContext_MultipartParams(t *testing.T) {

}

func TestContext_RequestBody(t *testing.T) {

	data := "asdkjhasdkjhdiouwdkwjdnxaxas"

	request := httptest.NewRequest(http.MethodPost, "/endpoint", strings.NewReader(data))
	webCtx := &Context{Request: request, Writer: httptest.NewRecorder()}

	raw, err := io.ReadAll(webCtx.RequestBody())
	assert.NoError(t, err)
	assert.Equal(t, data, string(raw))
}

func TestContext_IsWebsocket(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/endpoint", nil)
	request.Header.Set("Connection", "upgrade")

	webCtx := &Context{Request: request, Writer: httptest.NewRecorder()}
	assert.False(t, webCtx.IsWebsocket())

	request.Header.Set("Upgrade", "websocket")
	assert.True(t, webCtx.IsWebsocket())
}

func TestContext_Status(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/endpoint", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}
	webCtx.Status(http.StatusForbidden)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestContext_SetHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/endpoint", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	webCtx.SetHeader("X-Token", "askdjhsadkjasdjhasd")
	assert.Equal(t, "askdjhsadkjasdjhasd", response.Header().Get("X-Token"))
	assert.Equal(t, "", response.Header().Get("X-Token-notfound"))

}

func TestContext_SetCookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/endpoint", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}
	webCtx.SetSameSite(http.SameSiteLaxMode)
	webCtx.SetCookie("token", "fgdhdfgfgdsf", 600, "/", "localhost", true, true)

	assert.Equal(t, "token=fgdhdfgfgdsf; Path=/; Domain=localhost; Max-Age=600; HttpOnly; Secure; SameSite=Lax", response.Header().Get("Set-Cookie"))
}

func TestContext_Bind(t *testing.T) {
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "admin888")

	request := httptest.NewRequest(http.MethodPost, "/endpoint?ad=876543", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Token", "893ehd892nd")
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	var params = &struct {
		Username string `form:"username"`
		Password string `form:"password"`
		Ad       string `query:"ad"`
		Token    string `header:"X-Token"`
	}{}

	err := webCtx.Bind(params)
	assert.NoError(t, err)
	assert.Equal(t, "admin", params.Username)
	assert.Equal(t, "admin888", params.Password)
	assert.Equal(t, "876543", params.Ad)
	assert.Equal(t, "893ehd892nd", params.Token)
}

func TestContext_Render(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/endpoint", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	err := webCtx.Render(404, render.TextRenderer{Format: "hello: %s", Args: []interface{}{"gs"}})
	assert.NoError(t, err)
	assert.Equal(t, 404, response.Code)
	assert.Equal(t, "hello: gs", response.Body.String())
}

func TestContext_StringRender(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/endpoint", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	err := webCtx.String(200, "hello: %s", "gs")
	assert.NoError(t, err)
	assert.Equal(t, 200, response.Code)
	assert.Equal(t, "hello: gs", response.Body.String())
}

func TestContext_DataRender(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/endpoint", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	err := webCtx.JSON(200, "go-spring")
	assert.NoError(t, err)
	assert.Equal(t, 200, response.Code)
	assert.Equal(t, "\"go-spring\"\n", response.Body.String())
}

func TestContext_XMLRender(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/endpoint", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	err := webCtx.XML(200, "go-spring")
	assert.NoError(t, err)
	assert.Equal(t, 200, response.Code)
	assert.Equal(t, "<string>go-spring</string>", response.Body.String())
}

func TestContext_RemoteIP(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/endpoint", nil)
	request.RemoteAddr = "192.168.1.100:5432"
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}
	assert.Equal(t, "192.168.1.100", webCtx.RemoteIP())
}

func TestContext_ClientIP(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/endpoint", nil)
	request.RemoteAddr = "192.168.1.100:5432"
	request.Header.Set("X-Forwarded-For", "192.168.1.111")
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}
	assert.Equal(t, "192.168.1.111", webCtx.ClientIP())
}
