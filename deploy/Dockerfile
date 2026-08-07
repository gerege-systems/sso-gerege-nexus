# Production Multi-stage Dockerfile for the Gerege SSO backend
# Build context is the repository root: docker build -f deploy/Dockerfile .
#
# The Go toolchain version must satisfy backend/go.mod. Official golang images
# pin GOTOOLCHAIN=local, so an image older than the go directive fails the
# build outright.
FROM golang:1.26-alpine AS builder

# The upstream AI client currently requires Go 1.26. Keep auto enabled so
# patch-level toolchain requirements do not make an otherwise valid build fail.
ENV GOTOOLCHAIN=auto

WORKDIR /app/backend

# Copy dependency manifests first so `go mod download` stays cached.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy the module source
COPY backend/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /bin/migrate ./cmd/migrate

# Production Alpine image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Ulaanbaatar

# Run as an unprivileged user.
RUN addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=builder /bin/api /app/api
COPY --from=builder /bin/migrate /app/migrate
# App-store catalog + goose migrations are read from disk at runtime.
COPY catalog/ /app/catalog/
COPY backend/db/migrations/ /app/db/migrations/

USER app

EXPOSE 8080

CMD ["/app/api"]
