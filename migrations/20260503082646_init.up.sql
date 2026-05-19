create table root
(
    id   text  not null
        primary key,
    data jsonb not null default '{}'
);

create table app
(
    id     text    not null default gen_random_uuid()
        primary key,
    active boolean not null default true,
    data   jsonb   not null default '{}'
);

create table endpoint
(
    id     text    not null default gen_random_uuid()
        primary key,
    app_id text    not null
        references app (id) on delete cascade,
    active boolean not null default true,
    data   jsonb   not null default '{}'
);

create table usr
(
    id       bigserial not null
        primary key,
    name     text      not null default '',
    username text      not null default '',
    password text      not null default ''
);
