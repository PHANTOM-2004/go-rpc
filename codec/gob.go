package codec

import (
	"bufio"
	"encoding/gob"
	"go-rpc/common"
	"io"

	log "github.com/sirupsen/logrus"
)

type GobCodec struct {
	conn io.ReadWriteCloser
	dec  *gob.Decoder
	enc  *gob.Encoder
	buf  *bufio.Writer
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
  //这里其实也可以用conn作为writer, 但是我们希望指定缓冲区
	buf := bufio.NewWriter(conn)
	res := &GobCodec{
		conn: conn,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
		buf:  buf,
	}
	return res
}

func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

func (c *GobCodec) ReadBody(body any) error {
	return c.dec.Decode(body)
}

func (c *GobCodec) Write(h *Header, body any) (err error) {
	defer func() {
		// 写后刷新
		common.ShouldSucc(c.buf.Flush())
		if err != nil {
			common.ShouldSucc(c.Close())
		}
	}()

	if err := c.enc.Encode(h); err != nil {
		log.Error("rpc codec: gob error encoding header: ", err)
		return err
	}
	if err := c.enc.Encode(body); err != nil {
		log.Error("rpc codec: gob error encoding body:", err)
		return err
	}
	return nil
}

func (c *GobCodec) Close() error {
	return c.conn.Close()
}
