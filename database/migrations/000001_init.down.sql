-- Drop in reverse dependency order so foreign keys don't block us.
DROP TABLE IF EXISTS external_team_profiles;
DROP TABLE IF EXISTS match_audit_logs;
DROP TABLE IF EXISTS standings_snapshot_rows;
DROP TABLE IF EXISTS standings_snapshots;
DROP TABLE IF EXISTS matches;
DROP TABLE IF EXISTS league_teams;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS leagues;
