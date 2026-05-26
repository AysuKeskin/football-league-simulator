package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// StandingsSnapshotRepo implements domain.StandingsSnapshotRepository.
//
// A snapshot is the cached league table at the end of a given week.
// Two tables back it: standings_snapshots (one row per league+week) and
// standings_snapshot_rows (one row per team in that snapshot). The
// snapshot row owns its detail rows via ON DELETE CASCADE.
type StandingsSnapshotRepo struct {
	q Querier
}

func NewStandingsSnapshotRepo(q Querier) *StandingsSnapshotRepo {
	return &StandingsSnapshotRepo{q: q}
}

// Upsert writes (or overwrites) the snapshot for one league+week.
//
// It is a three-step sequence: ensure the snapshot row exists, clear any
// previous detail rows, then insert the new ones. Re-running a week must
// replace its rows wholesale, which is why old rows are deleted first.
// Callers that need the three steps to be atomic should pass a pgx.Tx.
func (r *StandingsSnapshotRepo) Upsert(ctx context.Context, leagueID int64, week int, rows []domain.StandingRow) error {
	// Bump captured_at on conflict so re-running a week refreshes its
	// timestamp (UNIQUE on league_id+week_number).
	const upsertSnapshot = `
		INSERT INTO standings_snapshots (league_id, week_number)
		VALUES ($1, $2)
		ON CONFLICT (league_id, week_number)
		DO UPDATE SET captured_at = NOW()
		RETURNING id
	`
	var snapshotID int64
	if err := r.q.QueryRow(ctx, upsertSnapshot, leagueID, week).Scan(&snapshotID); err != nil {
		return fmt.Errorf("upsert snapshot: %w", err)
	}

	// Clear previous detail rows first. A plain INSERT ... ON CONFLICT on
	// the rows table would leave stale rows behind when the new row-set is
	// smaller (e.g. a team removed from the league), so we replace wholesale.
	if _, err := r.q.Exec(ctx,
		`DELETE FROM standings_snapshot_rows WHERE snapshot_id = $1`, snapshotID,
	); err != nil {
		return fmt.Errorf("clear snapshot rows: %w", err)
	}

	if len(rows) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(`
		INSERT INTO standings_snapshot_rows
			(snapshot_id, team_id, rank, played, won, drawn, lost,
			 goals_for, goals_against, goal_difference, points)
		VALUES `)
	const cols = 11
	args := make([]any, 0, len(rows)*cols)
	for i, row := range rows {
		if i > 0 {
			sb.WriteByte(',')
		}
		b := i * cols
		fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			b+1, b+2, b+3, b+4, b+5, b+6, b+7, b+8, b+9, b+10, b+11)
		args = append(args,
			snapshotID, row.TeamID, row.Rank, row.Played, row.Won, row.Drawn, row.Lost,
			row.GoalsFor, row.GoalsAgainst, row.GoalDifference, row.Points,
		)
	}
	if _, err := r.q.Exec(ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("insert snapshot rows: %w", err)
	}
	return nil
}

// GetByWeek returns the cached table for one week, joined with team
// names. Returns an empty slice (not an error) when no snapshot exists
// for that week — the caller decides whether absence is meaningful.
func (r *StandingsSnapshotRepo) GetByWeek(ctx context.Context, leagueID int64, week int) ([]domain.StandingRow, error) {
	const query = `
		SELECT sr.rank, sr.team_id, t.name,
		       sr.played, sr.won, sr.drawn, sr.lost,
		       sr.goals_for, sr.goals_against, sr.goal_difference, sr.points
		FROM standings_snapshots s
		JOIN standings_snapshot_rows sr ON sr.snapshot_id = s.id
		JOIN teams t                    ON t.id = sr.team_id
		WHERE s.league_id = $1 AND s.week_number = $2
		ORDER BY sr.rank
	`
	rows, err := r.q.Query(ctx, query, leagueID, week)
	if err != nil {
		return nil, fmt.Errorf("select snapshot: %w", err)
	}
	defer rows.Close()
	return scanStandingRows(rows)
}

// ListHistory returns every snapshot for a league, week-ascending, each
// carrying its captured_at timestamp. Rows arrive ordered by week then
// rank, so we fold consecutive rows into one snapshot per week.
func (r *StandingsSnapshotRepo) ListHistory(ctx context.Context, leagueID int64) ([]domain.StandingsSnapshot, error) {
	const query = `
		SELECT s.week_number, s.captured_at,
		       sr.rank, sr.team_id, t.name,
		       sr.played, sr.won, sr.drawn, sr.lost,
		       sr.goals_for, sr.goals_against, sr.goal_difference, sr.points
		FROM standings_snapshots s
		JOIN standings_snapshot_rows sr ON sr.snapshot_id = s.id
		JOIN teams t                    ON t.id = sr.team_id
		WHERE s.league_id = $1
		ORDER BY s.week_number, sr.rank
	`
	rows, err := r.q.Query(ctx, query, leagueID)
	if err != nil {
		return nil, fmt.Errorf("select snapshots: %w", err)
	}
	defer rows.Close()

	var out []domain.StandingsSnapshot
	for rows.Next() {
		var week int
		var capturedAt time.Time
		var sr domain.StandingRow
		if err := rows.Scan(
			&week, &capturedAt,
			&sr.Rank, &sr.TeamID, &sr.TeamName,
			&sr.Played, &sr.Won, &sr.Drawn, &sr.Lost,
			&sr.GoalsFor, &sr.GoalsAgainst, &sr.GoalDifference, &sr.Points,
		); err != nil {
			return nil, fmt.Errorf("scan snapshot row: %w", err)
		}
		// Start a new snapshot when the week changes (rows are week-ordered).
		if len(out) == 0 || out[len(out)-1].Week != week {
			out = append(out, domain.StandingsSnapshot{Week: week, CapturedAt: capturedAt})
		}
		last := &out[len(out)-1]
		last.Rows = append(last.Rows, sr)
	}
	return out, rows.Err()
}

// DeleteFromWeek removes snapshots for week >= fromWeek. Used after a
// match-result edit: every snapshot at or after the edited week is now
// stale and must be recomputed. Detail rows go with their snapshot via
// ON DELETE CASCADE.
func (r *StandingsSnapshotRepo) DeleteFromWeek(ctx context.Context, leagueID int64, fromWeek int) error {
	const stmt = `DELETE FROM standings_snapshots WHERE league_id = $1 AND week_number >= $2`
	if _, err := r.q.Exec(ctx, stmt, leagueID, fromWeek); err != nil {
		return fmt.Errorf("delete snapshots from week: %w", err)
	}
	return nil
}

// scanStandingRows reads detail rows that include the joined team name.
func scanStandingRows(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]domain.StandingRow, error) {
	var out []domain.StandingRow
	for rows.Next() {
		var sr domain.StandingRow
		if err := rows.Scan(
			&sr.Rank, &sr.TeamID, &sr.TeamName,
			&sr.Played, &sr.Won, &sr.Drawn, &sr.Lost,
			&sr.GoalsFor, &sr.GoalsAgainst, &sr.GoalDifference, &sr.Points,
		); err != nil {
			return nil, fmt.Errorf("scan standing row: %w", err)
		}
		out = append(out, sr)
	}
	return out, rows.Err()
}
