package gorpc

import (
	"encoding/json"
	"fmt"
	"go-rpc/codec"
	"go-rpc/common"
	"io"
	"net"
	"reflect"
	"sync"

	log "github.com/sirupsen/logrus"
)

// NOTE: 约定通信结构如下所示:

// Option: JSON
// Header1: ...
// Body1: ...
// Header2: ...
// Body3: ...

const MagicNumber = 190514

type Option struct {
	MagicNumber int
	CodecType   codec.Type
}

type request struct {
	header *codec.Header
	argv   reflect.Value
	replyv reflect.Value
}

type Server struct{}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

var (
	invalidRequest = struct{}{}
	DefaultServer  = NewServer()
)

func NewServer() *Server {
	return &Server{}
}

func Accept(l net.Listener) {
	DefaultServer.Accept(l)
}

func (s *Server) Accept(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Error("rpc server: accept error:", err)
			return
		}
		go s.ServeConn(conn)
	}
}

func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { common.ShouldSucc(conn.Close()) }()

	// 1. 第一步从conn中读出信息, 首先解码header的JSON格式
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Error("rpc server: options error: ", err)
		return
	}

	// 检查magic
	if opt.MagicNumber != MagicNumber {
		log.Errorf("rpc server: invalid magic number [%x]", opt.MagicNumber)
		return
	}

	// 根据header得到对应的编码方法
	constructor, ok := codec.NewCodecFuncMap[opt.CodecType]
	if !ok {
		log.Errorf("rpc server: invalid codec type [%s]", opt.CodecType)
		return
	}

	// 由对应的编码方法进行serve每一个connection
	s.serveCodec(constructor(conn))
}

// handleRequest 使用了协程并发执行请求。
// 处理请求是并发的，但是回复请求的报文必须是逐个发送的，
// 并发容易导致多个回复报文交织在一起，客户端无法解析。
// 在这里使用锁(mutex)保证。
// 尽力而为，只有在 header 解析失败时，才终止循环。

func (s *Server) serveCodec(c codec.Codec) {
	// 这里是每一个connection的不同request, 每一个connection持有一个response锁
	mu := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		// NOTE: 对于每一个connection, 可能有多个请求
		// 当读不到请求的时候, 关闭连接
		req, err := s.readReq(c)
		if err != nil {
			if req == nil {
				// 直接关闭连接, request读不到内容
				break
			}
			req.header.ErrMsg = err.Error()
			s.sendResp(mu, c, req.header, invalidRequest)
			continue
		}

		wg.Add(1)
		// 对于每一个合法请求, 进行对应的回应
		go s.handleReq(mu, wg, c, req)
	}

	wg.Wait()
	common.ShouldSucc(c.Close())
}

func (s *Server) handleReq(mu *sync.Mutex, wg *sync.WaitGroup, c codec.Codec, req *request) {
	defer wg.Done()

	log.Info(req.header, req.argv.Elem())
	req.replyv = reflect.ValueOf(
		fmt.Sprintf("go-rpc resp as string %d", req.header.SeqNum),
	)

	s.sendResp(mu, c, req.header, req.replyv.Interface())
}

// helper 读取request header
func (s *Server) readReqHeader(c codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := c.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Error("rpc server: read header error:", err)
		} else {
			// it encounters EOF, but should not encounter EOF here
		}
		return nil, err
	}
	return &h, nil
}

// 读取request
func (s *Server) readReq(c codec.Codec) (*request, error) {
	h, err := s.readReqHeader(c)
	if err != nil {
		return nil, err
	}
	req := &request{header: h}
	// TODO: 暂且把argv作为string处理
	req.argv = reflect.New(reflect.TypeOf(""))
	if err = c.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
		return req, err
	}

	return req, nil
}

// 发送回复
func (s *Server) sendResp(mu *sync.Mutex, c codec.Codec, h *codec.Header, body any) {
	mu.Lock()
	defer mu.Unlock()

	if err := c.Write(h, body); err != nil {
		log.Error("rpc server: write response error:", err)
	}
}
