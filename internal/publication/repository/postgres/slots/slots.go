package slots

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
	"github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

type Repository struct{ getter postgres.Getter }

func New(getter postgres.Getter) *Repository { return &Repository{getter: getter} }

func (r *Repository) Create(ctx context.Context, channelID int64, scheduledAt time.Time) (models.Slot, error) {
	query, args := createQuery(channelID, scheduledAt)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return models.Slot{}, fmt.Errorf("create publication slot: %w", err)
	}
	entity, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[slotEntity])
	if errors.Is(err, pgx.ErrNoRows) {
		query, args = getByOccurrenceQuery(channelID, scheduledAt)
		rows, err = r.getter.Get(ctx).Query(ctx, query, args)
		if err != nil {
			return models.Slot{}, fmt.Errorf("get replayed publication slot: %w", err)
		}
		entity, err = pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[slotEntity])
	}
	if err != nil {
		return models.Slot{}, fmt.Errorf("create publication slot: %w", err)
	}
	return entityToModel(entity), nil
}

func (r *Repository) Recover(ctx context.Context) ([]models.Slot, error) {
	query, args := recoverSlotsQuery()
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("recover publication slots: %w", err)
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[slotEntity])
	if err != nil {
		return nil, fmt.Errorf("recover publication slots: %w", err)
	}
	return entitiesToModels(entities), nil
}

func (r *Repository) GetSlots(ctx context.Context, filters models.GetSlotsFilters) ([]models.Slot, error) {
	query, args := getSlotsQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("get publication slots: %w", err)
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[slotEntity])
	if err != nil {
		return nil, fmt.Errorf("get publication slots: %w", err)
	}
	return entitiesToModels(entities), nil
}

func (r *Repository) MarkLinkAlerted(ctx context.Context, id uuid.UUID) error {
	query, args := markLinkAlertedQuery(id)
	tag, err := r.getter.Get(ctx).Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("mark publication link alert: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("publication link alert was not marked")
	}
	return nil
}

func (r *Repository) MarkPublishAlerted(ctx context.Context, id uuid.UUID) error {
	query, args := markPublishAlertedQuery(id)
	tag, err := r.getter.Get(ctx).Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("mark publication failure alert: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("publication failure alert was not marked")
	}
	return nil
}

func (r *Repository) GetSlotsCount(ctx context.Context, filters models.GetSlotsFilters) (int, error) {
	query, args := getSlotsCountQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return 0, fmt.Errorf("count publication slots: %w", err)
	}
	entity, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[slotsCountEntity])
	if err != nil {
		return 0, fmt.Errorf("count publication slots: %w", err)
	}
	return entity.Count, nil
}

func (r *Repository) BindLink(ctx context.Context, id uuid.UUID, channelID, groupID, channelMessageID, groupMessageID int64) (models.Slot, error) {
	query, args := bindLinkQuery(id, channelID, groupID, channelMessageID, groupMessageID)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return models.Slot{}, fmt.Errorf("bind publication link: %w", err)
	}
	entity, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[slotEntity])
	if err != nil {
		return models.Slot{}, fmt.Errorf("bind publication link: %w", err)
	}
	return entityToModel(entity), nil
}

func (r *Repository) Transition(ctx context.Context, id uuid.UUID, from, to models.State, channelMessageID, linkedMessageID, leadUserID *int64) (models.Slot, error) {
	query, args := setStateQuery(id, from, to, channelMessageID, linkedMessageID, leadUserID)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return models.Slot{}, fmt.Errorf("transition publication slot: %w", err)
	}
	entity, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[slotEntity])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Slot{}, errors.New("publication slot state conflict")
	}
	if err != nil {
		return models.Slot{}, fmt.Errorf("transition publication slot: %w", err)
	}
	return entityToModel(entity), nil
}

func (r *Repository) SetLead(ctx context.Context, id uuid.UUID, leadID *int64) (models.Slot, error) {
	query, args := setLeadQuery(id, leadID)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return models.Slot{}, fmt.Errorf("set publication lead: %w", err)
	}
	entity, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[slotEntity])
	if err != nil {
		return models.Slot{}, fmt.Errorf("set publication lead: %w", err)
	}
	return entityToModel(entity), nil
}
