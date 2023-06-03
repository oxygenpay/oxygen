package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
)

type Marshable interface {
	MarshalBinary() ([]byte, error)
}

type Client struct {
	handler http.HandlerFunc
}

func (c *Client) GET() *Request {
	return c.newRequest(http.MethodGet)
}

func (c *Client) POST() *Request {
	return c.newRequest(http.MethodPost)
}

func (c *Client) PUT() *Request {
	return c.newRequest(http.MethodPut)
}
func (c *Client) DELETE() *Request {
	return c.newRequest(http.MethodDelete)
}

func (c *Client) newRequest(method string) *Request {
	return &Request{
		handler: c.handler,
		method:  method,
		params:  make(map[string]string),
		query:   make(map[string]string),
		headers: make(map[string]string),
	}
}

type Request struct {
	handler  http.HandlerFunc
	method   string
	path     string
	params   map[string]string
	query    map[string]string
	headers  map[string]string
	withCSRF bool
	body     []byte
}

func (r *Request) Path(path string) *Request {
	r.path = path
	return r
}

func (r *Request) Param(key, value string) *Request {
	r.params[":"+key] = value
	return r
}

func (r *Request) Query(key, value string) *Request {
	r.query[key] = value
	return r
}

func (r *Request) WithToken(token string) *Request {
	r.headers[middleware.TokenHeader] = token
	return r
}

func (r *Request) Body(body []byte) *Request {
	r.body = body
	return r
}

func (r *Request) JSON(m Marshable) *Request {
	body, err := m.MarshalBinary()
	if err != nil {
		panic("unable to marshal json: " + err.Error())
	}

	r.body = body
	return r
}

func (r *Request) WithCSRF() *Request {
	r.withCSRF = true
	return r
}

func (r *Request) Do() *Response {
	// substitute path
	path := r.path
	for search, value := range r.params {
		path = strings.Replace(path, search, value, -1)
	}

	req := httptest.NewRequest(r.method, path, bytes.NewReader(r.body))

	// substitute query
	query := req.URL.Query()
	for q, value := range r.query {
		query.Set(q, value)
	}
	req.URL.RawQuery = query.Encode()

	// substitute headers
	for h, value := range r.headers {
		req.Header.Set(h, value)
	}

	if r.method == http.MethodPost || r.method == http.MethodPut {
		req.Header.Set("Content-Type", echo.MIMEApplicationJSON)

		if r.withCSRF {
			applyCSRF(req)
		}
	}

	res := httptest.NewRecorder()
	r.handler(res, req)

	return &Response{res}
}

func applyCSRF(req *http.Request) {
	csrf := "csrf-token-goes-here"
	req.Header.Set(echo.HeaderXCSRFToken, csrf)
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: csrf, Path: "/"})
}

type Response struct {
	res *httptest.ResponseRecorder
}

func (r *Response) StatusCode() int {
	return r.res.Code
}

func (r *Response) String() string {
	return r.res.Body.String()
}

func (r *Response) Bytes() []byte {
	data, _ := io.ReadAll(r.res.Body)
	return data
}

func (r *Response) Headers() http.Header {
	return r.res.Header()
}

func (r *Response) JSON(output any) error {
	data, err := io.ReadAll(r.res.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, output)
}
