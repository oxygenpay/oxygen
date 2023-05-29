-- name: CreateJobLog :exec
insert into job_logs (created_at, level, job_id, message, metadata)
values ($1, $2, $3, $4, $5);

-- name: ListJobLogsByID :many
select * from job_logs where job_id = $1 order by id limit $2;