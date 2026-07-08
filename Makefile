.PHONY: build test frontend-build up down logs clean

build:
	cd backend && go build ./cmd/api
	cd agent && go build ./cmd/agent

test:
	cd backend && go test ./...
	cd agent && go test ./...

frontend-build:
	docker build -t vmlens-frontend-check ./frontend

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f control-plane dashboard datastore

clean:
	docker compose down -v --remove-orphans
