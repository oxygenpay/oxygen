-- +migrate Up
create table if not exists registries
(
    id          bigserial constraint registries_pkey primary key,
    created_at  timestamp    not null,
    updated_at  timestamp    not null,
    merchant_id bigint       not null,
    key         varchar(128) not null,
    value       varchar(128) not null,
    description varchar(250) not null,

    CONSTRAINT registry_item_is_unique UNIQUE (merchant_id, key)
);

create index if not exists registries_key_index on registries (key, merchant_id);

-- +migrate Down
drop index if exists registries_key_index;
drop table if exists registries;
