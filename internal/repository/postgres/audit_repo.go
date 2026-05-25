package postgres

import (
	"context"
	"fmt"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// MatchAuditRepo implements domain.MatchAuditRepository. Audit rows are
// append-only: Create inserts, nothing updates or deletes them.
type MatchAuditRepo struct {
	q Querier
}

func NewMatchAuditRepo(q Querier) *MatchAuditRepo {
	return &MatchAuditRepo{q: q}
}

func (r *MatchAuditRepo) Create(ctx context.Context, audit *domain.MatchAudit) error {
	if audit == nil {
		return fmt.Errorf("%w: audit is nil", domain.ErrInvalidInput)
	}
	const stmt = `
		INSERT INTO match_audit_logs
			(match_id, old_home_goals, old_away_goals, new_home_goals, new_away_goals, reason)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, changed_at
	`
	err := r.q.QueryRow(ctx, stmt,
		audit.MatchID, audit.OldHomeGoals, audit.OldAwayGoals,
		audit.NewHomeGoals, audit.NewAwayGoals, audit.Reason,
	).Scan(&audit.ID, &audit.ChangedAt)
	if err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
}

// ListByMatch returns a match's edit history, newest first.
func (r *MatchAuditRepo) ListByMatch(ctx context.Context, matchID int64) ([]domain.MatchAudit, error) {
	const query = `
		SELECT id, match_id, old_home_goals, old_away_goals,
		       new_home_goals, new_away_goals, reason, changed_at
		FROM match_audit_logs
		WHERE match_id = $1
		ORDER BY changed_at DESC, id DESC
	`
	rows, err := r.q.Query(ctx, query, matchID)
	if err != nil {
		return nil, fmt.Errorf("select audits: %w", err)
	}
	defer rows.Close()

	var out []domain.MatchAudit
	for rows.Next() {
		var a domain.MatchAudit
		if err := rows.Scan(
			&a.ID, &a.MatchID, &a.OldHomeGoals, &a.OldAwayGoals,
			&a.NewHomeGoals, &a.NewAwayGoals, &a.Reason, &a.ChangedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
