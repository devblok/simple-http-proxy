package main

import (
	"flag"
	"net"
	"os"
	"os/signal"
)

var (
	listenAddress = flag.String("listen", "localhost:8080", "Listen address for server, eg. 127.0.0.1:8080")
	userConfig    = flag.String("users", "users.json", "Users configuration file")
)

func main() {
	flag.Parse()

	config, err := loadConfig(*userConfig)
	if err != nil {
		panic(err)
	}

	handler := NewAccountingHandler(config.Users...)

	listener, err := net.Listen("tcp", *listenAddress)
	if err != nil {
		panic(err)
	}

	prx := newProxy(listener, handler)
	prx.serve()

	sig := make(chan os.Signal, 5)
	signal.Notify(sig, os.Interrupt)

	<-sig
	prx.close()
}
