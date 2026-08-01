package room

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"streamly/internal/resolve"
)

const playbackResolveTimeout = 40 * time.Second

type Resolver interface {
	Play(ctx context.Context, item resolve.Item, sourceIndex int, room string) (*resolve.Playback, error)
}

// Recorder persists recently played titles per Discord guild (optional).
type Recorder interface {
	Record(ctx context.Context, guildID string, item resolve.Item) error
}

// A room already dies when its last participant leaves, so none of this outlives the process (see _docs/DESIGN.md §2.4).
type Room struct {
	id string
	guildID string

	resolver Resolver
	history Recorder

	mu sync.Mutex

	state State
	conns map[*Conn]bool

	lastFailover time.Time
}

func newRoom(id string, resolver Resolver, history Recorder) *Room {

	return &Room{

		id: id,

		resolver: resolver,
		history: history,
		state: State{

			Queue: []resolve.Item{},
		},

		conns: map[*Conn]bool{},
	}

}

func (r *Room) setGuild(guildID string) {

	if guildID == "" {

		return

	}

	r.mu.Lock()

	r.guildID = guildID

	r.mu.Unlock()

}

func (r *Room) join(conn *Conn) {

	r.mu.Lock()

	r.conns[conn] = true

	state := r.state
	participants := r.participantList()

	r.mu.Unlock()

	conn.send(ServerFrame{

		Type: "welcome",

		State:        &state,
		Participants: participants,

		ServerTime: nowMs(),
	})

	r.broadcastParticipants()

	r.notice(conn, NoticeAction, conn.user.Name+" joined")

}

func (r *Room) leave(conn *Conn) bool {

	r.mu.Lock()

	delete(r.conns, conn)

	empty := len(r.conns) == 0

	r.mu.Unlock()

	if !empty {

		r.broadcastParticipants()

		r.notice(conn, NoticeAction, conn.user.Name+" left")

	}

	return empty

}

func (r *Room) empty() bool {

	r.mu.Lock()

	defer r.mu.Unlock()

	return len(r.conns) == 0

}

func (r *Room) control(ctx context.Context, conn *Conn, frame ClientFrame) error {

	switch frame.Action {

	case ActionSetItem:

		return r.setItem(ctx, conn, frame.Item, 0)

	case ActionNext:

		return r.step(ctx, conn, 1)

	case ActionPrev:

		return r.step(ctx, conn, -1)

	default:

		r.apply(conn, frame)
		return nil

	}

}

// Deliberate actions rewrite the anchor and propagate; buffering never reaches here (§4.1).
func (r *Room) apply(conn *Conn, frame ClientFrame) {

	r.mu.Lock()

	now := nowMs()

	switch frame.Action {

	case ActionPlay:

		r.state.Playing = true
		r.state.AnchorAt = now

	case ActionPause:

		r.state.AnchorMs = expected(r.state, now)
		r.state.Playing = false
		r.state.AnchorAt = now

	case ActionSeek:

		position := frame.PositionMs

		if position < 0 {

			position = 0

		}

		r.state.AnchorMs = position
		r.state.AnchorAt = now

	case ActionSetSubtitle:

		r.state.Subtitle = frame.Track

	default:

		r.mu.Unlock()
		return

	}

	r.state.LastActor = &Actor{

		UserID: conn.user.UserID,
		Name:   conn.user.Name,

		Action: frame.Action,
		At:     now,
	}

	state := r.state

	r.mu.Unlock()

	r.broadcast(ServerFrame{Type: "room", State: &state})

	r.notice(conn, NoticeAction, conn.user.Name+" "+phrase(frame.Action))

}

