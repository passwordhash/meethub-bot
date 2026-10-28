package gitlab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
)

func TestOpenMergeRequests(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.EscapedPath()+"?"+r.URL.RawQuery)
		if r.Header.Get("PRIVATE-TOKEN") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.EscapedPath() {
		case "/api/v4/groups/money-mania%2Fteam/merge_requests":
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("X-Next-Page", "2")
				_, _ = w.Write([]byte(`[{"project_id":1,"iid":406,"title":"лимит","draft":false,"updated_at":"2026-10-29T10:00:00Z","author":{"username":"tema"},"reviewers":[{"username":"yarik"}],"references":{"full":"money-mania/team/cascade!406"}},
					{"project_id":1,"iid":407,"title":"Draft: wip","draft":true,"author":{"username":"tema"},"reviewers":[{"username":"yarik"}],"references":{"full":"money-mania/team/cascade!407"}}]`))
				return
			}
			_, _ = w.Write([]byte(`[{"project_id":2,"iid":8,"title":"без ревьюверов","author":{"username":"dima"},"reviewers":[],"references":{"full":"money-mania/support-bot!8"}}]`))
		case "/api/v4/projects/1/merge_requests/406/reviewers":
			_, _ = w.Write([]byte(`[{"user":{"username":"yarik"},"state":"requested_changes"},{"user":{"username":"dima"},"state":"something_new"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	got, err := New(server.URL+"/", "secret", "money-mania/team").OpenMergeRequests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []models.MergeRequest{
		{ProjectID: 1, IID: 406, Reference: "cascade!406", Title: "лимит", Author: "tema", UpdatedAt: time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC), Reviewers: []models.Reviewer{{Username: "yarik", State: models.StateRequestedChanges}, {Username: "dima", State: models.StateUnreviewed}}},
		{ProjectID: 1, IID: 407, Reference: "cascade!407", Title: "Draft: wip", Author: "tema", Draft: true},
		{ProjectID: 2, IID: 8, Reference: "support-bot!8", Title: "без ревьюверов", Author: "dima"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merge requests = %+v; want %+v", got, want)
	}
	wantRequests := []string{
		"/api/v4/groups/money-mania%2Fteam/merge_requests?page=1&per_page=100&state=opened",
		"/api/v4/groups/money-mania%2Fteam/merge_requests?page=2&per_page=100&state=opened",
		"/api/v4/projects/1/merge_requests/406/reviewers?",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %v; want %v", requests, wantRequests)
	}
}

func TestOpenMergeRequestsErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"forbidden", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }},
		{"broken json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"oops"`)) }},
		{"reviewers fail", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v4/groups/g/merge_requests" {
				_, _ = w.Write([]byte(`[{"project_id":1,"iid":1,"reviewers":[{"username":"a"}]}]`))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		}},
		{"endless pages", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Next-Page", "2")
			_, _ = w.Write([]byte(`[]`))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()
			if _, err := New(server.URL, "token", "g").OpenMergeRequests(context.Background()); !errors.Is(err, ErrAPI) {
				t.Fatalf("error = %v; want ErrAPI", err)
			}
		})
	}
}

func TestShortReference(t *testing.T) {
	tests := []struct {
		full string
		want string
	}{
		{"group/project!5", "project!5"},
		{"group/sub/project!5", "project!5"},
		{"project!5", "project!5"},
		{"", "!5"},
	}
	for _, tt := range tests {
		if got := shortReference(tt.full, 5); got != tt.want {
			t.Errorf("shortReference(%q) = %q; want %q", tt.full, got, tt.want)
		}
	}
}
