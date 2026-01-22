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
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go-spring.dev/web/binding"
)

type MockRequest struct {
	method      string
	contentType string
	headers     map[string]string
	queryParams map[string]string
	pathParams  map[string]string
	cookies     map[string]string
	formParams  url.Values
	requestBody string
}

func (r *MockRequest) Method() string {
	return r.method
}

var _ binding.Request = &MockRequest{}

func (r *MockRequest) ContentType() string {
	return r.contentType
}

func (r *MockRequest) Header(key string) (string, bool) {
	value, ok := r.headers[key]
	return value, ok
}

func (r *MockRequest) Cookie(name string) (string, bool) {
	value, ok := r.cookies[name]
	return value, ok
}

func (r *MockRequest) QueryParam(name string) (string, bool) {
	value, ok := r.queryParams[name]
	return value, ok
}

func (r *MockRequest) PathParam(name string) (string, bool) {
	value, ok := r.pathParams[name]
	return value, ok
}

func (r *MockRequest) FormParams() (url.Values, error) {
	return r.formParams, nil
}

func (r *MockRequest) MultipartParams(maxMemory int64) (*multipart.Form, error) {
	return nil, fmt.Errorf("not impl")
}

func (r *MockRequest) RequestBody() io.Reader {
	return strings.NewReader(r.requestBody)
}

type NestParam struct {
	A1 string `path:"a"`
	B1 int    `path:"b"`
}

type ScopeBindParam struct {
	NestParam
	A string        `path:"a"`
	B int           `path:"b"`
	C uint          `path:"c" query:"c"`
	D float32       `query:"d"`
	E string        `query:"e" header:"e"`
	F string        `cookie:"f"`
	G bool          `query:"g"`
	H time.Duration `query:"h"`
	I time.Time     `query:"i"`
}

func TestScopeBind(t *testing.T) {

	t1 := time.Date(2013, 23, 22, 20, 19, 18, 0, time.UTC)

	ctx := &MockRequest{
		method: "GET",
		headers: map[string]string{
			"e": "6",
		},
		queryParams: map[string]string{
			"c": "3",
			"d": "4",
			"e": "5",
			"g": "true",
			"h": "10m",
			"i": t1.Format(time.DateTime),
		},
		pathParams: map[string]string{
			"a": "1",
			"b": "2",
		},
		cookies: map[string]string{
			"f": "7",
		},
	}

	expect := ScopeBindParam{
		NestParam: NestParam{A1: "1", B1: 2},
		A:         "1",
		B:         2,
		C:         3,
		D:         4,
		E:         "6",
		F:         "7",
		G:         true,
		H:         10 * time.Minute,
		I:         t1,
	}

	var p ScopeBindParam
	err := binding.Bind(&p, ctx)
	assert.Nil(t, err)
	assert.Equal(t, expect, p)
}

func TestBind_NoContentType(t *testing.T) {
	// Test GET request with no content-type (should be ignored)
	ctx := &MockRequest{
		method: "GET",
		contentType: "",
	}

	var p struct{}
	err := binding.Bind(&p, ctx)
	assert.Nil(t, err)

	// Test POST request with no content-type (should return ErrNoContentType)
	ctx.method = "POST"
	err = binding.Bind(&p, ctx)
	assert.ErrorContains(t, err, "missing content type")
	assert.Contains(t, err.Error(), "binding failed")
}

func TestRegisterBodyBinder(t *testing.T) {
	called := false
	customBinder := func(i interface{}, r binding.Request) error {
		called = true
		return nil
	}

	// Register custom binder for a test MIME type
	binding.RegisterBodyBinder("test/mime", customBinder, "POST")

	// Create a request with the custom MIME type
	ctx := &MockRequest{
		method: "POST",
		contentType: "test/mime",
	}

	var p struct{}
	err := binding.Bind(&p, ctx)
	assert.Nil(t, err)
	assert.True(t, called, "custom binder should be called")

	// Test with wrong method (GET) - should not call custom binder
	called = false
	ctx.method = "GET"
	err = binding.Bind(&p, ctx)
	assert.Nil(t, err)
	assert.False(t, called, "custom binder should not be called for GET")

	// Test with unknown MIME type (should be ignored)
	ctx.method = "POST"
	ctx.contentType = "unknown/type"
	err = binding.Bind(&p, ctx)
	assert.Nil(t, err)
}

func TestRegisterValidator(t *testing.T) {
	validationCalled := false
	customValidator := func(i interface{}) error {
		validationCalled = true
		return fmt.Errorf("validation failed")
	}

	binding.RegisterValidator(customValidator)

	// Create a simple request
	ctx := &MockRequest{
		method: "GET",
	}

	type TestStruct struct {
		Name string `query:"name"`
	}

	var s TestStruct
	err := binding.Bind(&s, ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validate failed")
	assert.True(t, validationCalled, "custom validator should be called")

	// Reset validator to nil for other tests
	binding.RegisterValidator(nil)
}

func TestBind_Errors(t *testing.T) {
	// Test binding non-pointer
	ctx := &MockRequest{method: "GET"}
	var s struct{}
	err := binding.Bind(s, ctx) // Not a pointer
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not pointer")

	// Test binding non-struct pointer
	var i int
	err = binding.Bind(&i, ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a struct pointer")

	// Test binding with parse error
	type BadStruct struct {
		Num int `query:"num"`
	}
	ctx.queryParams = map[string]string{"num": "not-a-number"}
	var bs BadStruct
	err = binding.Bind(&bs, ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binding failed")
}

func TestBindData_UnsupportedType(t *testing.T) {
	// Test bindData with unsupported type (e.g., slice)
	// This is internal function, but we can test through binding
	type ComplexStruct struct {
		Data []byte `query:"data"`
	}
	ctx := &MockRequest{
		method: "GET",
		queryParams: map[string]string{"data": "value"},
	}
	var cs ComplexStruct
	err := binding.Bind(&cs, ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binding failed")
}

func TestParseDurationError(t *testing.T) {
	type DurationStruct struct {
		D time.Duration `query:"d"`
	}
	ctx := &MockRequest{
		method: "GET",
		queryParams: map[string]string{"d": "invalid"},
	}
	var ds DurationStruct
	err := binding.Bind(&ds, ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binding failed")
}

func TestParseTimeError(t *testing.T) {
	type TimeStruct struct {
		T time.Time `query:"t"`
	}
	ctx := &MockRequest{
		method: "GET",
		queryParams: map[string]string{"t": "invalid-date"},
	}
	var ts TimeStruct
	err := binding.Bind(&ts, ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binding failed")
}

