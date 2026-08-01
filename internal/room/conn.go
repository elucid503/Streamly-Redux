package room

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeTimeout = 10 * time.Second
	readTimeout  = 90 * time.Second

	keepAliveInterval = 25 * time.Second

	maxFrameBytes = 64 << 10
	sendBuffer    = 64
)

type Conn struct {
	ws *websocket.Conn

	user Participant
	room *Room

	out chan []byte

	closeOnce sync.Once
}

func newConn(ws *websocket.Conn) *Conn {

	return &Conn{

		ws: ws,

		out: make(chan []byte, sendBuffer),
	}

}

func (c *Conn) send(frame ServerFrame) {

	payload, err := json.Marshal(frame)

	if err != nil {

		slog.Error("frame encode failed", "type", frame.Type, "err", err)
		return

	}

	c.sendRaw(payload)

}

// A backed-up client loses frames, not its seat. Dropping them used to close the room for everyone.
func (c *Conn) sendRaw(payload []byte) {

	select {

	case c.out <- payload:

	default:

		slog.Warn("dropping websocket frame for slow client")

	}

}

func (c *Conn) close() {

	c.closeOnce.Do(func() {

		close(c.out)
		_ = c.ws.Close()

	})

}

func (c *Conn) writePump() {

	ticker := time.NewTicker(keepAliveInterval)

	defer ticker.Stop()

	for {

		select {

		case payload, open := <-c.out:

			if !open {

				_ = c.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(writeTimeout))
				_ = c.ws.Close()
				return

			}

			_ = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))

			if err := c.ws.WriteMessage(websocket.TextMessage, payload); err != nil {

				_ = c.ws.Close()
				return

			}

		case <-ticker.C:

			_ = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))

			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {

				_ = c.ws.Close()
				return

			}

		}

	}

}

func (c *Conn) readPump(ctx context.Context, hub *Hub) {

	c.ws.SetReadLimit(maxFrameBytes)

	_ = c.ws.SetReadDeadline(time.Now().Add(readTimeout))

	c.ws.SetPongHandler(func(string) error {

		return c.ws.SetReadDeadline(time.Now().Add(readTimeout))

	})

	for {

		_, payload, err := c.ws.ReadMessage()

		if err != nil {

			return

		}

		_ = c.ws.SetReadDeadline(time.Now().Add(readTimeout))

		var frame ClientFrame

		if err := json.Unmarshal(payload, &frame); err != nil {

			slog.Debug("unreadable client frame", "err", err)
			continue

		}

		if c.room == nil {

			if frame.Type != "hello" || !hub.welcome(ctx, c, frame) {

				return

			}

			continue

		}

		c.handle(ctx, frame)

	}

}

func (c *Conn) handle(ctx context.Context, frame ClientFrame) {

	switch frame.Type {

	case "ping":

		c.send(ServerFrame{Type: "pong", T0: frame.T0, T1: nowMs()})

	case "control":

		_ = c.room.control(ctx, c, frame)

	case "queue":

		c.room.queueOp(ctx, c, frame)

	}

}
