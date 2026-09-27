package app

import (
	"context"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/clients/telegram"
)

type ownerNotifier struct {
	telegram *telegram.Client
	ownerID  int64
}

func (n ownerNotifier) NotifyPublicationProblem(ctx context.Context, _ int64) error {
	return n.telegram.Send(ctx, n.ownerID, "Публикация требует проверки. Проверьте права бота и состояние канала.")
}

func (n ownerNotifier) NotifySummaryProblem(ctx context.Context, _ int64) error {
	return n.telegram.Send(ctx, n.ownerID, "Доставка сводки требует проверки. Проверьте права бота и состояние группы.")
}
