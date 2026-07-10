FROM golang:1.26-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/git-mirror-sync ./cmd

FROM alpine

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/git-mirror-sync /usr/local/bin/git-mirror-sync

WORKDIR /data

ENTRYPOINT ["git-mirror-sync"]
CMD ["-config", "/config/config.toml"]
