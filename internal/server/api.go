package server

import (
	"errors"
	"log/slog"
	"net/http"

	"streamly/internal/auth"
	"streamly/internal/config"
	"streamly/internal/proxy"
	"streamly/internal/sources/daddylive"

	"github.com/gin-gonic/gin"
)

type api struct {

	cfg *config.Config

	auth *auth.Client
	live *daddylive.Client

}

func (a *api) config(c *gin.Context) {

	c.JSON(http.StatusOK, gin.H{

		"clientId": a.cfg.DiscordClientID,

	})

}

func (a *api) token(c *gin.Context) {

	var body struct {

		Code string `json:"code"`

	}

	if err := c.ShouldBindJSON(&body); err != nil || body.Code == "" {

		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return

	}

	token, err := a.auth.Exchange(c.Request.Context(), body.Code)

	if err != nil {

		slog.Error("token exchange failed", "err", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "token exchange failed"})
		return

	}

	slog.Info("token exchanged")

	c.JSON(http.StatusOK, gin.H{

		"accessToken": token,

	})

}

func (a *api) stream(c *gin.Context) {

	channelID := c.Param("id")

	stream, err := a.live.Resolve(c.Request.Context(), channelID)

	if err != nil {

		slog.Error("stream resolve failed", "channel", channelID, "err", err)

		if errors.Is(err, daddylive.ErrChannelOffline) {

			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channel is not broadcasting"})
			return

		}

		c.JSON(http.StatusBadGateway, gin.H{"error": "could not resolve stream"})
		return

	}

	c.JSON(http.StatusOK, gin.H{

		"url": proxy.MediaURL(stream.URL, "daddylive", stream.Referer),

	})

}
