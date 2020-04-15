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

type HttpRequestMetrics struct {
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

// HttpRequest is a global function to send a HTTP request
//  ctx - Context to be used in request
//  url - The URL for HTTP request
//  resolveTo - The ip:port of where to resolve to
//  method - GET, POST, DELETE, PATCH
//  content - The HTTP request content type (text/plain, application/json, or nil)
//  headers - An array of Headers to be sent (KEY=VALUE) []string{"Authentication=12345", ...}
//  body - The body or form data to send with HTTP request
//  timeout - Specific duration to timeout on. time.Duration(30 * time.Seconds)
//  You can use a HTTP Proxy if you HTTP_PROXY environment variable
func HttpRequest(ctx context.Context, url, resolveTo, method string, content interface{}, headers []string, body io.Reader, timeout time.Duration, verifySSL bool) ([]byte, *http.Response, *HttpRequestMetrics, error) {
	var err error
	var req *http.Request
	metrics := &HttpRequestMetrics{}

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

	req.Header.Set("User-Agent", "netlify-scout")
	if content != nil {
		req.Header.Set("Content-Type", content.(string))
	}

	verifyHost := req.URL.Hostname()
	for _, h := range headers {
		keyVal := strings.SplitN(h, "=", 2)
		if len(keyVal) == 2 {
			if keyVal[0] != "" && keyVal[1] != "" {
				if strings.ToLower(keyVal[0]) == "host" {
					req.Host = strings.TrimSpace(keyVal[1])
					verifyHost = req.Host
				} else {
					req.Header.Set(keyVal[0], keyVal[1])
				}
			}
		}
	}
	var resp *http.Response

	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: timeout,
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !verifySSL,
			ServerName:         verifyHost,
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
func (m *HttpRequestMetrics) NetworkLatency() int64 {
	return time.Unix(0, m.ConnectDone).Sub(time.Unix(0, m.GetConn)).Milliseconds()
}

// RequestLatency returns the request latency in ms
func (m *HttpRequestMetrics) RequestLatency() int64 {
	return time.Unix(0, m.GotResponse).Sub(time.Unix(0, m.GetConn)).Milliseconds()
}

// NetworkLatencyDuration returns the network connection latency as a Duration
func (m *HttpRequestMetrics) NetworkLatencyDuration() time.Duration {
	n := time.Unix(0, m.ConnectDone).Sub(time.Unix(0, m.GetConn)).Nanoseconds()
	return time.Duration(n) * time.Nanosecond
}

// RequestLatencyDuration returns the request latency as a Duration
func (m *HttpRequestMetrics) RequestLatencyDuration() time.Duration {
	n := time.Unix(0, m.GotResponse).Sub(time.Unix(0, m.GetConn)).Nanoseconds()
	return time.Duration(n) * time.Nanosecond
}
