package server

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"streamly/internal/auth"
	"streamly/internal/catalog"
	"streamly/internal/config"
	"streamly/internal/history"
	"streamly/internal/proxy"
	"streamly/internal/resolve"
	"streamly/internal/room"
	"streamly/internal/sources/daddylive"
	"streamly/internal/sources/febbox"
	"streamly/internal/sources/introdb"
	"streamly/internal/sources/ntv"
	"streamly/internal/sources/showbox"
	"streamly/internal/sources/subdl"
	"streamly/internal/sources/tmdb"
	"streamly/internal/sources/tvmaze"
	"streamly/internal/sports"

	"github.com/gin-gonic/gin"
)

func Run(cfg *config.Config) error {

	live := daddylive.New()

	channels := catalog.New(live)

	// An unreachable upstream must not stop the binary booting; the catalog simply starts empty and fills on the next pass.
	if err := channels.Refresh(context.Background()); err != nil {

		slog.Error("initial catalog build failed", "err", err)

	}

	go channels.Watch(context.Background())

	authenticator := auth.New(cfg.DiscordClientID, cfg.DiscordClientSecret)

	box := showbox.New()
	files := febbox.New(cfg.FebboxUICookie)

	resolver := resolve.New(channels, live, ntv.New(), box, files)

	var historyStore *history.Store

	if cfg.MongoURI != "" {

		store, err := history.Open(context.Background(), cfg.MongoURI)

		if err != nil {

			slog.Error("mongo history unavailable", "err", err)

		} else {

			historyStore = store
			slog.Info("mongo history connected")

		}

	} else {

		slog.Warn("MONGO_URI not set — per-server history disabled")

	}

	var historyRecorder room.Recorder

	if historyStore != nil {

		historyRecorder = historyStore

	}

	hub := room.NewHub(resolver, authenticator, historyRecorder)

	media := proxy.New(cfg.AllowAnyOrigin)

	media.OnFailure(func(id string, _ string) {

		hub.SourceFailed(id)

	})

	routes := &api{

		cfg: cfg,

		auth: authenticator,
		hub:  hub,

		history: historyStore,

		catalog:  channels,
		resolver: resolver,

		showbox: box,
		tmdb:    tmdb.New(cfg.TMDBAPIKey),
		tvmaze:  tvmaze.New(),
		sports:  sports.New(channels),

		subdl:   subdl.New(cfg.SubdlAPIKey),
		introdb: introdb.New(cfg.IntroDBToken),

		tickets: map[string]socketTicket{},
	}

	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	engine.Use(gin.Recovery())

	group := engine.Group("/api", logRequests())

	group.GET("/config", routes.config)
	group.POST("/token", routes.token)

	group.GET("/channels", routes.channels)
	group.GET("/channels/:id/stream", routes.channelStream)
	group.GET("/sports", routes.sportsMatches)

	group.GET("/search", routes.search)
	group.GET("/trending", routes.trending)
	group.GET("/title/:boxType/:id", routes.title)

	group.GET("/subtitles", routes.subtitles)
	group.GET("/subtitle", routes.subtitle)
	group.GET("/intro", routes.intro)
	group.GET("/room", routes.roomState)
	group.POST("/room", routes.roomAction)

	group.GET("/history", routes.historyList)
	group.GET("/history/resume", routes.historyResume)
	group.POST("/history/progress", routes.historyProgress)
	group.POST("/history/clear-progress", routes.historyClearProgress)

	group.POST("/client-error", routes.clientError)

	engine.GET("/ws", routes.socket)

	engine.GET("/proxy/media", media.OriginCheck(), media.Media)
	engine.GET("/proxy/image", media.OriginCheck(), media.Image)
	// Google Fonts CSS + gstatic files — blocked by the activity CSP at their origin (§2.1).
	engine.GET("/proxy/fonts/css", media.OriginCheck(), media.FontCSS)
	engine.GET("/proxy/fonts/file", media.OriginCheck(), media.FontFile)

	serveSPA(engine, cfg.StaticDir)

	server := &http.Server{

		Addr:    cfg.ListenAddr,
		Handler: stripProxyPrefix(engine),
	}

	slog.Info("listening", "addr", cfg.ListenAddr, "static", cfg.StaticDir, "channels", len(channels.Channels()), "vod", cfg.FebboxUICookie != "", "curation", cfg.TMDBAPIKey != "", "subtitles", cfg.SubdlAPIKey != "")

	return server.ListenAndServe()

}

// Media requests are far too frequent to log, so this is scoped to the API group only.
func logRequests() gin.HandlerFunc {

	return func(c *gin.Context) {

		c.Next()

		if c.Request.Method != http.MethodGet || c.Request.URL.Path != "/api/room" {

			slog.Info("api", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status())

		}

	}

}

// Gin matches routes before middleware runs, so the prefix has to come off above the engine.
func stripProxyPrefix(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if strings.HasPrefix(r.URL.Path, proxy.PathPrefix+"/") {

			r.URL.Path = strings.TrimPrefix(r.URL.Path, proxy.PathPrefix)

		}

		next.ServeHTTP(w, r)

	})

}

func serveSPA(engine *gin.Engine, dir string) {

	engine.Static("/assets", filepath.Join(dir, "assets"))

	engine.NoRoute(func(c *gin.Context) {

		path := c.Request.URL.Path

		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/proxy/") {

			c.Status(http.StatusNotFound)
			return

		}

		c.File(filepath.Join(dir, "index.html"))

	})

}
