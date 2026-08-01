package febbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://www.febbox.com"

const maxBodyBytes = 8 << 20

// The ui cookie is a manually supplied per-account token; its expiry stops all VOD at once (see _docs/DESIGN.md §5.4).
var ErrNotAuthenticated = errors.New("febbox: ui cookie missing or expired")

type File struct {

	FileID string
	Name string

	IsDir bool

}

type Quality struct {

	URL string
	Quality string

	Name string
	Size string

}

type Client struct {

	http *http.Client

	cookie string

}

func New(cookie string) *Client {

	httpClient := &http.Client{

		Timeout: 25 * time.Second,

	}

	return &Client{

		http: httpClient,

		cookie: cookie,

	}

}

func (c *Client) Files(ctx context.Context, shareKey string, parentID string) ([]File, error) {

	if parentID == "" {

		parentID = "0"

	}

	target := fmt.Sprintf("%s/file/file_share_list?share_key=%s&pwd=&parent_id=%s&is_html=0", baseURL, shareKey, parentID)

	body, err := c.get(ctx, target)

	if err != nil {

		return nil, err

	}

	var result struct {

		Data struct {

			FileList []struct {

				FileID json.Number `json:"fid"`
				Name string `json:"file_name"`

				IsDir int `json:"is_dir"`

			} `json:"file_list"`

		} `json:"data"`

	}

	if err := json.Unmarshal(body, &result); err != nil {

		return nil, fmt.Errorf("febbox: unreadable file list: %w", err)

	}

	files := make([]File, 0, len(result.Data.FileList))

	for _, entry := range result.Data.FileList {

		files = append(files, File{

			FileID: entry.FileID.String(),
			Name: entry.Name,

			IsDir: entry.IsDir == 1,

		})

	}

	return files, nil

}

func (c *Client) Qualities(ctx context.Context, fileID string) ([]Quality, error) {

	if c.cookie == "" {

		return nil, ErrNotAuthenticated

	}

	body, err := c.get(ctx, fmt.Sprintf("%s/console/video_quality_list?fid=%s", baseURL, fileID))

	if err != nil {

		return nil, err

	}

	var result struct {

		HTML string `json:"html"`

	}

	if err := json.Unmarshal(body, &result); err != nil {

		return nil, fmt.Errorf("febbox: unreadable quality list: %w", err)

	}

	qualities := parseQualities(result.HTML)

	// An empty list is what an expired cookie looks like, so it must not be reported as "no qualities".
	if len(qualities) == 0 {

		return nil, ErrNotAuthenticated

	}

	return qualities, nil

}

func (c *Client) get(ctx context.Context, target string) ([]byte, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)

	if err != nil {

		return nil, err

	}

	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", baseURL+"/")

	if c.cookie != "" {

		req.Header.Set("Cookie", "ui="+c.cookie)

	}

	resp, err := c.http.Do(req)

	if err != nil {

		return nil, fmt.Errorf("febbox: request failed: %w", err)

	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {

		return nil, ErrNotAuthenticated

	}

	if resp.StatusCode != http.StatusOK {

		return nil, fmt.Errorf("febbox: %s returned %s", target, resp.Status)

	}

	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

}
