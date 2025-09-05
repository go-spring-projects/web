/*
 * Copyright 2019 the original author or authors.
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

package binding_test

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go-spring.dev/web/binding"
)

type FormBindParamCommon struct {
	A string   `form:"a"`
	B []string `form:"b"`
}

type FormBindParam struct {
	FormBindParamCommon
	C int   `form:"c"`
	D []int `form:"d"`
	E int   `form:"e"`
}

func TestBindForm(t *testing.T) {

	ctx := &MockRequest{
		method:      "POST",
		contentType: "application/x-www-form-urlencoded",
		formParams: url.Values{
			"a": {"1"},
			"b": {"2", "3"},
			"c": {"4"},
			"d": {"5", "6"},
		},
	}

	expect := FormBindParam{
		FormBindParamCommon: FormBindParamCommon{
			A: "1",
			B: []string{"2", "3"},
		},
		C: 4,
		D: []int{5, 6},
	}

	var p FormBindParam
	err := binding.Bind(&p, ctx)
	assert.Nil(t, err)
	assert.Equal(t, expect, p)
}

func TestBindMultipartForm(t *testing.T) {
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)
	a, err := mw.CreateFormField("a")
	if assert.NoError(t, err) {
		_, err = a.Write([]byte("111"))
		assert.NoError(t, err)
	}
	b, err := mw.CreateFormField("b")
	if assert.NoError(t, err) {
		_, err = b.Write([]byte("hello"))
		assert.NoError(t, err)
	}
	c, err := mw.CreateFormField("c")
	if assert.NoError(t, err) {
		_, err = c.Write([]byte("first"))
		assert.NoError(t, err)
	}
	c, err = mw.CreateFormField("c")
	if assert.NoError(t, err) {
		_, err = c.Write([]byte("second"))
		assert.NoError(t, err)
	}
	w, err := mw.CreateFormFile("file", "test1")
	if assert.NoError(t, err) {
		_, err = w.Write([]byte("test1111111"))
		assert.NoError(t, err)
	}

	w, err = mw.CreateFormFile("file", "test2")
	if assert.NoError(t, err) {
		_, err = w.Write([]byte("test2222222"))
		assert.NoError(t, err)
	}
	mw.Close()

	request, err := http.NewRequest("POST", "/", buf)
	assert.NoError(t, err)
	request.Header.Set("Content-Type", mw.FormDataContentType())

	var params = &struct {
		A     int                     `form:"a"`
		B     string                  `form:"b"`
		C     []string                `form:"c"`
		Files []*multipart.FileHeader `form:"file"`
	}{}

	err = binding.BindMultipartForm(params, testRequest{request})
	assert.NoError(t, err)
	assert.Equal(t, 111, params.A)
	assert.Equal(t, "hello", params.B)
	assert.Equal(t, []string{"first", "second"}, params.C)
	assert.Equal(t, 2, len(params.Files))
	assert.Equal(t, "test1", params.Files[0].Filename)
	assert.Equal(t, "test2", params.Files[1].Filename)

	{
		file, err := params.Files[0].Open()
		assert.NoError(t, err)
		defer file.Close()

		fileData, err := io.ReadAll(file)
		assert.NoError(t, err)
		assert.Equal(t, "test1111111", string(fileData))
	}

	{
		file, err := params.Files[1].Open()
		assert.NoError(t, err)
		defer file.Close()

		fileData, err := io.ReadAll(file)
		assert.NoError(t, err)
		assert.Equal(t, "test2222222", string(fileData))
	}
}

type testRequest struct {
	*http.Request
}

func (r testRequest) Method() string {
	return r.Request.Method
}

func (r testRequest) ContentType() string {
	contentType := r.Request.Header.Get("Content-Type")
	return contentType
}

func (r testRequest) Header(key string) (string, bool) {
	if values, ok := r.Request.Header[textproto.CanonicalMIMEHeaderKey(key)]; ok && len(values) > 0 {
		return values[0], true
	}
	return "", false
}

func (r testRequest) Cookie(name string) (string, bool) {
	cookie, err := r.Request.Cookie(name)
	if err != nil {
		return "", false
	}
	if val, err := url.QueryUnescape(cookie.Value); nil == err {
		return val, true
	}
	return cookie.Value, true
}

func (r testRequest) PathParam(name string) (string, bool) {
	return "", false
}

func (r testRequest) QueryParam(name string) (string, bool) {
	if values := r.Request.URL.Query(); nil != values {
		if value, ok := values[name]; ok && len(value) > 0 {
			return value[0], true
		}
	}
	return "", false
}

func (r testRequest) FormParams() (url.Values, error) {
	if err := r.Request.ParseForm(); nil != err {
		return nil, err
	}
	return r.Request.Form, nil
}

func (r testRequest) MultipartParams(maxMemory int64) (*multipart.Form, error) {
	if !strings.Contains(r.ContentType(), binding.MIMEMultipartForm) {
		return nil, fmt.Errorf("require `multipart/form-data` request")
	}

	if nil == r.Request.MultipartForm {
		if err := r.Request.ParseMultipartForm(maxMemory); nil != err {
			return nil, err
		}
	}
	return r.Request.MultipartForm, nil
}

func (r testRequest) RequestBody() io.Reader {
	return r.Request.Body
}
