DROP INDEX IF EXISTS league_teams_team_idx;
ALTER TABLE matches DROP CONSTRAINT IF EXISTS matches_unique_fixture;
ALTER TABLE matches DROP CONSTRAINT IF EXISTS matches_played_goals_consistent;
