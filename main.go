package main

import (
	"time"
)

func main() {
	prx := newProxy()
	prx.start()

	<-time.After(time.Second)

	prx.close()
}
