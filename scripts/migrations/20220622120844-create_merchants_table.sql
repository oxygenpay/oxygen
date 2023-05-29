-- +migrate Up
create table if not exists merchants
(
    id         bigserial constraint merchants_pkey primary key,
    uuid       uuid         not null constraint merchants_uuid_unique unique,

    created_at timestamp(0) not null,
    updated_at timestamp(0) not null,
    deleted_at timestamp(0) null,

    name       varchar(255) not null,
    website    varchar(255) not null,

    creator_id bigint       not null,

    settings   jsonb default '{}'::jsonb
);

create index merchants_creator_id on merchants (creator_id);

-- +migrate Down
drop table if exists merchants;
