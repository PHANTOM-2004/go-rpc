package main

import (
	"fmt"
	gorpc "go-rpc"
	"net"
	"sync"

	log "github.com/sirupsen/logrus"
)

func startServer(addr chan string) {
	// random port
	l, err := net.Listen("tcp", "")
	if err != nil {
		log.Panic(err)
	}

	log.Info("rpc server on: ", l.Addr())
	addr <- l.Addr().String()

	gorpc.Accept(l)
}

func main() {
	// server launched
	addr := make(chan string)
	go startServer(addr)

	// client
	client, err := gorpc.Dial("tcp", <-addr)
	if err != nil {
		log.Panic(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			args := fmt.Sprintf("gorpc req [%d]", j)
			var reply string
			err := client.Call("Foo.Bar", args, &reply)
			if err != nil {
				log.Panic(err)
			}
		}(i)
	}
	wg.Wait()
}
