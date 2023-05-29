-- +migrate Up
create table if not exists job_logs
(
    id         bigserial constraint job_logs_pkey primary key,
    created_at timestamp    not null,
    level      smallint     not null,
    job_id     varchar(128) null,
    message    text         not null,
    metadata   jsonb        null
);

create index if not exists job_logs_job_id_index on job_logs (job_id);

-- +migrate Down
drop index if exists job_logs_job_id_index;
drop table job_logs;
