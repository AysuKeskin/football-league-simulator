-- =============================================================================
-- Football League Simulator — canonical schema
-- =============================================================================
-- This file is the deliverable copy of the database schema. It mirrors the
-- migration in database/migrations/000001_init.up.sql exactly; changes must
-- be applied to both files in the same commit.
--
-- Tables, in dependency order:
--   leagues                    - one row per simulated competition
--   teams                      - one row per club, reusable across leagues
--   league_teams               - many-to-many between leagues and teams
--   matches                    - every fixture in every league
--   standings_snapshots        - per-week cached league tables
--   standings_snapshot_rows    - one row per team per snapshot
--   match_audit_logs           - immutable history of match-result edits
--   external_team_profiles     - cached payloads from upstream metadata APIs
-- =============================================================================

-- One row per simulated competition. A league owns its fixtures, snapshots,
-- and audit log entries (ON DELETE CASCADE).
CREATE TABLE leagues (
    id           BIGSERIAL    PRIMARY KEY,
    name         TEXT         NOT NULL,
    current_week INT          NOT NULL DEFAULT 0 CHECK (current_week >= 0),
    total_weeks  INT          NOT NULL          CHECK (total_weeks > 0),
    status       TEXT         NOT NULL          CHECK (status IN ('NOT_STARTED','IN_PROGRESS','FINISHED')),
    random_seed  BIGINT       NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- One row per club. Teams exist independently of any league so they can be
-- reused across multiple simulations.
CREATE TABLE teams (
    id         BIGSERIAL    PRIMARY KEY,
    name       TEXT         NOT NULL UNIQUE,
    attack     INT          NOT NULL CHECK (attack   BETWEEN 1 AND 100),
    midfield   INT          NOT NULL CHECK (midfield BETWEEN 1 AND 100),
    defense    INT          NOT NULL CHECK (defense  BETWEEN 1 AND 100),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Join table for league membership. Composite PK prevents duplicate
-- entries; the secondary index supports reverse lookups by team.
CREATE TABLE league_teams (
    league_id BIGINT NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    team_id   BIGINT NOT NULL REFERENCES teams(id),
    PRIMARY KEY (league_id, team_id)
);

CREATE INDEX league_teams_team_idx ON league_teams (team_id);

-- One row per scheduled or played fixture. Goal columns are NULL until the
-- match is played; status is the explicit lifecycle marker.
CREATE TABLE matches (
    id           BIGSERIAL    PRIMARY KEY,
    league_id    BIGINT       NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    week_number  INT          NOT NULL CHECK (week_number >= 1),
    home_team_id BIGINT       NOT NULL REFERENCES teams(id),
    away_team_id BIGINT       NOT NULL REFERENCES teams(id),
    home_goals   INT          CHECK (home_goals IS NULL OR home_goals >= 0),
    away_goals   INT          CHECK (away_goals IS NULL OR away_goals >= 0),
    status       TEXT         NOT NULL DEFAULT 'SCHEDULED' CHECK (status IN ('SCHEDULED','PLAYED')),
    played_at    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CHECK (home_team_id <> away_team_id),
    -- Lifecycle invariant: played rows have both goals set, scheduled
    -- rows have neither. Prevents drift between status and result.
    CONSTRAINT matches_played_goals_consistent CHECK (
        (status = 'PLAYED'    AND home_goals IS NOT NULL AND away_goals IS NOT NULL) OR
        (status = 'SCHEDULED' AND home_goals IS NULL     AND away_goals IS NULL)
    ),
    -- A fixture is unique within a league week+pair; the generator
    -- should never produce the same triple twice.
    CONSTRAINT matches_unique_fixture UNIQUE (league_id, week_number, home_team_id, away_team_id)
);

CREATE INDEX matches_league_week_idx ON matches (league_id, week_number);

-- One snapshot per league per week. Cached projection of the live standings
-- derived from matches; can always be rebuilt with no data loss.
CREATE TABLE standings_snapshots (
    id          BIGSERIAL    PRIMARY KEY,
    league_id   BIGINT       NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    week_number INT          NOT NULL CHECK (week_number >= 0),
    captured_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (league_id, week_number)
);

-- One row per team per snapshot. rank is 1-based; tie-breakers are applied
-- by the caller before insertion so rank ordering is stable.
CREATE TABLE standings_snapshot_rows (
    snapshot_id     BIGINT NOT NULL REFERENCES standings_snapshots(id) ON DELETE CASCADE,
    team_id         BIGINT NOT NULL REFERENCES teams(id),
    rank            INT    NOT NULL CHECK (rank >= 1),
    played          INT    NOT NULL CHECK (played >= 0),
    won             INT    NOT NULL CHECK (won >= 0),
    drawn           INT    NOT NULL CHECK (drawn >= 0),
    lost            INT    NOT NULL CHECK (lost >= 0),
    goals_for       INT    NOT NULL CHECK (goals_for >= 0),
    goals_against   INT    NOT NULL CHECK (goals_against >= 0),
    goal_difference INT    NOT NULL,
    points          INT    NOT NULL CHECK (points >= 0),
    PRIMARY KEY (snapshot_id, team_id)
);

-- Immutable record of every match-result edit. Never updated, only inserted;
-- gives us a full history of corrections for debugging and demos.
CREATE TABLE match_audit_logs (
    id             BIGSERIAL    PRIMARY KEY,
    match_id       BIGINT       NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    old_home_goals INT          NOT NULL,
    old_away_goals INT          NOT NULL,
    new_home_goals INT          NOT NULL,
    new_away_goals INT          NOT NULL,
    reason         TEXT,
    changed_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX match_audit_logs_match_idx ON match_audit_logs (match_id);

-- Cached payload from an upstream metadata provider (e.g. TheSportsDB).
-- Source is the human-readable provider name; fetched_at drives TTL.
CREATE TABLE external_team_profiles (
    team_id    BIGINT       PRIMARY KEY REFERENCES teams(id) ON DELETE CASCADE,
    payload    JSONB        NOT NULL,
    source     TEXT         NOT NULL,
    fetched_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
