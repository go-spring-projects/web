module examples

go 1.21

replace go-spring.dev/web => ../

require (
	github.com/go-netty/go-netty-ws v1.0.5
	go-spring.dev/web v0.0.0-00010101000000-000000000000
	gopkg.in/validator.v2 v2.0.1
)

require (
	github.com/go-netty/go-netty v1.6.4 // indirect
	github.com/go-netty/go-netty-transport v1.7.7 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.3.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
)
