-- +migrate Up
create table if not exists users
(
    id                bigserial constraint users_pkey primary key,
    name              varchar(255) not null,
    email             varchar(255) not null,

    uuid              uuid         not null constraint users_uuid_unique unique,
    google_id         varchar(255) constraint users_google_id_unique unique,
    profile_image_url text,

    created_at        timestamp(0) not null,
    updated_at        timestamp(0) not null,
    deleted_at        timestamp(0) null,

    settings          jsonb default '{}'::jsonb
);

-- +migrate Down
drop table if exists users;
