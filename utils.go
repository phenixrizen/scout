package scout

import (
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

// HttpRequest is a global function to send a HTTP request
//  url - The URL for HTTP request
//  resolveTo - The ip:port of where to resolve to
//  method - GET, POST, DELETE, PATCH
//  content - The HTTP request content type (text/plain, application/json, or nil)
//  headers - An array of Headers to be sent (KEY=VALUE) []string{"Authentication=12345", ...}
//  body - The body or form data to send with HTTP request
//  timeout - Specific duration to timeout on. time.Duration(30 * time.Seconds)
//  You can use a HTTP Proxy if you HTTP_PROXY environment variable
func HttpRequest(url, resolveTo, method string, content interface{}, headers []string, body io.Reader, timeout time.Duration, verifySSL bool) ([]byte, *http.Response, error) {
	var err error
	var req *http.Request
	if req, err = http.NewRequest(method, url, body); err != nil {
		return nil, nil, err
	}
	http.U
	req.Header.Set("User-Agent", "phenixrizen-scout")
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
		return nil, resp, err
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	return contents, resp, err
}
