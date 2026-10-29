package telegram

import (
	"context"
	"errors"
)

// SendBoard posts the MR board silently: it is a status message, not a notification.
func (c *Client) SendBoard(ctx context.Context, groupID int64, text string) (int64, error) {
	return c.sendMessage(ctx, map[string]any{"chat_id": groupID, "text": text, "disable_notification": true, "link_preview_options": map[string]bool{"is_disabled": true}})
}

// EditBoard treats an unchanged text as success so a retried edit does not look like a lost board.
func (c *Client) EditBoard(ctx context.Context, groupID, messageID int64, text string) error {
	err := c.call(ctx, "editMessageText", map[string]any{"chat_id": groupID, "message_id": messageID, "text": text, "link_preview_options": map[string]bool{"is_disabled": true}}, nil)
	if errors.Is(err, ErrMessageNotModified) {
		return nil
	}
	return err
}

func (c *Client) PinBoard(ctx context.Context, groupID, messageID int64) error {
	return c.call(ctx, "pinChatMessage", map[string]any{"chat_id": groupID, "message_id": messageID, "disable_notification": true}, nil)
}

// PinnedMessageID returns the most recently pinned message of the group, or 0 when none is pinned.
func (c *Client) PinnedMessageID(ctx context.Context, groupID int64) (int64, error) {
	var group chat
	if err := c.call(ctx, "getChat", map[string]any{"chat_id": groupID}, &group); err != nil {
		return 0, err
	}
	return group.PinnedMessageID, nil
}

// CanPin checks that the bot may send and pin messages in the group.
func (c *Client) CanPin(ctx context.Context, groupID int64) (bool, error) {
	var me identity
	if err := c.call(ctx, "getMe", map[string]any{}, &me); err != nil {
		return false, err
	}
	var group chat
	if err := c.call(ctx, "getChat", map[string]any{"chat_id": groupID}, &group); err != nil {
		return false, err
	}
	var bot member
	if err := c.call(ctx, "getChatMember", map[string]any{"chat_id": groupID, "user_id": me.ID}, &bot); err != nil {
		return false, err
	}
	return pinPermission(bot, group), nil
}

func pinPermission(bot member, group chat) bool {
	_, canSend := groupPermissions(bot, group)
	if !canSend {
		return false
	}
	switch bot.Status {
	case "creator":
		return true
	case "administrator":
		// Basic groups omit the field: their administrators may always pin.
		if bot.CanPin == nil {
			return group.Type == "group"
		}
		return *bot.CanPin
	case "restricted":
		return bot.CanPin != nil && *bot.CanPin
	}
	return group.Permissions.CanPinMessages
}
