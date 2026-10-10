// Package ghcomment creates or updates a single pull-request comment carrying
// the tfimpact marker, using only the GitHub REST API and the job token.
package ghcomment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Marker identifies the comment tfimpact owns.
const Marker = "<!-- tfimpact -->"

// Env carries the GitHub Actions context the command needs.
type Env struct {
	Token      string
	Repository string // owner/repo
	APIURL     string
	EventPath  string
	PRNumber   int
}

// FromEnvironment reads GITHUB_TOKEN, GITHUB_REPOSITORY, GITHUB_API_URL and the
// pull request number from GITHUB_EVENT_PATH (pull_request.number or issue.number).
func FromEnvironment() (Env, error) {
	e := Env{
		Token:      os.Getenv("GITHUB_TOKEN"),
		Repository: os.Getenv("GITHUB_REPOSITORY"),
		APIURL:     os.Getenv("GITHUB_API_URL"),
		EventPath:  os.Getenv("GITHUB_EVENT_PATH"),
	}
	if e.APIURL == "" {
		e.APIURL = "https://api.github.com"
	}
	if e.EventPath != "" {
		data, err := os.ReadFile(e.EventPath)
		if err != nil {
			return e, fmt.Errorf("read event payload: %w", err)
		}
		var ev struct {
			PullRequest *struct {
				Number int `json:"number"`
			} `json:"pull_request"`
			Issue *struct {
				Number int `json:"number"`
			} `json:"issue"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return e, fmt.Errorf("parse event payload: %w", err)
		}
		if ev.PullRequest != nil {
			e.PRNumber = ev.PullRequest.Number
		} else if ev.Issue != nil {
			e.PRNumber = ev.Issue.Number
		}
	}
	return e, nil
}

// ErrNotPullRequest is returned when the event has no pull request number.
var ErrNotPullRequest = errors.New("not a pull request event")

// Upsert posts body as the tfimpact comment, updating an existing one.
func Upsert(e Env, body string) error {
	if e.PRNumber == 0 {
		return ErrNotPullRequest
	}
	if e.Token == "" || e.Repository == "" {
		return errors.New("GITHUB_TOKEN and GITHUB_REPOSITORY are required")
	}
	if !strings.Contains(body, Marker) {
		body = Marker + "\n" + body
	}
	client := &http.Client{Timeout: 30 * time.Second}
	existing, err := findExisting(client, e)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"body": body})
	var req *http.Request
	if existing != 0 {
		req, err = http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/repos/%s/issues/comments/%d", e.APIURL, e.Repository, existing), bytes.NewReader(payload))
	} else {
		req, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/repos/%s/issues/%d/comments", e.APIURL, e.Repository, e.PRNumber), bytes.NewReader(payload))
	}
	if err != nil {
		return err
	}
	resp, err := do(client, req, e.Token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("github api %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

func findExisting(client *http.Client, e Env) (int64, error) {
	for page := 1; page <= 20; page++ {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=100&page=%d", e.APIURL, e.Repository, e.PRNumber, page), nil)
		if err != nil {
			return 0, err
		}
		resp, err := do(client, req, e.Token)
		if err != nil {
			return 0, err
		}
		var comments []struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
		}
		err = json.NewDecoder(resp.Body).Decode(&comments)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("github api %s listing comments", resp.Status)
		}
		if err != nil {
			return 0, fmt.Errorf("decode comments: %w", err)
		}
		for _, c := range comments {
			if strings.Contains(c.Body, Marker) {
				return c.ID, nil
			}
		}
		if len(comments) < 100 {
			break
		}
	}
	return 0, nil
}

func do(client *http.Client, req *http.Request, token string) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "tfimpact")
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return client.Do(req)
}
