package reports

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

type Repository struct{ getter postgres.Getter }

func New(getter postgres.Getter) *Repository { return &Repository{getter: getter} }

func (r *Repository) SaveLatest(ctx context.Context, report models.Report) error {
	query, args := saveLatestQuery(report)
	if _, err := r.getter.Get(ctx).Exec(ctx, query, args); err != nil {
		return fmt.Errorf("save latest report: %w", err)
	}
	return nil
}

func (r *Repository) GetReports(ctx context.Context, filters models.GetReportsFilters) ([]models.Report, error) {
	if filters.SlotID == nil {
		return []models.Report{}, nil
	}
	query, args := getLatestQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("get reports: %w", err)
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[reportEntity])
	if err != nil {
		return nil, fmt.Errorf("get reports: %w", err)
	}
	return entitiesToModels(entities), nil
}

func (r *Repository) GetReportsCount(ctx context.Context, filters models.GetReportsFilters) (int, error) {
	if filters.SlotID == nil {
		return 0, nil
	}
	query, args := getLatestCountQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return 0, fmt.Errorf("count reports: %w", err)
	}
	entity, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[countEntity])
	if err != nil {
		return 0, fmt.Errorf("count reports: %w", err)
	}
	return entity.Count, nil
}

func (r *Repository) CreateSnapshot(ctx context.Context, editionSlotID, sourceSlotID uuid.UUID) error {
	query, args := createSnapshotQuery(editionSlotID, sourceSlotID)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return fmt.Errorf("create report snapshot: %w", err)
	}
	result, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[snapshotCreationEntity])
	if err != nil {
		return fmt.Errorf("create report snapshot: %w", err)
	}
	switch result.Outcome {
	case "created", "existing":
		return nil
	case "source_unavailable":
		return fmt.Errorf("create report snapshot: %w", models.ErrSnapshotSourceUnavailable)
	case "source_conflict":
		return fmt.Errorf("create report snapshot: %w", models.ErrSnapshotSourceConflict)
	case "concurrent_conflict":
		return r.resolveSnapshotConflict(ctx, editionSlotID, sourceSlotID)
	default:
		return fmt.Errorf("create report snapshot: unknown outcome %q", result.Outcome)
	}
}

func (r *Repository) resolveSnapshotConflict(ctx context.Context, editionSlotID, sourceSlotID uuid.UUID) error {
	query, args := getSnapshotEditionQuery(editionSlotID)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return fmt.Errorf("resolve report snapshot conflict: %w", err)
	}
	entity, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[snapshotEditionEntity])
	if err != nil {
		return fmt.Errorf("resolve report snapshot conflict: %w", err)
	}
	if entity.SourceSlotID != sourceSlotID {
		return fmt.Errorf("create report snapshot: %w", models.ErrSnapshotSourceConflict)
	}
	return nil
}

func (r *Repository) GetSnapshots(ctx context.Context, filters models.GetSnapshotsFilters) ([]models.Snapshot, error) {
	if filters.EditionSlotID == nil {
		return []models.Snapshot{}, nil
	}
	query, args := getSnapshotsQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("get report snapshots: %w", err)
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[snapshotEntity])
	if err != nil {
		return nil, fmt.Errorf("get report snapshots: %w", err)
	}
	return snapshotEntitiesToModels(entities), nil
}

func (r *Repository) GetSnapshotsCount(ctx context.Context, filters models.GetSnapshotsFilters) (int, error) {
	if filters.EditionSlotID == nil {
		return 0, nil
	}
	query, args := getSnapshotsCountQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return 0, fmt.Errorf("count report snapshots: %w", err)
	}
	entity, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[countEntity])
	if err != nil {
		return 0, fmt.Errorf("count report snapshots: %w", err)
	}
	return entity.Count, nil
}

func (r *Repository) PurgeExpiredText(ctx context.Context) error {
	if _, err := r.getter.Get(ctx).Exec(ctx, purgeReportsQuery()); err != nil {
		return fmt.Errorf("purge expired report text: %w", err)
	}
	if _, err := r.getter.Get(ctx).Exec(ctx, purgeSnapshotsQuery()); err != nil {
		return fmt.Errorf("purge expired snapshot text: %w", err)
	}
	return nil
}
