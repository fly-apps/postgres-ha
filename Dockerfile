ARG PG_VERSION=14.4
ARG VERSION=custom

FROM golang as builder
WORKDIR /go/src/app
COPY . .

RUN go install -v ./cmd/flyadmin \
  ./cmd/start \
  ./.flyctl/cmd/pg-restart \
  ./.flyctl/cmd/pg-role \
  ./.flyctl/cmd/pg-failover \
  ./.flyctl/cmd/stolonctl-run \
  ./.flyctl/cmd/pg-settings \
  ./stolon/cmd/keeper \
  ./stolon/cmd/sentinel \
  ./stolon/cmd/proxy \
  ./stolon/cmd/stolonctl

FROM wrouesnel/postgres_exporter:latest AS postgres_exporter

FROM postgres:${PG_VERSION}
ARG VERSION 
ARG POSTGIS_MAJOR=3

LABEL fly.app_role=postgres_cluster
LABEL fly.version=${VERSION}
LABEL fly.pg-version=${PG_VERSION}

RUN apt-get update && apt-get install --no-install-recommends -y \
    ca-certificates curl bash dnsutils vim-tiny procps jq haproxy \
    postgresql-$PG_MAJOR-postgis-$POSTGIS_MAJOR \
    postgresql-$PG_MAJOR-postgis-$POSTGIS_MAJOR-scripts \    
    && apt autoremove -y

ADD /scripts/* /fly/
ADD /config/* /fly/
RUN useradd -ms /bin/bash stolon
RUN mkdir -p /run/haproxy/

COPY --from=builder /go/bin/* /usr/local/bin/
COPY --from=postgres_exporter /postgres_exporter /usr/local/bin/

EXPOSE 5432

CMD ["start"]
