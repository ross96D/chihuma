package chihuma

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/queryparam"
	chiV4 "github.com/go-chi/chi"
	"github.com/go-chi/chi/v5"
)

type chiContext struct {
	op *huma.Operation
	r  *http.Request
	w  http.ResponseWriter
	v4 bool
}

func (c chiContext) Operation() *huma.Operation {
	return c.op
}

func (c chiContext) Context() context.Context {
	return c.r.Context()
}

func (c chiContext) Method() string {
	return c.r.Method
}

func (c chiContext) Host() string {
	return c.r.Host
}

func (c chiContext) URL() url.URL {
	return *c.r.URL
}

func (c chiContext) Param(name string) string {
	if !c.v4 {
		return chi.URLParam(c.r, name)
	}

	return chiV4.URLParam(c.r, name)
}

func (c chiContext) Query(name string) string {
	return queryparam.Get(c.r.URL.RawQuery, name)
}

func (c chiContext) Header(name string) string {
	return c.r.Header.Get(name)
}

func (c chiContext) EachHeader(cb func(name, value string)) {
	for name, values := range c.r.Header {
		for _, value := range values {
			cb(name, value)
		}
	}
}

func (c chiContext) BodyReader() io.Reader {
	return c.r.Body
}

func (c chiContext) GetMultipartForm() (*multipart.Form, error) {
	err := c.r.ParseMultipartForm(8 * 1024)
	return c.r.MultipartForm, err
}

func (c chiContext) SetReadDeadline(deadline time.Time) error {
	return huma.SetReadDeadline(c.w, deadline)
}

func (c chiContext) SetStatus(code int) {
	c.w.WriteHeader(code)
}

func (c chiContext) AppendHeader(name string, value string) {
	c.w.Header().Add(name, value)
}

func (c chiContext) SetHeader(name string, value string) {
	c.w.Header().Set(name, value)
}

func (c chiContext) BodyWriter() io.Writer {
	return c.w
}

// NewContext creates a new Huma context from an HTTP request and response.
func NewContext(op *huma.Operation, r *http.Request, w http.ResponseWriter) huma.Context {
	return &chiContext{op: op, r: r, w: w}
}

var defaultHandler = func(a *chiAdapter, op *huma.Operation, handler func(huma.Context)) {
	a.router.MethodFunc(op.Method, op.Path, func(w http.ResponseWriter, r *http.Request) {
		handler(&chiContext{op: op, r: r, w: w})
	})
}

type params struct {
	op      *huma.Operation
	handler func(huma.Context)
}

type chiAdapter struct {
	router   chi.Router
	route    func(a *chiAdapter, op *huma.Operation, handler func(huma.Context))
	handlers []params
}

func (a *chiAdapter) Handle(op *huma.Operation, handler func(huma.Context)) {
	// a.router.MethodFunc(op.Method, op.Path, func(w http.ResponseWriter, r *http.Request) {
	// 	handler(&chiContext{op: op, r: r, w: w})
	// })
	a.route(a, op, handler)
}

func (a chiAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}

// fn is a function that register all the operations registrations with the given middleware
func (a *chiAdapter) Group(fn func(), middlewares ...func(http.Handler) http.Handler) {
	a.route = func(c *chiAdapter, op *huma.Operation, handler func(huma.Context)) {
		if c.handlers == nil {
			c.handlers = make([]params, 0)
		}
		c.handlers = append(c.handlers, params{op: op, handler: handler})
	}
	defer func() {
		a.route = defaultHandler
	}()

	fn()
	if a.handlers == nil {
		return
	}
	defer func() {
		a.handlers = a.handlers[0:0]
	}()

	a.router.Group(func(r chi.Router) {
		r.Use(middlewares...)
		for i := 0; i < len(a.handlers); i++ {
			h := a.handlers[i]
			r.MethodFunc(h.op.Method, h.op.Path, func(w http.ResponseWriter, r *http.Request) {
				h.handler(&chiContext{op: h.op, r: r, w: w})
			})
		}
	})
}

// NewAdapter creates a new adapter for the given chi router.
func NewAdapter(r chi.Router) huma.Adapter {
	return &chiAdapter{router: r, route: defaultHandler}
}

// New creates a new Huma API using the latest v5.x.x version of Chi.
func New(r chi.Router, config huma.Config) huma.API {
	return huma.NewAPI(config, &chiAdapter{router: r, route: defaultHandler})
}

type chiAdapterV4 struct {
	router chiV4.Router
}

func (a chiAdapterV4) Handle(op *huma.Operation, handler func(huma.Context)) {
	a.router.MethodFunc(op.Method, op.Path, func(w http.ResponseWriter, r *http.Request) {
		handler(&chiContext{op: op, r: r, w: w, v4: true})
	})
}

func (a chiAdapterV4) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}

// NewV4 creates a new Huma API using the older v4.x.x version of Chi.
func NewV4(r chiV4.Router, config huma.Config) huma.API {
	return huma.NewAPI(config, chiAdapterV4{router: r})
}
