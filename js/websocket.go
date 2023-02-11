//go:build js

package js

import (
	"context"
	"syscall/js"

	"capnproto.org/go/capnp/v3"
	"capnproto.org/go/capnp/v3/exp/spsc"
	"capnproto.org/go/capnp/v3/rpc/transport"
)

var _ transport.Codec = &Conn{}

type Conn struct {
	value js.Value
	msgs  spsc.Queue[*capnp.Message]
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
		var jsProtos []any
		for _, p := range subprotocols {
			jsProtos = append(jsProtos, p)
		}
		value = websocketCls.New(url, jsProtos)
	}
	value.Set("binaryType", "arraybuffer")
	ret := &Conn{
		value: value,
		msgs:  spsc.New[*capnp.Message](),
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
				ret.err = err
				ret.msgs.Close()
				return nil
			}
			ret.msgs.Send(msg)
			return nil
		}))
	ret.value.Call("addEventListener", "error",
		js.FuncOf(func(this js.Value, args []js.Value) any {
			ret.err = websocketError{event: args[0]}
			ret.msgs.Close()
			return nil
		}))
	ret.value.Call("addEventListener", "open",
		js.FuncOf(func(this js.Value, args []js.Value) any {
			close(ret.ready)
			return nil
		}))
	return ret
}

func (c *Conn) Encode(msg *capnp.Message) error {
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

func (c *Conn) Decode() (*capnp.Message, error) {
	msg, _ := c.msgs.Recv(context.Background())
	return msg, c.err
}

func (c *Conn) Close() error {
	c.value.Call("close")
	return nil
}

func (c *Conn) ReleaseMessage(*capnp.Message) {
}
