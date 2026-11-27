-- Hits of seen origins per day of core's clock (TZ of the pod), so the admin
-- shows the requests of the last 7 days instead of a total since the first
-- one. core prunes the days that left the window.
CREATE TABLE seen_origin_day
(
    origin text   NOT NULL REFERENCES seen_origin (origin) ON DELETE CASCADE,
    day    date   NOT NULL,
    hits   bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (origin, day)
);

CREATE INDEX seen_origin_day_day_idx ON seen_origin_day (day);

-- the running totals cannot be split into days: the 7-day counters start
-- from zero
ALTER TABLE seen_origin
    DROP COLUMN hits;
