FROM golang:1.25-alpine

RUN go install github.com/bokwoon95/wgo@latest

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["wgo", "go", "run", "./cmd/testapp/main.go"]
