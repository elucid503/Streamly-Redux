package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"streamly/internal/auth"
	"streamly/internal/catalog"
	"streamly/internal/config"
	"streamly/internal/history"
	"streamly/internal/proxy"
	"streamly/internal/resolve"
	"streamly/internal/room"
	"streamly/internal/sources/introdb"
	"streamly/internal/sources/showbox"
	"streamly/internal/sources/subdl"
	"streamly/internal/sources/tmdb"
	"streamly/internal/sources/tvmaze"
	"streamly/internal/sports"

	"github.com/gin-gonic/gin"
)

type api struct {
	cfg *config.Config

	auth *auth.Client
	hub  *room.Hub

	history *history.Store

	catalog  *catalog.Catalog
	resolver *resolve.Resolver

	showbox *showbox.Client
	tmdb    *tmdb.Client
	tvmaze  *tvmaze.Client
	sports  *sports.Client

	picksMu sync.Mutex
	picks   map[string]pickCache

	ticketsMu sync.Mutex
	tickets   map[string]socketTicket

	subdl   *subdl.Client
	introdb *introdb.Client
}

type socketTicket struct {
	accessToken string
	expires     time.Time
	user        auth.User
}

type pickCache struct {
	titles  []titleView
	expires time.Time
}

func (a *api) config(c *gin.Context) {

	c.JSON(http.StatusOK, gin.H{

		"clientId": a.cfg.DiscordClientID,

		"vodEnabled":       a.cfg.FebboxUICookie != "",
		"subtitlesEnabled": a.subdl.Configured(),
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

	ticketBytes := make([]byte, 32)

	if _, err := rand.Read(ticketBytes); err != nil {

		slog.Error("socket ticket generation failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "socket ticket generation failed"})
		return

	}

	ticket := hex.EncodeToString(ticketBytes)

	a.ticketsMu.Lock()
	a.tickets[ticket] = socketTicket{accessToken: token, expires: time.Now().Add(12 * time.Hour)}
	a.ticketsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{

		"accessToken":  token,
		"socketTicket": ticket,
	})

}

func (a *api) consumeSocketTicket(ticket string) (string, bool) {

	a.ticketsMu.Lock()

	entry, ok := a.tickets[ticket]

	if ok && time.Now().After(entry.expires) {

		delete(a.tickets, ticket)
		ok = false

	}

	a.ticketsMu.Unlock()

	if !ok {

		return "", false

	}

	return entry.accessToken, true

}

func (a *api) socketUser(ctx context.Context, ticket string) (auth.User, error) {

	a.ticketsMu.Lock()

	entry, ok := a.tickets[ticket]

	if ok && time.Now().After(entry.expires) {

		delete(a.tickets, ticket)
		ok = false

	}

	if ok && entry.user.ID != "" {

		a.ticketsMu.Unlock()
		return entry.user, nil

	}

	a.ticketsMu.Unlock()

	if !ok {

		return auth.User{}, fmt.Errorf("socket ticket is invalid")

	}

	user, err := a.auth.Me(ctx, entry.accessToken)

	if err != nil {

		return auth.User{}, err

	}

	a.ticketsMu.Lock()

	current, exists := a.tickets[ticket]

	if exists {

		current.user = user
		a.tickets[ticket] = current

	}

	a.ticketsMu.Unlock()

	return user, nil

}

func (a *api) roomAction(c *gin.Context) {

	var body struct {
		InstanceID string           `json:"instanceId"`
		GuildID    string           `json:"guildId"`
		Ticket     string           `json:"ticket"`
		Frame      room.ClientFrame `json:"frame"`
	}

	if err := c.ShouldBindJSON(&body); err != nil || body.InstanceID == "" || body.Ticket == "" {

		c.JSON(http.StatusBadRequest, gin.H{"error": "room action is invalid"})
		return

	}

	if body.Frame.Type != "control" && body.Frame.Type != "queue" {

		c.JSON(http.StatusBadRequest, gin.H{"error": "room action type is invalid"})
		return

	}

	user, err := a.socketUser(c.Request.Context(), body.Ticket)

	if err != nil {

		slog.Warn("room action authentication failed", "err", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "room authentication failed"})
		return

	}

	slog.Info("room action received", "room", body.InstanceID, "user", user.DisplayName, "type", body.Frame.Type, "action", body.Frame.Action)

	err = a.hub.HandleWithGuild(c.Request.Context(), body.InstanceID, body.GuildID, room.Participant{

		UserID: user.ID,
		Name:   user.DisplayName,
		Avatar: proxy.ImageURL(user.Avatar),
	}, body.Frame)

	if err != nil {

		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "room action failed"})
		return

	}

	c.Status(http.StatusNoContent)

}

