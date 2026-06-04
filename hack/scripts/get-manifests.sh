#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

fetch_component() {
    local component_name="$1"
    local repo_name="$2"
    local src_path="$3"
    local odh_commit="$4"
    local rhoai_commit="$5"

    local dst_manifests_dir="${PROJECT_ROOT}/config/manifests/${component_name}"

    # Always wipe the component dir before copy — see manifest-script.md in
    # .agents/skills/odh-component-to-module/references/

    local repo_url
    local commit_sha
    if [[ "${ODH_PLATFORM_TYPE:-OpenDataHub}" == "OpenDataHub" ]]; then
        echo "Downloading manifests for ODH"
        repo_url="https://github.com/opendatahub-io/${repo_name}"
        commit_sha="$odh_commit"
    else
        echo "Downloading manifests for RHOAI"
        repo_url="https://github.com/red-hat-data-services/${repo_name}"
        commit_sha="$rhoai_commit"
    fi

    if [[ -z "${commit_sha}" ]]; then
        echo "[ERROR] No commit SHA for ${component_name} (platform: ${ODH_PLATFORM_TYPE:-OpenDataHub})" >&2
        return 1
    fi

    if [[ "${USE_LOCAL:-}" == "true" ]] && [[ -d "${PROJECT_ROOT}/../${repo_name}" ]]; then
        echo "Copying manifests from adjacent ${repo_name} checkout"
        rm -rf "${dst_manifests_dir}"
        mkdir -p "${dst_manifests_dir}"
        cp -a "${PROJECT_ROOT}/../${repo_name}/${src_path}/." "${dst_manifests_dir}/"
        echo "Manifests copied to ${dst_manifests_dir}"
        return
    fi

    local tmp_dir=$(mktemp -d -t "odh-${component_name}-manifests.XXXXXXXXXX")

    git -C "${tmp_dir}" init -q
    git -C "${tmp_dir}" remote add origin "${repo_url}"
    git -C "${tmp_dir}" fetch --depth 1 -q origin "${commit_sha}"
    git -C "${tmp_dir}" reset -q --hard "${commit_sha}"

    rm -rf "${dst_manifests_dir}"
    mkdir -p "${dst_manifests_dir}"
    cp -a "${tmp_dir}/${src_path}/." "${dst_manifests_dir}/"

    rm -rf "${tmp_dir}"

    echo "[${component_name}] Manifests ready at ${dst_manifests_dir}"
}

# TODO: remove once quay.io/opendatahub/odh-batch-gateway-operator is published
patch_batchgateway_image() {
    local dst="$1"
    sed -i.bak 's|BATCH_GATEWAY_OPERATOR_IMAGE=.*|BATCH_GATEWAY_OPERATOR_IMAGE=ghcr.io/opendatahub-io/batch-gateway-operator:main|' \
        "${dst}/base/params.env"
    rm -f "${dst}/base/params.env.bak"
}

# Update batchgateway manifests, change the commit SHA below and run: make get-manifests
fetch_component "batchgateway" "llm-d-batch-gateway-operator" "config" "75286a071268b91db9904f8eeba19b6daa6250d4" ""
patch_batchgateway_image "${PROJECT_ROOT}/config/manifests/batchgateway"
