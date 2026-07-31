package server

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"streamly/internal/auth"
	"streamly/internal/config"
	"streamly/internal/proxy"
	"streamly/internal/sources/daddylive"

	"github.com/gin-gonic/gin"
)

func Run(cfg *config.Config) error {

	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	engine.Use(gin.Recovery())

	routes := &api{

		cfg: cfg,

		auth: auth.New(cfg.DiscordClientID, cfg.DiscordClientSecret),
		live: daddylive.New(),

	}

	media := proxy.New(cfg.AllowAnyOrigin)

	group := engine.Group("/api", logRequests())

	group.GET("/config", routes.config)
	group.POST("/token", routes.token)
	group.GET("/stream/:id", routes.stream)

	engine.GET("/proxy/media", media.OriginCheck(), media.Media)

	serveSPA(engine, cfg.StaticDir)

	server := &http.Server{

		Addr: cfg.ListenAddr,
		Handler: stripProxyPrefix(engine),

	}

	slog.Info("listening", "addr", cfg.ListenAddr, "static", cfg.StaticDir, "allowAnyOrigin", cfg.AllowAnyOrigin)

	return server.ListenAndServe()

}

// Media requests are far too frequent to log, so this is scoped to the API group only.
func logRequests() gin.HandlerFunc {

	return func(c *gin.Context) {

		c.Next()

		slog.Info("api", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status())

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
