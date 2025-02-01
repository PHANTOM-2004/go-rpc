package gorpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	log "github.com/sirupsen/logrus"
)

type Goo struct{}

func (f Goo) Bar(argv int, replyv *[]int) error {
	*replyv = append(*replyv, argv)
	if (*replyv)[0] > 3 {
		return errors.New("error occurs, argv > 3")
	}
	return nil
}

func (f Goo) Goo(argv *string, replyv *map[string]int) error {
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

	Accept(l)
}

var once sync.Once

func TestServer(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	// server launched
	addr := make(chan string)
	once.Do(
		func() {
			Register(&Goo{})
		},
	)

	go startServer(addr)

	// client
	client, err := Dial("tcp", <-addr)
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
			err := client.Call(context.TODO(), "Goo.Bar", args, reply)
			if err != nil {
				log.Errorln(err)
				return
			}
			t.Log(reply)
		}(i)
	}
	wg.Wait()

	fmt.Println("case 2: ", strings.Repeat("=", 30))
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			args := strconv.Itoa(i)
			var replyv map[string]int
			err := client.Call(context.TODO(), "Goo.Goo", args, &replyv)
			if err != nil {
				log.Errorln(err)
				return
			}
			t.Log(replyv)
		}()
	}
	wg.Wait()
}
