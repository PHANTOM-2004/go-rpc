package gorpc

import (
	"log"
	"net"
	"strings"
	"testing"
	"time"
)

func _assert(expr bool, msg string) {
	if !expr {
		log.Panic(msg)
	}
}

func TestClientdialTimeout(t *testing.T) {
	t.Parallel()
	l, _ := net.Listen("tcp", ":0")

	f := func(conn net.Conn, opt *Option) (client *Client) {
		_ = conn.Close()
		time.Sleep(time.Second * 2)
		return nil
	}

	t.Run("timeout", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &Option{TimeOut: time.Second})
		_assert(err != nil && strings.Contains(err.Error(), "connect timeout"), "expect a timeout error")
	})

	t.Run("0", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &Option{TimeOut: 0})
		_assert(err == nil, "0 means no limit")
	})
}
