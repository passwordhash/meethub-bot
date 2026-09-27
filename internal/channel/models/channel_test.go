package models

import "testing"

func TestPermissionsValidate(t *testing.T) {
	valid := Permissions{ChannelType: "channel", LinkedGroupID: -2, OwnerIsAdmin: true, BotIsAdmin: true, BotCanPost: true, GroupType: "supergroup", GroupLinkedChatID: -1, BotInGroup: true, BotCanSend: true}
	tests := []struct {
		name        string
		permissions Permissions
		channelID   int64
		wantErr     bool
	}{
		{name: "valid reciprocal link", permissions: valid, channelID: -1},
		{name: "wrong chat type", permissions: func() Permissions { p := valid; p.ChannelType = "group"; return p }(), channelID: -1, wantErr: true},
		{name: "owner lacks channel admin", permissions: func() Permissions { p := valid; p.OwnerIsAdmin = false; return p }(), channelID: -1, wantErr: true},
		{name: "bot lacks post rights", permissions: func() Permissions { p := valid; p.BotCanPost = false; return p }(), channelID: -1, wantErr: true},
		{name: "unverifiable default group rights", permissions: func() Permissions { p := valid; p.BotCanSend = false; return p }(), channelID: -1, wantErr: true},
		{name: "mismatched reciprocal link", permissions: func() Permissions { p := valid; p.GroupLinkedChatID = -3; return p }(), channelID: -1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.permissions.Validate(tt.channelID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}
