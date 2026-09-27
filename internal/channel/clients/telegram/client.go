package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

var (
	ErrAPI               = errors.New("telegram API request failed")
	ErrTelegramRejected  = models.ErrTelegramRejected
	ErrTelegramUncertain = errors.New("telegram request outcome is uncertain")
)

type Client struct {
	token    string
	client   *http.Client
	endpoint string
}

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
}

type chat struct {
	ID           int64  `json:"id"`
	Type         string `json:"type"`
	LinkedChatID int64
	Permissions  struct{ CanSendMessages bool } `json:"permissions"`
}

func (c *chat) UnmarshalJSON(data []byte) error {
	type plainChat chat
	var decoded plainChat
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if err := unmarshalOptional(fields["linked_chat_id"], &decoded.LinkedChatID); err != nil {
		return err
	}
	var permissions map[string]json.RawMessage
	if err := unmarshalOptional(fields["permissions"], &permissions); err != nil {
		return err
	}
	if err := unmarshalOptional(permissions["can_send_messages"], &decoded.Permissions.CanSendMessages); err != nil {
		return err
	}
	*c = chat(decoded)
	return nil
}

type member struct {
	Status   string `json:"status"`
	CanPost  bool
	CanSend  *bool
	IsMember bool
}

func (m *member) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if err := unmarshalOptional(fields["status"], &m.Status); err != nil {
		return err
	}
	if err := unmarshalOptional(fields["can_post_messages"], &m.CanPost); err != nil {
		return err
	}
	if err := unmarshalOptional(fields["can_send_messages"], &m.CanSend); err != nil {
		return err
	}
	if err := unmarshalOptional(fields["is_member"], &m.IsMember); err != nil {
		return err
	}
	return nil
}

func unmarshalOptional(data json.RawMessage, target any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

type identity struct {
	ID int64 `json:"id"`
}

type Update struct {
	ID            int64
	Message       *message
	EditedMessage *message
	ChannelPost   *message
	CallbackQuery *callbackQuery
}

func (u *Update) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if err := unmarshalOptional(fields["update_id"], &u.ID); err != nil {
		return err
	}
	for _, field := range []struct {
		key    string
		target any
	}{
		{key: "message", target: &u.Message},
		{key: "edited_message", target: &u.EditedMessage},
		{key: "channel_post", target: &u.ChannelPost},
		{key: "callback_query", target: &u.CallbackQuery},
	} {
		if err := unmarshalOptional(fields[field.key], field.target); err != nil {
			return err
		}
	}
	return nil
}

type callbackQuery struct {
	ID      string   `json:"id"`
	From    *user    `json:"from"`
	Message *message `json:"message"`
	Data    string   `json:"data"`
}

func mapUpdate(update Update) models.Update {
	if update.CallbackQuery != nil {
		mapped := models.Update{ID: update.ID, Kind: "callback_query", CallbackQueryID: update.CallbackQuery.ID, CallbackData: update.CallbackQuery.Data}
		if update.CallbackQuery.From != nil {
			mapped.FromID = update.CallbackQuery.From.ID
		}
		if update.CallbackQuery.Message != nil {
			mapped.ChatID = update.CallbackQuery.Message.Chat.ID
			mapped.ChatType = update.CallbackQuery.Message.Chat.Type
			mapped.MessageID = update.CallbackQuery.Message.ID
		}
		return mapped
	}
	kind, source := "", (*message)(nil)
	switch {
	case update.Message != nil:
		kind, source = "message", update.Message
	case update.EditedMessage != nil:
		kind, source = "edited_message", update.EditedMessage
	case update.ChannelPost != nil:
		kind, source = "channel_post", update.ChannelPost
	}
	mapped := models.Update{ID: update.ID, Kind: kind}
	if source == nil {
		return mapped
	}
	mapped.Text, mapped.Caption = source.Text, source.Caption
	mapped.MessageID, mapped.ChatID, mapped.ChatType = source.ID, source.Chat.ID, source.Chat.Type
	mapped.Date, mapped.EditDate = source.Date, source.EditDate
	mapped.IsAutomaticForward = source.IsAutomaticForward
	if source.From != nil {
		mapped.FromID = source.From.ID
		mapped.FromName = strings.TrimSpace(source.From.FirstName + " " + source.From.LastName)
		mapped.SenderIsBot = source.From.IsBot
	}
	if source.SenderChat != nil {
		mapped.SenderChatID = source.SenderChat.ID
	}
	if source.ReplyToMessage != nil {
		mapped.ReplyToMessageID = source.ReplyToMessage.ID
	}
	if source.ForwardOrigin != nil {
		mapped.ForwardOriginType = source.ForwardOrigin.Type
		if source.ForwardOrigin.Chat != nil {
			mapped.ForwardChatID = source.ForwardOrigin.Chat.ID
		}
		mapped.ForwardMessageID = source.ForwardOrigin.MessageID
	}
	return mapped
}

