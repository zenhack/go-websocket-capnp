// Package websocketcapnp integrates websockets with capnproto.
package websocketcapnp

import (
	"context"
	"io"
	"time"

	"capnproto.org/go/capnp/v3"
	"capnproto.org/go/capnp/v3/rpc/transport"
	"github.com/gorilla/websocket"
)

type websocketCodec struct {
	conn *websocket.Conn
}

func (c websocketCodec) Encode(ctx context.Context, msg *capnp.Message) error {
	w, err := c.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return err
	}
	defer w.Close()
	return capnp.NewEncoder(w).Encode(msg)
}

func (c websocketCodec) Decode(ctx context.Context) (*capnp.Message, error) {
	var (
		typ int
		r   io.Reader
		err error
	)
	for ctx.Err() == nil && typ != websocket.BinaryMessage {
		typ, r, err = c.conn.NextReader()
		if err != nil {
			return nil, err
		}
		if typ == websocket.PingMessage {
			err = c.conn.WriteMessage(websocket.PongMessage, nil)
			if err != nil {
				return nil, err
			}
		}
	}
	return capnp.NewDecoder(r).Decode()
}

func (p websocketCodec) Close() error {
	return p.conn.Close()
}

func (websocketCodec) SetPartialWriteTimeout(time.Duration) {}

// Return a transport.Codec that sends messages over the websocket connection.
// Sends each capnproto message in its own websocket binary message.
func NewCodec(conn *websocket.Conn) transport.Codec {
	return websocketCodec{conn}
}

// Like NewCodec, but returns a transport.Transport.
func NewTransport(conn *websocket.Conn) transport.Transport {
	return transport.New(NewCodec(conn))
}
