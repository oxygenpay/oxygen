-- name: ListMerchantAddresses :many
select * from merchant_addresses
where merchant_id = $1
order by id desc;

-- name: GetMerchantAddressByUUID :one
select * from merchant_addresses
where merchant_id = $1 and uuid = $2
limit 1;

-- name: GetMerchantAddressByID :one
select * from merchant_addresses
where merchant_id = $1 and id = $2
limit 1;

-- name: GetMerchantAddressByAddress :one
select * from merchant_addresses
where merchant_id = $1 and address = $2 and blockchain = $3
limit 1;

-- name: CreateMerchantAddress :one
insert into merchant_addresses(
    created_at, updated_at,
    uuid,
    merchant_id, name, blockchain,
    address
) values ($1, $2, $3, $4, $5, $6, $7)
returning *;

-- name: UpdateMerchantAddress :one
update merchant_addresses
set name = $3, updated_at = $4
where merchant_id = $1 and id = $2
returning *;

-- name: DeleteMerchantAddress :exec
delete from merchant_addresses where merchant_id = $1 and id = $2;