func (r *Room) setItem(ctx context.Context, conn *Conn, item *resolve.Item, sourceIndex int) error {

	// Tuning to a channel replaces what is playing; stopping it returns the room to the queue (§4.6).
	if item == nil {

		r.mu.Lock()

		queued := r.currentQueued()

		r.mu.Unlock()

		if queued == nil {

			r.clear(conn)
			return nil

		}

		item = queued

	}

	resolveCtx, cancel := context.WithTimeout(ctx, playbackResolveTimeout)
	defer cancel()

	playback, err := r.resolver.Play(resolveCtx, *item, sourceIndex, r.id)

	if err != nil {

		slog.Error("resolve failed", "room", r.id, "item", item.Key(), "err", err)

		if errors.Is(err, resolve.ErrProviderAuth) {

			r.notice(nil, NoticeError, "The Febbox session has expired — VOD is unavailable until it is refreshed")
			return err

		}

		r.notice(nil, NoticeError, "Could not start "+item.Title)

		return err

	}

	r.mu.Lock()

	now := nowMs()

	r.state.Item = item
	r.state.Playback = playback

	r.state.Playing = true

	r.state.AnchorMs = 0
	r.state.AnchorAt = now

	r.state.Subtitle = nil

	if item.Kind == resolve.KindVOD {

		r.alignQueue(*item)

	}

	if conn != nil {

		r.state.LastActor = &Actor{

			UserID: conn.user.UserID,
			Name:   conn.user.Name,

			Action: ActionSetItem,
			At:     now,
		}

	}

	state := r.state

	r.mu.Unlock()

	r.broadcast(ServerFrame{Type: "room", State: &state})

	if conn != nil {

		r.notice(conn, NoticeAction, conn.user.Name+" started "+item.Title)

	}

	r.recordHistory(*item)

	return nil

}

func (r *Room) recordHistory(item resolve.Item) {

	if r.history == nil {

		return

	}

	r.mu.Lock()

	guildID := r.guildID

	r.mu.Unlock()

	if guildID == "" {

		return

	}

	go func() {

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		defer cancel()

		if err := r.history.Record(ctx, guildID, item); err != nil {

			slog.Warn("history record failed", "guild", guildID, "item", item.Key(), "err", err)

		}

	}()

}

func (r *Room) clear(conn *Conn) {

	r.mu.Lock()

	r.state.Item = nil
	r.state.Playback = nil

	r.state.Playing = false

	r.state.AnchorMs = 0
	r.state.AnchorAt = nowMs()

	r.state.Subtitle = nil

	state := r.state

	r.mu.Unlock()

	r.broadcast(ServerFrame{Type: "room", State: &state})

	if conn != nil {

		r.notice(conn, NoticeAction, conn.user.Name+" stopped playback")

	}

}

// next and prev have no meaning while a channel is playing and are inert in that state (§4.6).
func (r *Room) step(ctx context.Context, conn *Conn, delta int) error {

	r.mu.Lock()

	if r.state.Item != nil && r.state.Item.Kind == resolve.KindChannel {

		r.mu.Unlock()
		return nil

	}

	target := r.state.QueueIndex + delta

	if target < 0 || target >= len(r.state.Queue) {

		r.mu.Unlock()
		return nil

	}

	r.state.QueueIndex = target

	item := r.state.Queue[target]

	r.mu.Unlock()

	return r.setItem(ctx, conn, &item, 0)

}

func (r *Room) snapshot() (State, []Participant) {

	r.mu.Lock()

	state := r.state
	state.Queue = append([]resolve.Item(nil), r.state.Queue...)

	participants := r.participantList()

	r.mu.Unlock()

	return state, participants

}

func (r *Room) queueOp(ctx context.Context, conn *Conn, frame ClientFrame) {

	r.mu.Lock()

	switch frame.Op {

	case OpAdd:

		// Channels are allowed so scheduled sports can be queued before tip-off;
		// they still bypass autoplay/next while playing (§4.6).
		if frame.Item == nil || (frame.Item.Kind != resolve.KindVOD && frame.Item.Kind != resolve.KindChannel) {

			r.mu.Unlock()
			return

		}

		// Refuse duplicates — play used to auto-append, and re-queueing the same
		// title would stack identical rows in the list.
		if queueContains(r.state.Queue, *frame.Item) {

			r.mu.Unlock()
			return

		}

		r.state.Queue = append(r.state.Queue, *frame.Item)

	case OpRemove:

		if frame.Index < 0 || frame.Index >= len(r.state.Queue) {

			r.mu.Unlock()
			return

		}

		r.state.Queue = append(r.state.Queue[:frame.Index], r.state.Queue[frame.Index+1:]...)

		if frame.Index < r.state.QueueIndex {

			r.state.QueueIndex--

		}

	case OpMove:

		if !inRange(frame.Index, len(r.state.Queue)) || !inRange(frame.To, len(r.state.Queue)) {

			r.mu.Unlock()
			return

		}

		moved := r.state.Queue[frame.Index]

		rest := append(r.state.Queue[:frame.Index:frame.Index], r.state.Queue[frame.Index+1:]...)

		r.state.Queue = append(rest[:frame.To], append([]resolve.Item{moved}, rest[frame.To:]...)...)

	default:

		r.mu.Unlock()
		return

	}

	empty := r.state.Item == nil

	next := r.currentQueued()

	state := r.state

	r.mu.Unlock()

	r.broadcast(ServerFrame{Type: "room", State: &state})

	if empty && next != nil && frame.Op == OpAdd {

		r.setItem(ctx, conn, next, 0)

	}

}

