-- The fixed eight-team catalog. A league is created by picking from these
-- (any even count, 4 or more); the catalog itself is not edited at runtime.
--
-- Ratings are illustrative, not based on real-world data; they are tuned so
-- that simulations produce visibly different outcomes across the teams.
-- Idempotent: ON CONFLICT DO NOTHING lets the file be re-applied safely.

INSERT INTO teams (name, attack, midfield, defense) VALUES
    ('Manchester City',   90, 88, 85),
    ('Arsenal',           85, 84, 83),
    ('Liverpool',         87, 85, 82),
    ('Chelsea',           82, 80, 78),
    ('Manchester United', 80, 79, 77),
    ('Tottenham',         81, 78, 74),
    ('Newcastle',         78, 76, 75),
    ('Aston Villa',       76, 75, 73)
ON CONFLICT (name) DO NOTHING;

-- A fresh, not-yet-started demo league so the UI has something to open on a
-- first visit: four of the pool teams, a full double round-robin (six weeks,
-- twelve SCHEDULED fixtures, no results yet). This is static demo data, not a
-- substitute for the Go fixture generator — it just needs to be a valid
-- schedule. Idempotent: skipped if a league with this name already exists.
DO $$
DECLARE
    lid BIGINT;
    mc  BIGINT;
    ars BIGINT;
    liv BIGINT;
    che BIGINT;
BEGIN
    IF EXISTS (SELECT 1 FROM leagues WHERE name = 'Premier League') THEN
        RETURN;
    END IF;

    SELECT id INTO mc  FROM teams WHERE name = 'Manchester City';
    SELECT id INTO ars FROM teams WHERE name = 'Arsenal';
    SELECT id INTO liv FROM teams WHERE name = 'Liverpool';
    SELECT id INTO che FROM teams WHERE name = 'Chelsea';

    INSERT INTO leagues (name, current_week, total_weeks, status, random_seed)
    VALUES ('Premier League', 0, 6, 'NOT_STARTED', 42)
    RETURNING id INTO lid;

    INSERT INTO league_teams (league_id, team_id) VALUES
        (lid, mc), (lid, ars), (lid, liv), (lid, che);

    -- Double round-robin: weeks 1-3 first leg, weeks 4-6 with home/away swapped.
    INSERT INTO matches (league_id, week_number, home_team_id, away_team_id) VALUES
        (lid, 1, mc,  che), (lid, 1, ars, liv),
        (lid, 2, mc,  liv), (lid, 2, che, ars),
        (lid, 3, mc,  ars), (lid, 3, liv, che),
        (lid, 4, che, mc),  (lid, 4, liv, ars),
        (lid, 5, liv, mc),  (lid, 5, ars, che),
        (lid, 6, ars, mc),  (lid, 6, che, liv);
END $$;
