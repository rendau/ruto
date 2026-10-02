-- Duplicate usernames could be created before this index existed. The oldest
-- account keeps its username; later ones get their id appended, so the index
-- can be built without failing the migration or deleting anyone. The loop
-- covers the unlikely case when a renamed value collides with another row.
DO
$$
    BEGIN
        LOOP
            UPDATE usr u
            SET username = u.username || '_' || u.id
            WHERE EXISTS (SELECT 1 FROM usr o WHERE o.username = u.username AND o.id < u.id);
            EXIT WHEN NOT FOUND;
        END LOOP;
    END
$$;

CREATE UNIQUE INDEX usr_username_uidx ON usr (username);
