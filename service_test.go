package gorpc

import (
	"testing"

	"github.com/sirupsen/logrus"
)

type Foo struct{}

type Args struct {
	Num1 int
	Num2 int
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// it's not a exported Method
func (f Foo) sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) Dick(args Args, reply *int, val_array []int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

type bar struct{}

func TestNewService(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	s := newService(&Foo{})
	if len(s.method) != 1 {
		t.Error("only 1 exported and valid method, actual:", len(s.method))
		return
	}
	mType := s.method["Sum"]
	t.Log(mType.method.Name)

	defer func() {
		if err := recover(); err == nil {
			t.Error("unexported receiver should fail")
		} else {
			t.Log("pass: caught panic caused by unexported receiver")
		}
	}()
	_ = newService(&bar{})
}
