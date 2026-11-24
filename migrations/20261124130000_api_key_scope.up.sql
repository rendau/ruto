-- What a key may do: 'full' — the MCP server (reads, change requests, block
-- list); 'blocklist' — only the block list (BlockedClient on /api and the
-- block list MCP tools), for services such as pulse.
ALTER TABLE api_key ADD COLUMN scope text NOT NULL DEFAULT 'full';
