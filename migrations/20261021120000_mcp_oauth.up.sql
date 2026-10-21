-- A connection of an OAuth client (e.g. Claude) to the built-in MCP server,
-- approved by an admin on the consent page. Its tokens act like an api key of
-- that admin. A grant without connected_at is waiting for its code exchange.
CREATE TABLE oauth_grant
(
    id             text        NOT NULL DEFAULT gen_random_uuid()
        PRIMARY KEY,
    usr_id         bigint      NOT NULL
        REFERENCES usr (id) ON DELETE CASCADE,
    client_id      text        NOT NULL,
    client_name    text        NOT NULL DEFAULT '',
    redirect_uri   text        NOT NULL DEFAULT '',
    code_challenge text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    connected_at   timestamptz,
    last_used_at   timestamptz
);

CREATE INDEX oauth_grant_usr_id_idx ON oauth_grant (usr_id);

-- Authorization codes, access and refresh tokens of grants. Only the sha256 of
-- a token is stored.
CREATE TABLE oauth_token
(
    token_hash text        NOT NULL
        PRIMARY KEY,
    grant_id   text        NOT NULL
        REFERENCES oauth_grant (id) ON DELETE CASCADE,
    kind       text        NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX oauth_token_grant_id_idx ON oauth_token (grant_id);
CREATE INDEX oauth_token_expires_at_idx ON oauth_token (expires_at);
