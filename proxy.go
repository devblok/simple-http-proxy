package main

import (
	"context"
	"log"
	"net"
	"sync"
)

// Proxy performs these functions:
// 1. Is created with a listener
// 2. Accepts connections
// 3. Attempts to reason via HTTP
// 4. Based on the request spins off process:
// 	4a. If HTTP, do simple proxy by calling the destination.
// 	4b. If HTTPS (indicated by CONNECT), opens a TCP connection to destination, tunnels.
//	4c. Gets an error, returns the original error
//	4d. There is a proxy error, return custom error. This might be:
//		a. User used up allowance
//		b. User not authenticated (should return 407 probably)
// 5. Keeps running until closed
// 6. Waits for goroutines and cleans up

type handler func(context.Context, net.Conn) error

// newProxy creates a proxy with configuration. Proxy owns the listener
// and will close it itself.
func newProxy(hlr handler, listener net.Listener) *proxy {
	return &proxy{
		handler:  hlr,
		listener: listener,
		closeSig: make(chan struct{}),
		wg:       new(sync.WaitGroup),
	}
}

type proxy struct {
	handler  handler
	listener net.Listener

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
		if err := p.handler(ctx, conn); err != nil {
			log.Println(err)
		}
	}()
}

func (p *proxy) close() {
	close(p.closeSig)
	p.listener.Close()
	p.wg.Wait()
}
