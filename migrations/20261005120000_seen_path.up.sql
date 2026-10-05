-- Concrete paths served by wildcard endpoints, reported by gateways. It is a
-- to-do list for replacing a wildcard with explicit endpoints.
CREATE TABLE seen_path
(
    endpoint_id    text        NOT NULL,
    method         text        NOT NULL,
    path           text        NOT NULL,
    app_id         text        NOT NULL,
    hits           bigint      NOT NULL DEFAULT 0,
    hits_not_found bigint      NOT NULL DEFAULT 0,
    first_seen_at  timestamptz NOT NULL DEFAULT now(),
    last_seen_at   timestamptz NOT NULL DEFAULT now(),
    sample         text        NOT NULL DEFAULT '',
    PRIMARY KEY (endpoint_id, method, path)
);

CREATE INDEX seen_path_app_id_idx ON seen_path (app_id);
