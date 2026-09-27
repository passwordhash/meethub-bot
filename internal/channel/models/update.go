package models

type Update struct {
	ID                 int64
	Text               string
	Caption            string
	FromID             int64
	FromName           string
	SenderIsBot        bool
	SenderChatID       int64
	ChatID             int64
	ChatType           string
	MessageID          int64
	ReplyToMessageID   int64
	ForwardChatID      int64
	ForwardMessageID   int64
	ForwardOriginType  string
	IsAutomaticForward bool
	Date               int64
	EditDate           int64
	Kind               string
}
