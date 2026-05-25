package service

import (
	"context"
	"fmt"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// MatchService handles manual edits to match results and their audit
// trail. Editing a played result rewrites the affected standings
// snapshots so the league stays internally consistent.
type MatchService struct {
	repos     domain.Repositories
	tx        domain.Transactor
	standings domain.StandingsCalculator
}

func NewMatchService(repos domain.Repositories, tx domain.Transactor, standings domain.StandingsCalculator) *MatchService {
	return &MatchService{repos: repos, tx: tx, standings: standings}
}

// UpdateResultInput is the service-level request for editing a result.
type UpdateResultInput struct {
	HomeGoals int
	AwayGoals int
	Reason    string
}

// EditResult is returned after a successful edit: the updated match and
// the recomputed current table.
type EditResult struct {
	Match     domain.Match
	Standings []domain.StandingRow
}

// UpdateResult edits a played match's score, records an audit entry, and
// rebuilds the standings snapshots from the edited week onward — all in
// one transaction with the league row locked.
//
// Only PLAYED matches can be edited; editing a scheduled fixture is a
// conflict. Editing is allowed regardless of league status (correcting a
// result after the season has finished is the primary use case).
func (s *MatchService) UpdateResult(ctx context.Context, matchID int64, in UpdateResultInput) (*EditResult, error) {
	if in.HomeGoals < 0 || in.AwayGoals < 0 {
		return nil, fmt.Errorf("%w: goals must be non-negative", domain.ErrInvalidInput)
	}

	var result *EditResult
	err := s.tx.WithinTx(ctx, func(r domain.Repositories) error {
		match, err := r.Matches().GetByID(ctx, matchID)
		if err != nil {
			return err
		}
		// Lock the owning league so a concurrent play/reset cannot
		// interleave with this edit's snapshot rebuild.
		league, err := r.Leagues().GetByIDForUpdate(ctx, match.LeagueID)
		if err != nil {
			return err
		}
		if match.Status != domain.MatchStatusPlayed {
			return fmt.Errorf("%w: only played matches can be edited", domain.ErrConflict)
		}

		// match.HomeGoals/AwayGoals are non-nil because the match is PLAYED.
		audit := &domain.MatchAudit{
			MatchID:      matchID,
			OldHomeGoals: *match.HomeGoals,
			OldAwayGoals: *match.AwayGoals,
			NewHomeGoals: in.HomeGoals,
			NewAwayGoals: in.AwayGoals,
			Reason:       in.Reason,
		}
		if err := r.Audits().Create(ctx, audit); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		if err := r.Matches().UpdateResult(ctx, matchID, in.HomeGoals, in.AwayGoals); err != nil {
			return fmt.Errorf("update result: %w", err)
		}

		// Snapshots from the edited week onward are now stale.
		if err := rebuildSnapshots(ctx, r, s.standings, league, match.WeekNumber); err != nil {
			return err
		}

		updated, err := r.Matches().GetByID(ctx, matchID)
		if err != nil {
			return err
		}
		teams, err := r.Teams().ListByLeague(ctx, league.ID)
		if err != nil {
			return fmt.Errorf("list teams: %w", err)
		}
		all, err := r.Matches().ListByLeague(ctx, league.ID)
		if err != nil {
			return fmt.Errorf("list matches: %w", err)
		}
		result = &EditResult{
			Match:     *updated,
			Standings: s.standings.Calculate(teams, all),
		}
		return nil
	})
	return result, err
}

// GetAudit returns a match's edit history, newest first. Returns
// domain.ErrNotFound when the match does not exist.
func (s *MatchService) GetAudit(ctx context.Context, matchID int64) ([]domain.MatchAudit, error) {
	if _, err := s.repos.Matches().GetByID(ctx, matchID); err != nil {
		return nil, err
	}
	return s.repos.Audits().ListByMatch(ctx, matchID)
}
