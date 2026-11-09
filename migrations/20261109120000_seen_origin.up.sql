-- Browser origins (Origin header) seen by gateways. It is a pick list for
-- CORS allow_origins in the root configuration.
CREATE TABLE seen_origin
(
    origin        text        NOT NULL,
    hits          bigint      NOT NULL DEFAULT 0,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (origin)
);
