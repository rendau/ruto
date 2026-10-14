-- Long-lived keys for the built-in MCP server. Only the sha256 of a key is
-- stored; the key itself is shown to its owner once, on creation.
CREATE TABLE api_key
(
    id           text        NOT NULL DEFAULT gen_random_uuid()
        PRIMARY KEY,
    usr_id       bigint      NOT NULL
        REFERENCES usr (id) ON DELETE CASCADE,
    name         text        NOT NULL DEFAULT '',
    key_hash     text        NOT NULL,
    key_prefix   text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);

CREATE UNIQUE INDEX api_key_key_hash_uidx ON api_key (key_hash);
CREATE INDEX api_key_usr_id_idx ON api_key (usr_id);

-- Configuration changes proposed by an agent over MCP. Nothing is applied until
-- an admin approves the request in the admin UI.
CREATE TABLE change_request
(
    id             bigserial   NOT NULL
        PRIMARY KEY,
    status         text        NOT NULL DEFAULT 'pending',
    title          text        NOT NULL DEFAULT '',
    description    text        NOT NULL DEFAULT '',
    publish        boolean     NOT NULL DEFAULT false,
    plan           jsonb       NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now(),
    author_usr_id  bigint      NOT NULL DEFAULT 0,
    api_key_id     text        NOT NULL DEFAULT '',
    api_key_name   text        NOT NULL DEFAULT '',
    decided_at     timestamptz,
    decided_usr_id bigint      NOT NULL DEFAULT 0,
    comment        text        NOT NULL DEFAULT '',
    error          text        NOT NULL DEFAULT '',
    result         jsonb       NOT NULL DEFAULT '{}',
    published_at   timestamptz,
    publish_error  text        NOT NULL DEFAULT ''
);

CREATE INDEX change_request_status_idx ON change_request (status, created_at);
