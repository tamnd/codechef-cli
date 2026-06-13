// Package codechef is the library behind the cc command line:
// the HTTP client, request shaping, and typed data models for CodeChef.
//
// The public APIs at codechef.com require no key — problems, contests,
// and problem search are all open. The Client here paces requests,
// retries transients, and decodes JSON into the typed models every
// command shares.
package codechef

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to CodeChef.
const DefaultUserAgent = "cc/dev (+https://github.com/tamnd/codechef-cli)"

// ErrNotFound is returned when an API response carries no usable data.
var ErrNotFound = fmt.Errorf("not found")

// Config holds all constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://www.codechef.com",
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   5,
		Timeout:   30 * time.Second,
	}
}

// Client talks to CodeChef over HTTP.
type Client struct {
	http      *http.Client
	baseURL   string
	userAgent string
	rate      time.Duration
	retries   int
	last      time.Time
}

// NewClient returns a Client with the given Config.
func NewClient(cfg Config) *Client {
	return &Client{
		http:      &http.Client{Timeout: cfg.Timeout},
		baseURL:   cfg.BaseURL,
		userAgent: cfg.UserAgent,
		rate:      cfg.Rate,
		retries:   cfg.Retries,
	}
}

// get fetches rawURL with pacing and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

func (c *Client) getJSON(ctx context.Context, rawURL string, v any) error {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

// ─── Problems ────────────────────────────────────────────────────────────────

// Problem is one item from the CodeChef problem list.
type Problem struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Difficulty  string `json:"difficulty"`
	Accuracy    string `json:"accuracy"`
	Submissions int    `json:"submissions"`
	URL         string `json:"url"`
}

// wireProblems is the JSON shape returned by /api/list/problems/all.
type wireProblems struct {
	Status  string         `json:"status"`
	Data    []wireProblem  `json:"data"`
	Count   int            `json:"count"`
}

type wireProblem struct {
	ID                          string   `json:"id"`
	Code                        string   `json:"code"`
	Name                        string   `json:"name"`
	DifficultyRating            string   `json:"difficulty_rating"`
	TotalSubmissions            string   `json:"total_submissions"`
	SuccessfulSubmissions       string   `json:"successful_submissions"`
	DistinctSuccessfulSubmissions string `json:"distinct_successful_submissions"`
	ContestCode                 string   `json:"contest_code"`
}

// Problems fetches the most recent problems from the CodeChef problem list.
func (c *Client) Problems(ctx context.Context, limit int) ([]Problem, error) {
	if limit <= 0 {
		limit = 20
	}
	u := fmt.Sprintf("%s/api/list/problems/all?limit=%d&offset=0", c.baseURL, limit)
	var resp wireProblems
	if err := c.getJSON(ctx, u, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("API error: status=%s", resp.Status)
	}
	out := make([]Problem, 0, len(resp.Data))
	for _, p := range resp.Data {
		out = append(out, wireProblemToModel(p, c.baseURL))
	}
	return out, nil
}

// Search fetches problems matching query (in-memory filter on name/code).
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Problem, error) {
	// Fetch a broad set then filter client-side — the API has no text search.
	fetch := limit * 50
	if fetch < 1000 {
		fetch = 1000
	}
	u := fmt.Sprintf("%s/api/list/problems/all?limit=%d&offset=0", c.baseURL, fetch)
	var resp wireProblems
	if err := c.getJSON(ctx, u, &resp); err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []Problem
	for _, p := range resp.Data {
		if strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Code), q) {
			out = append(out, wireProblemToModel(p, c.baseURL))
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func wireProblemToModel(p wireProblem, base string) Problem {
	diff := difficultyLabel(p.DifficultyRating)
	acc := accuracy(p.SuccessfulSubmissions, p.TotalSubmissions)
	subs := parseInt(p.TotalSubmissions)
	link := base + "/problems/" + url.PathEscape(p.Code)
	return Problem{
		Code:        p.Code,
		Name:        p.Name,
		Difficulty:  diff,
		Accuracy:    acc,
		Submissions: subs,
		URL:         link,
	}
}

// difficultyLabel converts a raw difficulty_rating integer string to a label.
// CodeChef uses -1 for "unrated", 0–1 for beginner, and higher for harder.
func difficultyLabel(rating string) string {
	r := parseInt(rating)
	switch {
	case r < 0:
		return "unrated"
	case r == 0:
		return "beginner"
	case r <= 1500:
		return "easy"
	case r <= 2000:
		return "medium"
	case r <= 2500:
		return "hard"
	default:
		return "expert"
	}
}

// accuracy returns a formatted percentage string or "-".
func accuracy(successful, total string) string {
	s := parseInt(successful)
	t := parseInt(total)
	if t == 0 {
		return "-"
	}
	pct := float64(s) / float64(t) * 100.0
	return fmt.Sprintf("%.1f%%", pct)
}

func parseInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

// ─── Contests ────────────────────────────────────────────────────────────────

// Contest is one contest record.
type Contest struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Duration  string `json:"duration"`
	Users     int    `json:"users"`
	Status    string `json:"status"`
	URL       string `json:"url"`
}

type wireContests struct {
	Status          string          `json:"status"`
	PresentContests []wireContest   `json:"present_contests"`
	FutureContests  []wireContest   `json:"future_contests"`
	PastContests    []wireContest   `json:"past_contests"`
}

type wireContest struct {
	ContestCode          string `json:"contest_code"`
	ContestName          string `json:"contest_name"`
	ContestStartDateISO  string `json:"contest_start_date_iso"`
	ContestEndDateISO    string `json:"contest_end_date_iso"`
	ContestDuration      string `json:"contest_duration"`
	DistinctUsers        int    `json:"distinct_users"`
}

// Contests fetches ongoing, upcoming, and recent past contests.
func (c *Client) Contests(ctx context.Context, limit int) ([]Contest, error) {
	u := fmt.Sprintf("%s/api/list/contests/all?limit=20", c.baseURL)
	var resp wireContests
	if err := c.getJSON(ctx, u, &resp); err != nil {
		return nil, err
	}

	var all []Contest
	for _, wc := range resp.PresentContests {
		all = append(all, wireContestToModel(wc, "ongoing", c.baseURL))
	}
	for _, wc := range resp.FutureContests {
		all = append(all, wireContestToModel(wc, "upcoming", c.baseURL))
	}
	for _, wc := range resp.PastContests {
		all = append(all, wireContestToModel(wc, "past", c.baseURL))
	}

	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}
	return all, nil
}

func wireContestToModel(wc wireContest, status, base string) Contest {
	return Contest{
		Code:      wc.ContestCode,
		Name:      wc.ContestName,
		StartDate: wc.ContestStartDateISO,
		EndDate:   wc.ContestEndDateISO,
		Duration:  durationLabel(wc.ContestDuration),
		Users:     wc.DistinctUsers,
		Status:    status,
		URL:       base + "/" + wc.ContestCode,
	}
}

// durationLabel converts minutes string to a human label.
func durationLabel(mins string) string {
	m := parseInt(mins)
	if m <= 0 {
		return "-"
	}
	h := m / 60
	rem := m % 60
	if h == 0 {
		return fmt.Sprintf("%dm", rem)
	}
	if rem == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, rem)
}
