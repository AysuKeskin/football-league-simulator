-- Initial schema. See database/schema.sql for the canonical reference and
-- per-table commentary. Any change here must be mirrored in schema.sql.

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

CREATE TABLE teams (
    id         BIGSERIAL    PRIMARY KEY,
    name       TEXT         NOT NULL UNIQUE,
    attack     INT          NOT NULL CHECK (attack   BETWEEN 1 AND 100),
    midfield   INT          NOT NULL CHECK (midfield BETWEEN 1 AND 100),
    defense    INT          NOT NULL CHECK (defense  BETWEEN 1 AND 100),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE league_teams (
    league_id BIGINT NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    team_id   BIGINT NOT NULL REFERENCES teams(id),
    PRIMARY KEY (league_id, team_id)
);

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
    CHECK (home_team_id <> away_team_id)
);

CREATE INDEX matches_league_week_idx ON matches (league_id, week_number);

CREATE TABLE standings_snapshots (
    id          BIGSERIAL    PRIMARY KEY,
    league_id   BIGINT       NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    week_number INT          NOT NULL CHECK (week_number >= 0),
    captured_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (league_id, week_number)
);

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

CREATE TABLE external_team_profiles (
    team_id    BIGINT       PRIMARY KEY REFERENCES teams(id) ON DELETE CASCADE,
    payload    JSONB        NOT NULL,
    source     TEXT         NOT NULL,
    fetched_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
