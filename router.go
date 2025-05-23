/*
 * Copyright 2023 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Router registers routes to be matched and dispatches a handler.
//
// Registers a new route with a matcher for the URL pattern.
// Automatic binding request to handler input params and validate params.
//
// Router.Any() Router.Get() Router.Head() Router.Post() Router.Put() Router.Patch() Router.Delete() Router.Connect() Router.Options() Router.Trace()
//
// The handler accepts the following functional signatures:
//
// func(ctx context.Context)
//
// func(ctx context.Context) R
//
// func(ctx context.Context) error
//
// func(ctx context.Context, req T) R
//
// func(ctx context.Context, req T) error
//
// func(ctx context.Context, req T) (R, error)
//
// It implements the http.Handler interface, so it can be registered to serve
// requests:
//
//		func main() {
//	   var router = web.NewRouter()
//		  router.Get("/greeting", func(ctx context.Context) string {
//		    return "greeting!!!"
//		  })
//		  http.ListenAndServe(":8080", router)
//		}
type Router interface {
	Routes
	http.Handler

	// Use appends a MiddlewareFunc to the chain.
	Use(mwf ...MiddlewareFunc) Router

	// Renderer to be used Response renderer in default.
	Renderer(renderer Renderer) Router

	// Group creates a new router group.
	Group(pattern string, fn ...func(r Router)) Router

	// Mount attaches another http.Handler along ./pattern/*
	Mount(pattern string, h http.Handler)

	// Handle registers a new route with a matcher for the URL pattern.
	Handle(pattern string, handler http.Handler)

	// HandleFunc registers a new route with a matcher for the URL pattern.
	HandleFunc(pattern string, handler http.HandlerFunc)

	// Any registers a route that matches all the HTTP methods.
	// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE.
	Any(pattern string, handler interface{})

	// Get registers a new GET route with a matcher for the URL path of the get method.
	Get(pattern string, handler interface{})

	// Head registers a new HEAD route with a matcher for the URL path of the head method.
	Head(pattern string, handler interface{})

	// Post registers a new POST route with a matcher for the URL path of the post method.
	Post(pattern string, handler interface{})

	// Put registers a new PUT route with a matcher for the URL path of the put method.
	Put(pattern string, handler interface{})

	// Patch registers a new PATCH route with a matcher for the URL path of the patch method.
	Patch(pattern string, handler interface{})

	// Delete registers a new DELETE route with a matcher for the URL path of the delete method.
	Delete(pattern string, handler interface{})

	// Connect registers a new CONNECT route with a matcher for the URL path of the connect method.
	Connect(pattern string, handler interface{})

	// Options registers a new OPTIONS route with a matcher for the URL path of the options method.
	Options(pattern string, handler interface{})

	// Trace registers a new TRACE route with a matcher for the URL path of the trace method.
	Trace(pattern string, handler interface{})

	// NotFound to be used when no route matches.
	NotFound(handler http.HandlerFunc)

	// MethodNotAllowed to be used when the request method does not match the route.
	MethodNotAllowed(handler http.HandlerFunc)
}

type Routes interface {
	// Routes returns the routing tree in an easily traversable structure.
	Routes() []Route

	// Middlewares returns the list of middlewares in use by the router.
	Middlewares() Middlewares

	// Match searches the routing tree for a handler that matches
	// the method/path - similar to routing a http request, but without
	// executing the handler thereafter.
	Match(ctx *RouteContext, method, path string) bool

	// Find searches the routing tree for the pattern that matches
	// the method/path.
	Find(ctx *RouteContext, method, path string) string
}

// NewRouter returns a new router instance.
func NewRouter() Router {
	return &routerGroup{
		tree:     &node{},
		renderer: JsonRender(),
		pool:     &sync.Pool{New: func() interface{} { return &RouteContext{} }},
	}
}

type routerGroup struct {
	handler           http.Handler
	inline            bool
	tree              *node
	parent            *routerGroup
	middlewares       Middlewares
	renderer          Renderer
	notFoundHandler   http.HandlerFunc
	notAllowedHandler http.HandlerFunc
	pool              *sync.Pool
}

// Use appends a MiddlewareFunc to the chain.
// Middleware can be used to intercept or otherwise modify requests and/or responses, and are executed in the order that they are applied to the Router.
func (rg *routerGroup) Use(mwf ...MiddlewareFunc) Router {
	if rg.handler != nil {
		panic("middlewares must be defined before routes registers")
	}
	rg.middlewares = append(rg.middlewares, mwf...)
	return rg
}

// Renderer to be used Response renderer in default.
func (rg *routerGroup) Renderer(renderer Renderer) Router {
	if rg.handler != nil {
		panic("renderer must be defined before routes registers")
	}
	rg.renderer = renderer
	return rg
}

func (rg *routerGroup) NotFoundHandler() http.Handler {
	if rg.notFoundHandler != nil {
		return rg.notFoundHandler
	}
	return notFound()
}

func (rg *routerGroup) NotAllowedHandler() http.Handler {
	if rg.notAllowedHandler != nil {
		return rg.notAllowedHandler
	}
	return notAllowed()
}

// ServeHTTP dispatches the handler registered in the matched route.
func (rg *routerGroup) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if nil == rg.handler {
		rg.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	ctx := FromRouteContext(r.Context())
	if nil != ctx {
		rg.handler.ServeHTTP(w, r)
		return
	}

	// get context from pool
	ctx = rg.pool.Get().(*RouteContext)
	ctx.Routes = rg

	// with context
	r = r.WithContext(WithRouteContext(r.Context(), ctx))
	rg.handler.ServeHTTP(w, r)

	// put context to pool
	ctx.Reset()
	rg.pool.Put(ctx)

}

// Recursively update data on child routers.
func (rg *routerGroup) updateSubRoutes(fn func(subMux *routerGroup)) {
	for _, r := range rg.tree.routes() {
		subMux, ok := r.SubRoutes.(*routerGroup)
		if !ok {
			continue
		}
		fn(subMux)
	}
}

func (rg *routerGroup) nextRoutePath(ctx *RouteContext) string {
	routePath := "/"
	nx := len(ctx.routeParams.Keys) - 1 // index of last param in list
	if nx >= 0 && ctx.routeParams.Keys[nx] == "*" && len(ctx.routeParams.Values) > nx {
		routePath = "/" + ctx.routeParams.Values[nx]
	}
	return routePath
}

// routeHTTP Routes a http.Request through the routing tree to serve
// the matching handler for a particular http method.
func (rg *routerGroup) routeHTTP(w http.ResponseWriter, r *http.Request) {
	// Grab the route context object
	ctx := FromRouteContext(r.Context())

	// The request routing path
	routePath := ctx.RoutePath
	if routePath == "" {
		if r.URL.RawPath != "" {
			routePath = r.URL.RawPath
		} else {
			routePath = r.URL.Path
		}
		if routePath == "" {
			routePath = "/"
		}
	}

	if ctx.RouteMethod == "" {
		ctx.RouteMethod = r.Method
	}

	method, ok := methodMap[ctx.RouteMethod]
	if !ok {
		rg.NotAllowedHandler().ServeHTTP(w, r)
		return
	}

	// Find the route
	if _, _, h := rg.tree.FindRoute(ctx, method, routePath); h != nil {
		// sets the path values in the Request value based on the provided request context.
		setPathValue(ctx, r)

		// serve http request.
		h.ServeHTTP(w, r)
		return
	}
	if ctx.methodNotAllowed {
		rg.NotAllowedHandler().ServeHTTP(w, r)
	} else {
		rg.NotFoundHandler().ServeHTTP(w, r)
	}
}

// Group creates a new router group.
func (rg *routerGroup) Group(pattern string, fn ...func(r Router)) Router {
	subRouter := &routerGroup{tree: &node{}, renderer: rg.renderer, pool: rg.pool}
	for _, f := range fn {
		f(subRouter)
	}
	rg.Mount(pattern, subRouter)
	return subRouter
}

// Mount attaches another http.Handler or RouterGroup as a subrouter along a routing
// path. It's very useful to split up a large API as many independent routers and
// compose them as a single service using Mount.
func (rg *routerGroup) Mount(pattern string, handler http.Handler) {
	if handler == nil {
		panic(fmt.Sprintf("attempting to Mount() a nil handler on '%s'", pattern))
	}

	// Provide runtime safety for ensuring a pattern isn't mounted on an existing
	// routing pattern.
	if rg.tree.findPattern(pattern+"*") || rg.tree.findPattern(pattern+"/*") {
		panic(fmt.Sprintf("attempting to Mount() a handler on an existing path, '%s'", pattern))
	}

	// Assign sub-Router'rg with the parent not found & method not allowed handler if not specified.
	subr, ok := handler.(*routerGroup)
	if ok && subr.notFoundHandler == nil && rg.notFoundHandler != nil {
		subr.NotFound(rg.notFoundHandler)
	}
	if ok && subr.notAllowedHandler == nil && rg.notAllowedHandler != nil {
		subr.MethodNotAllowed(rg.notAllowedHandler)
	}

	mountHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := FromRouteContext(r.Context())

		// shift the url path past the previous subrouter
		ctx.RoutePath = rg.nextRoutePath(ctx)

		// reset the wildcard URLParam which connects the subrouter
		n := len(ctx.URLParams.Keys) - 1
		if n >= 0 && ctx.URLParams.Keys[n] == "*" && len(ctx.URLParams.Values) > n {
			ctx.URLParams.Values[n] = ""
		}

		handler.ServeHTTP(w, r)
	})

	if pattern == "" || pattern[len(pattern)-1] != '/' {
		rg.handle(mALL|mSTUB, pattern, mountHandler)
		rg.handle(mALL|mSTUB, pattern+"/", mountHandler)
		pattern += "/"
	}

	method := mALL
	subroutes, _ := handler.(Routes)
	if subroutes != nil {
		method |= mSTUB
	}
	n := rg.handle(method, pattern+"*", mountHandler)

	if subroutes != nil {
		n.subroutes = subroutes
	}
}

// bind a new route with a matcher for the URL pattern.
// Automatic binding request to handler input params and validate params.
func (rg *routerGroup) bind(method methodTyp, pattern string, handler interface{}) *node {
	return rg.handle(method, pattern, Bind(handler, rg.renderer))
}

func (rg *routerGroup) handle(method methodTyp, pattern string, handler http.Handler) *node {
	if len(pattern) == 0 || pattern[0] != '/' {
		panic(fmt.Sprintf("routing pattern must begin with '/' in '%s'", pattern))
	}
	if !rg.inline && rg.handler == nil {
		rg.handler = rg.middlewares.HandlerFunc(rg.routeHTTP)
	}

	if rg.inline {
		rg.handler = http.HandlerFunc(rg.routeHTTP)
		handler = rg.middlewares.Handler(handler)
	}

	// Add the endpoint to the tree
	return rg.tree.InsertRoute(method, pattern, handler)
}

func (rg *routerGroup) method(method, pattern string, handler http.Handler) {
	if m, ok := methodMap[method]; ok {
		rg.handle(m, pattern, handler)
		return
	}
	panic(fmt.Errorf("%q http method is not supported", method))
}

// Handle registers a new route with a matcher for the URL pattern.
func (rg *routerGroup) Handle(pattern string, handler http.Handler) {
	if parts := strings.SplitN(pattern, " ", 2); 2 == len(parts) {
		rg.method(parts[0], parts[1], handler)
		return
	}
	rg.handle(mALL, pattern, handler)
}

// HandleFunc registers a new route with a matcher for the URL pattern.
func (rg *routerGroup) HandleFunc(pattern string, handler http.HandlerFunc) {
	if parts := strings.SplitN(pattern, " ", 2); 2 == len(parts) {
		rg.method(parts[0], parts[1], handler)
		return
	}
	rg.handle(mALL, pattern, handler)
}

// Any registers a route that matches all the HTTP methods.
// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE.
func (rg *routerGroup) Any(pattern string, handler interface{}) {
	rg.bind(mALL, pattern, handler)
}

// Get registers a new GET route with a matcher for the URL pattern of the get method.
func (rg *routerGroup) Get(pattern string, handler interface{}) {
	rg.bind(mGET, pattern, handler)
}

// Head registers a new HEAD route with a matcher for the URL pattern of the get method.
func (rg *routerGroup) Head(pattern string, handler interface{}) {
	rg.bind(mHEAD, pattern, handler)
}

// Post registers a new POST route with a matcher for the URL pattern of the get method.
func (rg *routerGroup) Post(pattern string, handler interface{}) {
	rg.bind(mPOST, pattern, handler)
}

// Put registers a new PUT route with a matcher for the URL pattern of the get method.
func (rg *routerGroup) Put(pattern string, handler interface{}) {
	rg.bind(mPUT, pattern, handler)
}

// Patch registers a new PATCH route with a matcher for the URL pattern of the get method.
func (rg *routerGroup) Patch(pattern string, handler interface{}) {
	rg.bind(mPATCH, pattern, handler)
}

// Delete registers a new DELETE route with a matcher for the URL pattern of the get method.
func (rg *routerGroup) Delete(pattern string, handler interface{}) {
	rg.bind(mDELETE, pattern, handler)
}

// Connect registers a new CONNECT route with a matcher for the URL pattern of the get method.
func (rg *routerGroup) Connect(pattern string, handler interface{}) {
	rg.bind(mCONNECT, pattern, handler)
}

// Options registers a new OPTIONS route with a matcher for the URL pattern of the get method.
func (rg *routerGroup) Options(pattern string, handler interface{}) {
	rg.bind(mOPTIONS, pattern, handler)
}

// Trace registers a new TRACE route with a matcher for the URL pattern of the get method.
func (rg *routerGroup) Trace(pattern string, handler interface{}) {
	rg.bind(mTRACE, pattern, handler)
}

// NotFound to be used when no route matches.
// This can be used to render your own 404 Not Found errors.
func (rg *routerGroup) NotFound(handler http.HandlerFunc) {
	// Build NotFound handler chain
	m := rg
	hFn := handler
	if rg.inline && rg.parent != nil {
		m = rg.parent
		hFn = rg.middlewares.HandlerFunc(hFn).ServeHTTP
	}

	// Update the notFoundHandler from this point forward
	m.notFoundHandler = hFn
	m.updateSubRoutes(func(subMux *routerGroup) {
		if subMux.notFoundHandler == nil {
			subMux.NotFound(hFn)
		}
	})
}

// MethodNotAllowed to be used when the request method does not match the route.
// This can be used to render your own 405 Method Not Allowed errors.
func (rg *routerGroup) MethodNotAllowed(handler http.HandlerFunc) {
	// Build MethodNotAllowed handler chain
	m := rg
	hFn := handler
	if rg.inline && rg.parent != nil {
		m = rg.parent
		hFn = rg.middlewares.HandlerFunc(hFn).ServeHTTP
	}

	// Update the methodNotAllowedHandler from this point forward
	m.notAllowedHandler = hFn
	m.updateSubRoutes(func(subMux *routerGroup) {
		if subMux.notAllowedHandler == nil {
			subMux.MethodNotAllowed(hFn)
		}
	})
}

// Routes returns a slice of routing information from the tree,
// useful for traversing available Routes of a router.
func (rg *routerGroup) Routes() []Route {
	return rg.tree.routes()
}

// Middlewares returns a slice of middleware handler functions.
func (rg *routerGroup) Middlewares() Middlewares {
	return rg.middlewares
}

// Match searches the routing tree for a handler that matches the method/path.
// It's similar to routing a http request, but without executing the handler
// thereafter.
func (rg *routerGroup) Match(ctx *RouteContext, method, path string) bool {
	m, ok := methodMap[method]
	if !ok {
		return false
	}

	node, _, h := rg.tree.FindRoute(ctx, m, path)

	if node != nil && node.subroutes != nil {
		ctx.RoutePath = rg.nextRoutePath(ctx)
		return node.subroutes.Match(ctx, method, ctx.RoutePath)
	}

	return h != nil
}

// Find searches the routing tree for the pattern that matches the method/path.
func (rg *routerGroup) Find(ctx *RouteContext, method, path string) string {
	m, ok := methodMap[method]
	if !ok {
		return ""
	}

	node, _, _ := rg.tree.FindRoute(ctx, m, path)
	pattern := ctx.routePattern

	if node != nil {
		if node.subroutes == nil {
			e := node.endpoints[m]
			return e.pattern
		}

		ctx.RoutePath = rg.nextRoutePath(ctx)
		subPattern := node.subroutes.Find(ctx, method, ctx.RoutePath)
		if subPattern == "" {
			return ""
		}

		pattern = strings.TrimSuffix(pattern, "/*")
		pattern += subPattern
	}

	return pattern
}
