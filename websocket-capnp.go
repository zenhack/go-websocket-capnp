// Package websocketcapnp integrates websockets with capnproto.
package websocketcapnp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"capnproto.org/go/capnp/v3"
	"capnproto.org/go/capnp/v3/rpc/transport"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

var (
	ErrUnexpectedText = errors.New("Unexpected websocket text frame (expected binary only)")
)

type websocketCodec struct {
	wlock   sync.Mutex // Must hold to write to rwc
	rwc     io.ReadWriteCloser
	handler wsutil.ControlHandler
}

func (c websocketCodec) Encode(msg *capnp.Message) error {
	c.wlock.Lock()
	defer c.wlock.Unlock()
	size, err := msg.TotalSize()
	if err != nil {
		return err
	}
	err = ws.WriteHeader(c.rwc, ws.Header{
		Fin:    true,
		OpCode: ws.OpBinary,
		Length: int64(size),
	})

	if err != nil {
		return err
	}
	_, err = msg.WriteTo(c.rwc)
	return err
}

func (c websocketCodec) Decode() (*capnp.Message, error) {
	for {

		hdr, err := ws.ReadHeader(c.rwc)
		if err != nil {
			return nil, err
		}
		switch hdr.OpCode {
		case ws.OpText:
			return nil, ErrUnexpectedText
		case ws.OpBinary:
			// TODO(perf): re-use buffers
			buf := make([]byte, hdr.Length)
			_, err := io.ReadFull(c.rwc, buf)
			if err != nil {
				return nil, err
			}
			fr := ws.UnmaskFrameInPlace(ws.Frame{
				Header:  hdr,
				Payload: buf,
			})
			return capnp.Unmarshal(fr.Payload)
		default:
			c.wlock.Lock()
			err = c.handler.Handle(hdr)
			c.wlock.Unlock()
			if err != nil {
				return nil, err
			}
		}
	}
}

// UpgradeHTTP upgrades an http request to a websocket connection, and returns
// a transport.Codec that sends capnp messages over it.
func UpgradeHTTP(
	up ws.HTTPUpgrader,
	req *http.Request,
	w http.ResponseWriter,
) (transport.Codec, error) {
	conn, bufRw, _, err := up.Upgrade(req, w)
	if err != nil {
		return nil, fmt.Errorf("Error upgrading websocket connection: %w", err)
	}
	if n := bufRw.Reader.Buffered(); n > 0 {
		return nil, fmt.Errorf("TODO: support buffered data on hijacked connection (%v bytes buffered)", n)
	}
	if err := bufRw.Writer.Flush(); err != nil {
		fmt.Errorf("Flush(): %w", err)
	}
	return NewCodec(conn, true), nil
}

func (p websocketCodec) Close() error {
	return p.rwc.Close()
}

func (websocketCodec) ReleaseMessage(msg *capnp.Message) {
}

// Return a transport.Codec that sends messages over the websocket connection.
// Sends each capnproto message in its own websocket binary message.
func NewCodec(rwc io.ReadWriteCloser, isServer bool) transport.Codec {
	ret := websocketCodec{
		rwc: rwc,
	}
	if isServer {
		ret.handler.State = ws.StateServerSide
	} else {
		ret.handler.State = ws.StateClientSide
	}
	ret.handler.Src = rwc
	ret.handler.Dst = rwc
	return ret
}

// Like NewCodec, but returns a transport.Transport.
func NewTransport(rwc io.ReadWriteCloser, isServer bool) transport.Transport {
	return transport.New(NewCodec(rwc, isServer))
}
