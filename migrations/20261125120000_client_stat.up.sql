-- Requests per client address as gateways report them in heartbeats, summed
-- into 5-minute buckets of the time core received them. Kept for 7 days.
CREATE TABLE client_stat
(
    -- start of the 5-minute bucket (UTC)
    bucket    timestamptz NOT NULL,
    -- client address (an IPv6 client as its /64), the network of a trusted
    -- client, or '' for clients that did not fit the gateway's report
    ip        text        NOT NULL,
    -- name of the trusted client, '' for others
    trusted   text        NOT NULL DEFAULT '',
    requests  bigint      NOT NULL DEFAULT 0,
    -- refused by the block list
    blocked   bigint      NOT NULL DEFAULT 0,
    -- rejected by a rate-limit rule
    limited   bigint      NOT NULL DEFAULT 0,
    -- rule of the latest rejection in the bucket
    last_rule text        NOT NULL DEFAULT '',
    PRIMARY KEY (bucket, ip)
);

-- the timeline of one client
CREATE INDEX client_stat_ip_bucket_idx ON client_stat (ip, bucket);
