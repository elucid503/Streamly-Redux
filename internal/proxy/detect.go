package proxy

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// A dying source fails on many segments at once; one report per window is enough to move the room along.
const failureWindow = 20 * time.Second

// The proxy sees upstream failure directly, which makes the observation authoritative and room-wide (see _docs/DESIGN.md §5.2).
func (h *Handler) reportFailure(c *gin.Context, room string, target string) {

	if h.onFailure == nil || room == "" {

		return

	}

	// A viewer navigating away cancels in-flight segments; that is not an upstream fault.
	if c.Request.Context().Err() != nil {

		return

	}

	h.mu.Lock()

	if time.Since(h.lastFailure[room]) < failureWindow {

		h.mu.Unlock()
		return

	}

	h.lastFailure[room] = time.Now()

	h.mu.Unlock()

	slog.Warn("upstream failure reported to room", "room", room, "target", target)

	go h.onFailure(room, target)

}
