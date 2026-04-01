.PHONY: build frontend dev clean

build: frontend
	go build -o check-in-bot ./cmd/check-in-bot

frontend:
	cd frontend && bun install && bun run build

dev:
	cd frontend && bun install && bun run dev &
	go run ./cmd/check-in-bot --dev

clean:
	rm -f check-in-bot
	rm -rf internal/checkinbot/frontend_dist frontend/node_modules
