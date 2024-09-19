package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var (
	ErrRequestForHTTPS = errors.New("Request is for HTTPS")
)

type ConnContext struct {
	Host, ProxyAuthorization, ProxyConnection string
}

type Handler interface {
	SendUpstream(ctx ConnContext, w io.Writer, r io.Reader) error
	SendDownstream(ctx ConnContext, w io.Writer, r io.Reader) error
}

type SimpleHandler struct{}

func (SimpleHandler) SendUpstream(_ ConnContext, w io.Writer, r io.Reader) (err error) {
	_, err = io.Copy(w, r)
	return
}

func (SimpleHandler) SendDownstream(_ ConnContext, w io.Writer, r io.Reader) (err error) {
	_, err = io.Copy(w, r)
	return
}

// newProxy creates a proxy with configuration. Proxy owns the listener
// and will close it itself.
func newProxy(listener net.Listener, handler Handler) *proxy {
	return &proxy{
		listener: listener,
		handler:  handler,
		closeSig: make(chan struct{}),
		wg:       new(sync.WaitGroup),
	}
}

type proxy struct {
	listener net.Listener
	handler  Handler

	closeSig chan struct{}
	wg       *sync.WaitGroup
}

func (p *proxy) serve() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for {
			conn, err := p.listener.Accept()
			if err != nil {
				select {
				case <-p.closeSig:
					return
				default:
					log.Println(err)
					continue
				}
			}

			go p.handle(ctx, conn)
		}
	}()
}

func (p *proxy) handle(ctx context.Context, conn net.Conn) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer conn.Close()

		bufReader := bufio.NewReader(conn)
		req, err := http.ReadRequest(bufReader)
		if err != nil {
			log.Println(err)
		}

		connCtx := ConnContext{
			Host:               req.Host,
			ProxyAuthorization: req.Header.Get("Proxy-Authorization"),
			ProxyConnection:    req.Header.Get("Proxy-Connection"),
		}

		log.Printf("Proxy request for %s", connCtx.Host)

		if req.Method == "CONNECT" {
			if err := httpsHandler(ctx, connCtx, conn, p.handler); err != nil {
				log.Println(err)
			}
			return
		}

		if err := httpHandler(ctx, connCtx, conn, req, p.handler); err != nil {
			log.Println(err)
		}
	}()
}

func (p *proxy) close() {
	close(p.closeSig)
	p.listener.Close()
	p.wg.Wait()
}

func fmtURL(url *url.URL) string {
	return fmt.Sprintf("http://%s%s?%s",
		url.Host, url.Path, url.RawQuery,
	)
}

func httpHandler(ctx context.Context, connCtx ConnContext, conn net.Conn, req *http.Request, handler Handler) error {
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

	proxyConn, err := net.Dial("tcp", produceHostPort(connCtx.Host))
	if err != nil {
		return err
	}
	defer proxyConn.Close()

	upstreamBuf := new(bytes.Buffer)
	if err := proxyReq.Write(upstreamBuf); err != nil {
		return err
	}

	if err := handler.SendUpstream(connCtx, proxyConn, upstreamBuf); err != nil {
		return err
	}

	proxyBufReader := bufio.NewReader(proxyConn)
	proxyResp, err := http.ReadResponse(proxyBufReader, proxyReq)
	if err != nil {
		return err
	}

	downstreamBuf := new(bytes.Buffer)
	if err := proxyResp.Write(downstreamBuf); err != nil {
		return err
	}

	if err := handler.SendDownstream(connCtx, conn, downstreamBuf); err != nil {
		return err
	}

	return nil
}

func httpsHandler(ctx context.Context, connCtx ConnContext, conn net.Conn, handler Handler) error {
	respSuccess := http.Response{
		Status:     "200 Connection Established",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}

	proxyConn, err := net.Dial("tcp", connCtx.Host)
	if err != nil {
		return err
	}

	if err := respSuccess.Write(conn); err != nil {
		return err
	}

	errSig := make(chan error, 2)
	go func() {
		errSig <- handler.SendUpstream(connCtx, conn, proxyConn)
	}()

	go func() {
		errSig <- handler.SendDownstream(connCtx, proxyConn, conn)
	}()

	err = <-errSig
	if err != nil {
		return err
	}

	err = <-errSig
	if err != nil {
		return err
	}

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
