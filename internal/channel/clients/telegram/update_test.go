package telegram

import (
	"encoding/json"
	"testing"
)

func TestDecodeUpdateMapsBotAPIShapes(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		wantKind       string
		wantChatID     int64
		wantMsgID      int64
		wantFromID     int64
		wantName       string
		wantText       string
		wantCap        string
		wantReply      int64
		wantOrigin     int64
		wantSource     int64
		wantOriginType string
		wantBot        bool
		wantAutomatic  bool
		wantSenderChat int64
		wantEdit       int64
	}{
		{
			name:     "owner private command",
			body:     `{"update_id":10,"message":{"message_id":11,"from":{"id":12,"is_bot":false,"first_name":"Ada","last_name":"Lovelace"},"chat":{"id":12,"type":"private"},"date":1700000000,"text":"/connect -100123"}}`,
			wantKind: "message", wantChatID: 12, wantMsgID: 11, wantFromID: 12, wantName: "Ada Lovelace", wantText: "/connect -100123",
		},
		{
			name:     "channel post uses its own message id",
			body:     `{"update_id":20,"channel_post":{"message_id":21,"chat":{"id":-1001,"type":"channel"},"date":1700000001,"caption":"daily"}}`,
			wantKind: "channel_post", wantChatID: -1001, wantMsgID: 21, wantCap: "daily",
		},
		{
			name:     "linked group auto-forward origin",
			body:     `{"update_id":30,"message":{"message_id":31,"chat":{"id":-2002,"type":"supergroup"},"date":1700000002,"forward_origin":{"type":"channel","chat":{"id":-1001,"type":"channel"},"message_id":21},"is_automatic_forward":true,"text":"forward"}}`,
			wantKind: "message", wantChatID: -2002, wantMsgID: 31, wantText: "forward", wantOrigin: -1001, wantSource: 21, wantOriginType: "channel", wantAutomatic: true,
		},
		{
			name:     "direct reply",
			body:     `{"update_id":40,"message":{"message_id":41,"from":{"id":42,"first_name":"Grace"},"chat":{"id":-2002,"type":"supergroup"},"date":1700000003,"reply_to_message":{"message_id":31},"text":"report"}}`,
			wantKind: "message", wantChatID: -2002, wantMsgID: 41, wantFromID: 42, wantName: "Grace", wantReply: 31, wantText: "report",
		},
		{
			name:     "edited message",
			body:     `{"update_id":50,"edited_message":{"message_id":41,"from":{"id":42,"first_name":"Grace"},"chat":{"id":-2002,"type":"supergroup"},"date":1700000003,"edit_date":1700000004,"reply_to_message":{"message_id":31},"text":"edited report"}}`,
			wantKind: "edited_message", wantChatID: -2002, wantMsgID: 41, wantFromID: 42, wantName: "Grace", wantReply: 31, wantText: "edited report", wantEdit: 1700000004,
		},
		{
			name:     "bot sender",
			body:     `{"update_id":60,"message":{"message_id":61,"from":{"id":62,"is_bot":true,"first_name":"A bot"},"chat":{"id":-2002,"type":"supergroup"},"date":1700000005,"text":"bot message"}}`,
			wantKind: "message", wantChatID: -2002, wantMsgID: 61, wantFromID: 62, wantName: "A bot", wantText: "bot message", wantBot: true,
		},
		{
			name:     "anonymous sender has sender chat, no user id",
			body:     `{"update_id":70,"message":{"message_id":71,"sender_chat":{"id":-2002,"type":"supergroup"},"chat":{"id":-2002,"type":"supergroup"},"date":1700000006,"text":"anonymous"}}`,
			wantKind: "message", wantChatID: -2002, wantMsgID: 71, wantText: "anonymous", wantSenderChat: -2002,
		},
		{
			name:     "incomplete origin fails closed to zero identifiers",
			body:     `{"update_id":80,"message":{"message_id":81,"chat":{"id":-2002,"type":"supergroup"},"forward_origin":{"type":"channel","message_id":21}}}`,
			wantKind: "message", wantChatID: -2002, wantMsgID: 81, wantSource: 21, wantOriginType: "channel",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dto Update
			if err := json.Unmarshal([]byte(tt.body), &dto); err != nil {
				t.Fatalf("decode update: %v", err)
			}
			got := mapUpdate(dto)
			if got.Kind != tt.wantKind || got.ChatID != tt.wantChatID || got.MessageID != tt.wantMsgID || got.FromID != tt.wantFromID || got.FromName != tt.wantName || got.Text != tt.wantText || got.Caption != tt.wantCap || got.ReplyToMessageID != tt.wantReply || got.ForwardChatID != tt.wantOrigin || got.ForwardMessageID != tt.wantSource || got.ForwardOriginType != tt.wantOriginType || got.SenderIsBot != tt.wantBot || got.IsAutomaticForward != tt.wantAutomatic || got.SenderChatID != tt.wantSenderChat || got.EditDate != tt.wantEdit {
				t.Fatalf("mapped update = %+v", got)
			}
		})
	}
}

func TestDecodeUpdateRejectsMalformedJSON(t *testing.T) {
	var update Update
	if err := json.Unmarshal([]byte(`{"update_id":"not-an-id","message":{}}`), &update); err == nil {
		t.Fatal("expected malformed update to fail decoding")
	}
}
