// Package bot keeps the Discord application online and handles the /launch slash command.
package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	apiBase = "https://discord.com/api/v10"

	opDispatch       = 0
	opHeartbeat      = 1
	opIdentify       = 2
	opReconnect      = 7
	opInvalidSession = 9
	opHello          = 10
	opHeartbeatACK   = 11

	interactionPing               = 1
	interactionApplicationCommand = 2

	callbackPong           = 1
	callbackLaunchActivity = 12

	launchCommand = "launch"
)

// Start connects the bot user and keeps it online until ctx is cancelled.
// Missing token is a no-op so local boots without BOT_TOKEN still work.
func Start(ctx context.Context, token string, applicationID string) {

	token = strings.TrimSpace(token)
	applicationID = strings.TrimSpace(applicationID)

	if token == "" {

		slog.Warn("BOT_TOKEN not set — Discord bot stays offline")
		return

	}

	if applicationID == "" {

		slog.Warn("DISCORD_CLIENT_ID missing — cannot register /launch")
		return

	}

	go run(ctx, token, applicationID)

}

func run(ctx context.Context, token string, applicationID string) {

	backoff := time.Second

	for {

		if ctx.Err() != nil {

			return

		}

		err := session(ctx, token, applicationID)

		if ctx.Err() != nil {

			return

		}

		if err != nil {

			slog.Error("discord bot session ended", "err", err)

		} else {

			slog.Warn("discord bot session closed")

		}

		select {

		case <-ctx.Done():

			return

		case <-time.After(backoff):

		}

		if backoff < 30*time.Second {

			backoff *= 2

		}

	}

}

func session(ctx context.Context, token string, applicationID string) error {

	gatewayURL, err := gatewayURL(ctx)

	if err != nil {

		return err

	}

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}

	conn, _, err := dialer.DialContext(ctx, gatewayURL+"?v=10&encoding=json", nil)

	if err != nil {

		return fmt.Errorf("gateway dial: %w", err)

	}

	defer conn.Close()

	var (
		seq  atomic.Int64
		writeMu sync.Mutex
		identified bool
	)

	seq.Store(-1)

	heartbeatStop := make(chan struct{})
	defer close(heartbeatStop)

	send := func(v any) error {

		writeMu.Lock()
		defer writeMu.Unlock()

		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteJSON(v)

	}

	for {

		if err := ctx.Err(); err != nil {

			return err

		}

		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		_, payload, err := conn.ReadMessage()

		if err != nil {

			return fmt.Errorf("gateway read: %w", err)

		}

		var envelope gatewayPayload

		if err := json.Unmarshal(payload, &envelope); err != nil {

			slog.Warn("discord gateway bad frame", "err", err)
			continue

		}

		if envelope.S != nil {

			seq.Store(*envelope.S)

		}

		switch envelope.Op {

		case opHello:

			var hello helloData

			if err := json.Unmarshal(envelope.D, &hello); err != nil {

				return fmt.Errorf("hello decode: %w", err)

			}

			interval := time.Duration(hello.HeartbeatInterval) * time.Millisecond
			go heartbeat(send, &seq, interval, heartbeatStop)

			if err := send(gatewayPayload{

				Op: opIdentify,
				D: mustJSON(identifyPayload{

					Token:   token,
					Intents: 0,
					Properties: identifyProperties{

						OS:      "linux",
						Browser: "streamly",
						Device:  "streamly",

					},
					Presence: &presencePayload{

						Status: "online",
						AFK:    false,

					},

				}),

			}); err != nil {

				return fmt.Errorf("identify: %w", err)

			}

		case opHeartbeat:

			if err := send(heartbeatPayload(&seq)); err != nil {

				return fmt.Errorf("heartbeat reply: %w", err)

			}

		case opHeartbeatACK:

			// nothing

		case opReconnect:

			return fmt.Errorf("gateway requested reconnect")

		case opInvalidSession:

			return fmt.Errorf("invalid session")

		case opDispatch:

			if envelope.T == nil {

				continue

			}

			switch *envelope.T {

			case "READY":

				if identified {

					continue

				}

				identified = true
				slog.Info("discord bot online")

				if err := registerLaunchCommand(ctx, token, applicationID); err != nil {

					slog.Error("register /launch failed", "err", err)

				} else {

					slog.Info("slash command ready", "name", launchCommand)

				}

			case "INTERACTION_CREATE":

				var interaction interactionCreate

				if err := json.Unmarshal(envelope.D, &interaction); err != nil {

					slog.Warn("interaction decode failed", "err", err)
					continue

				}

				if err := handleInteraction(ctx, token, interaction); err != nil {

					slog.Warn("interaction handle failed", "err", err, "name", interaction.Data.Name)

				}

			}

		}

	}

}

func heartbeat(send func(any) error, seq *atomic.Int64, interval time.Duration, stop <-chan struct{}) {

	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {

		select {

		case <-stop:

			return

		case <-timer.C:

			if err := send(heartbeatPayload(seq)); err != nil {

				return

			}

			timer.Reset(interval)

		}

	}

}

