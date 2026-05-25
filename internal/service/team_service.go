package service

import (
	"context"
	"fmt"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// ratingMin and ratingMax bound each rating attribute, mirroring the
// CHECK constraints in the schema. Validating here turns out-of-range
// input into a clean ErrInvalidInput (400) instead of a DB CHECK 500.
const (
	ratingMin = 1
	ratingMax = 100
)

// TeamService manages the team catalog: listing a league's teams and
// updating a team's ratings. Writes are single statements, so it needs
// no transactor — pool-backed repositories suffice.
type TeamService struct {
	repos domain.Repositories
}

func NewTeamService(repos domain.Repositories) *TeamService {
	return &TeamService{repos: repos}
}

// UpdateRatingInput is the full rating triple; all three are replaced.
type UpdateRatingInput struct {
	Attack   int
	Midfield int
	Defense  int
}

// UpdateRating replaces a team's ratings and returns the updated team.
//
// The change affects only future simulation and predictions; already
// recorded match results are fixed and are not touched here.
func (s *TeamService) UpdateRating(ctx context.Context, teamID int64, in UpdateRatingInput) (*domain.Team, error) {
	for _, v := range []struct {
		name string
		val  int
	}{{"attack", in.Attack}, {"midfield", in.Midfield}, {"defense", in.Defense}} {
		if v.val < ratingMin || v.val > ratingMax {
			return nil, fmt.Errorf("%w: %s must be between %d and %d", domain.ErrInvalidInput, v.name, ratingMin, ratingMax)
		}
	}

	rating := domain.Rating{Attack: in.Attack, Midfield: in.Midfield, Defense: in.Defense}
	if err := s.repos.Teams().UpdateRating(ctx, teamID, rating); err != nil {
		return nil, err
	}
	return s.repos.Teams().GetByID(ctx, teamID)
}

// ListByLeague returns the teams that belong to a league. Returns
// domain.ErrNotFound when the league does not exist.
func (s *TeamService) ListByLeague(ctx context.Context, leagueID int64) ([]domain.Team, error) {
	if _, err := s.repos.Leagues().GetByID(ctx, leagueID); err != nil {
		return nil, err
	}
	return s.repos.Teams().ListByLeague(ctx, leagueID)
}
