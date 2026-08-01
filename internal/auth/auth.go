package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (

	tokenURL = "https://discord.com/api/oauth2/token"
	userURL = "https://discord.com/api/users/@me"

)

type User struct {

	ID string
	Username string

	DisplayName string
	Avatar string

}

type Client struct {

	clientID string
	clientSecret string

	http *http.Client

}

type tokenResponse struct {

	AccessToken string `json:"access_token"`

}

func New(clientID string, clientSecret string) *Client {

	httpClient := &http.Client{

		Timeout: 10 * time.Second,

	}

	return &Client{

		clientID: clientID,
		clientSecret: clientSecret,

		http: httpClient,

	}

}

func (c *Client) Exchange(ctx context.Context, code string) (string, error) {

	form := url.Values{}

	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))

	if err != nil {

		return "", err

	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)

	if err != nil {

		return "", err

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return "", fmt.Errorf("discord token exchange returned %s", resp.Status)

	}

	var token tokenResponse

	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {

		return "", err

	}

	if token.AccessToken == "" {

		return "", fmt.Errorf("discord token exchange returned no access token")

	}

	return token.AccessToken, nil

}

// Verified once at connect; the result is cached for the connection's lifetime and never stored beyond it (see _docs/DESIGN.md §3).
func (c *Client) Me(ctx context.Context, accessToken string) (User, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)

	if err != nil {

		return User{}, err

	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.http.Do(req)

	if err != nil {

		return User{}, err

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return User{}, fmt.Errorf("discord user lookup returned %s", resp.Status)

	}

	var user struct {

		ID string `json:"id"`
		Username string `json:"username"`

		GlobalName string `json:"global_name"`
		Avatar string `json:"avatar"`

	}

	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {

		return User{}, err

	}

	displayName := user.GlobalName

	if displayName == "" {

		displayName = user.Username

	}

	avatar := ""

	if user.Avatar != "" {

		avatar = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png?size=64", user.ID, user.Avatar)

	}

	return User{

		ID: user.ID,
		Username: user.Username,

		DisplayName: displayName,
		Avatar: avatar,

	}, nil

}
