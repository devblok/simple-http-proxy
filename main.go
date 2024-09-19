package main

import (
	"net"
	"os"
	"os/signal"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	prx := newProxy(listener, new(SimpleHandler))
	prx.serve()

	sig := make(chan os.Signal, 5)
	signal.Notify(sig, os.Interrupt)

	<-sig
	prx.close()
}
