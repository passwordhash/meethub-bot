package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

func TestInlineKeyboardAndCallbackAnswer(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		switch calls {
		case 1:
			if r.URL.Path != "/bottoken/sendMessage" || got["chat_id"] != float64(12) || got["text"] != "Menu" {
				t.Errorf("keyboard request = %s %#v", r.URL.Path, got)
			}
			want := map[string]any{"inline_keyboard": []any{[]any{map[string]any{"text": "Schedule", "callback_data": "ui:schedule"}}}}
			if !sameJSON(got["reply_markup"], want) {
				t.Errorf("reply_markup = %#v, want %#v", got["reply_markup"], want)
			}
		case 2:
			if r.URL.Path != "/bottoken/answerCallbackQuery" || got["callback_query_id"] != "query-1" || got["text"] != "Done" {
				t.Errorf("callback answer = %s %#v", r.URL.Path, got)
			}
		default:
			t.Errorf("unexpected request %d", calls)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()
	client := New("token")
	client.endpoint = server.URL
	if err := client.SendKeyboard(context.Background(), 12, "Menu", [][]models.InlineButton{{{Text: "Schedule", CallbackData: "ui:schedule"}}}); err != nil {
		t.Fatal(err)
	}
	if err := client.AnswerCallback(context.Background(), "query-1", "Done"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("got %d API calls, want 2", calls)
	}
}
