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

const tokenURL = "https://discord.com/api/oauth2/token"

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
