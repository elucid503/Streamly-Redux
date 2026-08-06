package room

import (
	"context"
	"testing"
	"time"

	"streamly/internal/resolve"
)

type stubResolver struct {

	sourceIndex int
	fail bool

}

func (s *stubResolver) Play(_ context.Context, _ resolve.Item, sourceIndex int, _ string) (*resolve.Playback, error) {

	s.sourceIndex = sourceIndex

	return &resolve.Playback{

		Kind: "hls",
		URL: "/stream",

		SourceIndex: sourceIndex,
		SourceCount: 2,

	}, nil

}

func testRoom() (*Room, *Conn, *stubResolver) {

	resolver := &stubResolver{}

	room := newRoom("instance", resolver, nil)

	conn := &Conn{

		user: Participant{UserID: "1", Name: "Ada"},

		out: make(chan []byte, 64),

	}

	room.conns[conn] = true

	return room, conn, resolver

}

func TestPauseCapturesElapsedPosition(t *testing.T) {

	room, conn, _ := testRoom()

	room.state.Playing = true
	room.state.AnchorMs = 1000
	room.state.AnchorAt = nowMs() - 5000

	room.apply(conn, ClientFrame{Action: ActionPause})

	if room.state.Playing {

		t.Fatal("pause left the room playing")

	}

	if room.state.AnchorMs < 5500 || room.state.AnchorMs > 6500 {

		t.Fatalf("anchor after pause was %d, want about 6000", room.state.AnchorMs)

	}

}

func TestSeekRewritesAnchor(t *testing.T) {

	room, conn, _ := testRoom()

	room.apply(conn, ClientFrame{Action: ActionSeek, PositionMs: 42000})

	if room.state.AnchorMs != 42000 {

		t.Fatalf("anchor was %d, want 42000", room.state.AnchorMs)

	}

	if room.state.LastActor == nil || room.state.LastActor.Action != ActionSeek {

		t.Fatal("seek was not attributed")

	}

}

func TestQueueOpsKeepCursorOnUpcomingItem(t *testing.T) {

	room, conn, _ := testRoom()

	first := resolve.Item{Kind: resolve.KindVOD, ID: "a", Title: "A"}
	second := resolve.Item{Kind: resolve.KindVOD, ID: "b", Title: "B"}
	third := resolve.Item{Kind: resolve.KindVOD, ID: "c", Title: "C"}

	// Currently playing something else; queue is pure up-next with cursor on C.
	room.state.Queue = []resolve.Item{first, second, third}
	room.state.QueueIndex = 2
	room.state.Item = &resolve.Item{Kind: resolve.KindVOD, ID: "now", Title: "Now"}

	room.queueOp(context.Background(), conn, ClientFrame{Op: OpRemove, Index: 0})

	if len(room.state.Queue) != 2 || room.state.QueueIndex != 1 || room.state.Queue[1].ID != "c" {

		t.Fatalf("queue was %d long at index %d, want 2 at 1 (c)", len(room.state.Queue), room.state.QueueIndex)

	}

	room.queueOp(context.Background(), conn, ClientFrame{Op: OpMove, Index: 1, To: 0})

	if room.state.Queue[0].ID != "c" {

		t.Fatalf("queue head was %q, want c", room.state.Queue[0].ID)

	}

	if room.state.QueueIndex != 0 {

		t.Fatalf("queue index after move was %d, want 0 (still on c)", room.state.QueueIndex)

	}

}

func TestQueueRemoveClampsWhenCursorRowIsDeleted(t *testing.T) {

	room, conn, _ := testRoom()

	first := resolve.Item{Kind: resolve.KindVOD, ID: "a", Title: "A"}
	second := resolve.Item{Kind: resolve.KindVOD, ID: "b", Title: "B"}

	room.state.Queue = []resolve.Item{first, second}
	room.state.QueueIndex = 1

	room.queueOp(context.Background(), conn, ClientFrame{Op: OpRemove, Index: 1})

	if len(room.state.Queue) != 1 {

		t.Fatalf("queue length was %d, want 1", len(room.state.Queue))

	}

	if room.state.QueueIndex != 0 {

		t.Fatalf("queue index was %d after removing the last row, want 0", room.state.QueueIndex)

	}

}

