package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// MatchRepo implements domain.MatchRepository against Postgres.
type MatchRepo struct {
	q Querier
}

func NewMatchRepo(q Querier) *MatchRepo {
	return &MatchRepo{q: q}
}

// BulkCreate inserts a whole fixture list in one statement. Matches are
// created in SCHEDULED state with NULL goals; goals are filled later by
// UpdateResult when the week is played.
func (r *MatchRepo) BulkCreate(ctx context.Context, matches []domain.Match) error {
	if len(matches) == 0 {
		return nil
	}

	// Assemble a single multi-row INSERT: one round-trip for the entire
	// schedule regardless of match count.
	var sb strings.Builder
	sb.WriteString(`INSERT INTO matches (league_id, week_number, home_team_id, away_team_id, status) VALUES `)
	args := make([]any, 0, len(matches)*5)
	for i, m := range matches {
		if i > 0 {
			sb.WriteByte(',')
		}
		base := i * 5
		fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4, base+5)
		args = append(args, m.LeagueID, m.WeekNumber, m.HomeTeamID, m.AwayTeamID, string(domain.MatchStatusScheduled))
	}

	if _, err := r.q.Exec(ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("bulk insert matches: %w", err)
	}
	return nil
}

func (r *MatchRepo) GetByID(ctx context.Context, id int64) (*domain.Match, error) {
	const query = `
		SELECT id, league_id, week_number, home_team_id, away_team_id,
		       home_goals, away_goals, status, played_at
		FROM matches
		WHERE id = $1
	`
	m, err := scanMatch(r.q.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select match: %w", err)
	}
	return m, nil
}

func (r *MatchRepo) ListByLeague(ctx context.Context, leagueID int64) ([]domain.Match, error) {
	const query = `
		SELECT id, league_id, week_number, home_team_id, away_team_id,
		       home_goals, away_goals, status, played_at
		FROM matches
		WHERE league_id = $1
		ORDER BY week_number, id
	`
	return r.queryMatches(ctx, query, leagueID)
}

func (r *MatchRepo) ListByLeagueAndWeek(ctx context.Context, leagueID int64, week int) ([]domain.Match, error) {
	const query = `
		SELECT id, league_id, week_number, home_team_id, away_team_id,
		       home_goals, away_goals, status, played_at
		FROM matches
		WHERE league_id = $1 AND week_number = $2
		ORDER BY id
	`
	return r.queryMatches(ctx, query, leagueID, week)
}

// UpdateResult records a score and flips the match to PLAYED. Setting
// both goals together satisfies the matches_played_goals_consistent
// CHECK constraint (PLAYED rows must have non-NULL goals).
func (r *MatchRepo) UpdateResult(ctx context.Context, id int64, homeGoals, awayGoals int) error {
	const stmt = `
		UPDATE matches
		SET home_goals = $1, away_goals = $2, status = 'PLAYED',
		    played_at = NOW(), updated_at = NOW()
		WHERE id = $3
	`
	tag, err := r.q.Exec(ctx, stmt, homeGoals, awayGoals, id)
	if err != nil {
		return fmt.Errorf("update match result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ClearResults resets every match in a league to SCHEDULED with NULL
// goals. Setting status and goals together satisfies the
// matches_played_goals_consistent CHECK (SCHEDULED rows must have NULL
// goals).
func (r *MatchRepo) ClearResults(ctx context.Context, leagueID int64) error {
	const stmt = `
		UPDATE matches
		SET home_goals = NULL, away_goals = NULL, status = 'SCHEDULED',
		    played_at = NULL, updated_at = NOW()
		WHERE league_id = $1
	`
	if _, err := r.q.Exec(ctx, stmt, leagueID); err != nil {
		return fmt.Errorf("clear match results: %w", err)
	}
	return nil
}

// queryMatches runs a multi-row match query and scans every row.
func (r *MatchRepo) queryMatches(ctx context.Context, query string, args ...any) ([]domain.Match, error) {
	rows, err := r.q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select matches: %w", err)
	}
	defer rows.Close()

	var out []domain.Match
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, fmt.Errorf("scan match: %w", err)
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// rowScanner abstracts pgx.Row and pgx.Rows so a single scanMatch helper
// serves both single-row and multi-row queries.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanMatch reads one match row. home_goals/away_goals/played_at are
// nullable in the schema, so they scan into the *int / *time.Time
// pointer fields directly — pgx leaves them nil when the column is NULL.
func scanMatch(row rowScanner) (*domain.Match, error) {
	var m domain.Match
	var status string
	if err := row.Scan(
		&m.ID, &m.LeagueID, &m.WeekNumber, &m.HomeTeamID, &m.AwayTeamID,
		&m.HomeGoals, &m.AwayGoals, &status, &m.PlayedAt,
	); err != nil {
		return nil, err
	}
	m.Status = domain.MatchStatus(status)
	return &m, nil
}
