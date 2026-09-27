package telegram

import (
	"encoding/json"
	"testing"
)

func TestDecodeGroupPermissionsAndMemberRights(t *testing.T) {
	tests := []struct {
		name       string
		groupJSON  string
		memberJSON string
		wantMember bool
		wantSend   bool
	}{
		{name: "ordinary member default allows sending", groupJSON: `{"id":-1,"type":"supergroup","permissions":{"can_send_messages":true}}`, memberJSON: `{"status":"member"}`, wantMember: true, wantSend: true},
		{name: "ordinary member default denies sending", groupJSON: `{"id":-1,"type":"supergroup","permissions":{"can_send_messages":false}}`, memberJSON: `{"status":"member"}`, wantMember: true},
		{name: "restricted member explicitly allowed", groupJSON: `{"id":-1,"type":"supergroup"}`, memberJSON: `{"status":"restricted","is_member":true,"can_send_messages":true}`, wantMember: true, wantSend: true},
		{name: "restricted member explicitly denied", groupJSON: `{"id":-1,"type":"supergroup","permissions":{"can_send_messages":true}}`, memberJSON: `{"status":"restricted","is_member":true,"can_send_messages":false}`, wantMember: true},
		{name: "restricted member without explicit right", groupJSON: `{"id":-1,"type":"supergroup","permissions":{"can_send_messages":true}}`, memberJSON: `{"status":"restricted","is_member":true}`, wantMember: true},
		{name: "restricted user no longer member", groupJSON: `{"id":-1,"type":"supergroup"}`, memberJSON: `{"status":"restricted","is_member":false,"can_send_messages":true}`},
		{name: "administrator rights are implicit", groupJSON: `{"id":-1,"type":"supergroup"}`, memberJSON: `{"status":"administrator"}`, wantMember: true, wantSend: true},
		{name: "left bot", groupJSON: `{"id":-1,"type":"supergroup","permissions":{"can_send_messages":true}}`, memberJSON: `{"status":"left"}`},
		{name: "missing member status fails closed", groupJSON: `{"id":-1,"type":"supergroup","permissions":{"can_send_messages":true}}`, memberJSON: `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var group chat
			if err := json.Unmarshal([]byte(tt.groupJSON), &group); err != nil {
				t.Fatalf("decode group: %v", err)
			}
			var bot member
			if err := json.Unmarshal([]byte(tt.memberJSON), &bot); err != nil {
				t.Fatalf("decode member: %v", err)
			}
			gotMember, gotSend := groupPermissions(bot, group)
			if gotMember != tt.wantMember || gotSend != tt.wantSend {
				t.Fatalf("groupPermissions() = (%t, %t), want (%t, %t)", gotMember, gotSend, tt.wantMember, tt.wantSend)
			}
		})
	}
}

func TestDecodeTelegramChatAndChannelMember(t *testing.T) {
	tests := []struct {
		name       string
		chatJSON   string
		memberJSON string
		wantChat   chat
		wantMember member
	}{
		{name: "channel linked to discussion group", chatJSON: `{"id":-1001,"type":"channel","linked_chat_id":-2002}`, memberJSON: `{"status":"administrator","can_post_messages":true}`, wantChat: chat{ID: -1001, Type: "channel", LinkedChatID: -2002}, wantMember: member{Status: "administrator", CanPost: true}},
		{name: "chat omits optional rights", chatJSON: `{"id":-2002,"type":"supergroup"}`, memberJSON: `{"status":"member"}`, wantChat: chat{ID: -2002, Type: "supergroup"}, wantMember: member{Status: "member"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotChat chat
			if err := json.Unmarshal([]byte(tt.chatJSON), &gotChat); err != nil {
				t.Fatalf("decode chat: %v", err)
			}
			var gotMember member
			if err := json.Unmarshal([]byte(tt.memberJSON), &gotMember); err != nil {
				t.Fatalf("decode member: %v", err)
			}
			if gotChat != tt.wantChat || gotMember.Status != tt.wantMember.Status || gotMember.CanPost != tt.wantMember.CanPost {
				t.Fatalf("decoded values = (%+v, %+v), want (%+v, %+v)", gotChat, gotMember, tt.wantChat, tt.wantMember)
			}
		})
	}
}
