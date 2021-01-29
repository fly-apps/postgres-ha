# High Availability Postgres on Fly

This is a ready to go HA Postgres app that runs on Fly.

## Prepare your app

### `flyctl init`
Init gets you going with a Fly application and generates a config file.

### Set secrets
This app requires `SU_PASSWORD` and `REPL_PASSWORD` environment variables.

`SU_PASSWORD` is the PostgreSQL super user password, the username is `flypgadmin`. You can use this to administer the database once it's running. You should create less privileged users for your applications to use.

`REPL_PASSWORD` is used to replicate between instances.

> `flyctl secrets set SU_PASSWORD=<PASSWORD> REPL_PASSWORD=<PASSWORD>`

## Deploy one instance

1. `flyctl volumes create pg_data --region ord --size 10`
3. `flyctl deploy`

## Add a replica

1. `flyctl volumes create pg_data --region ord --size 10`
2. `flyctl scale count 2`

## Add a replica in another region

1. `flyctl volumes create pg_data --region syd --size 10`
2. `flyctl scale count 2`