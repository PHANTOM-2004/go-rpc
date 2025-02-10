package gorpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-rpc/codec"
	"io"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var ErrShutdown = errors.New("connection has shutdown")

type Call struct {
	seq uint64

	ServiceMethod string
	Args          any
	Reply         any
	Error         error
	Done          chan *Call
}

func (c *Call) done() {
	c.Done <- c
}

type Client struct {
	cc     codec.Codec
	opt    *Option
	header codec.Header
	seq    uint64

	mu    sync.Mutex
	sigMu sync.Mutex

	pending  map[uint64]*Call
	closing  bool
	shutdown bool
}

// sanity check, *Client must immplement Close
var _ io.Closer = (*Client)(nil)

func NewClient(conn net.Conn, opt *Option) *Client {
	var err error
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Panic("rpc client: unsupported CodecType:", opt.CodecType)
	}

	err = json.NewEncoder(conn).Encode(opt)
	if err != nil {
		log.Panic("rpc client: options error: ", err)
	}

	client := &Client{
		seq:     1,
		cc:      f(conn),
		opt:     opt,
		pending: make(map[uint64]*Call),
	}

	go client.receive()

	return client
}

type newClientFunc = func(net.Conn, *Option) *Client

func dialTimeout(constructClient newClientFunc, network, addr string, opts ...*Option) (*Client, error) {
	opt := parseOption(opts...)
	conn, err := net.DialTimeout(
		network,
		addr,
		opt.TimeOut,
	)
	if err != nil {
		return nil, err
	}

	clientDone := make(chan struct{})
	var client *Client
	go func() {
		client = constructClient(conn, opt)
		clientDone <- struct{}{}
	}()

	if opt.TimeOut == 0 {
		// 没有限制
		<-clientDone
		return client, nil
	}

	select {
	case <-clientDone:
		return client, nil
	case <-time.After(opt.TimeOut):
		// _ = conn.Close()
		if err := conn.Close(); err != nil {
			log.Error(err)
		}
		return nil, fmt.Errorf(
			"rpc client: connect timeout: expect within %s",
			opt.TimeOut,
		)
	}
}

// 这里的效果是实现0或者1 的option作为可选参数, 而非多个option
func Dial(network, addr string, opts ...*Option) (*Client, error) {
	return dialTimeout(NewClient, network, addr, opts...)
}

func (c *Client) Available() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.shutdown && !c.closing
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return ErrShutdown
	}
	c.closing = true
	return c.cc.Close()
}

// client.Go 的好处在于参数 done chan *Call 可以自定义缓冲区的大小
// 可以给多个 client.Go 传入同一个 chan 对象，
// 从而控制异步请求并发的数量。
func (c *Client) Go(
	serviceMethod string,
	args, reply any, done chan *Call,
) *Call {
	if done == nil {
		// 为什么done要使用buffer channel?
		// NOTE: 如果不buffer, 那么发送完成信号的时候就阻塞了
		done = make(chan *Call, 1)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}

	// NOTE:此处异步请求不必等待请求发送的完成
	go c.send(call)

	return call
}

func (c *Client) Call(ctx context.Context, serviceMethod string, args, reply any) error {
	call := c.Go(serviceMethod, args, reply, make(chan *Call, 1))

	select {
	case <-ctx.Done():
		log.Debug("client call timeout, ctx.Done()")
		c.removeCall(call.seq)
		return errors.New("rpc client: call failed: " + ctx.Err().Error())
	case callRes := <-call.Done:
		return callRes.Error
	}
}

// 发送消息
func (c *Client) send(call *Call) {
	// 这里的send与terminate互斥
	// 保证发送消息与取消队列中的消息互斥
	c.sigMu.Lock()
	defer c.sigMu.Unlock()

	log.Debug("registering call")
	seq, err := c.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	c.header.ServiceMethod = call.ServiceMethod
	c.header.SeqNum = seq
	c.header.ErrMsg = ""

	// encode and send
	log.Debug("writing header")
	err = c.cc.Write(&c.header, call.Args)
	if err != nil {
		log.Debug(err)
		log.Debug("removing call")
		call := c.removeCall(seq)
		// 写入的时候只写入了一部分, client的接受goroutine
		// 可能会先把call remove掉, 所以说removeCall操作
		// 1. [Client.send], 2. [Client.receive]
		if call != nil {
			call.Error = err
			call.done()
		}
	}
	log.Debug("send done")
}

// 解析0个或1各参数, 否则panic
func parseOption(opts ...*Option) *Option {
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption
	}
	if len(opts) > 1 {
		log.Panic("rpc client: option error: more than 1 option")
	}
	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = DefaultOption.CodecType
	}
	return opt
}

func (c *Client) isAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closing && !c.shutdown
}

func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.shutdown {
		return 0, ErrShutdown
	}
	call.seq = c.seq
	c.pending[call.seq] = call
	c.seq++
	return call.seq, nil
}

func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	res := c.pending[seq]
	delete(c.pending, seq)
	return res
}

// terminateCalls：服务端或客户端发生错误时调用，
// 将 shutdown 设置为 true，且将错误信息通知所有 pending 状态的 call。
func (c *Client) terminateCalls(err error) {
	// 用于保护terminate方法

	c.sigMu.Lock()
	defer c.sigMu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
}

// 核心方法, 接受响应
func (c *Client) receive() {
	log.Debug("client starts receiving")
	var err error

	for err == nil {
		// 每次首先读取header
		var h codec.Header
		log.Debug("receive: reading header")
		err = c.cc.ReadHeader(&h)
		if err != nil {
			// log.Debug("rpc client: receive: ", err)
			log.Info("rpc client: connection closed: ", err)
			break
		}

		log.Debug("receive: removing call")
		call := c.removeCall(h.SeqNum)
		switch {
		case call == nil:
			// NOTE:需要注意什么时候会c.removeCall
			//
			//1.[Client.receive] 请求接收到的时候, 该call可以踢出pending
			//2.[Client.send] 发送请求有错误的时候, 该call被踢出pending
			//所以说, 如果请求有错误, 也就是write了一部分
			//那么请求的时候就会踢出pending, 此时只读到了部分
			//消息, 所以说removeCall之后得到call是空的,
			//此时抛弃Body消息
			log.Debug("receive: nil Call")
			err = c.cc.ReadBody(nil) // NOTE: discard

		case h.ErrMsg != "":
			log.Debug("receive: error occurs")
			call.Error = errors.New(h.ErrMsg)
			err = c.cc.ReadBody(nil)
			call.done()

		default:
			log.Debug("receive: default")
			log.Debug("receive: reading body")
			err = c.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("call error: reading body: " +
					err.Error())
			}
			call.done()
		}

	}

	log.Debug("receive: end")
	// error, or reach EOF(ErrEOF)
	c.terminateCalls(err)
}
