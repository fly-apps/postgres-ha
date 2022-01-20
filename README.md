# High Availability Postgres on Fly

This repo contains all the code and configuration necessary to run a [highly available Postgres cluster](https://fly.io/docs/reference/postgres/) in a Fly.io organization's private network. This source is packaged into [Docker images](https://hub.docker.com/r/flyio/postgres-ha/tags) which allow you to track and upgrade versions cleanly as new features are added.

If you just want to get a standard Postgres standalone or highly-available setup on Fly, [check out the docs](https://fly.io/docs/reference/postgres/).
## Customizing cluster behavior

Fly Postgres clusters are just regular Fly applications. If you need to customize Postgres in any way, you may fork this repo and deploy using normal Fly deployment procedures. You won't be able to use `fly postgres` commands with custom clusters. But it's a great way to experiment and potentially contribute back useful features!

Follow the rest of this README to run a customized setup.
## Prepare a new application

You'll need a fresh Fly application in your preferred region to get started. Run these commands within the fork of this repository.
### `fly launch --no-deploy`
This gets you started with a Fly application and an associated config file.
Choose `yes` when asked whether to copy the existing configuration to the newly generated app.

### Set secrets
This app requires a few secret environment variables. Generate a secure string for each, and save them.

`SU_PASSWORD` is the PostgreSQL super user password. The username is `flypgadmin`. Use these credentials to run high privilege administration tasks.

`REPL_PASSWORD` is used to replicate between instances.

`OPERATOR_PASSWORD` is the password for the standard user `postgres`. Use these credentials for connecting from your application.

> `fly secrets set SU_PASSWORD=<PASSWORD> REPL_PASSWORD=<PASSWORD> OPERATOR_PASSWORD=<PASSWORD>`

## Deploy one instance

First, get one instance deployed in your preferred start region.

1. `fly volumes create pg_data --region ord --size 10`
2. `fly deploy`
3. `fly status`

## Add a replica

Scaling up will automatically setup a replica for you. Do that now in the same region.

1. `fly volumes create pg_data --region ord --size 10`
2. `fly scale count 2`
3. `fly status`

## Add a replica in another region

Scale to another region by creating a volume there. Now you should have a primary/replica pair in `ord` and a replica in `syd`.

1. `fly volumes create pg_data --region syd --size 10`
2. `fly scale count 3`
3. `fly status`