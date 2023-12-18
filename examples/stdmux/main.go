package main

import (
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"

	"go-spring.dev/web"
)

func main() {
	http.Handle("/user/register", web.Bind(UserRegister, web.JsonRender()))

	http.ListenAndServe(":8080", nil)
}

type UserRegisterModel struct {
	Username  string                `form:"username"`     // username
	Password  string                `form:"password"`     // password
	Avatar    *multipart.FileHeader `form:"avatar"`       // avatar
	Captcha   string                `form:"captcha"`      // captcha
	UserAgent string                `header:"User-Agent"` // user agent
	Ad        string                `query:"ad"`          // advertising ID
	Token     string                `cookie:"token"`      // token
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
