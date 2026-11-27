-- CORS preflights among requests that the gateway answered itself: they reach
-- no endpoint, so no rate limit counts them, and the rates compared to the
-- limits leave them out
ALTER TABLE client_stat
    ADD COLUMN preflights bigint NOT NULL DEFAULT 0;
