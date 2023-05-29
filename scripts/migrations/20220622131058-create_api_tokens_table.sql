-- +migrate Up
create table if not exists api_tokens
(
    id          bigserial constraint personal_access_tokens_pkey primary key,
    entity_type varchar(255) not null,
    entity_id   bigint       not null,
    created_at  timestamp    not null,
    token       varchar(64)  not null constraint personal_access_tokens_token_unique unique,
    name        varchar(255) null,
    settings    jsonb        null,
    uuid        uuid unique  not null
);

create index api_tokens_entity_index on api_tokens (entity_type, entity_id);

-- +migrate Down
drop table if exists api_tokens;
