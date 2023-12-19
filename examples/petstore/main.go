package main

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"examples/petstore/api"
	"go-spring.dev/web"
)

var router = web.NewRouter()

func init() {
	// access log
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			t1 := time.Now()
			next.ServeHTTP(writer, request)
			slog.Info("access log", slog.String("method", request.Method), slog.String("api", request.URL.Path), slog.Duration("cost", time.Since(t1)))
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

	router.Renderer(web.RendererFunc(func(ctx *web.Context, err error, result interface{}) {
		if nil != err {
			var status = http.StatusInternalServerError
			var message = err.Error()

			var we web.HttpError
			if errors.As(err, &we) {
				status = we.Code
				message = we.Message
			}

			ctx.JSON(status, map[string]interface{}{"message": message})
			return
		}

		if nil == result {
			ctx.JSON(http.StatusOK, struct{}{})
		} else {
			ctx.JSON(http.StatusOK, result)
		}
	}))
}

func main() {
	// https://petstore.swagger.io/

	// register pet handler
	api.Register(router)

	// listen and serve
	http.ListenAndServe(":8080", router)
}
