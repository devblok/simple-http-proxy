package main // In this case needed because some things I decided to leave private

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type HelloHandler struct{}

func (HelloHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
	_, _ = w.Write([]byte("Hello!"))
}

func TestProxyHttp(t *testing.T) {
	server := httptest.NewServer(new(HelloHandler))
	defer server.Close()

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	proxy := newProxy(listener, new(SimpleHandler))
	proxy.serve()
	defer proxy.close()

	proxyDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	proxyTransport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           proxyDialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	proxyClient := http.Client{
		Transport: proxyTransport,
	}

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	resp, err := proxyClient.Do(req)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if "Hello!" != string(respBytes) {
		t.Errorf("Response expected: %s, got %s", "Hello!", string(respBytes))
	}
}
