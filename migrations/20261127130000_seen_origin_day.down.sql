ALTER TABLE seen_origin
    ADD COLUMN hits bigint NOT NULL DEFAULT 0;

UPDATE seen_origin o
SET hits = d.hits
FROM (SELECT origin, sum(hits)::bigint AS hits FROM seen_origin_day GROUP BY origin) d
WHERE d.origin = o.origin;

DROP TABLE seen_origin_day;
