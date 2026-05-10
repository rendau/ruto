create table root
(
    id              text  not null
        primary key,
    public_base_url text  not null default '',
    cors            jsonb not null default '{}',
    jwt             jsonb not null default '[]'
);
