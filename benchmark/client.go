package benchmark

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/valyala/fasthttp"
)

type client interface {
	do() (statusCode int, err error)
}

type bodyStreamProducer func() (io.ReadCloser, error)

type clientOpts struct {
	maxConns  uint64
	timeout   time.Duration
	tlsConfig *tls.Config

	headers     *map[string]string
	url, method string

	body    *string
	bodProd bodyStreamProducer
}

type fasthttpClient struct {
	client *fasthttp.HostClient

	headers                  *fasthttp.RequestHeader
	host, requestURI, method string

	body    *string
	bodProd bodyStreamProducer
}

func newFastHTTPClient(opts *clientOpts) client {
	c := new(fasthttpClient)
	u, err := url.Parse(opts.url)
	if err != nil {
		// opts.url guaranteed to be valid at this point
		panic(err)
	}
	c.host = u.Host
	c.requestURI = u.RequestURI()
	c.client = &fasthttp.HostClient{
		Addr:                          u.Host,
		IsTLS:                         u.Scheme == "https",
		MaxConns:                      int(opts.maxConns),
		ReadTimeout:                   opts.timeout,
		WriteTimeout:                  opts.timeout,
		DisableHeaderNamesNormalizing: true,
		TLSConfig:                     opts.tlsConfig,
		Dial: fasthttpDialFunc(
			opts.bytesRead, opts.bytesWritten,
		),
	}
	c.headers = headersToFastHTTPHeaders(opts.headers)
	c.method, c.body = opts.method, opts.body
	c.bodProd = opts.bodProd
	return c
}

func (c *fasthttpClient) generateHeaders(headers *map[string]string) fasthttp.RequestHeader {
	return nil
}

func (c *fasthttpClient) do() (code int, err error) {
	// prepare the request
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	if c.headers != nil {
		c.headers.CopyTo(&req.Header)
	}
	if len(req.Header.Host()) == 0 {
		req.Header.SetHost(c.host)
	}
	req.Header.SetMethod(c.method)
	req.SetRequestURI(c.requestURI)
	if c.body != nil {
		req.SetBodyString(*c.body)
	} else {
		bs, bserr := c.bodProd()
		if bserr != nil {
			return 0, bserr
		}
		req.SetBodyStream(bs, -1)
	}

	// fire the request
	err = c.client.Do(req, resp)
	if err != nil {
		code = -1
	} else {
		code = resp.StatusCode()
	}

	// release resources
	fasthttp.ReleaseRequest(req)
	fasthttp.ReleaseResponse(resp)

	return
}

type httpClient struct {
	client *http.Client

	headers http.Header
	url     *url.URL
	method  string

	body    *string
	bodProd bodyStreamProducer
}
