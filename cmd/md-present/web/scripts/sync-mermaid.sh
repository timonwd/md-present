#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
web_assets="$(cd -- "${script_dir}/.." && pwd)"
mermaid_package="${web_assets}/node_modules/mermaid"

cp "${mermaid_package}/dist/mermaid.min.js" "${web_assets}/mermaid.min.js"
cp "${mermaid_package}/LICENSE" "${web_assets}/mermaid.LICENSE.txt"
