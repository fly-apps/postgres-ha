ARG PG_VERSION=14.1
ARG VERSION=custom

FROM golang:1.16 as flyutil
ARG VERSION

WORKDIR /go/src/github.com/fly-examples/postgres-ha
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -v -o /fly/bin/flyadmin ./cmd/flyadmin
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /fly/bin/start ./cmd/start

RUN CGO_ENABLED=0 GOOS=linux go build -v -o /fly/bin/pg-restart ./.flyctl/cmd/pg-restart
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /fly/bin/pg-role ./.flyctl/cmd/pg-role
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /fly/bin/pg-failover ./.flyctl/cmd/pg-failover


COPY ./bin/* /fly/bin/


FROM flyio/stolon:b6b9aaf  as stolon

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

COPY --from=stolon /go/src/app/bin/* /usr/local/bin/
COPY --from=postgres_exporter /postgres_exporter /usr/local/bin/

ADD /scripts/* /fly/
ADD /config/* /fly/
RUN useradd -ms /bin/bash stolon
RUN mkdir -p /run/haproxy/
COPY --from=flyutil /fly/bin/* /usr/local/bin/

EXPOSE 5432

CMD ["start"]
