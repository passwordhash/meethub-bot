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
	query, args = getDailyTimesQuery(filters)
	rows, err = r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("get channel daily times: %w", err)
	}
	dailyTimes, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[dailyTimeEntity])
	if err != nil {
		return nil, fmt.Errorf("get channel daily times: %w", err)
	}
	return scheduleEntitiesToModels(entities, dailyTimes), nil
}

func (r *Repository) SetDailyTime(ctx context.Context, channelID int64, weekday time.Weekday, startsAt time.Duration) error {
	query, args := setDailyTimeQuery(channelID, weekday, startsAt)
	return r.execForChannel(ctx, "set channel daily time", query, args)
}

func (r *Repository) DeleteDailyTime(ctx context.Context, channelID int64, weekday time.Weekday) error {
	query, args := deleteDailyTimeQuery(channelID, weekday)
	if _, err := r.getter.Get(ctx).Exec(ctx, query, args); err != nil {
		return fmt.Errorf("delete channel daily time: %w", err)
	}
	return nil
}

func (r *Repository) SetPostOffset(ctx context.Context, channelID int64, minutes int) error {
	query, args := setPostOffsetQuery(channelID, minutes)
	return r.execForChannel(ctx, "set channel post offset", query, args)
}

func (r *Repository) SetLeadEnabled(ctx context.Context, channelID int64, enabled bool) error {
	query, args := setLeadEnabledQuery(channelID, enabled)
	return r.execForChannel(ctx, "set daily lead setting", query, args)
}

func (r *Repository) SetBoardView(ctx context.Context, channelID int64, view models.BoardView) error {
	query, args := setBoardViewQuery(channelID, view)
	return r.execForChannel(ctx, "set MR board view", query, args)
}

func (r *Repository) ClearScheduleReset(ctx context.Context, channelID int64) error {
	query, args := clearScheduleResetQuery(channelID)
	return r.execForChannel(ctx, "clear schedule reset notice", query, args)
}

func (r *Repository) GetIgnoredUsers(ctx context.Context, filters models.GetIgnoredUsersFilters) ([]models.IgnoredUser, error) {
	query, args := getIgnoredUsersQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("get ignored users: %w", err)
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[ignoredUserEntity])
	if err != nil {
		return nil, fmt.Errorf("get ignored users: %w", err)
	}
	users := make([]models.IgnoredUser, 0, len(entities))
	for _, entity := range entities {
		users = append(users, ignoredUserEntityToModel(entity))
	}
	return users, nil
}

func (r *Repository) AddIgnoredUser(ctx context.Context, channelID, userID int64) error {
	query, args := addIgnoredUserQuery(channelID, userID)
	return r.execForChannel(ctx, "add ignored user", query, args)
}

func (r *Repository) DeleteIgnoredUser(ctx context.Context, channelID, userID int64) error {
	query, args := deleteIgnoredUserQuery(channelID, userID)
	if _, err := r.getter.Get(ctx).Exec(ctx, query, args); err != nil {
		return fmt.Errorf("delete ignored user: %w", err)
	}
	return nil
}

func (r *Repository) GetParticipants(ctx context.Context, filters models.GetParticipantsFilters) ([]models.Participant, error) {
	query, args := getParticipantsQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("get participants: %w", err)
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[participantEntity])
	if err != nil {
		return nil, fmt.Errorf("get participants: %w", err)
	}
	participants := make([]models.Participant, 0, len(entities))
	for _, entity := range entities {
		participants = append(participants, participantEntityToModel(entity))
	}
	return participants, nil
}

func (r *Repository) SetParticipant(ctx context.Context, participant models.Participant) error {
	query, args := setParticipantQuery(participant)
	return r.execForChannel(ctx, "set participant", query, args)
}

func (r *Repository) DeleteParticipant(ctx context.Context, channelID int64, gitLabUsername string) error {
	query, args := deleteParticipantQuery(channelID, gitLabUsername)
	if _, err := r.getter.Get(ctx).Exec(ctx, query, args); err != nil {
		return fmt.Errorf("delete participant: %w", err)
	}
	return nil
}

func (r *Repository) execForChannel(ctx context.Context, operation, query string, args pgx.NamedArgs) error {
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	_, err = pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[struct {
		ChannelID int64 `db:"channel_id"`
	}])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ErrInvalidChannelID
	}
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
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
