package room

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"streamly/internal/auth"
	"streamly/internal/proxy"
	"streamly/internal/resolve"

	"github.com/gorilla/websocket"
)

// Discord's proxy drops sockets often enough that a short grace window is the difference between
// a reconnect and a wiped room.
const roomGrace = 90 * time.Second

type Authenticator interface {
	Me(ctx context.Context, accessToken string) (auth.User, error)
}

// Presence comes from live connections, not from the SDK participant list — the two can disagree (see _docs/DESIGN.md §3).
type Hub struct {
	resolver Resolver
	auth     Authenticator

	mu    sync.Mutex
	rooms map[string]*Room
}

func NewHub(resolver Resolver, authenticator Authenticator) *Hub {

	return &Hub{

		resolver: resolver,
		auth:     authenticator,

		rooms: map[string]*Room{},
	}

}

func (h *Hub) Attach(ctx context.Context, ws *websocket.Conn) {

	h.attach(ctx, ws, nil)

}

func (h *Hub) AttachAuthenticated(ctx context.Context, ws *websocket.Conn, instanceID string, accessToken string) {

	h.attach(ctx, ws, &ClientFrame{

		Type: "hello",

		InstanceID:  instanceID,
		AccessToken: accessToken,
	})

}

func (h *Hub) Handle(ctx context.Context, instanceID string, user Participant, frame ClientFrame) error {

	target := h.room(instanceID)

	actor := &Conn{

		user: user,
		room: target,
	}

	switch frame.Type {

	case "control":

		return target.control(ctx, actor, frame)

	case "queue":

		target.queueOp(ctx, actor, frame)
		return nil

	}

	return nil

}

// Snapshot never creates a room. Polling must not resurrect empty shells after a real close.
func (h *Hub) Snapshot(instanceID string) (State, []Participant) {

	h.mu.Lock()

	target, ok := h.rooms[instanceID]

	h.mu.Unlock()

	if !ok {

		return State{Queue: []resolve.Item{}}, nil

	}

	return target.snapshot()

}

func (h *Hub) attach(ctx context.Context, ws *websocket.Conn, hello *ClientFrame) {

	conn := newConn(ws)

	go conn.writePump()

	if hello != nil && !h.welcome(ctx, conn, *hello) {

		conn.close()
		return

	}

	conn.readPump(ctx, h)

	conn.close()

	if conn.room == nil {

		return

	}

	if conn.room.leave(conn) {

		h.scheduleRemove(conn.room.id)

	}

}

func (h *Hub) welcome(ctx context.Context, conn *Conn, frame ClientFrame) bool {

	if frame.InstanceID == "" || frame.AccessToken == "" {

		slog.Warn("websocket hello rejected", "instance", frame.InstanceID != "", "token", frame.AccessToken != "")

		return false

	}

	user, err := h.auth.Me(ctx, frame.AccessToken)

	if err != nil {

		slog.Warn("websocket authentication failed", "err", err)
		return false

	}

	conn.user = Participant{

		UserID: user.ID,
		Name:   user.DisplayName,

		// Discord's CDN is a third-party origin to the activity, so avatars route through the proxy like every other image (§2.1).
		Avatar: proxy.ImageURL(user.Avatar),
	}

	conn.room = h.room(frame.InstanceID)

	conn.room.join(conn)

	slog.Info("participant joined", "room", frame.InstanceID, "user", user.DisplayName)

	return true

}

func (h *Hub) room(id string) *Room {

	h.mu.Lock()

	defer h.mu.Unlock()

	if existing, ok := h.rooms[id]; ok {

		return existing

	}

	created := newRoom(id, h.resolver)

	h.rooms[id] = created

	return created

}

// Wait out brief disconnects so a reconnect reattaches to the same state instead of an empty room.
func (h *Hub) scheduleRemove(id string) {

	go func() {

		time.Sleep(roomGrace)

		h.mu.Lock()

		defer h.mu.Unlock()

		target, ok := h.rooms[id]

		if !ok || !target.empty() {

			return

		}

		delete(h.rooms, id)

		slog.Info("room closed", "room", id)

	}()

}

// Entry point for the proxy's upstream failure reports (§5.2).
func (h *Hub) SourceFailed(id string) {

	h.mu.Lock()

	target, ok := h.rooms[id]

	h.mu.Unlock()

	if !ok {

		return

	}

	target.Failover(context.Background())

}