// The proxy's observation is authoritative and room-wide, so the switch happens once, here (§5.2).
func (r *Room) Failover(ctx context.Context) {

	r.mu.Lock()

	if r.state.Item == nil || r.state.Playback == nil || r.state.Item.Kind != resolve.KindChannel {

		r.mu.Unlock()
		return

	}

	if time.Since(r.lastFailover) < time.Minute {

		r.mu.Unlock()
		return

	}

	next := r.state.Playback.SourceIndex + 1

	if next >= r.state.Playback.SourceCount {

		r.mu.Unlock()
		return

	}

	r.lastFailover = time.Now()

	item := *r.state.Item

	r.mu.Unlock()

	slog.Warn("switching to backup source", "room", r.id, "channel", item.ID, "source", next)

	playback, err := r.resolver.Play(ctx, item, next, r.id)

	if err != nil {

		r.notice(nil, NoticeError, item.Title+" is unavailable")
		return

	}

	r.mu.Lock()

	r.state.Playback = playback
	r.state.AnchorAt = nowMs()

	state := r.state

	r.mu.Unlock()

	r.broadcast(ServerFrame{Type: "room", State: &state})

	r.notice(nil, NoticeFailover, "Switched to backup source")

}

// If the item is already queued, point the index at it. Play must not invent
// queue entries — only the explicit enqueue op adds items (§4.6).
func (r *Room) alignQueue(item resolve.Item) {

	if index, ok := queueIndexOf(r.state.Queue, item); ok {

		r.state.QueueIndex = index

	}

}

func queueContains(queue []resolve.Item, item resolve.Item) bool {

	_, ok := queueIndexOf(queue, item)

	return ok

}

func queueIndexOf(queue []resolve.Item, item resolve.Item) (int, bool) {

	key := item.Key()

	for index, queued := range queue {

		if queued.Key() == key {

			return index, true

		}

	}

	return -1, false

}

func (r *Room) currentQueued() *resolve.Item {

	if r.state.QueueIndex < 0 || r.state.QueueIndex >= len(r.state.Queue) {

		return nil

	}

	item := r.state.Queue[r.state.QueueIndex]

	return &item

}

func (r *Room) participantList() []Participant {

	participants := make([]Participant, 0, len(r.conns))

	seen := map[string]bool{}

	for conn := range r.conns {

		if seen[conn.user.UserID] {

			continue

		}

		seen[conn.user.UserID] = true

		participants = append(participants, conn.user)

	}

	return participants

}

func (r *Room) broadcastParticipants() {

	r.mu.Lock()

	participants := r.participantList()

	r.mu.Unlock()

	r.broadcast(ServerFrame{Type: "participants", Participants: participants})

}

func (r *Room) notice(from *Conn, kind string, text string) {

	r.broadcastExcept(from, ServerFrame{Type: "notice", Kind: kind, Text: text})

}

func (r *Room) broadcast(frame ServerFrame) {

	r.broadcastExcept(nil, frame)

}

func (r *Room) broadcastExcept(skip *Conn, frame ServerFrame) {

	payload, err := json.Marshal(frame)

	if err != nil {

		slog.Error("frame encode failed", "room", r.id, "type", frame.Type, "err", err)
		return

	}

	r.mu.Lock()

	targets := make([]*Conn, 0, len(r.conns))

	for conn := range r.conns {

		if conn != skip {

			targets = append(targets, conn)

		}

	}

	r.mu.Unlock()

	for _, conn := range targets {

		conn.sendRaw(payload)

	}

}

func expected(state State, now int64) int64 {

	if !state.Playing {

		return state.AnchorMs

	}

	return state.AnchorMs + (now - state.AnchorAt)

}

func inRange(index int, length int) bool {

	return index >= 0 && index < length

}

func nowMs() int64 {

	return time.Now().UnixMilli()

}

func phrase(action string) string {

	switch action {

	case ActionPlay:

		return "resumed playback"

	case ActionPause:

		return "paused"

	case ActionSeek:

		return "jumped to a new position"

	case ActionSetSubtitle:

		return "changed subtitles"

	}

	return action

}