type message struct {
	ID                 int64
	Text               string
	Caption            string
	Date               int64
	EditDate           int64
	From               *user
	SenderChat         *senderChat
	Chat               chatIdentity
	ReplyToMessage     *replyMessage
	ForwardOrigin      *forwardOrigin
	IsAutomaticForward bool
}

type user struct {
	ID        int64
	IsBot     bool
	FirstName string
	LastName  string
}

type senderChat struct{ ID int64 }
type chatIdentity struct {
	ID   int64
	Type string
}
type replyMessage struct{ ID int64 }
type forwardOrigin struct {
	Type      string
	Chat      *senderChat
	MessageID int64
}

func (m *message) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, field := range []struct {
		key    string
		target any
	}{
		{key: "message_id", target: &m.ID},
		{key: "text", target: &m.Text},
		{key: "caption", target: &m.Caption},
		{key: "date", target: &m.Date},
		{key: "edit_date", target: &m.EditDate},
		{key: "from", target: &m.From},
		{key: "sender_chat", target: &m.SenderChat},
		{key: "chat", target: &m.Chat},
		{key: "reply_to_message", target: &m.ReplyToMessage},
		{key: "forward_origin", target: &m.ForwardOrigin},
		{key: "is_automatic_forward", target: &m.IsAutomaticForward},
	} {
		if err := unmarshalOptional(fields[field.key], field.target); err != nil {
			return err
		}
	}
	return nil
}

func unmarshalObject(data []byte, fields map[string]any) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for key, target := range fields {
		if err := unmarshalOptional(raw[key], target); err != nil {
			return err
		}
	}
	return nil
}

func (u *user) UnmarshalJSON(data []byte) error {
	return unmarshalObject(data, map[string]any{"id": &u.ID, "is_bot": &u.IsBot, "first_name": &u.FirstName, "last_name": &u.LastName})
}

func (c *chatIdentity) UnmarshalJSON(data []byte) error {
	return unmarshalObject(data, map[string]any{"id": &c.ID, "type": &c.Type})
}

func (c *senderChat) UnmarshalJSON(data []byte) error {
	return unmarshalObject(data, map[string]any{"id": &c.ID})
}

func (r *replyMessage) UnmarshalJSON(data []byte) error {
	return unmarshalObject(data, map[string]any{"message_id": &r.ID})
}

func (o *forwardOrigin) UnmarshalJSON(data []byte) error {
	return unmarshalObject(data, map[string]any{"type": &o.Type, "chat": &o.Chat, "message_id": &o.MessageID})
}

func New(token string) *Client {
	return &Client{token: token, client: &http.Client{Timeout: 40 * time.Second}, endpoint: "https://api.telegram.org"}
}