func heartbeatPayload(seq *atomic.Int64) gatewayPayload {

	n := seq.Load()

	if n < 0 {

		return gatewayPayload{Op: opHeartbeat, D: json.RawMessage("null")}

	}

	return gatewayPayload{Op: opHeartbeat, D: mustJSON(n)}

}

func handleInteraction(ctx context.Context, token string, interaction interactionCreate) error {

	switch interaction.Type {

	case interactionPing:

		return respondInteraction(ctx, interaction.ID, interaction.Token, interactionResponse{Type: callbackPong})

	case interactionApplicationCommand:

		if interaction.Data.Name != launchCommand {

			return respondInteraction(ctx, interaction.ID, interaction.Token, interactionResponse{

				Type: 4,
				Data: &interactionResponseData{Content: "Unknown command.", Flags: 64},

			})

		}

		// type 12 opens the app's Activity in the user's current voice channel.
		return respondInteraction(ctx, interaction.ID, interaction.Token, interactionResponse{Type: callbackLaunchActivity})

	default:

		return nil

	}

}

// Activities always ship with a PRIMARY_ENTRY_POINT command. Bulk PUT would delete it
// (Discord error 50240), so only create the chat /launch command when it is missing.
func registerLaunchCommand(ctx context.Context, token string, applicationID string) error {

	existing, err := listCommands(ctx, token, applicationID)

	if err != nil {

		return err

	}

	for _, cmd := range existing {

		// type 1 = CHAT_INPUT; type 4 is the Activity Entry Point (leave it alone).
		if cmd.Type == 1 && cmd.Name == launchCommand {

			return nil

		}

	}

	body := map[string]any{

		"name":        launchCommand,
		"description": "Launch Streamly in your current voice channel",
		"type":        1,

	}

	payload, err := json.Marshal(body)

	if err != nil {

		return err

	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/applications/"+applicationID+"/commands", bytes.NewReader(payload))

	if err != nil {

		return err

	}

	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)

	if err != nil {

		return err

	}

	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {

		raw, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("create /%s: %s: %s", launchCommand, res.Status, bytes.TrimSpace(raw))

	}

	return nil

}

func listCommands(ctx context.Context, token string, applicationID string) ([]applicationCommand, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/applications/"+applicationID+"/commands", nil)

	if err != nil {

		return nil, err

	}

	req.Header.Set("Authorization", "Bot "+token)

	res, err := http.DefaultClient.Do(req)

	if err != nil {

		return nil, err

	}

	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {

		raw, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return nil, fmt.Errorf("list commands: %s: %s", res.Status, bytes.TrimSpace(raw))

	}

	var commands []applicationCommand

	if err := json.NewDecoder(res.Body).Decode(&commands); err != nil {

		return nil, err

	}

	return commands, nil

}

func respondInteraction(ctx context.Context, id string, interactionToken string, response interactionResponse) error {

	payload, err := json.Marshal(response)

	if err != nil {

		return err

	}

	url := apiBase + "/interactions/" + id + "/" + interactionToken + "/callback"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))

	if err != nil {

		return err

	}

	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)

	if err != nil {

		return err

	}

	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {

		raw, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("callback %s: %s", res.Status, bytes.TrimSpace(raw))

	}

	return nil

}

func gatewayURL(ctx context.Context) (string, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/gateway", nil)

	if err != nil {

		return "", err

	}

	res, err := http.DefaultClient.Do(req)

	if err != nil {

		return "", err

	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {

		raw, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return "", fmt.Errorf("gateway: %s: %s", res.Status, bytes.TrimSpace(raw))

	}

	var body struct {

		URL string `json:"url"`

	}

	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {

		return "", err

	}

	if body.URL == "" {

		return "", fmt.Errorf("gateway url empty")

	}

	return body.URL, nil

}

func mustJSON(v any) json.RawMessage {

	raw, err := json.Marshal(v)

	if err != nil {

		panic(err)

	}

	return raw

}

type gatewayPayload struct {

	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  *int64          `json:"s"`
	T  *string         `json:"t"`

}

type helloData struct {

	HeartbeatInterval int `json:"heartbeat_interval"`

}

type identifyPayload struct {

	Token      string             `json:"token"`
	Intents    int                `json:"intents"`
	Properties identifyProperties `json:"properties"`
	Presence   *presencePayload   `json:"presence,omitempty"`

}

type identifyProperties struct {

	OS      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`

}

type presencePayload struct {

	Status string `json:"status"`
	AFK    bool   `json:"afk"`
	Since  *int64 `json:"since"`

}

type interactionCreate struct {

	ID    string          `json:"id"`
	Type  int             `json:"type"`
	Token string          `json:"token"`
	Data  interactionData `json:"data"`

}

type interactionData struct {

	Name string `json:"name"`

}

type interactionResponse struct {

	Type int                     `json:"type"`
	Data *interactionResponseData `json:"data,omitempty"`

}

type interactionResponseData struct {

	Content string `json:"content,omitempty"`
	Flags   int    `json:"flags,omitempty"`

}

type applicationCommand struct {

	ID   string `json:"id"`
	Name string `json:"name"`
	Type int    `json:"type"`

}
