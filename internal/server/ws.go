package server

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Authentication travels in an opaque server ticket because Discord's proxy can delay browser frames after an upgrade.
var upgrader = websocket.Upgrader{

	ReadBufferSize:  4096,
	WriteBufferSize: 4096,

	CheckOrigin: func(*http.Request) bool { return true },
}

func (a *api) socket(c *gin.Context) {

	slog.Info("websocket upgrade requested", "origin", c.GetHeader("Origin"))

	instanceID := c.Query("instanceId")
	accessToken, validTicket := a.consumeSocketTicket(c.Query("ticket"))

	if instanceID == "" || !validTicket {

		slog.Warn("websocket upgrade rejected", "instance", instanceID != "", "ticket", validTicket)
		c.Status(http.StatusUnauthorized)
		return

	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)

	if err != nil {

		slog.Warn("websocket upgrade failed", "err", err)
		return

	}

	slog.Info("websocket connected")

	defer ws.Close()

	a.hub.AttachAuthenticated(c.Request.Context(), ws, instanceID, accessToken)

}
