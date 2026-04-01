FROM oven/bun:1 AS frontend
WORKDIR /src

COPY frontend/package.json frontend/bun.lock ./frontend/
RUN cd frontend && bun install --frozen-lockfile

COPY frontend ./frontend
RUN mkdir -p internal/checkinbot && cd frontend && bun run build

FROM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend /src/internal/checkinbot/frontend_dist ./internal/checkinbot/frontend_dist

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/check-in-bot ./cmd/check-in-bot

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/check-in-bot /usr/local/bin/check-in-bot

EXPOSE 8080
ENTRYPOINT ["check-in-bot"]
