-- Tighten schema invariants surfaced.
--
-- 1. matches: a played row must have both goal columns set; a scheduled
--    row must have both NULL. Keeps the lifecycle invariant in the DB
--    instead of trusting every caller to maintain it.
-- 2. matches: no duplicate fixtures within a league. The fixture
--    generator should never produce the same (week, home, away) twice;
--    if it does, we want the insert to fail loudly.
-- 3. league_teams: an index on team_id so reverse lookups (e.g. "which
--    leagues is this team in?") do not seq-scan.

ALTER TABLE matches
    ADD CONSTRAINT matches_played_goals_consistent CHECK (
        (status = 'PLAYED'    AND home_goals IS NOT NULL AND away_goals IS NOT NULL) OR
        (status = 'SCHEDULED' AND home_goals IS NULL     AND away_goals IS NULL)
    );

ALTER TABLE matches
    ADD CONSTRAINT matches_unique_fixture UNIQUE (league_id, week_number, home_team_id, away_team_id);

CREATE INDEX league_teams_team_idx ON league_teams (team_id);
