-- name: AdvisoryTxLock :exec
select pg_advisory_xact_lock($1);