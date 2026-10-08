FROM golang:1.25.9-alpine AS base

RUN apk add --no-cache postgresql-client ca-certificates
RUN go install github.com/rubenv/sql-migrate/...@v1.8.1

WORKDIR /migrations

ENV POSTGRESQL_SSL_MODE=disable

COPY dbconfig.yaml [0-9][0-9][0-9]-*.sql ./

ENTRYPOINT ["sql-migrate"]
