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

	/*
		$ curl -i -X GET 'http://127.0.0.1:8080/greeting?name=world'
		HTTP/1.1 200 OK
		Content-Type: application/json; charset=utf-8
		Date: Mon, 25 Dec 2023 06:13:03 GMT
		Content-Length: 33

		{"code":0,"data":"Hello, world"}
	*/
}
