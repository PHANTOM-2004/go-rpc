package gorpc

import (
	"net"
	"net/http"
	"os"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestXDial(t *testing.T) {
	if runtime.GOOS != "linux" {
		return
	}

	t.Run("socket", func(t *testing.T) {
		ch := make(chan struct{})
		addr := "/tmp/geerpc.sock"
		go func() {
			_ = os.Remove(addr)
			l, err := net.Listen("unix", addr)
			if err != nil {
				t.Error("failed to listen unix socket")
			}
			ch <- struct{}{}
			Accept(l)
		}()

		<-ch
		_, err := XDial("unix@" + addr)
		_assert(err == nil, "failed to connect unix socket")
	})

	t.Run("http", func(t *testing.T) {
    logrus.SetLevel(logrus.DebugLevel)
		ch := make(chan struct{})
		addr := "127.0.0.1:21212"
		go func() {
			l, _ := net.Listen("tcp", addr)
			HandleHTTP()
			ch <- struct{}{}
			t.Log("serving http")
			_ = http.Serve(l, nil)
		}()

		<-ch

		_, err := XDial("http@" + addr)

		_assert(err == nil, "failed to connect http server")
	})
}
