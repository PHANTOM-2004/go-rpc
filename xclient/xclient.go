package xclient

import (
	"context"
	gorpc "go-rpc"
	"io"
	"reflect"
	"sync"
)

type (
	Option = gorpc.Option
	Client = gorpc.Client
)

type XClient struct {
	d    Discovery
	mode SelectMode

	opt     *Option
	mu      sync.Mutex
	clients map[string]*Client
}

var _ io.Closer = (*XClient)(nil)

func NewXClient(d Discovery, mode SelectMode, opt *Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*Client),
	}
}

func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	for key, client := range xc.clients {
		_ = client.Close()
		delete(xc.clients, key)
	}
	return nil
}

func (xc *XClient) dial(rpcAddr string) (*Client, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	client, ok := xc.clients[rpcAddr]
	// 尽量复用已有的连接
	if ok && !client.Available() {
		client.Close()
		delete(xc.clients, rpcAddr)
		client = nil
	}
	if client == nil {
		var err error
		client, err = gorpc.XDial(rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = client
	}
	return client, nil
}

func (xc *XClient) call(ctx context.Context, rpcAddr string, serviceMethod string, args, reply any) error {
	client, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	err = client.Call(ctx, serviceMethod, args, reply)
	return err
}

func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply any) error {
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	err = xc.call(ctx, rpcAddr, serviceMethod, args, reply)
	return err
}

// Broadcast 将请求广播到所有的服务实例，如果任意一个实例发生错误，则返回其中一个错误；如果调用成功，则返回其中一个的结果。
func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply any) error {
	servers, err := xc.d.GetAll()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var once sync.Once
	var e error

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, addr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var clonedReply any
			if reply != nil {
				t := reflect.ValueOf(reply).Elem()
				clonedReply = reflect.New(t.Type()).Interface()
			}
			err := xc.call(ctx, rpcAddr, serviceMethod, args, clonedReply)
			mu.Lock()
			defer mu.Unlock()

			if err != nil && e == nil {
				e = err
				cancel()
			}
			if err == nil {
				// 只做一次返回赋值
				once.Do(func() {
					t := reflect.ValueOf(clonedReply).Elem()
					reflect.ValueOf(reply).Elem().Set(t)
				})
			}
		}(addr)
	}

	wg.Wait()
	return e
}
