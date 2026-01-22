package web

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"html/template"

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

	// Test without route context
	request2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	webCtx2 := &Context{
		Request: request2,
		Writer:  httptest.NewRecorder(),
	}
	value, ok := webCtx2.PathParam("any")
	assert.False(t, ok)
	assert.Equal(t, "", value)
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
	// Create a multipart form with fields and a file
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add form fields
	writer.WriteField("username", "admin")
	writer.WriteField("email", "admin@example.com")

	// Add a file
	fileWriter, err := writer.CreateFormFile("avatar", "avatar.jpg")
	assert.NoError(t, err)
	fileWriter.Write([]byte("fake image content"))

	writer.Close()

	// Create request with multipart content type
	request := httptest.NewRequest(http.MethodPost, "/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	webCtx := &Context{Request: request, Writer: response}

	// Test with maxMemory
	form, err := webCtx.MultipartParams(10 << 20) // 10MB
	assert.NoError(t, err)
	assert.NotNil(t, form)

	// Verify form values
	assert.Equal(t, []string{"admin"}, form.Value["username"])
	assert.Equal(t, []string{"admin@example.com"}, form.Value["email"])

	// Verify file
	fileHeaders, ok := form.File["avatar"]
	assert.True(t, ok)
	assert.Equal(t, 1, len(fileHeaders))
	assert.Equal(t, "avatar.jpg", fileHeaders[0].Filename)

	// Test error when content type is not multipart
	request2 := httptest.NewRequest(http.MethodPost, "/upload", nil)
	request2.Header.Set("Content-Type", "application/json")
	webCtx2 := &Context{Request: request2, Writer: httptest.NewRecorder()}

	_, err = webCtx2.MultipartParams(10 << 20)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "require `multipart/form-data` request")
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

func TestContext_Redirect(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/old", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	err := webCtx.Redirect(http.StatusFound, "/new")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusFound, response.Code)
	assert.Equal(t, "/new", response.Header().Get("Location"))
}

func TestContext_Data(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	data := []byte("raw binary data")
	err := webCtx.Data(http.StatusOK, "application/octet-stream", data)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "application/octet-stream", response.Header().Get("Content-Type"))
	assert.Equal(t, data, response.Body.Bytes())
}

func TestContext_IndentedJSON(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	user := User{Name: "Alice", Age: 30}
	err := webCtx.IndentedJSON(http.StatusOK, user)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Header().Get("Content-Type"), "application/json")
	// Check that output is indented (contains newlines and spaces)
	assert.Contains(t, response.Body.String(), "\n")
	assert.Contains(t, response.Body.String(), "  ")
}

func TestContext_IndentedXML(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	type User struct {
		Name string `xml:"name"`
		Age  int    `xml:"age"`
	}
	user := User{Name: "Bob", Age: 25}
	err := webCtx.IndentedXML(http.StatusOK, user)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Header().Get("Content-Type"), "application/xml")
	// Check that output is indented
	assert.Contains(t, response.Body.String(), "\n")
	assert.Contains(t, response.Body.String(), "  ")
}

