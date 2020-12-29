# High Availability Postgres on Fly

This is a ready to go HA Postgres app that runs on Fly. It currently requires a Consul service.

## Deploy it

1. `flyctl init`
2. `flyctl volumes create pg_data --region ord --size 10`
3. `flyctl deploy`

## Add a replica

1. `flyctl volumes create pg_data --region ord --size 10`
2. `flyctl scale count 2`