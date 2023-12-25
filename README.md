# web
[![GoDoc][1]][2] [![Build Status][7]][8] [![Codecov][9]][10] [![Release][5]][6] [![license-Apache 2][3]][4]

[1]: https://godoc.org/go-spring.dev/web?status.svg
[2]: https://godoc.org/go-spring.dev/web
[3]: https://img.shields.io/badge/license-Apache%202-blue.svg
[4]: LICENSE
[5]: https://img.shields.io/github/v/release/go-spring-projects/web?color=orange
[6]: https://github.com/go-spring-projects/web/releases/latest
[7]: https://github.com/go-spring-projects/web/workflows/Go%20Test/badge.svg?branch=master
[8]: https://github.com/go-spring-projects/web/actions?query=branch%3Amaster
[9]: https://codecov.io/gh/go-spring-projects/web/graph/badge.svg?token=29W9JV6BUN
[10]: https://codecov.io/gh/go-spring-projects/web

The `web` package aims to provide a simpler and more user-friendly development experience.

*Note: This package does not depend on the go-spring*

## Install

`go get go-spring.dev/web@latest`

## Features:

* Automatically bind models based on `ContentType`.
* Automatically output based on function return type.
* Support binding value from `path/query/header/cookie/form/body`.
* Support binding files for easier file uploads handling.
* Support customizing global output formats and route-level custom output.
* Support custom parameter validators.
* Support handler converter, adding the above capabilities with just one line of code for all http servers based on the standard library solution.
* Support for middlewares based on chain of responsibility.


## Router