func TestContext_File(t *testing.T) {
	// Create a temporary file with content
	tmpDir := t.TempDir()
	fileContent := "Hello, World!\nThis is a test file."
	tmpFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(tmpFile, []byte(fileContent), 0644)
	assert.NoError(t, err)

	request := httptest.NewRequest(http.MethodGet, "/download?file=test.txt", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	// This should serve the file
	webCtx.File(tmpFile)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Hello, World!")
	assert.Equal(t, "text/plain; charset=utf-8", response.Header().Get("Content-Type"))
}

func TestContext_FileAttachment(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	fileContent := "Attachment file content"
	tmpFile := filepath.Join(tmpDir, "document.pdf")
	err := os.WriteFile(tmpFile, []byte(fileContent), 0644)
	assert.NoError(t, err)

	request := httptest.NewRequest(http.MethodGet, "/download", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	// Test with ASCII filename
	webCtx.FileAttachment(tmpFile, "document.pdf")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Attachment file content")
	assert.Equal(t, `attachment; filename="document.pdf"`, response.Header().Get("Content-Disposition"))

	// Test with non-ASCII filename
	response2 := httptest.NewRecorder()
	webCtx2 := &Context{Request: request, Writer: response2}
	webCtx2.FileAttachment(tmpFile, "文档.pdf")

	assert.Equal(t, http.StatusOK, response2.Code)
	assert.Contains(t, response2.Header().Get("Content-Disposition"), "filename*=UTF-8''")
}

func TestContext_SetHeader_Delete(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	// First set a header
	webCtx.SetHeader("X-Test", "value")
	assert.Equal(t, "value", response.Header().Get("X-Test"))

	// Then delete it by setting empty value
	webCtx.SetHeader("X-Test", "")
	assert.Equal(t, "", response.Header().Get("X-Test"))
}

func TestContext_isASCII(t *testing.T) {
	assert.True(t, isASCII("Hello"))
	assert.True(t, isASCII("123!@#"))
	assert.False(t, isASCII("Hello 世界"))
	assert.False(t, isASCII("café"))
}

func TestContext_escapeQuotes(t *testing.T) {
	assert.Equal(t, `hello`, escapeQuotes("hello"))
	assert.Equal(t, `\"hello\"`, escapeQuotes(`"hello"`))
	assert.Equal(t, `\\hello\\`, escapeQuotes(`\hello\`))
	assert.Equal(t, `\\\"hello\\\"`, escapeQuotes(`\"hello\"`))
}

func TestContext_replaceWildcards(t *testing.T) {
	assert.Equal(t, "/", replaceWildcards("/"))
	assert.Equal(t, "/api/users", replaceWildcards("/api/users"))
	assert.Equal(t, "/api/users", replaceWildcards("/api/*/users"))
	assert.Equal(t, "/api/users/posts", replaceWildcards("/api/*/users/*/posts"))
	assert.Equal(t, "/api/users/posts/", replaceWildcards("/api/*/users/*/posts/"))
}

func TestRouteContext_AllowedMethods(t *testing.T) {
	rc := &RouteContext{}
	// Initially empty
	assert.Empty(t, rc.AllowedMethods())

	// Add some methods
	rc.methodsAllowed = []methodTyp{mGET, mPOST}
	assert.Equal(t, []string{"GET", "POST"}, rc.AllowedMethods())
}

func TestRouteContext_URLParam(t *testing.T) {
	rc := &RouteContext{}
	rc.URLParams = RouteParams{
		Keys:   []string{"id", "name"},
		Values: []string{"123", "john"},
	}

	assert.Equal(t, "123", rc.URLParam("id"))
	assert.Equal(t, "john", rc.URLParam("name"))
	assert.Equal(t, "", rc.URLParam("nonexistent"))
}

func TestRouteContext_RoutePattern(t *testing.T) {
	rc := &RouteContext{}
	assert.Equal(t, "", rc.RoutePattern())

	rc.routePatterns = []string{"/api/", "users/", "{id}/"}
	assert.Equal(t, "/api/users/{id}", rc.RoutePattern())

	// Test with wildcards
	rc.routePatterns = []string{"/api/", "*/", "users/"}
	assert.Equal(t, "/api/users", rc.RoutePattern())
}

func TestRouteContext_Reset(t *testing.T) {
	rc := &RouteContext{
		Routes:          NewRouter(), // Use a concrete Router type
		RoutePath:       "/test",
		RouteMethod:     "GET",
		routePattern:    "/test",
		routePatterns:   []string{"/test"},
		URLParams:       RouteParams{Keys: []string{"id"}, Values: []string{"1"}},
		routeParams:     RouteParams{Keys: []string{"id"}, Values: []string{"1"}},
		methodNotAllowed: true,
		methodsAllowed:  []methodTyp{mGET, mPOST},
	}

	rc.Reset()

	assert.Nil(t, rc.Routes)
	assert.Equal(t, "", rc.RoutePath)
	assert.Equal(t, "", rc.RouteMethod)
	assert.Equal(t, "", rc.routePattern)
	assert.Empty(t, rc.routePatterns)
	assert.Empty(t, rc.URLParams.Keys)
	assert.Empty(t, rc.URLParams.Values)
	assert.Empty(t, rc.routeParams.Keys)
	assert.Empty(t, rc.routeParams.Values)
	assert.False(t, rc.methodNotAllowed)
	assert.Empty(t, rc.methodsAllowed)
}

func TestFromContext(t *testing.T) {
	// Test when context has no web context
	ctx := context.Background()
	assert.Nil(t, FromContext(ctx))

	// Test when context has web context
	webCtx := &Context{Request: &http.Request{}}
	ctx2 := WithContext(context.Background(), webCtx)
	assert.Equal(t, webCtx, FromContext(ctx2))
}

func TestURLParam(t *testing.T) {
	// Create a request with route context
	rc := &RouteContext{
		URLParams: RouteParams{
			Keys:   []string{"id"},
			Values: []string{"123"},
		},
	}
	ctx := WithRouteContext(context.Background(), rc)
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request = request.WithContext(ctx)

	assert.Equal(t, "123", URLParam(request, "id"))
	assert.Equal(t, "", URLParam(request, "nonexistent"))

	// Test without route context (should return empty string)
	request2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	assert.Equal(t, "", URLParam(request2, "id"))

	// Test with nil route context
	ctx3 := WithRouteContext(context.Background(), nil)
	request3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	request3 = request3.WithContext(ctx3)
	assert.Equal(t, "", URLParam(request3, "id"))
}

func TestHttpError_Error(t *testing.T) {
	err := HttpError{Code: 404, Message: "Not Found"}
	assert.Equal(t, "404: Not Found", err.Error())

	err2 := HttpError{Code: 500, Message: "Internal Server Error"}
	assert.Equal(t, "500: Internal Server Error", err2.Error())
}

func TestError(t *testing.T) {
	// Test with default message
	err := Error(http.StatusNotFound, "")
	assert.Equal(t, http.StatusNotFound, err.Code)
	assert.Equal(t, "Not Found", err.Message)

	// Test with custom message
	err2 := Error(http.StatusBadRequest, "Invalid input: %s", "email")
	assert.Equal(t, http.StatusBadRequest, err2.Code)
	assert.Equal(t, "Invalid input: email", err2.Message)
}

func TestContext_HTML(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	err := webCtx.HTML(200, "<h1>Hello, World!</h1>")
	assert.NoError(t, err)
	assert.Equal(t, 200, response.Code)
	assert.Equal(t, "text/html; charset=utf-8", response.Header().Get("Content-Type"))
	assert.Equal(t, "<h1>Hello, World!</h1>", response.Body.String())
}

func TestContext_HTMLTemplate(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	webCtx := &Context{Request: request, Writer: response}

	tmpl := template.Must(template.New("test").Parse("<h1>Hello, {{.Name}}!</h1>"))
	data := struct{ Name string }{Name: "World"}

	err := webCtx.HTMLTemplate(200, tmpl, "test", data)
	assert.NoError(t, err)
	assert.Equal(t, 200, response.Code)
	assert.Equal(t, "text/html; charset=utf-8", response.Header().Get("Content-Type"))
	assert.Equal(t, "<h1>Hello, World!</h1>", response.Body.String())
}
