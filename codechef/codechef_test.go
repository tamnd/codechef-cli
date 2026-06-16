package codechef_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/codechef-cli/codechef"
)

func newTestClient(baseURL string) *codechef.Client {
	cfg := codechef.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Rate = 0
	cfg.Retries = 0
	return codechef.NewClient(cfg)
}

func TestProblems(t *testing.T) {
	payload := map[string]any{
		"status": "success",
		"data": []map[string]any{
			{
				"id":                       "1",
				"code":                     "FLOW001",
				"name":                     "Find the Remainder",
				"difficulty_rating":        "0",
				"total_submissions":        "100",
				"successful_submissions":   "80",
				"distinct_successful_submissions": "70",
				"contest_code":             "",
			},
			{
				"id":                       "2",
				"code":                     "SORT",
				"name":                     "Sorting Problem",
				"difficulty_rating":        "1500",
				"total_submissions":        "200",
				"successful_submissions":   "50",
				"distinct_successful_submissions": "40",
				"contest_code":             "",
			},
		},
		"count": 2,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent header")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	problems, err := c.Problems(context.Background(), 10)
	if err != nil {
		t.Fatalf("Problems: %v", err)
	}
	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2", len(problems))
	}
	if problems[0].Code != "FLOW001" {
		t.Errorf("Code = %q, want FLOW001", problems[0].Code)
	}
	if problems[0].Difficulty != "beginner" {
		t.Errorf("Difficulty = %q, want beginner", problems[0].Difficulty)
	}
	if problems[0].URL == "" {
		t.Error("URL is empty")
	}
	if problems[1].Difficulty != "easy" {
		t.Errorf("Difficulty = %q, want easy", problems[1].Difficulty)
	}
}

func TestSearch(t *testing.T) {
	payload := map[string]any{
		"status": "success",
		"data": []map[string]any{
			{
				"id":   "1",
				"code": "SORT1",
				"name": "Sorting Array",
				"difficulty_rating":        "-1",
				"total_submissions":        "100",
				"successful_submissions":   "60",
				"distinct_successful_submissions": "50",
				"contest_code": "",
			},
			{
				"id":   "2",
				"code": "BSRCH",
				"name": "Binary Search Problem",
				"difficulty_rating":        "-1",
				"total_submissions":        "200",
				"successful_submissions":   "100",
				"distinct_successful_submissions": "90",
				"contest_code": "",
			},
			{
				"id":   "3",
				"code": "GRP",
				"name": "Graph Problem",
				"difficulty_rating":        "-1",
				"total_submissions":        "50",
				"successful_submissions":   "20",
				"distinct_successful_submissions": "18",
				"contest_code": "",
			},
		},
		"count": 3,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	results, err := c.Search(context.Background(), "sort", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (only 'Sorting Array' matches 'sort')", len(results))
	}
	if results[0].Code != "SORT1" {
		t.Errorf("Code = %q, want SORT1", results[0].Code)
	}
}

func TestContests(t *testing.T) {
	payload := map[string]any{
		"status": "success",
		"present_contests": []map[string]any{
			{
				"contest_code":         "LTIME100",
				"contest_name":         "Long Challenge 100",
				"contest_start_date_iso": "2026-06-01T00:00:00+05:30",
				"contest_end_date_iso":   "2026-06-10T00:00:00+05:30",
				"contest_duration":     "12960",
				"distinct_users":       500,
			},
		},
		"future_contests": []map[string]any{
			{
				"contest_code":         "START243",
				"contest_name":         "Starters 243",
				"contest_start_date_iso": "2026-06-17T20:00:00+05:30",
				"contest_end_date_iso":   "2026-06-17T22:00:00+05:30",
				"contest_duration":     "120",
				"distinct_users":       0,
			},
		},
		"past_contests": []map[string]any{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	contests, err := c.Contests(context.Background(), 10)
	if err != nil {
		t.Fatalf("Contests: %v", err)
	}
	if len(contests) != 2 {
		t.Fatalf("got %d contests, want 2", len(contests))
	}
	if contests[0].Status != "ongoing" {
		t.Errorf("Status = %q, want ongoing", contests[0].Status)
	}
	if contests[1].Status != "upcoming" {
		t.Errorf("Status = %q, want upcoming", contests[1].Status)
	}
	if contests[1].Duration != "2h" {
		t.Errorf("Duration = %q, want 2h", contests[1].Duration)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		payload := map[string]any{
			"status": "success",
			"data":   []any{},
			"count":  0,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	cfg := codechef.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := codechef.NewClient(cfg)

	problems, err := c.Problems(context.Background(), 5)
	if err != nil {
		t.Fatalf("Problems after retries: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("expected 0 problems, got %d", len(problems))
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}
