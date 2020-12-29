FROM golang:1.15 as flyutil

WORKDIR /go/src/github.com/fly-examples/postgres-ha
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -v -o flyadmin ./cmd/flyadmin
RUN CGO_ENABLED=0 GOOS=linux go build -v -o flycheck ./cmd/flycheck

FROM flyio/stolon:20210109-5 as stolon

FROM postgres:12

LABEL fly.app_role=postgres_cluster

RUN apt-get update && apt-get install --no-install-recommends -y \
    ca-certificates curl bash dnsutils tmux vim-tiny procps \
    && apt autoremove -y

COPY --from=stolon /go/src/app/bin/* /usr/local/bin/
ADD /bin/* /usr/local/bin/
ADD /scripts/* /fly/
RUN useradd -ms /bin/bash stolon
COPY --from=flyutil /go/src/github.com/fly-examples/postgres-ha/fly* /usr/local/bin/

EXPOSE 5432

CMD ["/fly/start.sh"]