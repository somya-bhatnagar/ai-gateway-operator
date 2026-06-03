#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

fetch_component() {
    local name="$1" repo="$2" commit="$3" src_path="$4"
    local repo_url="https://github.com/opendatahub-io/${repo}"
    local dst="${PROJECT_ROOT}/config/manifests/${name}"

    if [[ "${USE_LOCAL:-}" == "true" ]] && [[ -d "${PROJECT_ROOT}/../${repo}" ]]; then
        echo "[${name}] Copying manifests from adjacent ${repo} checkout"
        rm -rf "${dst}"
        mkdir -p "${dst}"
        cp -a "${PROJECT_ROOT}/../${repo}/${src_path}/." "${dst}/"
    else
        echo "[${name}] Fetching ${repo}@${commit:0:7}"
        local tmp
        tmp=$(mktemp -d -t "odh-${name}-manifests.XXXXXXXXXX")

        git -C "${tmp}" init -q
        git -C "${tmp}" remote add origin "${repo_url}"
        git -C "${tmp}" fetch --depth 1 -q origin "${commit}"
        git -C "${tmp}" reset -q --hard FETCH_HEAD

        rm -rf "${dst}"
        mkdir -p "${dst}"
        cp -a "${tmp}/${src_path}/." "${dst}/"
        rm -rf "${tmp}"
    fi

    echo "[${name}] Manifests ready at ${dst}"
}

# TODO: remove once quay.io/opendatahub/odh-batch-gateway-operator is published
patch_batchgateway_image() {
    local dst="$1"
    sed -i.bak 's|BATCH_GATEWAY_OPERATOR_IMAGE=.*|BATCH_GATEWAY_OPERATOR_IMAGE=ghcr.io/opendatahub-io/batch-gateway-operator:main|' \
        "${dst}/base/params.env"
    rm -f "${dst}/base/params.env.bak"
}

# Update batchgateway manifests, change the commit SHA below and run: make get-manifests
fetch_component "batchgateway" "llm-d-batch-gateway-operator" "75286a071268b91db9904f8eeba19b6daa6250d4" "config"
patch_batchgateway_image "${PROJECT_ROOT}/config/manifests/batchgateway"
