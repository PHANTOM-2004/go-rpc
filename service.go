package gorpc

import (
	"go-rpc/common"
	"go/ast"
	"reflect"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
	numCalls  uint64
}

func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

func (m *methodType) newArgv() reflect.Value {
	var argv reflect.Value
	// new返回的是指针类型
	if m.ArgType.Kind() == reflect.Ptr {
		// elem是指针指向的值
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *methodType) newRepv() reflect.Value {
	replv := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replv
}

type service struct {
	name     string
	typ      reflect.Type
	receiver reflect.Value
	method   map[string]*methodType
}

func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	ins := []reflect.Value{
		s.receiver,
		argv,
		replyv,
	}
	returnValues := f.Call(ins)
	err := returnValues[0].Interface()
	if err != nil {
		return err.(error)
	}
	return nil
}

func newService(receiver any) *service {
	s := new(service)
	s.receiver = reflect.ValueOf(receiver)
	s.name = reflect.Indirect(s.receiver).Type().Name()
	log.Debug("receiver name: ", s.name)
	s.typ = reflect.Indirect(s.receiver).Type() // NOTE:小心指针类型
	if !ast.IsExported(s.name) {
		log.Panicf("rpc server: method [%s] is not exported", s.name)
	}
	s.registerMethods()
	return s
}

// 注册methods
func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)
	log.Debug("method count:", s.typ.NumMethod())
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		log.Debug(i, " ", method.Name)
		mType := method.Type
		if !s.methodFilter(mType) {
			continue
		}

		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   mType.In(1),
			ReplyType: mType.In(2),
		}
		log.Infof("rpc server: register %s.%s\n",
			s.name, method.Name)

	}
}

func (s *service) methodFilter(mType reflect.Type) bool {
	if mType.NumIn() != 3 || mType.NumOut() != 1 {
		// NOTE: 要求In是3个, Out是1个
		log.Debug(mType.Name(), "in/out count fails")
		return false
	}
	//(error)(nil)：
	// 这是将 nil 转换成 error 类型的值。error 是一个接口类型，所以 (error)(nil)
	// 表示的是一个 nil 接口值（既没有具体类型也没有值）。
	// TypeOf((error)(nil)) 会返回接口类型 error，
	// 而不是它指向的具体类型（因为接口的零值是 nil）。
	if target := reflect.TypeOf((*error)(nil)).Elem(); mType.Out(0) != target {
		// NOTE:返回值必须是error
		log.Debug(mType.Name(), "out type fails")
		log.Debug("out type: ", mType.Out(0), ", should be: ", target.Name())
		return false
	}
	// NOTE: argType, replyType必须builtIn或者导出
	argType, replyType := mType.In(1), mType.In(2)
	if !common.IsExportedOrBuiltin(argType) ||
		!common.IsExportedOrBuiltin(replyType) {
		log.Debug(mType.Name(), "in/out unexported")
		return false
	}
	return true
}
