/*
 * Copyright 2023 the original author or authors.
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

package render

import (
	"html/template"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTMLRenderer(t *testing.T) {

	w := httptest.NewRecorder()
	templ := template.Must(template.New("t").Parse(`Hello {{.name}}`))

	htmlRender := HTMLRenderer{Template: templ, Name: "t", Data: map[string]interface{}{"name": "asdklajhdasdd"}}
	err := htmlRender.Render(w)

	assert.Nil(t, err)
	assert.Equal(t, "text/html; charset=utf-8", htmlRender.ContentType())
	assert.Equal(t, "Hello asdklajhdasdd", w.Body.String())
}
