FROM golang:1.25.9-alpine as base

RUN apk add --no-cache postgresql-client ca-certificates
RUN go install github.com/rubenv/sql-migrate/...@v1.8.1

WORKDIR /migrations

COPY . .

ENTRYPOINT ["sql-migrate"]
