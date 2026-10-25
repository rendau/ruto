-- strip_slashes is replaced by the endpoint flag allow_trailing_slash. The
-- published snapshot is edited in place rather than rebuilt: rebuilding it from
-- the config would publish unpublished changes. core recomputes the snapshot
-- hash from the edited data on start.

UPDATE root
SET data = data - 'strip_slashes'
WHERE data ? 'strip_slashes';

UPDATE snapshot
SET data = data - 'strip_slashes'
WHERE data ? 'strip_slashes';
