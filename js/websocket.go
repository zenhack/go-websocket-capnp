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
	ready chan struct{}
	err   error
}

type websocketError struct {
	event js.Value
}

func newUint8Array(args ...any) js.Value {
	return js.Global().Get("Uint8Array").New(args...)
}

func (e websocketError) Error() string {
	return "Websocket Error: " + e.event.Get("type").String()
}

func New(url string, subprotocols []string) *Conn {
	websocketCls := js.Global().Get("WebSocket")
	var value js.Value
	if subprotocols == nil {
		value = websocketCls.New(url)
	} else {
		value = websocketCls.New(url, subprotocols)
	}
	value.Set("binaryType", "arraybuffer")
	ret := &Conn{
		value: value,
		msgs:  make(chan *capnp.Message),
		ready: make(chan struct{}),
	}
	ret.value.Call("addEventListener", "message",
		js.FuncOf(func(this js.Value, args []js.Value) any {
			if ret.err != nil {
				return nil
			}
			data := newUint8Array(args[0].Get("data"))
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
	ret.value.Call("addEventListener", "error",
		js.FuncOf(func(this js.Value, args []js.Value) any {
			ret.err = websocketError{event: args[0]}
			close(ret.msgs)
			return nil
		}))
	ret.value.Call("addEventListener", "open",
		js.FuncOf(func(this js.Value, args []js.Value) any {
			close(ret.ready)
			return nil
		}))
	return ret
}

func (c *Conn) Encode(ctx context.Context, msg *capnp.Message) error {
	<-c.ready
	if c.err != nil {
		return c.err
	}
	buf, err := msg.Marshal()
	if err != nil {
		return err
	}
	array := newUint8Array(len(buf))
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