func (a *api) historyResume(c *gin.Context) {

	guildID := strings.TrimSpace(c.Query("guildId"))
	ticket := c.Query("ticket")
	kind := c.Query("kind")
	id := c.Query("id")

	if guildID == "" || ticket == "" || id == "" {

		c.JSON(http.StatusBadRequest, gin.H{"error": "guildId, ticket, and id are required"})
		return

	}

	if _, err := a.socketUser(c.Request.Context(), ticket); err != nil {

		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
		return

	}

	if a.history == nil || kind != string(resolve.KindVOD) {

		c.JSON(http.StatusOK, gin.H{"resume": false})
		return

	}

	season, _ := strconv.Atoi(c.Query("season"))
	episode, _ := strconv.Atoi(c.Query("episode"))

	item := resolve.Item{

		Kind: resolve.KindVOD,
		ID: id,
		Season: season,
		Episode: episode,

	}

	positionMs, durationMs, ok := a.history.ResumePosition(c.Request.Context(), guildID, item)

	c.JSON(http.StatusOK, gin.H{

		"resume": ok,
		"positionMs": positionMs,
		"durationMs": durationMs,

	})

}

func (a *api) historyList(c *gin.Context) {

	guildID := strings.TrimSpace(c.Query("guildId"))
	ticket := c.Query("ticket")

	if guildID == "" || ticket == "" {

		c.JSON(http.StatusBadRequest, gin.H{"error": "guildId and ticket are required"})
		return

	}

	if _, err := a.socketUser(c.Request.Context(), ticket); err != nil {

		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
		return

	}

	if a.history == nil {

		c.JSON(http.StatusOK, gin.H{"items": []history.Entry{}})
		return

	}

	items, err := a.history.List(c.Request.Context(), guildID, history.MaxEntries)

	if err != nil {

		slog.Error("history list failed", "err", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "history unavailable"})
		return

	}

	c.JSON(http.StatusOK, gin.H{"items": items})

}

