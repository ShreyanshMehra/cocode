# --- build stage: compile a static Go binary ---
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache module downloads first.
COPY go.mod go.sum ./
RUN go mod download

# Build the server (CGO disabled -> fully static binary).
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cocode ./cmd/server

# --- runtime stage: tiny image with just the binary + web assets ---
FROM alpine:3.20
WORKDIR /app

# Non-root user for the runtime.
RUN adduser -D -u 10001 cocode

COPY --from=build /out/cocode /app/cocode
COPY web /app/web

# PORT is read by the server; hosts (Render/Fly/HF) may override it.
# DATA_DIR holds version snapshots (ephemeral on free tiers, fine for a demo).
ENV PORT=8000 \
    DATA_DIR=/tmp/cocode-data

USER cocode
EXPOSE 8000

CMD ["/app/cocode"]
