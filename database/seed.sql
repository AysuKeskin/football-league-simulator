-- Default teams used by the demo flow in README.md.
--
-- Ratings are illustrative, not based on real-world data; they are tuned so
-- that simulations produce visibly different outcomes across the four teams.
-- Idempotent: ON CONFLICT DO NOTHING lets the file be re-applied safely.

INSERT INTO teams (name, attack, midfield, defense) VALUES
    ('Manchester City', 90, 88, 85),
    ('Arsenal',         85, 84, 83),
    ('Chelsea',         82, 80, 78),
    ('Liverpool',       87, 85, 82)
ON CONFLICT (name) DO NOTHING;
