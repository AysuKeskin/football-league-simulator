package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// TeamRepo implements domain.TeamRepository against Postgres.
type TeamRepo struct {
	q Querier
}

func NewTeamRepo(q Querier) *TeamRepo {
	return &TeamRepo{q: q}
}

func (r *TeamRepo) Create(ctx context.Context, team *domain.Team) error {
	if team == nil {
		return fmt.Errorf("%w: team is nil", domain.ErrInvalidInput)
	}
	const stmt = `
		INSERT INTO teams (name, attack, midfield, defense)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`
	err := r.q.QueryRow(ctx, stmt,
		team.Name, team.Attack, team.Midfield, team.Defense,
	).Scan(&team.ID, &team.CreatedAt, &team.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert team: %w", err)
	}
	return nil
}

func (r *TeamRepo) GetByID(ctx context.Context, id int64) (*domain.Team, error) {
	const query = `
		SELECT id, name, attack, midfield, defense, created_at, updated_at
		FROM teams
		WHERE id = $1
	`
	var t domain.Team
	err := r.q.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.Name, &t.Attack, &t.Midfield, &t.Defense, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select team: %w", err)
	}
	return &t, nil
}

// ListByLeague returns the teams that belong to a league, joined through
// league_teams and ordered by name for stable output.
func (r *TeamRepo) ListByLeague(ctx context.Context, leagueID int64) ([]domain.Team, error) {
	const query = `
		SELECT t.id, t.name, t.attack, t.midfield, t.defense, t.created_at, t.updated_at
		FROM teams t
		JOIN league_teams lt ON lt.team_id = t.id
		WHERE lt.league_id = $1
		ORDER BY t.name
	`
	rows, err := r.q.Query(ctx, query, leagueID)
	if err != nil {
		return nil, fmt.Errorf("select league teams: %w", err)
	}
	defer rows.Close()

	var out []domain.Team
	for rows.Next() {
		var t domain.Team
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Attack, &t.Midfield, &t.Defense, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateRating changes only the rating attributes. It deliberately does
// not touch played matches — rating changes affect future simulations
// only, never recorded results.
func (r *TeamRepo) UpdateRating(ctx context.Context, id int64, rating domain.Rating) error {
	const stmt = `
		UPDATE teams
		SET attack = $1, midfield = $2, defense = $3, updated_at = NOW()
		WHERE id = $4
	`
	tag, err := r.q.Exec(ctx, stmt, rating.Attack, rating.Midfield, rating.Defense, id)
	if err != nil {
		return fmt.Errorf("update rating: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
