package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
)

var (
	ErrRequestForHTTPS   = errors.New("Request is for HTTPS")
	ErrMissingHostHeader = errors.New("Host header is missing")
)

func fmtURL(url *url.URL) string {
	return fmt.Sprintf("http://%s%s?%s",
		url.Host, url.Path, url.RawQuery,
	)
}

func httpHandler(ctx context.Context, conn net.Conn) error {
	bufReader := bufio.NewReader(conn)
	req, err := http.ReadRequest(bufReader)
	if err != nil {
		return err
	}

	if req.Method == "CONNECT" {
		// TODO: Based on this error we switch to TCP tunnelling.
		return ErrRequestForHTTPS
	}

	// FIXME: Header value is promoted to Request.Host, so I cannot check
	// for it's availability. Not having this check is incorrect.
	// host := req.Header.Get("Host")
	// if host == "" {
	// 	return ErrMissingHostHeader
	// }

	proxyReq, err := http.NewRequest(req.Method, fmtURL(req.URL), req.Body)
	if err != nil {
		return err
	}

	// FIXME: Don't do dumb copy.
	proxyReq.Header = req.Header
	for _, cookie := range req.Cookies() {
		proxyReq.AddCookie(cookie)
	}

	// TODO: Add X-Forwarded-* headers.

	proxyConn, err := net.Dial("tcp", produceHostPort(req.Host))
	if err != nil {
		return err
	}
	defer proxyConn.Close()

	if err := proxyReq.Write(proxyConn); err != nil {
		return err
	}

	proxyBufReader := bufio.NewReader(proxyConn)
	proxyResp, err := http.ReadResponse(proxyBufReader, proxyReq)
	if err != nil {
		return err
	}

	bytes, err := httputil.DumpResponse(proxyResp, true)
	if err != nil {
		return err
	}

	if _, err := conn.Write(bytes); err != nil {
		return err
	}
	// TODO: can do rudimental accounting here.

	return nil
}

func produceHostPort(in string) string {
	if strings.Contains(in, ":") {
		if _, _, err := net.SplitHostPort(in); err != nil {
			return in
		}
	}
	return net.JoinHostPort(in, "80")
}

func main() {
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	prx := newProxy(httpHandler, listener)
	prx.serve()

	sig := make(chan os.Signal, 5)
	signal.Notify(sig, os.Interrupt)

	<-sig
	prx.close()
}
