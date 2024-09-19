package main // In this case needed because some things I decided to leave private

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type HelloHandler struct{}

func (HelloHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
	_, _ = w.Write([]byte("Hello!"))
}

// TestProxyHttp has an issue with this test. For some issue net.Dial in the
// proxy code fails to resolve IP:Port combinations for localhost. So if I make this test work,
// the real world calls don't in some cases. Tired of chasing it this evening,
// so I opted to keep the real world calls working.
func TestProxyHttp(t *testing.T) {
	server := httptest.NewServer(new(HelloHandler))
	defer server.Close()

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	defer listener.Close()

	proxy := newProxy(listener, new(SimpleHandler))
	proxy.serve()
	defer proxy.close()

	proxyURL, err := url.Parse(
		fmt.Sprintf("http://%s",
			listener.Addr().String(),
		),
	)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	client := server.Client()
	client.Transport.(*http.Transport).Proxy = http.ProxyURL(proxyURL)
	resp, err := client.Do(req)
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
