-- Gateway-wide block list of client addresses. It is not part of the
-- configuration: a block takes effect at once, without a change request or a
-- publish. Lifted and expired blocks stay as history.
CREATE TABLE blocked_client
(
    id            text        NOT NULL DEFAULT gen_random_uuid()
        PRIMARY KEY,
    -- normalized network in CIDR notation: 203.0.113.7/32, 2001:db8::/48
    ip            text        NOT NULL,
    reason        text        NOT NULL DEFAULT '',
    created_by    text        NOT NULL DEFAULT '',
    -- 'admin' | 'agent' (an api key: MCP or the REST API, e.g. pulse)
    source        text        NOT NULL DEFAULT 'admin',
    -- the api key (or OAuth grant) an agent block was made with
    api_key_id    text        NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    -- NULL = without end
    expires_at    timestamptz,
    removed_at    timestamptz,
    removed_by    text        NOT NULL DEFAULT '',
    remove_reason text        NOT NULL DEFAULT '',
    -- who lifted it: 'admin' (a person, kill switch included) | 'agent' |
    -- 'key_deleted' (its api key was deleted) | '' (closed after expiry)
    removed_source text       NOT NULL DEFAULT ''
);

-- one block per network that is not lifted; an expired one is closed before
-- the same network is blocked again
CREATE UNIQUE INDEX blocked_client_ip_uidx ON blocked_client (ip) WHERE removed_at IS NULL;

-- active blocks: not removed, expires_at in the future or NULL
CREATE INDEX blocked_client_active_idx ON blocked_client (expires_at) WHERE removed_at IS NULL;

CREATE INDEX blocked_client_created_at_idx ON blocked_client (created_at);

-- the shared hourly budget of agents: new agent blocks by created_at
CREATE INDEX blocked_client_agent_created_at_idx ON blocked_client (created_at) WHERE source = 'agent';

-- the cooldown after a person lifts a block
CREATE INDEX blocked_client_removed_by_admin_idx ON blocked_client (removed_at) WHERE removed_source = 'admin';
