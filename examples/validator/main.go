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
