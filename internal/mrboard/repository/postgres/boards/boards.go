package boards

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
)

type Repository struct{ getter postgres.Getter }

func New(getter postgres.Getter) *Repository { return &Repository{getter: getter} }

func (r *Repository) GetBoards(ctx context.Context, filters models.GetBoardsFilters) ([]models.Board, error) {
	query, args := getBoardsQuery(filters)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("get MR boards: %w", err)
	}
	entities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[boardEntity])
	if err != nil {
		return nil, fmt.Errorf("get MR boards: %w", err)
	}
	boards := make([]models.Board, 0, len(entities))
	for _, entity := range entities {
		boards = append(boards, boardEntityToModel(entity))
	}
	return boards, nil
}

func (r *Repository) SaveBoard(ctx context.Context, board models.Board) error {
	query, args := saveBoardQuery(board)
	if _, err := r.getter.Get(ctx).Exec(ctx, query, args); err != nil {
		return fmt.Errorf("save MR board: %w", err)
	}
	return nil
}

func (r *Repository) GetTracks(ctx context.Context) ([]models.ReviewerTrack, []models.SectionTrack, error) {
	rows, err := r.getter.Get(ctx).Query(ctx, getReviewerTracksQuery())
	if err != nil {
		return nil, nil, fmt.Errorf("get reviewer states: %w", err)
	}
	reviewerEntities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[reviewerTrackEntity])
	if err != nil {
		return nil, nil, fmt.Errorf("get reviewer states: %w", err)
	}
	rows, err = r.getter.Get(ctx).Query(ctx, getSectionTracksQuery())
	if err != nil {
		return nil, nil, fmt.Errorf("get MR sections: %w", err)
	}
	sectionEntities, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[sectionTrackEntity])
	if err != nil {
		return nil, nil, fmt.Errorf("get MR sections: %w", err)
	}
	reviewers := make([]models.ReviewerTrack, 0, len(reviewerEntities))
	for _, entity := range reviewerEntities {
		reviewers = append(reviewers, reviewerTrackEntityToModel(entity))
	}
	sections := make([]models.SectionTrack, 0, len(sectionEntities))
	for _, entity := range sectionEntities {
		sections = append(sections, sectionTrackEntityToModel(entity))
	}
	return reviewers, sections, nil
}

func (r *Repository) SaveTracks(ctx context.Context, reviewers []models.ReviewerTrack, sections []models.SectionTrack) error {
	query, args := saveTracksQuery(reviewers, sections)
	if _, err := r.getter.Get(ctx).Exec(ctx, query, args); err != nil {
		return fmt.Errorf("save MR tracks: %w", err)
	}
	return nil
}
