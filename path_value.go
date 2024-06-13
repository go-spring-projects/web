//go:build go1.22

package web

import "net/http"

// setPathValue sets the path values in the Request value
// based on the provided request context.
func setPathValue(rctx *RouteContext, r *http.Request) {
	for i, key := range rctx.URLParams.Keys {
		value := rctx.URLParams.Values[i]
		r.SetPathValue(key, value)
	}
}
