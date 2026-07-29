#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${VMLENS_REPO_URL:-https://github.com/sonix03/vmlens-ebpf.git}"
REPO_REF="${VMLENS_REPO_REF:-}"
SOURCE_DIR="${VMLENS_SOURCE_DIR:-${HOME}/vmlens-ebpf}"
FRESH_SOURCE="${FRESH_SOURCE:-true}"

normalize_bool() {
  local name="$1"
  local value="$2"
  case "${value,,}" in
    true|1|yes|on) printf 'true\n' ;;
    false|0|no|off) printf 'false\n' ;;
    *) echo "${name} must be true or false, got: ${value}" >&2; return 1 ;;
  esac
}

FRESH_SOURCE="$(normalize_bool FRESH_SOURCE "${FRESH_SOURCE}")"

command -v git >/dev/null 2>&1 || { echo "git is required" >&2; exit 1; }

if [[ -e "${SOURCE_DIR}" ]]; then
  if [[ "${FRESH_SOURCE}" == "true" ]]; then
    backup="${SOURCE_DIR}.backup.$(date +%Y%m%d%H%M%S)"
    mv "${SOURCE_DIR}" "${backup}"
    echo "Existing source moved to ${backup}"
  elif [[ -d "${SOURCE_DIR}/.git" ]]; then
    git -C "${SOURCE_DIR}" fetch --all --prune
    if [[ -n "${REPO_REF}" ]]; then
      git -C "${SOURCE_DIR}" checkout "${REPO_REF}"
    fi
    if git -C "${SOURCE_DIR}" symbolic-ref -q HEAD >/dev/null; then
      git -C "${SOURCE_DIR}" pull --ff-only
    else
      echo "Source is on detached ref ${REPO_REF}; skipped git pull"
    fi
    echo "${SOURCE_DIR}"
    exit 0
  else
    echo "${SOURCE_DIR} exists and is not a git repository. Set FRESH_SOURCE=true to move it aside." >&2
    exit 1
  fi
fi

clone_args=(clone --depth 1)
if [[ -n "${REPO_REF}" ]]; then
  clone_args+=(--branch "${REPO_REF}")
fi
clone_args+=("${REPO_URL}" "${SOURCE_DIR}")

git "${clone_args[@]}"
echo "${SOURCE_DIR}"
