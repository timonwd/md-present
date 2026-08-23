#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd -- "${script_dir}/.." && pwd)"

usage() {
  echo "usage: scripts/bump-version.sh [version]" >&2
  exit 1
}

increment_decimal() {
  local value="${1}"
  local result=""
  local carry=1
  local digit
  local index

  while [[ "${value}" == 0* && "${value}" != "0" ]]; do
    value="${value#0}"
  done

  for ((index = ${#value} - 1; index >= 0; index--)); do
    digit="${value:index:1}"
    if (( carry == 1 )); then
      if (( digit == 9 )); then
        digit=0
      else
        digit=$((digit + 1))
        carry=0
      fi
    fi
    result="${digit}${result}"
  done

  if (( carry == 1 )); then
    result="1${result}"
  fi
  printf '%s\n' "${result}"
}

if (( $# > 1 )); then
  usage
fi

requested_version="${1:-}"
version_pattern='^[0-9]+\.[0-9]+\.[0-9]+$'
if [[ -n "${requested_version}" && ! "${requested_version}" =~ ${version_pattern} ]]; then
  usage
fi

files=(
  "cmd/md-present/main.go"
  "plugins/md-present/.codex-plugin/plugin.json"
  "plugins/md-present/.claude-plugin/plugin.json"
  ".claude-plugin/marketplace.json"
)

versions=()
for path in "${files[@]}"; do
  if [[ "${path}" == "cmd/md-present/main.go" ]]; then
    matches="$(grep -Eo 'var version = "[^"]+"' "${root}/${path}" || true)"
    version="$(printf '%s\n' "${matches}" | sed -E 's/^var version = "([^"]+)"$/\1/')"
  else
    matches="$(grep -Eo '"version"[[:space:]]*:[[:space:]]*"[^"]+"' "${root}/${path}" || true)"
    version="$(printf '%s\n' "${matches}" | sed -E 's/^"version"[[:space:]]*:[[:space:]]*"([^"]+)"$/\1/')"
  fi

  match_count="$(printf '%s\n' "${matches}" | awk 'NF { count++ } END { print count + 0 }')"
  if [[ "${match_count}" -ne 1 ]]; then
    echo "${path}: expected exactly one version field, found ${match_count}" >&2
    exit 1
  fi

  versions+=("${version}")
done

current_version="${versions[0]}"
for index in "${!files[@]}"; do
  if [[ "${versions[${index}]}" != "${current_version}" ]]; then
    details=()
    for detail_index in "${!files[@]}"; do
      details+=("${files[${detail_index}]}=${versions[${detail_index}]}")
    done
    joined_details="${details[0]}"
    for detail_index in "${!details[@]}"; do
      if (( detail_index > 0 )); then
        joined_details+=", ${details[${detail_index}]}"
      fi
    done
    echo "version fields are not aligned: ${joined_details}" >&2
    exit 1
  fi
done

if [[ ! "${current_version}" =~ ${version_pattern} ]]; then
  echo "current version ${current_version} is not a plain semantic version" >&2
  exit 1
fi

if [[ -n "${requested_version}" ]]; then
  target_version="${requested_version}"
else
  prefix="${current_version%.*}"
  patch="${current_version##*.}"
  target_version="${prefix}.$(increment_decimal "${patch}")"
fi

if [[ "${current_version}" != "${target_version}" ]]; then
  for path in "${files[@]}"; do
    sed -E "s/(var version = \"|\"version\"[[:space:]]*:[[:space:]]*\")[^\"]+(\")/\\1${target_version}\\2/" \
      "${root}/${path}" > "${root}/${path}.tmp"
    mv "${root}/${path}.tmp" "${root}/${path}"
  done
fi

printf '%s\n' "${target_version}"