func (c *Client) call(ctx context.Context, method string, input, output any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return ErrAPI
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/bot"+c.token+"/"+method, bytes.NewReader(data))
	if err != nil {
		return ErrAPI
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(req)
	if err != nil {
		return errors.Join(ErrAPI, ErrTelegramUncertain)
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		return errors.Join(ErrAPI, ErrTelegramUncertain)
	}
	var result envelope
	if json.Unmarshal(data, &result) != nil {
		return errors.Join(ErrAPI, ErrTelegramUncertain)
	}
	if response.StatusCode >= 400 && response.StatusCode < 500 && !result.OK {
		return errors.Join(ErrAPI, ErrTelegramRejected)
	}
	if response.StatusCode != http.StatusOK || !result.OK {
		return errors.Join(ErrAPI, ErrTelegramUncertain)
	}
	if output != nil && json.Unmarshal(result.Result, output) != nil {
		return errors.Join(ErrAPI, ErrTelegramUncertain)
	}
	return nil
}

func (c *Client) VerifyChannel(ctx context.Context, channelID, ownerID int64) (models.Permissions, error) {
	var channel chat
	if err := c.call(ctx, "getChat", map[string]any{"chat_id": channelID}, &channel); err != nil {
		return models.Permissions{}, models.ErrUnverifiablePermissions
	}
	var owner member
	if err := c.call(ctx, "getChatMember", map[string]any{"chat_id": channelID, "user_id": ownerID}, &owner); err != nil {
		return models.Permissions{}, models.ErrUnverifiablePermissions
	}
	var me identity
	if err := c.call(ctx, "getMe", map[string]any{}, &me); err != nil {
		return models.Permissions{}, models.ErrUnverifiablePermissions
	}
	var botChannel member
	if err := c.call(ctx, "getChatMember", map[string]any{"chat_id": channelID, "user_id": me.ID}, &botChannel); err != nil {
		return models.Permissions{}, models.ErrUnverifiablePermissions
	}
	var group chat
	if err := c.call(ctx, "getChat", map[string]any{"chat_id": channel.LinkedChatID}, &group); err != nil {
		return models.Permissions{}, models.ErrUnverifiablePermissions
	}
	var botGroup member
	if err := c.call(ctx, "getChatMember", map[string]any{"chat_id": group.ID, "user_id": me.ID}, &botGroup); err != nil {
		return models.Permissions{}, models.ErrUnverifiablePermissions
	}
	botInGroup, canSend := groupPermissions(botGroup, group)
	var reciprocal chat
	if err := c.call(ctx, "getChat", map[string]any{"chat_id": group.ID}, &reciprocal); err != nil {
		return models.Permissions{}, models.ErrUnverifiablePermissions
	}
	return models.Permissions{
		ChannelType: channel.Type, LinkedGroupID: channel.LinkedChatID,
		OwnerIsAdmin: owner.Status == "administrator" || owner.Status == "creator",
		BotIsAdmin:   botChannel.Status == "administrator" || botChannel.Status == "creator", BotCanPost: botChannel.CanPost,
		GroupType: group.Type, GroupLinkedChatID: reciprocal.LinkedChatID,
		BotInGroup: botInGroup, BotCanSend: canSend,
	}, nil
}

func groupPermissions(bot member, group chat) (bool, bool) {
	switch bot.Status {
	case "administrator", "creator":
		return true, true
	case "member":
		return true, group.Permissions.CanSendMessages
	case "restricted":
		return bot.IsMember, bot.IsMember && bot.CanSend != nil && *bot.CanSend
	default:
		return false, false
	}
}

func (c *Client) Updates(ctx context.Context, offset int64) ([]models.Update, error) {
	var updates []Update
	if err := c.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 30, "allowed_updates": []string{"message", "edited_message", "channel_post", "callback_query"}}, &updates); err != nil {
		return nil, err
	}
	result := make([]models.Update, 0, len(updates))
	for _, update := range updates {
		result = append(result, mapUpdate(update))
	}
	return result, nil
}

func (c *Client) Send(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, nil)
}

func (c *Client) SendKeyboard(ctx context.Context, chatID int64, text string, keyboard [][]models.InlineButton) error {
	rows := make([][]map[string]string, 0, len(keyboard))
	for _, row := range keyboard {
		buttons := make([]map[string]string, 0, len(row))
		for _, button := range row {
			buttons = append(buttons, map[string]string{"text": button.Text, "callback_data": button.CallbackData})
		}
		rows = append(rows, buttons)
	}
	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID, "text": text,
		"reply_markup": map[string]any{"inline_keyboard": rows},
	}, nil)
}

func (c *Client) AnswerCallback(ctx context.Context, callbackQueryID, text string) error {
	return c.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": callbackQueryID, "text": text}, nil)
}

type sentMessage struct {
	MessageID int64
}

func (m *sentMessage) UnmarshalJSON(data []byte) error {
	return unmarshalObject(data, map[string]any{"message_id": &m.MessageID})
}

func (c *Client) SendPost(ctx context.Context, channelID int64, htmlText string) (int64, error) {
	return c.sendMessage(ctx, map[string]any{"chat_id": channelID, "text": htmlText, "parse_mode": "HTML"})
}

func (c *Client) SendReply(ctx context.Context, linkedGroupID, autoForwardMessageID int64, plainText string) (int64, error) {
	if autoForwardMessageID <= 0 {
		return 0, errors.Join(ErrAPI, errors.New("invalid reply message ID"))
	}
	return c.sendMessage(ctx, map[string]any{
		"chat_id": linkedGroupID, "text": plainText,
		"reply_parameters": map[string]int64{"message_id": autoForwardMessageID},
	})
}

func (c *Client) sendMessage(ctx context.Context, input map[string]any) (int64, error) {
	var sent sentMessage
	if err := c.call(ctx, "sendMessage", input, &sent); err != nil {
		return 0, err
	}
	if sent.MessageID <= 0 {
		return 0, errors.Join(ErrAPI, ErrTelegramUncertain)
	}
	return sent.MessageID, nil
}
