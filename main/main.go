package main

import (
	"context"
	"errors"
	"fmt"
	gorpc "go-rpc"
	"net"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type Foo struct{}

func (f Foo) Bar(argv int, replyv *[]int) error {
	*replyv = append(*replyv, argv)
	if (*replyv)[0] > 3 {
		return errors.New("error occurs, argv > 3")
	}
	return nil
}

func (f Foo) Goo(argv *string, replyv *map[string]int) error {
	(*replyv)[*argv] = 1
	return nil
}

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
	log.SetLevel(log.DebugLevel)
	// server launched
	addr := make(chan string)
	gorpc.Register(&Foo{})

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
			args := j
			reply := &[]int{}
			err := client.Call(context.TODO(), "Foo.Bar", args, reply)
			if err != nil {
				log.Errorln(err)
				return
			}
			log.Info(reply)
		}(i)
	}
	wg.Wait()

	fmt.Println(strings.Repeat("=", 30))
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			args := strconv.Itoa(i)
			var replyv map[string]int
			err := client.Call(context.TODO(), "Foo.Goo", args, &replyv)
			if err != nil {
				log.Errorln(err)
				return
			}
			log.Info(replyv)
		}()
	}
	wg.Wait()
}