func TestQueueCursorSurvivesChannelWhileReorder(t *testing.T) {

	room, conn, _ := testRoom()

	first := resolve.Item{Kind: resolve.KindVOD, ID: "a", Title: "A"}
	second := resolve.Item{Kind: resolve.KindVOD, ID: "b", Title: "B"}
	third := resolve.Item{Kind: resolve.KindVOD, ID: "c", Title: "C"}
	channel := resolve.Item{Kind: resolve.KindChannel, ID: "espn", Title: "ESPN"}

	room.state.Queue = []resolve.Item{first, second, third}
	room.state.QueueIndex = 2
	room.state.Item = &channel

	room.queueOp(context.Background(), conn, ClientFrame{Op: OpRemove, Index: 0})

	if room.state.QueueIndex != 1 || room.state.Queue[room.state.QueueIndex].ID != "c" {

		t.Fatalf("resume cursor drifted to index %d id %q, want c at 1", room.state.QueueIndex, room.state.Queue[room.state.QueueIndex].ID)

	}

}

func TestSetItemConsumesQueueEntry(t *testing.T) {

	room, conn, _ := testRoom()

	first := resolve.Item{Kind: resolve.KindVOD, ID: "a", Title: "A"}
	second := resolve.Item{Kind: resolve.KindVOD, ID: "b", Title: "B"}

	room.state.Queue = []resolve.Item{first, second}
	room.state.QueueIndex = 0

	if err := room.setItem(context.Background(), conn, &second, 0); err != nil {

		t.Fatalf("setItem: %v", err)

	}

	if len(room.state.Queue) != 1 || room.state.Queue[0].ID != "a" {

		t.Fatalf("queue after play was %+v, want only a remaining", room.state.Queue)

	}

	if room.state.Item == nil || room.state.Item.ID != "b" {

		t.Fatalf("item was %+v, want b", room.state.Item)

	}

}

func TestConcurrentNextFromSameItemAdvancesOnce(t *testing.T) {

	room, conn, _ := testRoom()

	first := resolve.Item{Kind: resolve.KindVOD, ID: "a", Title: "A"}
	second := resolve.Item{Kind: resolve.KindVOD, ID: "b", Title: "B"}
	third := resolve.Item{Kind: resolve.KindVOD, ID: "c", Title: "C"}

	// A already started and was consumed; B and C remain up next.
	room.state.Queue = []resolve.Item{second, third}
	room.state.QueueIndex = 0
	room.state.Item = &first

	if err := room.step(context.Background(), conn, 1, &first); err != nil {

		t.Fatalf("first next: %v", err)

	}

	if err := room.step(context.Background(), conn, 1, &first); err != nil {

		t.Fatalf("second next: %v", err)

	}

	if room.state.Item == nil || room.state.Item.ID != "b" {

		t.Fatalf("item was %+v, want b after concurrent next-from-a", room.state.Item)

	}

	if len(room.state.Queue) != 1 || room.state.Queue[0].ID != "c" {

		t.Fatalf("queue was %+v, want only c remaining", room.state.Queue)

	}

}

func TestSetItemDoesNotAppendToQueue(t *testing.T) {

	room, conn, _ := testRoom()

	existing := resolve.Item{Kind: resolve.KindVOD, ID: "queued", Title: "Queued"}
	playing := resolve.Item{Kind: resolve.KindVOD, ID: "playing", Title: "Playing"}

	room.state.Queue = []resolve.Item{existing}
	room.state.QueueIndex = 0

	if err := room.setItem(context.Background(), conn, &playing, 0); err != nil {

		t.Fatalf("setItem: %v", err)

	}

	if len(room.state.Queue) != 1 || room.state.Queue[0].ID != "queued" {

		t.Fatalf("queue was modified: %+v", room.state.Queue)

	}

	if room.state.Item == nil || room.state.Item.ID != "playing" {

		t.Fatalf("item was %+v, want playing", room.state.Item)

	}

}

func TestSetItemOnEmptyQueueStaysEmpty(t *testing.T) {

	room, conn, _ := testRoom()

	playing := resolve.Item{Kind: resolve.KindVOD, ID: "solo", Title: "Solo"}

	if err := room.setItem(context.Background(), conn, &playing, 0); err != nil {

		t.Fatalf("setItem: %v", err)

	}

	if len(room.state.Queue) != 0 {

		t.Fatalf("play leaked into the queue: %+v", room.state.Queue)

	}

}

