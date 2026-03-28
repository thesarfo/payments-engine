FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/engine ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/seed ./cmd/seed && \
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata postgresql-client && \
    adduser -D -h /home/app app

COPY --from=builder /out/engine /usr/local/bin/engine
COPY --from=builder /out/seed /usr/local/bin/seed
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY migrations /app/migrations
COPY scripts/entrypoint.sh /app/scripts/entrypoint.sh

RUN sed -i 's/\r$//' /app/scripts/entrypoint.sh && \
    chmod +x /app/scripts/entrypoint.sh && \
    chown -R app:app /app

USER app

EXPOSE 8080

ENTRYPOINT ["/app/scripts/entrypoint.sh"]
