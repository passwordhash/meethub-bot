package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfileMapping(t *testing.T) {
	for _, tt := range []struct {
		name, twitter, want string
		forbidden           bool
	}{
		{name: "with at", twitter: "@yarik_money_mania", want: "@yarik_money_mania"},
		{name: "without at", twitter: "yarik_money_mania", want: "@yarik_money_mania"},
		{name: "empty"},
		{name: "invalid", twitter: "https://t.me/yarik"},
		{name: "forbidden", forbidden: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v4/groups/g/merge_requests":
					fmt.Fprint(w, `[{"project_id":1,"iid":1,"author":{"id":8,"username":"Yarik"},"reviewers":[{"id":8,"username":"Yarik"}]},{"project_id":1,"iid":2,"author":{"id":8,"username":"Yarik"},"reviewers":[{"id":8,"username":"Yarik"}]}]`)
				case "/api/v4/projects/1/merge_requests/1/reviewers", "/api/v4/projects/1/merge_requests/2/reviewers":
					fmt.Fprint(w, `[{"user":{"id":8,"username":"Yarik"},"state":"unreviewed"}]`)
				case "/api/v4/users/8":
					calls++
					if tt.forbidden {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					fmt.Fprintf(w, `{"twitter":%q}`, tt.twitter)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			mrs, err := New(server.URL, "secret", "g").OpenMergeRequests(context.Background())
			if tt.forbidden {
				if !errors.Is(err, ErrAPI) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("profile requests = %d; want 1 per poll", calls)
			}
			for _, mr := range mrs {
				if got := mr.TelegramLabels["yarik"]; got != tt.want {
					t.Fatalf("label = %q; want %q", got, tt.want)
				}
			}
		})
	}
}
