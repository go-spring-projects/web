//go:build !go1.22

package web

import "net/http"

// setPathValue sets the path values in the Request value
// based on the provided request context.
//
// setPathValue is only supported in Go 1.22 and above so
// this is just a blank function so that it compiles.
func setPathValue(rctx *RouteContext, r *http.Request) {
}
