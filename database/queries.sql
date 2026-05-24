-- =============================================================================
-- Representative read queries used by the application.
-- =============================================================================
-- These are reference queries the service layer issues via pgx. They are kept
-- here so reviewers can see the non-trivial SQL in one place without having
-- to read Go code. Each query is named and commented.
-- =============================================================================


-- ---------------------------------------------------------------------------
-- Q1: Live standings for a league, aggregated from matches.
--
-- Used when the snapshot for the current week has not yet been written, and
-- as the source of truth when rebuilding snapshots after a match edit.
-- Tie-break ordering: points DESC, goal difference DESC, goals for DESC,
-- wins DESC, team name ASC.
-- ---------------------------------------------------------------------------
WITH played AS (
    SELECT
        m.home_team_id AS team_id,
        m.home_goals   AS goals_for,
        m.away_goals   AS goals_against,
        CASE
            WHEN m.home_goals >  m.away_goals THEN 3
            WHEN m.home_goals =  m.away_goals THEN 1
            ELSE 0
        END AS points,
        (m.home_goals >  m.away_goals)::int AS won,
        (m.home_goals =  m.away_goals)::int AS drawn,
        (m.home_goals <  m.away_goals)::int AS lost
    FROM matches m
    WHERE m.league_id = $1 AND m.status = 'PLAYED'

    UNION ALL

    SELECT
        m.away_team_id,
        m.away_goals,
        m.home_goals,
        CASE
            WHEN m.away_goals >  m.home_goals THEN 3
            WHEN m.away_goals =  m.home_goals THEN 1
            ELSE 0
        END,
        (m.away_goals >  m.home_goals)::int,
        (m.away_goals =  m.home_goals)::int,
        (m.away_goals <  m.home_goals)::int
    FROM matches m
    WHERE m.league_id = $1 AND m.status = 'PLAYED'
)
SELECT
    t.id   AS team_id,
    t.name AS team_name,
    COUNT(*)                          AS played,
    COALESCE(SUM(p.won),   0)         AS won,
    COALESCE(SUM(p.drawn), 0)         AS drawn,
    COALESCE(SUM(p.lost),  0)         AS lost,
    COALESCE(SUM(p.goals_for),     0) AS goals_for,
    COALESCE(SUM(p.goals_against), 0) AS goals_against,
    COALESCE(SUM(p.goals_for) - SUM(p.goals_against), 0) AS goal_difference,
    COALESCE(SUM(p.points), 0)        AS points
FROM teams t
JOIN league_teams lt ON lt.team_id = t.id
LEFT JOIN played p   ON p.team_id  = t.id
WHERE lt.league_id = $1
GROUP BY t.id, t.name
ORDER BY points DESC, goal_difference DESC, goals_for DESC, won DESC, t.name ASC;


-- ---------------------------------------------------------------------------
-- Q2: All matches for a league, grouped by week.
--
-- Used by the fixtures endpoint and by the week-detail endpoint when the
-- caller wants the full schedule rather than a single week.
-- ---------------------------------------------------------------------------
SELECT
    id,
    week_number,
    home_team_id,
    away_team_id,
    home_goals,
    away_goals,
    status,
    played_at
FROM matches
WHERE league_id = $1
ORDER BY week_number, id;


-- ---------------------------------------------------------------------------
-- Q3: A single week's snapshot rows, joined with team names for display.
--
-- Used by GET /api/v1/leagues/{id}/weeks/{w}.
-- ---------------------------------------------------------------------------
SELECT
    r.rank, r.team_id, t.name AS team_name,
    r.played, r.won, r.drawn, r.lost,
    r.goals_for, r.goals_against, r.goal_difference, r.points
FROM standings_snapshots s
JOIN standings_snapshot_rows r ON r.snapshot_id = s.id
JOIN teams t                    ON t.id = r.team_id
WHERE s.league_id = $1 AND s.week_number = $2
ORDER BY r.rank;
