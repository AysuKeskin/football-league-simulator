package service

import (
	"context"
	"fmt"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// rebuildSnapshots recomputes and overwrites the standings snapshot for
// every week in [fromWeek, league.CurrentWeek].
//
// Each week K's snapshot is the cumulative table through week K, so it
// is computed from the matches with WeekNumber <= K (the calculator
// ignores any that are not yet PLAYED). Editing a result in week W only
// affects snapshots from W onward; earlier weeks don't include that
// match, so callers pass fromWeek = W to avoid needless rewrites.
//
// Must run inside a transaction (it issues multiple writes); callers
// invoke it from within Transactor.WithinTx.
func rebuildSnapshots(ctx context.Context, r domain.Repositories, calc domain.StandingsCalculator, league *domain.League, fromWeek int) error {
	if fromWeek < 1 {
		fromWeek = 1
	}

	teams, err := r.Teams().ListByLeague(ctx, league.ID)
	if err != nil {
		return fmt.Errorf("list teams: %w", err)
	}
	all, err := r.Matches().ListByLeague(ctx, league.ID)
	if err != nil {
		return fmt.Errorf("list matches: %w", err)
	}

	for week := fromWeek; week <= league.CurrentWeek; week++ {
		upto := make([]domain.Match, 0, len(all))
		for _, m := range all {
			if m.WeekNumber <= week {
				upto = append(upto, m)
			}
		}
		rows := calc.Calculate(teams, upto)
		if err := r.Snapshots().Upsert(ctx, league.ID, week, rows); err != nil {
			return fmt.Errorf("upsert week %d snapshot: %w", week, err)
		}
	}
	return nil
}
