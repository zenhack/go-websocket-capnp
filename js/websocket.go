//go:build js

package js

import (
	"context"
	"syscall/js"
	"time"

	"capnproto.org/go/capnp/v3"
	"capnproto.org/go/capnp/v3/rpc/transport"
)

var _ transport.Codec = &Conn{}

type Conn struct {
	value js.Value
	msgs  chan *capnp.Message
	err   error
}

func New(url string) *Conn {
	ret := &Conn{
		value: js.Global().Get("WebSocket").New(url),
	}
	ret.value.Call("addEventListener", "message",
		js.FuncOf(func(this js.Value, args []js.Value) any {
			if ret.err != nil {
				return nil
			}
			data := args[0].Get("data")
			length := data.Get("length").Int()
			buf := make([]byte, length)
			js.CopyBytesToGo(buf, data)
			msg, err := capnp.Unmarshal(buf)
			if err != nil {
				close(ret.msgs)
				ret.err = err
				return nil
			}
			ret.msgs <- msg
			return nil
		}))
	return ret
}

func (c *Conn) Encode(ctx context.Context, msg *capnp.Message) error {
	if c.err != nil {
		return c.err
	}
	buf, err := msg.Marshal()
	if err != nil {
		return err
	}
	array := js.Global().Get("Uint8Array").New(len(buf))
	js.CopyBytesToJS(array, buf)
	c.value.Call("send", array)
	return nil
}

func (c *Conn) Decode(ctx context.Context) (*capnp.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-c.msgs:
		return msg, c.err
	}
}

func (c *Conn) Close() error {
	c.value.Call("close")
	return nil
}

func (*Conn) SetPartialWriteTimeout(time.Duration) {}
