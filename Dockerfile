ARG PG_VERSION=14.4
ARG VERSION=custom

FROM golang as builder

ARG LDFLAGS="-w -X github.com/sorintlab/stolon/cmd.Version=$VERSION"

WORKDIR /go/src/app
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .

RUN go build -o ./bin/flyadmin ./cmd/flyadmin
RUN go build -o ./bin/start ./cmd/start

RUN go build -o ./bin/pg-restart ./.flyctl/cmd/pg-restart
RUN go build -o ./bin/pg-role ./.flyctl/cmd/pg-role
RUN go build -o ./bin/pg-failover ./.flyctl/cmd/pg-failover
RUN go build -o ./bin/stolonctl-run ./.flyctl/cmd/stolonctl-run
RUN go build -o ./bin/pg-settings ./.flyctl/cmd/pg-settings

RUN go build -ldflags "$LDFLAGS" -o ./bin/stolon-keeper ./stolon/cmd/keeper
RUN go build -ldflags "$LDFLAGS" -o ./bin/stolon-sentinel ./stolon/cmd/sentinel
RUN go build -ldflags "$LDFLAGS" -o ./bin/stolon-proxy ./stolon/cmd/proxy
RUN go build -ldflags "$LDFLAGS" -o ./bin/stolonctl ./stolon/cmd/stolonctl

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

COPY --from=builder /go/src/app/bin/* /usr/local/bin/
COPY --from=postgres_exporter /postgres_exporter /usr/local/bin/

EXPOSE 5432

CMD ["start"]
