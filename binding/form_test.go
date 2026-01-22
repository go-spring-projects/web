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

func TestBindFormErrors(t *testing.T) {
	// Test BindForm with non-pointer (should return nil)
	type FormStruct struct {
		Name string `form:"name"`
	}

	ctx := &MockRequest{
		method:      "POST",
		contentType: "application/x-www-form-urlencoded",
		formParams: url.Values{"name": []string{"test"}},
	}

	var fs FormStruct
	// Directly test BindForm with non-pointer
	err := binding.BindForm(fs, ctx)
	assert.Nil(t, err, "BindForm should return nil for non-pointer")

	// Test with non-struct pointer
	var i int
	err = binding.BindForm(&i, ctx)
	assert.Nil(t, err, "BindForm should return nil for non-struct pointer")

	// Test with pointer to slice
	var s []string
	err = binding.BindForm(&s, ctx)
	assert.Nil(t, err, "BindForm should return nil for pointer to slice")

	// Test with unexported field (should be skipped)
	type UnexportedForm struct {
		name string `form:"name"` // unexported
		Age  int    `form:"age"`
	}

	ctx.formParams = url.Values{
		"name": []string{"test"},
		"age":  []string{"30"},
	}

	var uf UnexportedForm
	err = binding.BindForm(&uf, ctx)
	assert.Nil(t, err)
	assert.Equal(t, 30, uf.Age) // Age is exported and should be bound
	// name field is unexported and won't be bound

	// Test with anonymous non-struct field (should be skipped)
	type AnonymousNonStruct struct {
		FormBindParamCommon
		Extra string `form:"extra"`
	}

	// This should work since FormBindParamCommon is a struct
	ctx.formParams = url.Values{
		"a": []string{"1"},
		"extra": []string{"test"},
	}

	var ans AnonymousNonStruct
	err = binding.BindForm(&ans, ctx)
	assert.Nil(t, err)
	assert.Equal(t, "1", ans.A)
}

func TestBindMultipartFormErrors(t *testing.T) {
	// Test BindMultipartForm with non-pointer
	type MultipartStruct struct {
		File *multipart.FileHeader `form:"file"`
	}

	ctx := &MockRequest{
		method:      "POST",
		contentType: "multipart/form-data",
		formParams:  url.Values{},
	}

	var ms MultipartStruct
	err := binding.BindMultipartForm(ms, ctx)
	assert.Error(t, err, "BindMultipartForm should return error from MultipartParams even for non-pointer")
	assert.Contains(t, err.Error(), "not impl")

	// Test with non-struct pointer
	var i int
	err = binding.BindMultipartForm(&i, ctx)
	assert.Error(t, err, "BindMultipartForm should return error from MultipartParams for non-struct pointer")
	assert.Contains(t, err.Error(), "not impl")

	// Test with struct pointer (same error from MockRequest)
	err = binding.BindMultipartForm(&ms, ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not impl")
}

func TestBindFormStructEdgeCases(t *testing.T) {
	// Test with field without form tag (should be skipped)
	type NoTagStruct struct {
		Name string
		Age  int `form:"age"`
	}

	ctx := &MockRequest{
		method:      "POST",
		contentType: "application/x-www-form-urlencoded",
		formParams: url.Values{
			"Name": []string{"John"}, // No tag, won't bind
			"age":  []string{"30"},
		},
	}

	var nts NoTagStruct
	err := binding.BindForm(&nts, ctx)
	assert.Nil(t, err)
	assert.Equal(t, "", nts.Name) // Should remain empty
	assert.Equal(t, 30, nts.Age)

	// Test with empty values (should be skipped)
	type EmptyValuesStruct struct {
		Name string `form:"name"`
	}

	ctx.formParams = url.Values{
		"name": []string{}, // Empty slice
	}

	var evs EmptyValuesStruct
	err = binding.BindForm(&evs, ctx)
	assert.Nil(t, err)
	assert.Equal(t, "", evs.Name)
}

func TestBindMultipartFormFiles(t *testing.T) {
	// Test with single file (non-slice)
	// This requires creating a real multipart request
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)

	w, err := mw.CreateFormFile("singlefile", "test.txt")
	assert.NoError(t, err)
	_, err = w.Write([]byte("test content"))
	assert.NoError(t, err)

	mw.Close()

	request, err := http.NewRequest("POST", "/", buf)
	assert.NoError(t, err)
	request.Header.Set("Content-Type", mw.FormDataContentType())

	var params struct {
		SingleFile *multipart.FileHeader `form:"singlefile"`
	}

	err = binding.BindMultipartForm(&params, testRequest{request})
	assert.NoError(t, err)
	assert.NotNil(t, params.SingleFile)
	assert.Equal(t, "test.txt", params.SingleFile.Filename)
}

