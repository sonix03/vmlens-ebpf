.PHONY: build test agent-test backend-test frontend-build up down logs clean schema-dbml schema-dbml-check

build:
	cd backend && go build ./cmd/api
	cd agent && go build ./cmd/agent

test: backend-test agent-test

backend-test:
	cd backend && go test ./...

agent-test:
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

# Regenerate docs/schema/vmlens.dbml (dbdiagram.io input) from the migrations.
# Uses a throwaway database, so it never touches the compose stack or its volume.
schema-dbml:
	bash scripts/generate-schema-dbml.sh

# Fail when docs/schema/vmlens.dbml no longer matches the migrations. For CI.
schema-dbml-check:
	bash scripts/generate-schema-dbml.sh --check
