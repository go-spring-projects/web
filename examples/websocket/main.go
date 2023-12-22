package main

import (
	"log/slog"
	"net/http"

	"github.com/go-netty/go-netty-ws"
	"go-spring.dev/web"
)

func main() {

	var ws = nettyws.NewWebsocket()

	ws.OnOpen = func(conn nettyws.Conn) {
		slog.Info("websocket: connection open", slog.String("remoteAddr", conn.RemoteAddr()))
	}

	ws.OnData = func(conn nettyws.Conn, data []byte) {
		slog.Info("websocket: received message", slog.String("remoteAddr", conn.RemoteAddr()), slog.String("message", string(data)))
	}

	ws.OnClose = func(conn nettyws.Conn, err error) {
		slog.Info("websocket: connection closed", slog.String("remoteAddr", conn.RemoteAddr()), slog.Any("err", err))
	}

	var router = web.NewRouter()
	router.Get("/ws", ws)

	// listen and serve
	http.ListenAndServe(":8080", router)
}
