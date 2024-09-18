package main

import (
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

func newProxy() *proxy {
	return &proxy{
		closeSig: make(chan struct{}),
		wg:       new(sync.WaitGroup),
	}
}

type proxy struct {
	closeSig chan struct{}
	wg       *sync.WaitGroup
}

func (p *proxy) start() {
	p.wg.Add(1)
	go func() {
		<-p.closeSig
		p.wg.Done()
	}()
}

func (p *proxy) close() {
	close(p.closeSig)
	p.wg.Wait()
}
