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

// Package binding ...
package binding

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"
)

var ErrBinding = errors.New("binding failed")
var ErrValidate = errors.New("validate failed")
var ErrNoContentType = errors.New("missing content type")

const (
	MIMEApplicationJSON = "application/json"
	MIMEApplicationXML  = "application/xml"
	MIMETextXML         = "text/xml"
	MIMEApplicationForm = "application/x-www-form-urlencoded"
	MIMEMultipartForm   = "multipart/form-data"
	MIMEMissing         = ""
)

type Request interface {
	Method() string
	ContentType() string
	Header(key string) (string, bool)
	Cookie(name string) (string, bool)
	PathParam(name string) (string, bool)
	QueryParam(name string) (string, bool)
	FormParams() (url.Values, error)
	MultipartParams(maxMemory int64) (*multipart.Form, error)
	RequestBody() io.Reader
}

type FieldConverter func(v reflect.Value, val string) error

type BindScope int

const (
	BindScopeURI BindScope = iota
	BindScopeQuery
	BindScopeHeader
	BindScopeCookie
	BindScopeBody
)

var scopeTags = map[BindScope]string{
	BindScopeURI:    "path",
	BindScopeQuery:  "query",
	BindScopeHeader: "header",
	BindScopeCookie: "cookie",
}

var scopeGetters = map[BindScope]func(r Request, name string) (string, bool){
	BindScopeURI:    Request.PathParam,
	BindScopeQuery:  Request.QueryParam,
	BindScopeHeader: Request.Header,
	BindScopeCookie: Request.Cookie,
}

var fieldConverters = map[reflect.Type]FieldConverter{}

// ValidateStruct validates a single struct.
var validateStruct func(i interface{}) error

type BodyBinder func(i interface{}, r Request) error

type bodyBinderOptions struct {
	binder  BodyBinder
	methods []string
}

var defaultBodyMethods = []string{http.MethodPost, http.MethodPut, http.MethodPatch}

var bodyBinders = map[string]bodyBinderOptions{
	MIMEApplicationForm: {BindForm, defaultBodyMethods},
	MIMEMultipartForm:   {BindMultipartForm, defaultBodyMethods},
	MIMEApplicationJSON: {BindJSON, defaultBodyMethods},
	MIMEApplicationXML:  {BindXML, defaultBodyMethods},
	MIMETextXML:         {BindXML, defaultBodyMethods},
	MIMEMissing:         {noContentType, defaultBodyMethods},
}

func noContentType(i interface{}, r Request) error {
	return ErrNoContentType
}

// RegisterBodyBinder register body binder.
func RegisterBodyBinder(mime string, binder BodyBinder, enableMethods ...string) {
	bodyBinders[mime] = bodyBinderOptions{binder, enableMethods}
}

// RegisterValidator register custom validator.
func RegisterValidator(validator func(i interface{}) error) {
	validateStruct = validator
}

// RegisterConverter register custom field type converter.
func RegisterConverter(typ reflect.Type, converter FieldConverter) {
	fieldConverters[typ] = converter
}

// Bind checks the Method and Content-Type to select a binding engine automatically,
// Depending on the "Content-Type" header different bindings are used, for example:
//
//	"application/json" --> JSON binding
//	"application/xml"  --> XML binding
func Bind(i interface{}, r Request) error {
	if err := bindScope(i, r); err != nil {
		return fmt.Errorf("%w: %v", ErrBinding, err)
	}

	if err := bindBody(i, r); err != nil {
		return fmt.Errorf("%w: %v", ErrBinding, err)
	}

	if nil != validateStruct {
		if err := validateStruct(i); nil != err {
			return fmt.Errorf("%w: %v", ErrValidate, err)
		}
	}
	return nil
}

func contains(methods []string, method string) bool {
	for _, m := range methods {
		if m == method {
			return true
		}
	}
	return false
}

func bindBody(i interface{}, r Request) (err error) {

	var mediaType = MIMEMissing

	// parse ContentType from http request.
	if contentType := r.ContentType(); contentType != "" {
		if mediaType, _, err = mime.ParseMediaType(contentType); nil != err {
			return err
		}
	}

	// check and bind body.
	if opts, ok := bodyBinders[mediaType]; ok && (len(opts.methods) == 0 || contains(opts.methods, r.Method())) {
		return opts.binder(i, r)
	}

	// ignore unknown mine-type or mismatch methods.
	return nil
}

func bindScope(i interface{}, r Request) error {
	t := reflect.TypeOf(i)
	if t.Kind() != reflect.Ptr {
		return fmt.Errorf("%s: is not pointer", t.String())
	}

	et := t.Elem()
	if et.Kind() != reflect.Struct {
		return fmt.Errorf("%s: is not a struct pointer", t.String())
	}

	ev := reflect.ValueOf(i).Elem()
	for j := 0; j < ev.NumField(); j++ {
		fv := ev.Field(j)
		ft := et.Field(j)
		if ft.Anonymous {
			if err := bindScope(fv.Addr().Interface(), r); nil != err {
				return err
			}
			continue
		}
		for scope := BindScopeURI; scope < BindScopeBody; scope++ {
			if err := bindScopeField(scope, fv, ft, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func bindScopeField(scope BindScope, v reflect.Value, field reflect.StructField, r Request) error {
	if tag, loaded := scopeTags[scope]; loaded {
		if name, ok := field.Tag.Lookup(tag); ok && name != "-" {
			if val, exists := scopeGetters[scope](r, name); exists {
				if err := bindData(v, val); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func bindData(v reflect.Value, val string) error {

	if fn, ok := fieldConverters[v.Type()]; ok {
		return fn(v, val)
	}

	switch v.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(val, 0, 0)
		if err != nil {
			return err
		}
		v.SetUint(u)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(val, 0, 0)
		if err != nil {
			return err
		}
		v.SetInt(i)
		return nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		v.SetFloat(f)
		return nil
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		v.SetBool(b)
		return nil
	case reflect.String:
		v.SetString(val)
		return nil
	default:
		return fmt.Errorf("unsupported binding type %q", v.Type().String())
	}
}

func parseDuration(v reflect.Value, val string) error {
	du, err := time.ParseDuration(val)
	if nil != err {
		return err
	}

	v.Set(reflect.ValueOf(du))
	return nil
}

func parseTime(v reflect.Value, val string) error {
	var layouts = []string{
		time.Layout,
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
		time.RFC3339Nano,
		time.Kitchen,
		time.Stamp,
		time.StampMilli,
		time.StampMicro,
		time.StampNano,
		time.DateTime,
		time.DateOnly,
		time.TimeOnly,
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, val, time.UTC); nil == err {
			v.Set(reflect.ValueOf(t))
			return nil
		}
	}

	return fmt.Errorf("parse time.Time failed: %s", val)
}

func init() {
	RegisterConverter(reflect.TypeOf(time.Duration(0)), parseDuration)
	RegisterConverter(reflect.TypeOf(time.Time{}), parseTime)
}
