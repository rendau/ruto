-- The snapshot keeps the field: older gateways ignore it.
UPDATE endpoint
SET data = data - 'track_paths'
WHERE data ? 'track_paths';
