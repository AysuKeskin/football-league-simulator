package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// LeagueRepo implements domain.LeagueRepository against Postgres.
//
// Create writes both the league row and its league_teams memberships.
// Callers that need atomicity across both inserts should construct the
// repo with a pgx.Tx; constructed with a *pgxpool.Pool the two inserts
// are sequential but not atomic.
type LeagueRepo struct {
	q Querier
}

func NewLeagueRepo(q Querier) *LeagueRepo {
	return &LeagueRepo{q: q}
}

func (r *LeagueRepo) Create(ctx context.Context, league *domain.League, teamIDs []int64) error {
	if league == nil {
		return fmt.Errorf("%w: league is nil", domain.ErrInvalidInput)
	}

	const insertLeague = `
		INSERT INTO leagues (name, current_week, total_weeks, status, random_seed)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	err := r.q.QueryRow(ctx, insertLeague,
		league.Name, league.CurrentWeek, league.TotalWeeks, string(league.Status), league.RandomSeed,
	).Scan(&league.ID, &league.CreatedAt, &league.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert league: %w", err)
	}

	if len(teamIDs) == 0 {
		return nil
	}

	// Build a single multi-row INSERT so we hit the DB once regardless
	// of team count. pgx handles the slice expansion via $-placeholders
	// generated in a loop.
	const insertMembership = `INSERT INTO league_teams (league_id, team_id) VALUES `
	values := make([]any, 0, 1+len(teamIDs))
	values = append(values, league.ID)
	placeholders := ""
	for i, teamID := range teamIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += fmt.Sprintf("($1,$%d)", i+2)
		values = append(values, teamID)
	}
	if _, err := r.q.Exec(ctx, insertMembership+placeholders, values...); err != nil {
		return fmt.Errorf("insert league_teams: %w", err)
	}
	return nil
}

func (r *LeagueRepo) GetByID(ctx context.Context, id int64) (*domain.League, error) {
	const query = `
		SELECT id, name, current_week, total_weeks, status, random_seed, created_at, updated_at
		FROM leagues
		WHERE id = $1
	`
	var league domain.League
	var status string
	err := r.q.QueryRow(ctx, query, id).Scan(
		&league.ID, &league.Name, &league.CurrentWeek, &league.TotalWeeks,
		&status, &league.RandomSeed, &league.CreatedAt, &league.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select league: %w", err)
	}
	league.Status = domain.LeagueStatus(status)
	return &league, nil
}

func (r *LeagueRepo) List(ctx context.Context) ([]domain.League, error) {
	const query = `
		SELECT id, name, current_week, total_weeks, status, random_seed, created_at, updated_at
		FROM leagues
		ORDER BY created_at DESC
	`
	rows, err := r.q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("select leagues: %w", err)
	}
	defer rows.Close()

	var out []domain.League
	for rows.Next() {
		var l domain.League
		var status string
		if err := rows.Scan(
			&l.ID, &l.Name, &l.CurrentWeek, &l.TotalWeeks,
			&status, &l.RandomSeed, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan league: %w", err)
		}
		l.Status = domain.LeagueStatus(status)
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *LeagueRepo) UpdateStatusAndWeek(ctx context.Context, id int64, status domain.LeagueStatus, currentWeek int) error {
	const stmt = `
		UPDATE leagues
		SET status = $1, current_week = $2, updated_at = NOW()
		WHERE id = $3
	`
	tag, err := r.q.Exec(ctx, stmt, string(status), currentWeek, id)
	if err != nil {
		return fmt.Errorf("update league: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *LeagueRepo) Delete(ctx context.Context, id int64) error {
	const stmt = `DELETE FROM leagues WHERE id = $1`
	tag, err := r.q.Exec(ctx, stmt, id)
	if err != nil {
		return fmt.Errorf("delete league: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
