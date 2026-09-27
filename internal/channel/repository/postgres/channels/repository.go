package channels

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
)

type Repository struct{ getter postgres.Getter }

func New(getter postgres.Getter) *Repository { return &Repository{getter: getter} }

func (r *Repository) GetChannels(ctx context.Context, filters models.GetChannelsFilters) ([]models.Channel, error) {
	query, args := getChannelsQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("get channels: %w", err)
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[channelEntity])
	if err != nil {
		return nil, fmt.Errorf("get channels: %w", err)
	}
	channels := make([]models.Channel, 0, len(entities))
	for _, entity := range entities {
		channels = append(channels, entityToModel(entity))
	}
	return channels, nil
}

func (r *Repository) GetSchedules(ctx context.Context, filters models.GetSchedulesFilters) ([]models.Schedule, error) {
	query, args := getSchedulesQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("get channel schedules: %w", err)
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[scheduleEntity])
	if err != nil {
		return nil, fmt.Errorf("get channel schedules: %w", err)
	}
	return scheduleEntitiesToModels(entities), nil
}

func (r *Repository) SetSchedule(ctx context.Context, schedule models.Schedule, keepOverride bool) error {
	query, args := setScheduleQuery(schedule, keepOverride)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return fmt.Errorf("set channel schedule: %w", err)
	}
	_, err = pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[struct {
		ChannelID int64 `db:"channel_id"`
	}])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ErrInvalidChannelID
	}
	if err != nil {
		return fmt.Errorf("set channel schedule: %w", err)
	}
	return nil
}

func (r *Repository) SetScheduleCheckedAt(ctx context.Context, channelID int64, checkedAt time.Time) error {
	query, args := setScheduleCheckedAtQuery(channelID, checkedAt)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return fmt.Errorf("set schedule watermark: %w", err)
	}
	_, err = pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[struct {
		ChannelID int64 `db:"channel_id"`
	}])
	if err != nil {
		return fmt.Errorf("set schedule watermark: %w", err)
	}
	return nil
}

func (r *Repository) SetOverride(ctx context.Context, channelID int64, target time.Time) error {
	query, args := setOverrideQuery(channelID, target)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return fmt.Errorf("set channel move: %w", err)
	}
	_, err = pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[struct {
		ChannelID int64 `db:"channel_id"`
	}])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ErrInvalidSchedule
	}
	if err != nil {
		return fmt.Errorf("set channel move: %w", err)
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, channel models.Channel) (models.Channel, error) {
	query, args := createQuery(channel)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return models.Channel{}, fmt.Errorf("create channel: %w", err)
	}
	stored, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[channelEntity])
	if err == nil {
		return entityToModel(stored), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return models.Channel{}, fmt.Errorf("create channel: %w", err)
	}
	query, args = getConnectionConflictQuery(channel)
	rows, err = r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return models.Channel{}, fmt.Errorf("get channel connection: %w", err)
	}
	stored, err = pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[channelEntity])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Channel{}, errors.New("channel connection conflict")
	}
	if err != nil {
		return models.Channel{}, fmt.Errorf("get channel connection: %w", err)
	}
	if stored.ChannelID == channel.ChannelID && stored.GroupID == channel.GroupID {
		return entityToModel(stored), nil
	}
	if stored.ChannelID == channel.ChannelID {
		return models.Channel{}, models.ErrAlreadyConnected
	}
	return models.Channel{}, models.ErrGroupAlreadyConnected
}
