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
	contentType string
	headers     map[string]string
	queryParams map[string]string
	pathParams  map[string]string
	cookies     map[string]string
	formParams  url.Values
	requestBody string
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
