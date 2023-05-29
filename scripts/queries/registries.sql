-- name: CreateRegistryItem :one
insert into registries (
created_at, updated_at,
merchant_id, key, value,
description
) values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: GetRegistryItemByKey :one
select * from registries where merchant_id = $1 and key = $2;

-- name: UpdateRegistryItem :one
update registries
set updated_at = $3, value = $4, description = $5
where key = $1 and merchant_id = $2
returning *;
