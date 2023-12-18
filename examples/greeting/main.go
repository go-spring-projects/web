package main

import (
	"context"
	"net/http"

	"go-spring.dev/web"
)

func main() {
	var router = web.NewRouter()

	router.Get("/greeting", func(ctx context.Context) string {
		return "greeting!!!"
	})

	http.ListenAndServe(":8080", router)
}
