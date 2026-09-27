package summary

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
	"github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

var errExpired = errors.New("summary parts expired and cannot be delivered")

type Repository struct{ getter postgres.Getter }

func New(getter postgres.Getter) *Repository { return &Repository{getter: getter} }

func (r *Repository) CreateSummaryParts(ctx context.Context, editionSlotID uuid.UUID, texts []string) error {
	if len(texts) == 0 {
		return errors.New("summary must contain at least one part")
	}
	query, args := createPartsQuery(editionSlotID, texts)
	var accepted, existing int
	if err := r.getter.Get(ctx).QueryRow(ctx, query, args).Scan(&accepted, &existing); err != nil {
		return fmt.Errorf("create summary parts: %w", err)
	}
	if accepted != len(texts) || existing != 0 && existing != len(texts) {
		return errors.New("summary batch conflicts with immutable stored content")
	}
	return nil
}

func (r *Repository) GetSummaryParts(ctx context.Context, filters models.GetSummaryPartsFilters) ([]models.SummaryPart, error) {
	query, args := getSummaryPartsQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("get summary parts: %w", err)
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[partEntity])
	if err != nil {
		return nil, fmt.Errorf("get summary parts: %w", err)
	}
	parts, err := entitiesToModels(entities)
	if err != nil {
		return nil, err
	}
	return parts, nil
}

func (r *Repository) transition(ctx context.Context, id uuid.UUID, index int, from, to string, messageID *int64) error {
	query, args := transitionQuery(id, index, from, to, messageID)
	tag, err := r.getter.Get(ctx).Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("transition summary part: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("summary part state conflict")
	}
	return nil
}

func (r *Repository) Begin(ctx context.Context, id uuid.UUID, index int) error {
	return r.transition(ctx, id, index, "pending", "attempting", nil)
}
func (r *Repository) Sent(ctx context.Context, id uuid.UUID, index int, messageID int64) error {
	if messageID <= 0 {
		return errors.New("group message ID must be positive")
	}
	return r.transition(ctx, id, index, "attempting", "sent", &messageID)
}
func (r *Repository) Rejected(ctx context.Context, id uuid.UUID, index int) error {
	return r.transition(ctx, id, index, "attempting", "pending", nil)
}
func (r *Repository) Unknown(ctx context.Context, id uuid.UUID, index int) error {
	return r.transition(ctx, id, index, "attempting", "unknown", nil)
}
func (r *Repository) ListAttemptingChannels(ctx context.Context) ([]int64, error) {
	rows, err := r.getter.Get(ctx).Query(ctx, listAttemptingChannelsQuery())
	if err != nil {
		return nil, fmt.Errorf("list channels with attempting summary parts: %w", err)
	}
	channelIDs, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("list channels with attempting summary parts: %w", err)
	}
	return channelIDs, nil
}
func (r *Repository) RecoverAttempting(ctx context.Context) error {
	_, err := r.getter.Get(ctx).Exec(ctx, `-- recoverAttemptingSummaryParts
UPDATE publication_summary_parts SET status = 'unknown' WHERE status = 'attempting'`)
	if err != nil {
		return fmt.Errorf("recover attempting summary parts: %w", err)
	}
	return nil
}
func (r *Repository) PurgeExpired(ctx context.Context) error {
	_, err := r.getter.Get(ctx).Exec(ctx, `-- purgeExpiredSummaryParts
UPDATE publication_summary_parts SET text = NULL WHERE created_at < now() - interval '30 days' AND text IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("purge expired summary parts: %w", err)
	}
	return nil
}
