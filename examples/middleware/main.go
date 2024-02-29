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

	router.Use(web.Recovery())

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

		r.Get("/panic", func(ctx context.Context) {
			panic("panic test")
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
