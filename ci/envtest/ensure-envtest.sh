#!/bin/bash
# Copyright Contributors to the KubeOpenCode project

# envtest environment setup script for KubeOpenCode
#
# Installs setup-envtest and downloads the required envtest binaries
# (kube-apiserver, etcd, kubectl). Outputs ONLY the KUBEBUILDER_ASSETS
# path to stdout; all progress/diagnostic messages go to stderr, so
# Makefiles can capture the path via $(shell ...).
#
# Usage:
#   Makefile integration:
#     integration-test:
#       KUBEBUILDER_ASSETS="$$(./ci/envtest/ensure-envtest.sh)" go test ...
#
#   Direct execution:
#     export KUBEBUILDER_ASSETS=$(./ci/envtest/ensure-envtest.sh)
#     go test -tags=integration ./internal/controller/...
#
# Environment variables:
#   ENVTEST_K8S_VERSION    - Override auto-detected K8s version (e.g. "1.35.3")
#   ENVTEST_SETUP_VERSION  - Override setup-envtest branch (e.g. "release-0.23")
#   LOCALBIN               - Binary install directory (default: ./bin)

set -eo pipefail

log() {
    echo "$@" >&2
}

###############################################################################
# detect_k8s_version
#
# Determines the K8s version from go.mod: k8s.io/api v0.X.Y -> 1.X.Y
###############################################################################
detect_k8s_version() {
    if [[ -n "${ENVTEST_K8S_VERSION:-}" ]]; then
        log "Using user-specified K8s version: ${ENVTEST_K8S_VERSION}"
        echo "${ENVTEST_K8S_VERSION}"
        return
    fi

    if [[ ! -f "go.mod" ]]; then
        log "Error: go.mod not found and ENVTEST_K8S_VERSION not set"
        exit 1
    fi

    local k8s_mod_version
    k8s_mod_version=$(grep -E '^\s+k8s\.io/api\s+v' go.mod | head -1 | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' || true)

    if [[ -z "${k8s_mod_version}" ]]; then
        log "Error: Could not detect k8s.io/api version from go.mod"
        log "Set ENVTEST_K8S_VERSION explicitly"
        exit 1
    fi

    # v0.X.Y -> 1.X.Y
    local minor patch
    minor=$(echo "${k8s_mod_version}" | cut -d. -f2)
    patch=$(echo "${k8s_mod_version}" | cut -d. -f3)
    local k8s_version="1.${minor}.${patch}"

    log "Detected K8s version from go.mod (k8s.io/api ${k8s_mod_version}): ${k8s_version}"
    echo "${k8s_version}"
}

###############################################################################
# detect_setup_envtest_version
#
# Determines the setup-envtest branch from go.mod:
# controller-runtime v0.X.Y -> release-0.X
#
# Minimum: release-0.19 (older versions use the deprecated GCS bucket)
###############################################################################
MIN_SETUP_ENVTEST_VERSION="release-0.19"

detect_setup_envtest_version() {
    if [[ -n "${ENVTEST_SETUP_VERSION:-}" ]]; then
        log "Using user-specified setup-envtest version: ${ENVTEST_SETUP_VERSION}"
        echo "${ENVTEST_SETUP_VERSION}"
        return
    fi

    if [[ ! -f "go.mod" ]]; then
        log "Error: go.mod not found and ENVTEST_SETUP_VERSION not set"
        exit 1
    fi

    local cr_version
    cr_version=$(grep -E '^\s+sigs\.k8s\.io/controller-runtime\s+v' go.mod | head -1 | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' || true)

    if [[ -z "${cr_version}" ]]; then
        log "Error: Could not detect controller-runtime version from go.mod"
        log "Set ENVTEST_SETUP_VERSION explicitly"
        exit 1
    fi

    local major minor
    major=$(echo "${cr_version}" | sed 's/^v//' | cut -d. -f1)
    minor=$(echo "${cr_version}" | cut -d. -f2)
    local setup_version="release-${major}.${minor}"

    # Enforce minimum version
    local min_major min_minor
    min_major=$(echo "${MIN_SETUP_ENVTEST_VERSION}" | sed 's/^release-//' | cut -d. -f1)
    min_minor=$(echo "${MIN_SETUP_ENVTEST_VERSION}" | sed 's/^release-//' | cut -d. -f2)

    if (( major < min_major || (major == min_major && minor < min_minor) )); then
        log "Detected setup-envtest ${setup_version}, upgrading to ${MIN_SETUP_ENVTEST_VERSION} (older uses deprecated GCS bucket)"
        setup_version="${MIN_SETUP_ENVTEST_VERSION}"
    fi

    log "Detected setup-envtest version from go.mod (controller-runtime ${cr_version}): ${setup_version}"
    echo "${setup_version}"
}

###############################################################################
# install_setup_envtest
###############################################################################
install_setup_envtest() {
    local setup_version="$1"
    local bin_dir="$2"

    local setup_envtest="${bin_dir}/setup-envtest"
    local version_marker="${bin_dir}/.setup-envtest-version"

    if [[ -x "${setup_envtest}" ]] && [[ -f "${version_marker}" ]] && [[ "$(cat "${version_marker}")" == "${setup_version}" ]]; then
        log "setup-envtest@${setup_version} already installed"
        return
    fi

    log "Installing setup-envtest@${setup_version}..."
    GOBIN="${bin_dir}" go install "sigs.k8s.io/controller-runtime/tools/setup-envtest@${setup_version}"
    echo "${setup_version}" > "${version_marker}"
    log "setup-envtest installed successfully"
}

###############################################################################
# ensure_binaries
#
# Downloads envtest binaries. Tries exact version, then major.minor.x, then latest.
###############################################################################
ensure_binaries() {
    local k8s_version="$1"
    local bin_dir="$2"

    local setup_envtest="${bin_dir}/setup-envtest"
    local k8s_major_minor
    k8s_major_minor=$(echo "${k8s_version}" | cut -d. -f1,2)

    local try_version assets_path
    for try_version in "${k8s_version}" "${k8s_major_minor}.x" "latest"; do
        log "Trying envtest binaries for K8s ${try_version}..."
        assets_path=$("${setup_envtest}" use "${try_version}" --bin-dir "${bin_dir}" -p path 2>/dev/null || true)
        if [[ -n "${assets_path}" ]] && [[ -d "${assets_path}" ]]; then
            log "envtest binaries ready at: ${assets_path} (resolved from ${try_version})"
            echo "${assets_path}"
            return
        fi
        log "Version ${try_version} not available, trying next fallback..."
    done

    log "Error: could not find any usable envtest binaries"
    log "Tried: ${k8s_version}, ${k8s_major_minor}.x, latest"
    exit 1
}

###############################################################################
# main
###############################################################################
main() {
    log "=== envtest environment setup ==="

    if ! command -v go &>/dev/null; then
        log "Error: Go is not installed or not in PATH"
        exit 1
    fi

    local k8s_version setup_version bin_dir

    k8s_version=$(detect_k8s_version)
    setup_version=$(detect_setup_envtest_version)
    bin_dir="${LOCALBIN:-$(pwd)/bin}"

    mkdir -p "${bin_dir}"
    bin_dir="$(cd "${bin_dir}" && pwd)"

    install_setup_envtest "${setup_version}" "${bin_dir}"
    ensure_binaries "${k8s_version}" "${bin_dir}"
}

main "$@"
