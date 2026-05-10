create table root
(
    id   text  not null
        primary key,
    data jsonb not null default '{}'
);

create table app
(
    id          text    not null default gen_random_uuid()
        primary key,
    active      boolean not null default true,
    path_prefix text    not null default ''
        unique,
    name        text    not null default '',
    backend     jsonb   not null default '{}'
);

create table endpoint
(
    id     text    not null default gen_random_uuid()
        primary key,
    app_id text    not null
        references app (id) on delete cascade,
    active boolean not null default true,
    method text    not null default '',
    path   text    not null default '',
    data   jsonb   not null default '{}'
);
