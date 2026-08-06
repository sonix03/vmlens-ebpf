#!/usr/bin/env bash
# Regenerate docs/schema/vmlens.dbml from backend/internal/db/migrations.
#
# Paste the result into https://dbdiagram.io (or open it with any DBML tool) to
# get the ER diagram. Tables, columns, types, defaults, checks, indexes, foreign
# keys with their delete actions, and the semantic notes from
# scripts/schema-comments.sql all come along.
#
# How it works: start a throwaway Postgres, apply every migration in filename
# order, apply the comment file, then run @dbml/cli against it. The compose
# stack and its volume are never touched, so this is safe to run while VMLens
# is up.
#
# Usage:
#   bash scripts/generate-schema-dbml.sh          # regenerate
#   bash scripts/generate-schema-dbml.sh --check  # fail if the file is stale
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
migrations_dir="${repo_dir}/backend/internal/db/migrations"
comments_file="${repo_dir}/scripts/schema-comments.sql"
out_dir="${repo_dir}/docs/schema"
outputs=(vmlens vmlens-minimal)

pg_image="${SCHEMA_PG_IMAGE:-postgres:16-alpine}"
node_image="${SCHEMA_NODE_IMAGE:-node:20-alpine}"
dbml_version="${SCHEMA_DBML_VERSION:-9.1.1}"
net="vmlens-schema-net"
db="vmlens-schema-db"
dsn="postgres://vmlens:vmlens@${db}:5432/vmlens?sslmode=disable"

check_only=false
if [[ "${1:-}" == "--check" ]]; then
  check_only=true
fi

work_dir="$(mktemp -d)"
cleanup() {
  docker rm -f "${db}" >/dev/null 2>&1 || true
  docker network rm "${net}" >/dev/null 2>&1 || true
  rm -rf "${work_dir}"
}
trap cleanup EXIT

cleanup
work_dir="$(mktemp -d)"
docker network create "${net}" >/dev/null

echo "==> starting throwaway postgres (${pg_image})"
docker run -d --name "${db}" --network "${net}" \
  -e POSTGRES_DB=vmlens -e POSTGRES_USER=vmlens -e POSTGRES_PASSWORD=vmlens \
  "${pg_image}" >/dev/null

# initdb starts a temporary server before the real one, so a single pg_isready
# can pass and then the socket disappears mid-migration. Require the server to
# answer a real query on two consecutive checks.
ready=0
for _ in $(seq 1 60); do
  if docker exec -e PGPASSWORD=vmlens "${db}" \
      psql -qtA -U vmlens -d vmlens -c 'SELECT 1' >/dev/null 2>&1; then
    ready=$((ready + 1))
    if [[ "${ready}" -ge 2 ]]; then
      break
    fi
  else
    ready=0
  fi
  sleep 1
done
if [[ "${ready}" -lt 2 ]]; then
  echo "postgres did not become ready" >&2
  exit 1
fi

echo "==> applying migrations"
for file in "${migrations_dir}"/*.sql; do
  echo "    $(basename "${file}")"
  docker exec -i -e PGPASSWORD=vmlens "${db}" \
    psql -v ON_ERROR_STOP=1 -q -U vmlens -d vmlens < "${file}"
done

echo "==> applying schema comments"
docker exec -i -e PGPASSWORD=vmlens "${db}" \
  psql -v ON_ERROR_STOP=1 -q -U vmlens -d vmlens < "${comments_file}"

echo "==> generating DBML (@dbml/cli@${dbml_version})"
docker run --rm --network "${net}" -v "${work_dir}:/out" "${node_image}" \
  npx -y -p "@dbml/cli@${dbml_version}" db2dbml postgres "${dsn}" -o /out/raw.dbml >/dev/null

if [[ ! -s "${work_dir}/raw.dbml" ]]; then
  echo "DBML generation produced no output" >&2
  exit 1
fi

# db2dbml splits an index expression on its commas, so the COALESCE in
# uq_connection_configurations_identity comes out as two broken columns. Rejoin
# it. The substitution is a no-op once upstream stops doing this.
sed -i "s/\`COALESCE(network_id\`, \` ''::text)\`/\`COALESCE(network_id, ''::text)\`/" \
  "${work_dir}/raw.dbml"

# Group, colour and order the raw output, then derive the minimal profile from
# the same source so the two cannot drift. Fails loudly when a new table has no
# group assigned, so a migration cannot slip past the diagram unnoticed.
echo "==> curating"
docker run --rm -v "${work_dir}:/out" -v "${repo_dir}/scripts:/scripts:ro" "${node_image}" \
  node /scripts/curate-dbml.js /out/raw.dbml /out/vmlens.dbml /out/vmlens-minimal.dbml

# Both curated files must still parse as DBML.
for name in vmlens vmlens-minimal; do
  docker run --rm -v "${work_dir}:/out" "${node_image}" \
    npx -y -p "@dbml/cli@${dbml_version}" dbml2sql "/out/${name}.dbml" --postgres \
    -o "/out/${name}-roundtrip.sql" >/dev/null
done

if [[ "${check_only}" == true ]]; then
  stale=0
  for name in "${outputs[@]}"; do
    if diff -q "${out_dir}/${name}.dbml" "${work_dir}/${name}.dbml" >/dev/null 2>&1; then
      echo "==> docs/schema/${name}.dbml is up to date"
    else
      echo "docs/schema/${name}.dbml is stale — run: make schema-dbml" >&2
      diff "${out_dir}/${name}.dbml" "${work_dir}/${name}.dbml" || true
      stale=1
    fi
  done
  exit "${stale}"
else
  mkdir -p "${out_dir}"
  for name in "${outputs[@]}"; do
    cp "${work_dir}/${name}.dbml" "${out_dir}/${name}.dbml"
    echo "==> wrote docs/schema/${name}.dbml"
  done
  echo "    paste either into https://dbdiagram.io to draw the ER diagram"
fi
