package gitlabclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fluxa-api/internal/domain/servicecatalog"
	"fluxa-api/internal/shared"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) SearchProjects(ctx context.Context, keyword string) ([]servicecatalog.RemoteGitlabProject, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []servicecatalog.RemoteGitlabProject{}, nil
	}
	values := url.Values{}
	values.Set("search", keyword)
	values.Set("simple", "true")
	values.Set("per_page", "50")
	var projects []servicecatalog.RemoteGitlabProject
	if err := c.get(ctx, "/api/v4/projects?"+values.Encode(), &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

func (c *Client) GetProject(ctx context.Context, projectID int64) (servicecatalog.RemoteGitlabProject, error) {
	if projectID <= 0 {
		return servicecatalog.RemoteGitlabProject{}, shared.ErrInvalidInput
	}
	var project servicecatalog.RemoteGitlabProject
	if err := c.get(ctx, "/api/v4/projects/"+strconv.FormatInt(projectID, 10), &project); err != nil {
		return servicecatalog.RemoteGitlabProject{}, err
	}
	return project, nil
}

func (c *Client) get(ctx context.Context, path string, target any) error {
	if c.baseURL == "" || c.token == "" {
		return fmt.Errorf("gitlab is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Private-Token", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return shared.ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gitlab returned status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode gitlab response: %w", err)
	}
	return nil
}
