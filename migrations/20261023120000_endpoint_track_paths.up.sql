-- track_paths turns the seen paths report on per wildcard endpoint. Existing
-- wildcard endpoints keep reporting. The published snapshot is edited in place
-- rather than dropped: rebuilding it from the config would publish unpublished
-- changes. core recomputes the snapshot hash from the edited data on start.

CREATE FUNCTION endpoint_track_paths_up(ep jsonb) RETURNS jsonb AS $$
    SELECT CASE
        WHEN COALESCE(NULLIF(ep->>'type', ''), 'http') = 'http' AND ep->'http'->>'path' LIKE '%*'
            THEN ep || '{"track_paths": true}'
        ELSE ep
    END
$$ LANGUAGE sql IMMUTABLE;

UPDATE endpoint
SET data = endpoint_track_paths_up(data)
WHERE COALESCE(NULLIF(data->>'type', ''), 'http') = 'http'
  AND data->'http'->>'path' LIKE '%*';

UPDATE snapshot
SET data = jsonb_set(data, '{apps}', (
    SELECT COALESCE(jsonb_agg(
        CASE
            WHEN jsonb_typeof(app->'endpoints') = 'array' THEN jsonb_set(app, '{endpoints}', (
                SELECT COALESCE(jsonb_agg(endpoint_track_paths_up(ep) ORDER BY ep_ord), '[]'::jsonb)
                FROM jsonb_array_elements(app->'endpoints') WITH ORDINALITY AS e(ep, ep_ord)
            ))
            ELSE app
        END
        ORDER BY app_ord), '[]'::jsonb)
    FROM jsonb_array_elements(data->'apps') WITH ORDINALITY AS a(app, app_ord)
))
WHERE jsonb_typeof(data->'apps') = 'array';

DROP FUNCTION endpoint_track_paths_up(jsonb);
