package scout

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"
)

type HTTPRequestMetrics struct {
	GetConn              int64
	GotConn              int64
	GotFirstResponseByte int64
	DNSStart             int64
	DNSDone              int64
	ConnectStart         int64
	ConnectDone          int64
	TLSHandshakeStart    int64
	TLSHandshakeDone     int64
	WroteHeaderField     int64
	WroteHeaders         int64
	WroteRequest         int64
	GotResponse          int64
}

// HTTPRequest is a global function to send a HTTP request
//  ctx - Context to be used in request
//  url - The URL for HTTP request
//  resolveTo - The ip:port of where to resolve to
//  method - GET, POST, DELETE, PATCH
//  contentType - The HTTP request content type (text/plain, application/json, or nil)
//  headers - Headers to be used for the request
//  body - The body or form data to send with HTTP request
//  timeout - Specific duration to timeout on. time.Duration(30 * time.Seconds)
//  verifySSL - verify the SSL certificate
//  You can use a HTTP Proxy if you HTTP_PROXY environment variable
func HTTPRequest(ctx context.Context, url, resolveTo, method string, contentType interface{}, headers http.Header, body io.Reader, timeout time.Duration, verifySSL bool) ([]byte, *http.Response, *HTTPRequestMetrics, error) {
	var err error
	var req *http.Request
	metrics := &HTTPRequestMetrics{}

	if req, err = http.NewRequestWithContext(ctx, method, url, body); err != nil {
		return nil, nil, nil, err
	}
	trace := &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			metrics.GetConn = time.Now().UnixNano()
		},
		GotConn: func(httptrace.GotConnInfo) {
			metrics.GotConn = time.Now().UnixNano()
		},
		GotFirstResponseByte: func() {
			metrics.GotFirstResponseByte = time.Now().UnixNano()
		},
		DNSStart: func(httptrace.DNSStartInfo) {
			metrics.DNSStart = time.Now().UnixNano()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			metrics.DNSDone = time.Now().UnixNano()
		},
		ConnectStart: func(network, addr string) {
			metrics.ConnectStart = time.Now().UnixNano()
		},
		ConnectDone: func(network, addr string, err error) {
			metrics.ConnectDone = time.Now().UnixNano()
		},
		TLSHandshakeStart: func() {
			metrics.TLSHandshakeStart = time.Now().UnixNano()
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			metrics.TLSHandshakeDone = time.Now().UnixNano()
		},
		WroteHeaderField: func(key string, value []string) {
			metrics.WroteHeaderField = time.Now().UnixNano()
		},
		WroteHeaders: func() {
			metrics.WroteHeaders = time.Now().UnixNano()
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			metrics.WroteRequest = time.Now().UnixNano()
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	if headers != nil {
		if headers.Get("User-Agent") == "" {
			headers.Set("User-Agent", "phenixrizen-scout")
		}
		if contentType != nil {
			ct, ok := contentType.(string)
			if ok {
				headers.Set("Content-Type", ct)
			}
		}
	}

	req.Header = headers

	var resp *http.Response

	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: timeout,
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !verifySSL,
			ServerName:         req.URL.Hostname(),
		},
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: timeout,
		TLSHandshakeTimeout:   timeout,
		Proxy:                 http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if resolveTo != "" {
				addr = resolveTo
			} else {
				// redirect all connections to host specified in url
				addr = strings.Split(req.URL.Host, ":")[0] + addr[strings.LastIndex(addr, ":"):]
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	if resp, err = client.Do(req); err != nil {
		return nil, resp, metrics, err
	}
	metrics.GotResponse = time.Now().UnixNano()
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(contents))
	return contents, resp, metrics, err
}

// NetworkLatency returns the network connection latency in ms
func (m *HTTPRequestMetrics) NetworkLatency() int64 {
	return time.Unix(0, m.ConnectDone).Sub(time.Unix(0, m.GetConn)).Milliseconds()
}

// RequestLatency returns the request latency in ms
func (m *HTTPRequestMetrics) RequestLatency() int64 {
	return time.Unix(0, m.GotResponse).Sub(time.Unix(0, m.GetConn)).Milliseconds()
}

// NetworkLatencyDuration returns the network connection latency as a Duration
func (m *HTTPRequestMetrics) NetworkLatencyDuration() time.Duration {
	n := time.Unix(0, m.ConnectDone).Sub(time.Unix(0, m.GetConn)).Nanoseconds()
	return time.Duration(n) * time.Nanosecond
}

// RequestLatencyDuration returns the request latency as a Duration
func (m *HTTPRequestMetrics) RequestLatencyDuration() time.Duration {
	n := time.Unix(0, m.GotResponse).Sub(time.Unix(0, m.GetConn)).Nanoseconds()
	return time.Duration(n) * time.Nanosecond
}
