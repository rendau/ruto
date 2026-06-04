UPDATE endpoint
SET data = jsonb_set(
  data,
  '{http}',
  jsonb_build_object(
    'method', COALESCE(data->>'method', ''),
    'path', COALESCE(data->>'path', '')
  ),
  true
)
WHERE jsonb_typeof(data->'http') IS DISTINCT FROM 'object'
  AND COALESCE(data->>'type', 'http') <> 'grpc';

UPDATE root
SET data = data - 'variables'
WHERE data ? 'variables';

UPDATE app
SET data = data - 'variables'
WHERE data ? 'variables';

UPDATE endpoint
SET data = data - 'variables'
WHERE data ? 'variables';
