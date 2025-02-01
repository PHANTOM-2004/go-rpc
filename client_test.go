package gorpc

import (
	"context"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
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

type Bar struct{}

func (b Bar) Timeout(argv int, reply *int) error {
	time.Sleep(time.Second * 2)
	return nil
}

func helperStartServer(addr chan string) {
	Register(&Bar{})

	// pick a free port
	l, _ := net.Listen("tcp", ":0")
	addr <- l.Addr().String()
	Accept(l)
}

// NOTE: 网络环境非常复杂, 很容易出现conn超时的情况
// 也就是说默认读取的时候, ReadHeader等操作一直阻塞.
// 实际上或许需要提供conn的Deadline等等进行避免
func TestClientCall(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	t.Parallel()
	addrCh := make(chan string)
	go helperStartServer(addrCh)
	addr := <-addrCh

	time.Sleep(time.Second)

	t.Run("client timeout", func(t *testing.T) {
		client, _ := Dial("tcp", addr)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var reply int
		err := client.Call(ctx, "Bar.Timeout", 1, &reply)
		_assert(err != nil && strings.Contains(err.Error(), ctx.Err().Error()), "expect a timeout error")
	})

	for i := 0; i < 100; i++ {
		t.Run("server handle timeout", func(t *testing.T) {
			client, err := Dial("tcp", addr, &Option{
				TimeOut: time.Microsecond * 100,
			})
			if err != nil {
				logrus.Error(err)
				logrus.Debug("pass")
				return
			}
			var reply int
			t.Log("Dialed")
			err = client.Call(context.Background(), "Bar.Timeout", 1, &reply)
			_assert(err != nil && strings.Contains(err.Error(), "handle timeout"), "expect a timeout error")
		})
	}
}