func TestBindMultipartFormStructEdgeCases(t *testing.T) {
	// Test with anonymous non-struct field
	type AnonymousNonStruct struct {
		FormBindParamCommon
		Extra string `form:"extra"`
	}

	// Create multipart request with form fields
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)

	// Add form field
	a, err := mw.CreateFormField("a")
	assert.NoError(t, err)
	_, err = a.Write([]byte("111"))
	assert.NoError(t, err)

	// Add extra field
	extra, err := mw.CreateFormField("extra")
	assert.NoError(t, err)
	_, err = extra.Write([]byte("test"))
	assert.NoError(t, err)

	mw.Close()

	request, err := http.NewRequest("POST", "/", buf)
	assert.NoError(t, err)
	request.Header.Set("Content-Type", mw.FormDataContentType())

	var params AnonymousNonStruct
	err = binding.BindMultipartForm(&params, testRequest{request})
	assert.NoError(t, err)
	assert.Equal(t, "111", params.A)
	assert.Nil(t, params.B) // B should be nil since no value provided
	assert.Equal(t, "test", params.Extra)

	// Test with field without form tag
	type NoTagStruct struct {
		Name string
		Age  int `form:"age"`
	}

	buf2 := new(bytes.Buffer)
	mw2 := multipart.NewWriter(buf2)

	ageField, err := mw2.CreateFormField("age")
	assert.NoError(t, err)
	_, err = ageField.Write([]byte("30"))
	assert.NoError(t, err)

	mw2.Close()

	request2, err := http.NewRequest("POST", "/", buf2)
	assert.NoError(t, err)
	request2.Header.Set("Content-Type", mw2.FormDataContentType())

	var params2 NoTagStruct
	err = binding.BindMultipartForm(&params2, testRequest{request2})
	assert.NoError(t, err)
	assert.Equal(t, 30, params2.Age)
	assert.Equal(t, "", params2.Name) // Name should remain empty (no form tag)

	// Test with unexported field (should be skipped)
	type UnexportedStruct struct {
		name string `form:"name"` // unexported
		Age  int    `form:"age"`
	}

	buf3 := new(bytes.Buffer)
	mw3 := multipart.NewWriter(buf3)

	ageField3, err := mw3.CreateFormField("age")
	assert.NoError(t, err)
	_, err = ageField3.Write([]byte("25"))
	assert.NoError(t, err)

	nameField, err := mw3.CreateFormField("name")
	assert.NoError(t, err)
	_, err = nameField.Write([]byte("john"))
	assert.NoError(t, err)

	mw3.Close()

	request3, err := http.NewRequest("POST", "/", buf3)
	assert.NoError(t, err)
	request3.Header.Set("Content-Type", mw3.FormDataContentType())

	var params3 UnexportedStruct
	err = binding.BindMultipartForm(&params3, testRequest{request3})
	assert.NoError(t, err)
	assert.Equal(t, 25, params3.Age) // Age should be bound
	// name is unexported, can't check it directly

	// Test file field without files (should be skipped)
	type FileStruct struct {
		File *multipart.FileHeader `form:"file"`
		Text string                `form:"text"`
	}

	buf4 := new(bytes.Buffer)
	mw4 := multipart.NewWriter(buf4)

	textField, err := mw4.CreateFormField("text")
	assert.NoError(t, err)
	_, err = textField.Write([]byte("hello"))
	assert.NoError(t, err)

	// No file field added
	mw4.Close()

	request4, err := http.NewRequest("POST", "/", buf4)
	assert.NoError(t, err)
	request4.Header.Set("Content-Type", mw4.FormDataContentType())

	var params4 FileStruct
	err = binding.BindMultipartForm(&params4, testRequest{request4})
	assert.NoError(t, err)
	assert.Equal(t, "hello", params4.Text)
	assert.Nil(t, params4.File) // File should be nil since no file was uploaded

	// Test non-file field without values (should be skipped)
	type NoValuesStruct struct {
		Text string `form:"text"`
	}

	buf5 := new(bytes.Buffer)
	mw5 := multipart.NewWriter(buf5)

	// Don't add any form fields
	mw5.Close()

	request5, err := http.NewRequest("POST", "/", buf5)
	assert.NoError(t, err)
	request5.Header.Set("Content-Type", mw5.FormDataContentType())

	var params5 NoValuesStruct
	err = binding.BindMultipartForm(&params5, testRequest{request5})
	assert.NoError(t, err)
	assert.Equal(t, "", params5.Text) // Text should remain empty

	// Test with anonymous non-struct field (should be skipped)
	type MyInt int
	type AnonymousNonStructField struct {
		MyInt           // anonymous but not a struct
		Value  string   `form:"value"`
	}

	buf6 := new(bytes.Buffer)
	mw6 := multipart.NewWriter(buf6)

	valueField, err := mw6.CreateFormField("value")
	assert.NoError(t, err)
	_, err = valueField.Write([]byte("test"))
	assert.NoError(t, err)

	mw6.Close()

	request6, err := http.NewRequest("POST", "/", buf6)
	assert.NoError(t, err)
	request6.Header.Set("Content-Type", mw6.FormDataContentType())

	var params6 AnonymousNonStructField
	err = binding.BindMultipartForm(&params6, testRequest{request6})
	assert.NoError(t, err)
	assert.Equal(t, "test", params6.Value)
	// MyInt should remain zero value since it's not a struct
}