func TestQueueAddRejectsDuplicates(t *testing.T) {

	room, conn, _ := testRoom()

	item := resolve.Item{Kind: resolve.KindVOD, ID: "a", Title: "A"}

	room.state.Queue = []resolve.Item{item}
	room.state.QueueIndex = 0

	room.queueOp(context.Background(), conn, ClientFrame{Op: OpAdd, Item: &item})

	if len(room.state.Queue) != 1 {

		t.Fatalf("queue grew to %d, want 1", len(room.state.Queue))

	}

}

func TestEmptyQueueAddAutoplaysAndConsumes(t *testing.T) {

	room, conn, _ := testRoom()

	item := resolve.Item{Kind: resolve.KindVOD, ID: "a", Title: "A"}

	room.queueOp(context.Background(), conn, ClientFrame{Op: OpAdd, Item: &item})

	if room.state.Item == nil || room.state.Item.ID != "a" {

		t.Fatalf("item was %+v, want a autoplayed", room.state.Item)

	}

	if len(room.state.Queue) != 0 {

		t.Fatalf("playing item stayed in queue: %+v", room.state.Queue)

	}

}

func TestNextIsInertWhileAChannelPlays(t *testing.T) {

	room, conn, _ := testRoom()

	channel := resolve.Item{Kind: resolve.KindChannel, ID: "espn-us", Title: "ESPN"}
	queued := resolve.Item{Kind: resolve.KindVOD, ID: "a", Title: "A"}

	room.state.Item = &channel
	room.state.Queue = []resolve.Item{queued}
	room.state.QueueIndex = 0

	room.step(context.Background(), conn, 1, nil)

	if room.state.Item.ID != "espn-us" {

		t.Fatalf("next changed the item to %q while a channel was playing", room.state.Item.ID)

	}

}

func TestFailoverAdvancesToTheNextSource(t *testing.T) {

	room, _, resolver := testRoom()

	channel := resolve.Item{Kind: resolve.KindChannel, ID: "espn-us", Title: "ESPN"}

	room.state.Item = &channel
	room.state.Playback = &resolve.Playback{SourceIndex: 0, SourceCount: 2}

	room.Failover(context.Background())

	if resolver.sourceIndex != 1 {

		t.Fatalf("failover resolved source %d, want 1", resolver.sourceIndex)

	}

	if room.state.Playback.SourceIndex != 1 {

		t.Fatalf("playback stayed on source %d", room.state.Playback.SourceIndex)

	}

}

func TestFailoverWrapsFromLastSourceToFirst(t *testing.T) {

	room, _, resolver := testRoom()

	channel := resolve.Item{Kind: resolve.KindChannel, ID: "espn-us", Title: "ESPN"}

	room.state.Item = &channel
	room.state.Playback = &resolve.Playback{SourceIndex: 1, SourceCount: 2}

	// Pretend the first switch already opened the cycle window so cooldown is the only gate.
	room.lastFailover = time.Time{}
	room.failoverCycleStart = time.Now()

	room.Failover(context.Background())

	if resolver.sourceIndex != 0 {

		t.Fatalf("failover resolved source %d, want 0 (wrap)", resolver.sourceIndex)

	}

	if room.state.Playback.SourceIndex != 0 {

		t.Fatalf("playback stayed on source %d, want 0", room.state.Playback.SourceIndex)

	}

}

func TestFailoverStopsWrappingAfterCycleTimeout(t *testing.T) {

	room, _, resolver := testRoom()

	channel := resolve.Item{Kind: resolve.KindChannel, ID: "espn-us", Title: "ESPN"}

	room.state.Item = &channel
	room.state.Playback = &resolve.Playback{SourceIndex: 1, SourceCount: 2}

	room.failoverCycleStart = time.Now().Add(-maxFailoverCycle - time.Second)
	resolver.sourceIndex = -1

	room.Failover(context.Background())

	if resolver.sourceIndex != -1 {

		t.Fatal("failover wrapped after the cycle timeout")

	}

}