func (a *api) historyProgress(c *gin.Context) {

	var body struct {
		GuildID string       `json:"guildId"`
		Ticket  string       `json:"ticket"`
		Item    resolve.Item `json:"item"`

		PositionMs int64 `json:"positionMs"`
		DurationMs int64 `json:"durationMs"`
	}

	if err := c.ShouldBindJSON(&body); err != nil || body.GuildID == "" || body.Ticket == "" || body.Item.ID == "" {

		c.JSON(http.StatusBadRequest, gin.H{"error": "progress payload is invalid"})
		return

	}

	if _, err := a.socketUser(c.Request.Context(), body.Ticket); err != nil {

		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
		return

	}

	if a.history == nil {

		c.Status(http.StatusNoContent)
		return

	}

	if err := a.history.SaveProgress(c.Request.Context(), body.GuildID, body.Item, body.PositionMs, body.DurationMs); err != nil {

		slog.Warn("history progress failed", "err", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not save progress"})
		return

	}

	c.Status(http.StatusNoContent)

}

func (a *api) historyClearProgress(c *gin.Context) {

	var body struct {
		GuildID string       `json:"guildId"`
		Ticket  string       `json:"ticket"`
		Item    resolve.Item `json:"item"`
	}

	if err := c.ShouldBindJSON(&body); err != nil || body.GuildID == "" || body.Ticket == "" || body.Item.ID == "" {

		c.JSON(http.StatusBadRequest, gin.H{"error": "clear payload is invalid"})
		return

	}

	if _, err := a.socketUser(c.Request.Context(), body.Ticket); err != nil {

		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
		return

	}

	if a.history == nil {

		c.Status(http.StatusNoContent)
		return

	}

	if err := a.history.ClearProgress(c.Request.Context(), body.GuildID, body.Item); err != nil {

		slog.Warn("history clear failed", "err", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not clear progress"})
		return

	}

	c.Status(http.StatusNoContent)

}

func (a *api) roomState(c *gin.Context) {

	instanceID := c.Query("instanceId")
	ticket := c.Query("ticket")

	if instanceID == "" || ticket == "" {

		c.JSON(http.StatusBadRequest, gin.H{"error": "room query is invalid"})
		return

	}

	if _, err := a.socketUser(c.Request.Context(), ticket); err != nil {

		c.JSON(http.StatusUnauthorized, gin.H{"error": "room authentication failed"})
		return

	}

	state, participants := a.hub.Snapshot(instanceID)

	c.JSON(http.StatusOK, gin.H{

		"state":        state,
		"participants": participants,
		"serverTime":   time.Now().UnixMilli(),
	})

}

func (a *api) clientError(c *gin.Context) {

	var body struct {
		Message string `json:"message"`
		Stack   string `json:"stack"`
	}

	if err := c.ShouldBindJSON(&body); err != nil || body.Message == "" {

		c.Status(http.StatusBadRequest)
		return

	}

	if len(body.Message) > 2000 {

		body.Message = body.Message[:2000]

	}

	if len(body.Stack) > 12000 {

		body.Stack = body.Stack[:12000]

	}

	slog.Error("client react error", "message", body.Message, "stack", body.Stack)

	c.Status(http.StatusNoContent)

}

type channelView struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	Category string `json:"category,omitempty"`
	Country  string `json:"country,omitempty"`

	Logo string `json:"logo,omitempty"`

	Backups int `json:"backups"`
}

func (a *api) channels(c *gin.Context) {

	c.JSON(http.StatusOK, gin.H{

		"channels": channelViews(a.catalog.Channels()),
	})

}

// Direct resolve, outside any room. The room path never uses it; it exists so a channel can be checked on its own.
func (a *api) channelStream(c *gin.Context) {

	item := resolve.Item{

		Kind: resolve.KindChannel,
		ID:   c.Param("id"),
	}

	playback, err := a.resolver.Play(c.Request.Context(), item, 0, "")

	if err != nil {

		slog.Error("channel resolve failed", "channel", item.ID, "err", err)

		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channel is not available"})
		return

	}

	c.JSON(http.StatusOK, playback)

}

func channelViews(channels []catalog.Channel) []channelView {

	views := make([]channelView, 0, len(channels))

	for _, channel := range channels {

		views = append(views, channelView{

			ID:   channel.ID,
			Name: channel.Name,

			Category: category(channel.Categories),
			Country:  channel.Country,

			Logo: proxy.ImageURL(channel.Logo),

			Backups: len(channel.Sources) - 1,
		})

	}

	return views

}

// iptv-org categorises generously; the grid only has room for the primary one.
func category(categories []string) string {

	if len(categories) == 0 {

		return ""

	}

	primary := categories[0]

	return strings.ToUpper(primary[:1]) + primary[1:]

}

type sportsMatchView struct {

	ID string `json:"id"`
	Title string `json:"title"`
	Category string `json:"category"`
	League string `json:"league,omitempty"`

	HomeTeam string `json:"homeTeam,omitempty"`
	AwayTeam string `json:"awayTeam,omitempty"`
	HomeLogo string `json:"homeLogo,omitempty"`
	AwayLogo string `json:"awayLogo,omitempty"`

	HomeScore *int `json:"homeScore,omitempty"`
	AwayScore *int `json:"awayScore,omitempty"`
	StatusDetail string `json:"statusDetail,omitempty"`
	Status string `json:"status,omitempty"`

	StartsAt int64 `json:"startsAt"`
	Live bool `json:"live"`

	Broadcast string `json:"broadcast,omitempty"`
	Broadcasts []string `json:"broadcasts,omitempty"`

	Channel *matchedChannelView `json:"channel,omitempty"`

}

type matchedChannelView struct {

	ID string `json:"id"`
	Name string `json:"name"`
	Logo string `json:"logo,omitempty"`

}

func (a *api) sportsMatches(c *gin.Context) {

	matches, err := a.sports.Matches()

	if err != nil {

		slog.Error("sports matches failed", "err", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "sports feed unavailable"})
		return

	}

	views := make([]sportsMatchView, 0, len(matches))

	for _, match := range matches {

		views = append(views, sportsMatchViewFrom(match))

	}

	c.JSON(http.StatusOK, gin.H{"matches": views})

}

func sportsMatchViewFrom(match sports.Match) sportsMatchView {

	view := sportsMatchView{

		ID: match.ID,
		Title: match.Title,
		Category: match.Category,
		League: match.League,

		HomeScore: match.HomeScore,
		AwayScore: match.AwayScore,
		StatusDetail: match.StatusDetail,
		Status: match.Status,

		StartsAt: match.StartTime.Unix(),
		Live: match.Live,

		Broadcast: match.Broadcast,
		Broadcasts: append([]string(nil), match.Broadcasts...),

	}

	if match.HomeTeam != nil {

		view.HomeTeam = match.HomeTeam.Name
		view.HomeLogo = proxy.ImageURL(match.HomeTeam.Logo)

	}

	if match.AwayTeam != nil {

		view.AwayTeam = match.AwayTeam.Name
		view.AwayLogo = proxy.ImageURL(match.AwayTeam.Logo)

	}

	if match.Channel != nil {

		view.Channel = &matchedChannelView{

			ID: match.Channel.ID,
			Name: match.Channel.Name,
			Logo: proxy.ImageURL(match.Channel.Logo),

		}

	}

	return view

}