web router is based on a kind of [Patricia Radix trie](https://en.wikipedia.org/wiki/Radix_tree). The router is compatible with net/http.

<details>

<summary>Router interface:</summary>

```go
// Router registers routes to be matched and dispatches a handler.
//
type Router interface {
	Routes
	http.Handler

	// Use appends a MiddlewareFunc to the chain.
	Use(mwf ...MiddlewareFunc) Router

	// Renderer to be used Response renderer in default.
	Renderer(renderer Renderer) Router

	// Group creates a new router group.
	Group(pattern string, fn ...func(r Router)) Router

	// Handle registers a new route with a matcher for the URL pattern.
	Handle(pattern string, handler http.Handler)

	// HandleFunc registers a new route with a matcher for the URL pattern.
	HandleFunc(pattern string, handler http.HandlerFunc)

	// Any registers a route that matches all the HTTP methods.
	// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE.
	Any(pattern string, handler interface{})

	// Get registers a new GET route with a matcher for the URL path of the get method.
	Get(pattern string, handler interface{})

	// Head registers a new HEAD route with a matcher for the URL path of the head method.
	Head(pattern string, handler interface{})

	// Post registers a new POST route with a matcher for the URL path of the post method.
	Post(pattern string, handler interface{})

	// Put registers a new PUT route with a matcher for the URL path of the put method.
	Put(pattern string, handler interface{})

	// Patch registers a new PATCH route with a matcher for the URL path of the patch method.
	Patch(pattern string, handler interface{})

	// Delete registers a new DELETE route with a matcher for the URL path of the delete method.
	Delete(pattern string, handler interface{})

	// Connect registers a new CONNECT route with a matcher for the URL path of the connect method.
	Connect(pattern string, handler interface{})

	// Options registers a new OPTIONS route with a matcher for the URL path of the options method.
	Options(pattern string, handler interface{})

	// Trace registers a new TRACE route with a matcher for the URL path of the trace method.
	Trace(pattern string, handler interface{})

	// NotFound to be used when no route matches.
	NotFound(handler http.HandlerFunc)

	// MethodNotAllowed to be used when the request method does not match the route.
	MethodNotAllowed(handler http.HandlerFunc)
}
```
</details>

## Getting Started

### HelloWorld

```go
package main

import (
	"context"
	"net/http"

	"go-spring.dev/web"
)

func main() {
	var router = web.NewRouter()

	router.Get("/greeting", func(ctx context.Context, req struct {
		Name string `query:"name"`
	}) string {
		return "Hello, " + req.Name
	})

	http.ListenAndServe(":8080", router)
}
```

```
$ curl -i -X GET 'http://127.0.0.1:8080/greeting?name=world'
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Date: Mon, 25 Dec 2023 06:13:03 GMT
Content-Length: 33

{"code":0,"data":"Hello, world"}
```

### Custom Render

Allows you to customize the renderer, using the default `JsonRender` if not specified.

```go
package main

import (
	"context"
	"net/http"

	"go-spring.dev/web"
)

func main() {
	var router = web.NewRouter()

	router.Renderer(web.RendererFunc(func(ctx *web.Context, err error, result interface{}) {
		if nil != err {
			ctx.String(500, "%v", err)
		} else {
			ctx.String(200, "%v", result)
		}
	}))

	router.Get("/greeting", func(ctx context.Context, req struct {
		Name string `query:"name"`
	}) string {
		return "Hello, " + req.Name
	})

	http.ListenAndServe(":8080", router)

	/*
        $ curl -i -X GET 'http://127.0.0.1:8080/greeting?name=world'
        HTTP/1.1 200 OK
        Content-Type: text/plain; charset=utf-8
        Date: Mon, 25 Dec 2023 06:35:32 GMT
        Content-Length: 12
    
        Hello, world
	*/
}

```

### Custom validator

Allows you to register a custom value validator. If the value verification fails, request processing aborts.

In this example, we will use [go-validator/validator](https://github.com/go-validator/validator), you can refer to this example to register your custom validator.

```go
package main

import (
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"

	"go-spring.dev/web"
	"go-spring.dev/web/binding"
	"gopkg.in/validator.v2"
)

var router = web.NewRouter()
var validatorInst = validator.NewValidator().WithTag("validate")

func init() {
	binding.RegisterValidator(func(i interface{}) error {
		return validatorInst.Validate(i)
	})
}

type UserRegisterModel struct {
	Username  string                `form:"username" validate:"min=6,max=20"`  // username
	Password  string                `form:"password" validate:"min=10,max=20"` // password
	Avatar    *multipart.FileHeader `form:"avatar" validate:"nonzero"`         // avatar
	Captcha   string                `form:"captcha" validate:"min=4,max=4"`    // captcha
	UserAgent string                `header:"User-Agent"`                      // user agent
	Ad        string                `query:"ad"`                               // advertising ID
	Token     string                `cookie:"token"`                           // token
}

func main() {
	router.Post("/user/register", UserRegister)

	http.ListenAndServe(":8080", router)
}

func UserRegister(ctx context.Context, req UserRegisterModel) string {
	slog.Info("user register",
		slog.String("username", req.Username),
		slog.String("password", req.Password),
		slog.String("captcha", req.Captcha),
		slog.String("userAgent", req.UserAgent),
		slog.String("ad", req.Ad),
		slog.String("token", req.Token),
	)
	return "success"
}

```

### Middlewares

Compatible with middlewares based on standard library solutions.

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"go-spring.dev/web"
)

func main() {
	var router = web.NewRouter()

	// access log
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			t1 := time.Now()
			next.ServeHTTP(writer, request)
			slog.Info("access log", slog.String("path", request.URL.Path), slog.String("method", request.Method), slog.Duration("cost", time.Since(t1)))
		})
	})

	// cors
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Access-Control-Allow-Origin", "*")
			writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type")

			// preflight request
			if request.Method == http.MethodOptions {
				writer.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(writer, request)
		})
	})

	router.Group("/public", func(r web.Router) {
		r.Post("/register", func(ctx context.Context) string { return "register: do something" })
		r.Post("/forgot", func(ctx context.Context) string { return "forgot: do something" })
		r.Post("/login", func(ctx context.Context, req struct {
			Username string `form:"username"`
			Password string `form:"password"`
		}) error {
			if "admin" == req.Username && "admin123" == req.Password {
				web.FromContext(ctx).SetCookie("token", req.Username, 600, "/", "", false, false)
				return nil
			}
			return web.Error(400, "login failed")
		})
	})

	router.Group("/user", func(r web.Router) {

		// user login check
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				// check login state in cookies
				//
				if _, err := request.Cookie("token"); nil != err {
					writer.WriteHeader(http.StatusForbidden)
					return
				}

				// login check success
				next.ServeHTTP(writer, request)
			})
		})

		r.Get("/userInfo", func(ctx context.Context) interface{} {
			// TODO: load user from database
			//
			return map[string]interface{}{
				"username": "admin",
				"time":     time.Now().String(),
			}
		})

		r.Get("/logout", func(ctx context.Context) string {
			// delete cookie
			web.FromContext(ctx).SetCookie("token", "", -1, "/", "", false, false)
			return "success"
		})

	})

	http.ListenAndServe(":8080", router)
}

```

## Acknowledgments

* https://github.com/go-chi/chi
* https://github.com/gin-gonic/gin
* https://github.com/lvan100

### License

The repository released under version 2.0 of the Apache License.