package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPinPermission(t *testing.T) {
	tests := []struct {
		name       string
		groupJSON  string
		memberJSON string
		want       bool
	}{
		{name: "admin allowed to pin", groupJSON: `{"id":-1,"type":"supergroup"}`, memberJSON: `{"status":"administrator","can_pin_messages":true}`, want: true},
		{name: "admin without pin right", groupJSON: `{"id":-1,"type":"supergroup"}`, memberJSON: `{"status":"administrator","can_pin_messages":false}`},
		{name: "admin of a basic group", groupJSON: `{"id":-1,"type":"group"}`, memberJSON: `{"status":"administrator"}`, want: true},
		{name: "member with group default", groupJSON: `{"id":-1,"type":"supergroup","permissions":{"can_send_messages":true,"can_pin_messages":true}}`, memberJSON: `{"status":"member"}`, want: true},
		{name: "member without group default", groupJSON: `{"id":-1,"type":"supergroup","permissions":{"can_send_messages":true}}`, memberJSON: `{"status":"member"}`},
		{name: "member that cannot send", groupJSON: `{"id":-1,"type":"supergroup","permissions":{"can_pin_messages":true}}`, memberJSON: `{"status":"member"}`},
		{name: "restricted with pin right", groupJSON: `{"id":-1,"type":"supergroup"}`, memberJSON: `{"status":"restricted","is_member":true,"can_send_messages":true,"can_pin_messages":true}`, want: true},
		{name: "left bot", groupJSON: `{"id":-1,"type":"supergroup","permissions":{"can_send_messages":true,"can_pin_messages":true}}`, memberJSON: `{"status":"left"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var group chat
			if err := json.Unmarshal([]byte(tt.groupJSON), &group); err != nil {
				t.Fatal(err)
			}
			var bot member
			if err := json.Unmarshal([]byte(tt.memberJSON), &bot); err != nil {
				t.Fatal(err)
			}
			if got := pinPermission(bot, group); got != tt.want {
				t.Fatalf("pinPermission() = %t; want %t", got, tt.want)
			}
		})
	}
}

func TestBoardRequests(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		run       func(*Client) error
		method    string
		wantError error
	}{
		{name: "unchanged edit is success", status: http.StatusBadRequest, body: `{"ok":false,"description":"Bad Request: message is not modified: specified new message content and reply markup are exactly the same"}`, method: "editMessageText", run: func(c *Client) error { return c.EditBoard(context.Background(), -1, 5, "text") }},
		{name: "deleted message is rejected", status: http.StatusBadRequest, body: `{"ok":false,"description":"Bad Request: message to edit not found"}`, method: "editMessageText", run: func(c *Client) error { return c.EditBoard(context.Background(), -1, 5, "text") }, wantError: ErrTelegramRejected},
		{name: "pin", status: http.StatusOK, body: `{"ok":true,"result":true}`, method: "pinChatMessage", run: func(c *Client) error { return c.PinBoard(context.Background(), -1, 5) }},
		{name: "pinned message", status: http.StatusOK, body: `{"ok":true,"result":{"id":-1,"type":"supergroup","pinned_message":{"message_id":5}}}`, method: "getChat", run: func(c *Client) error {
			id, err := c.PinnedMessageID(context.Background(), -1)
			if err == nil && id != 5 {
				return errors.New("wrong pinned message")
			}
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/bottoken/"+tt.method {
					t.Errorf("path = %s", r.URL.Path)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			client := New("token")
			client.endpoint = server.URL
			if err := tt.run(client); !errors.Is(err, tt.wantError) || (tt.wantError == nil && err != nil) {
				t.Fatalf("error = %v; want %v", err, tt.wantError)
			}
		})
	}
}
