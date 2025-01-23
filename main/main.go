package main

import (
	"encoding/json"
	"fmt"
	gorpc "go-rpc"
	"go-rpc/codec"
	"net"

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
	conn, err := net.Dial("tcp", <-addr)
	if err != nil {
		log.Panic(err)
	}
	defer func() { _ = conn.Close() }()

	// send options
	_ = json.NewEncoder(conn).Encode(gorpc.DefaultOption)
	cc := codec.NewGobCodec(conn)
	log.Info("option sent")

	// send request & receive response
	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum.Bitch",
			SeqNum:        uint64(i),
		}
		_ = cc.Write(h, fmt.Sprintf("gorpc req %d", h.SeqNum))

		_ = cc.ReadHeader(h)
		var reply string
		_ = cc.ReadBody(&reply)
		log.Info("reply:", reply)
	}
}
