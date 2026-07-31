FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /acceptance-fixture ./tools/acceptance-fixture

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /acceptance-fixture /usr/local/bin/acceptance-fixture
EXPOSE 8081 8082 8083 9090
ENTRYPOINT ["acceptance-fixture"]
