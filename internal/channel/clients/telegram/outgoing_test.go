package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOutgoingMessages(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		status     int
		wantID     int64
		wantError  error
		operation  string
		wantFields map[string]any
	}{
		{name: "post includes HTML mode", body: `{"ok":true,"result":{"message_id":51}}`, status: http.StatusOK, wantID: 51, operation: "post", wantFields: map[string]any{"chat_id": int64(-100), "text": "<b>text</b>", "parse_mode": "HTML"}},
		{name: "reply references linked group forward", body: `{"ok":true,"result":{"message_id":52}}`, status: http.StatusOK, wantID: 52, operation: "reply", wantFields: map[string]any{"chat_id": int64(-200), "text": "plain", "reply_parameters": map[string]any{"message_id": float64(9)}}},
		{name: "telegram rejection is definite", body: `{"ok":false,"error_code":403}`, status: http.StatusForbidden, wantError: ErrTelegramRejected, operation: "post"},
		{name: "server error with ok false is uncertain", body: `{"ok":false,"error_code":500}`, status: http.StatusInternalServerError, wantError: ErrTelegramUncertain, operation: "post"},
		{name: "malformed result is uncertain", body: `{`, status: http.StatusOK, wantError: ErrTelegramUncertain, operation: "post"},
		{name: "invalid message ID is uncertain", body: `{"ok":true,"result":{"message_id":"invalid"}}`, status: http.StatusOK, wantError: ErrTelegramUncertain, operation: "post"},
		{name: "missing message ID is uncertain", body: `{"ok":true,"result":{}}`, status: http.StatusOK, wantError: ErrTelegramUncertain, operation: "post"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/bottoken/sendMessage" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decode request: %v", err)
				}
				if tt.wantFields != nil {
					for key, want := range tt.wantFields {
						if got[key] == nil || !sameJSON(got[key], want) {
							t.Errorf("field %s = %#v, want %#v", key, got[key], want)
						}
					}
					if tt.operation == "reply" && got["parse_mode"] != nil {
						t.Error("reply unexpectedly sets parse_mode")
					}
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			client := New("token")
			client.endpoint = server.URL
			var id int64
			var err error
			if tt.operation == "reply" {
				id, err = client.SendReply(context.Background(), -200, 9, "plain")
			} else {
				id, err = client.SendPost(context.Background(), -100, "<b>text</b>")
			}
			if id != tt.wantID || (tt.wantError == nil && err != nil) || (tt.wantError != nil && !errors.Is(err, tt.wantError)) {
				t.Fatalf("Send = (%d, %v), want (%d, %v)", id, err, tt.wantID, tt.wantError)
			}
		})
	}
}

func TestSendReplyRejectsMissingTarget(t *testing.T) {
	if _, err := New("token").SendReply(context.Background(), -1, 0, "text"); err == nil {
		t.Fatal("expected missing reply target error")
	}
}

func sameJSON(got, want any) bool {
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	return string(gotJSON) == string(wantJSON)
}
