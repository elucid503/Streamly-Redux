package room

import (
	"context"
	"testing"

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

	room := newRoom("instance", resolver)

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

func TestQueueOpsKeepIndexOnTheCurrentItem(t *testing.T) {

	room, conn, _ := testRoom()

	first := resolve.Item{Kind: resolve.KindVOD, ID: "a", Title: "A"}
	second := resolve.Item{Kind: resolve.KindVOD, ID: "b", Title: "B"}
	third := resolve.Item{Kind: resolve.KindVOD, ID: "c", Title: "C"}

	room.state.Queue = []resolve.Item{first, second, third}
	room.state.QueueIndex = 2
	room.state.Item = &third

	room.queueOp(context.Background(), conn, ClientFrame{Op: OpRemove, Index: 0})

	if len(room.state.Queue) != 2 || room.state.QueueIndex != 1 {

		t.Fatalf("queue was %d long at index %d, want 2 at 1", len(room.state.Queue), room.state.QueueIndex)

	}

	room.queueOp(context.Background(), conn, ClientFrame{Op: OpMove, Index: 1, To: 0})

	if room.state.Queue[0].ID != "c" {

		t.Fatalf("queue head was %q, want c", room.state.Queue[0].ID)

	}

}

func TestNextIsInertWhileAChannelPlays(t *testing.T) {

	room, conn, _ := testRoom()

	channel := resolve.Item{Kind: resolve.KindChannel, ID: "espn-us", Title: "ESPN"}
	queued := resolve.Item{Kind: resolve.KindVOD, ID: "a", Title: "A"}

	room.state.Item = &channel
	room.state.Queue = []resolve.Item{queued}
	room.state.QueueIndex = 0

	room.step(context.Background(), conn, 1)

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

func TestFailoverStopsAtTheLastSource(t *testing.T) {

	room, _, resolver := testRoom()

	channel := resolve.Item{Kind: resolve.KindChannel, ID: "espn-us", Title: "ESPN"}

	room.state.Item = &channel
	room.state.Playback = &resolve.Playback{SourceIndex: 1, SourceCount: 2}

	resolver.sourceIndex = -1

	room.Failover(context.Background())

	if resolver.sourceIndex != -1 {

		t.Fatal("failover tried to resolve past the last source")

	}

}
