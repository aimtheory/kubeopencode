#!/bin/bash
# Copyright Contributors to the KubeOpenCode project

set -e

# ============================================================================
# run-lint.sh - One-command golangci-lint runner for KubeOpenCode
#
# Automatically detects Go version from go.mod and selects a compatible
# golangci-lint version. Installs to LOCALBIN (default: ./bin) if needed.
#
# Usage in Makefile:
#   lint:
#     @./ci/lint/run-lint.sh
#
# Environment variables:
#   GOLANGCI_LINT_VERSION   - Override auto-detected golangci-lint version
#   LOCALBIN                - Binary install directory (default: ./bin)
# ============================================================================

LOCALBIN="${LOCALBIN:-$(pwd)/bin}"
LINT_BIN="${LOCALBIN}/golangci-lint"

# Detect Go version from go.mod (priority) or system
detect_go_version() {
    local go_version=""

    if [[ -f "go.mod" ]]; then
        go_version=$(grep -oE '^go [0-9]+\.[0-9]+' go.mod | sed 's/go //')
    fi

    if [[ -z "${go_version}" ]]; then
        go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
        echo "No go.mod found, using system Go version: ${go_version}" >&2
    fi

    echo "${go_version}"
}

# Select compatible golangci-lint version based on Go version
# Reference: https://github.com/golangci/golangci-lint/issues/5032
select_lint_version() {
    local go_version="$1"
    local major minor
    major=$(echo "$go_version" | cut -d. -f1)
    minor=$(echo "$go_version" | cut -d. -f2)

    # golangci-lint v2 compatibility:
    #   Go 1.24+ -> v2.8.0
    #   Go 1.23  -> v2.3.1 (last v2 supporting Go 1.23)
    if (( major == 1 && minor >= 24 )) || (( major > 1 )); then
        echo "v2.8.0"
    elif (( major == 1 && minor == 23 )); then
        echo "v2.3.1"
    else
        echo "Error: Go ${go_version} is not supported. Minimum required: Go 1.23" >&2
        exit 1
    fi
}

# Compare semantic versions: returns 0 if $1 >= $2
version_gte() {
    local v1="${1#v}" v2="${2#v}"
    local IFS='.'
    read -ra v1_parts <<< "$v1"
    read -ra v2_parts <<< "$v2"

    for i in 0 1 2; do
        local p1="${v1_parts[$i]:-0}"
        local p2="${v2_parts[$i]:-0}"
        if (( p1 > p2 )); then return 0; fi
        if (( p1 < p2 )); then return 1; fi
    done
    return 0
}

# Get the Go version that was used to build an existing binary
get_binary_go_version() {
    local bin="$1"
    "${bin}" version 2>/dev/null | grep -oE 'built with go[0-9]+\.[0-9]+' | sed 's/built with go//' || true
}

# Install golangci-lint if needed
# Uses `go install` to build from source so the binary matches the local Go toolchain.
# Pre-built binaries from GitHub releases may be compiled with an older Go version
# that cannot analyze projects targeting a newer Go version.
install_lint() {
    local target_version="$1"
    local go_version="$2"
    mkdir -p "${LOCALBIN}"

    # Check if current binary is already compatible
    if [[ -x "${LINT_BIN}" ]]; then
        local current_version
        current_version=$("${LINT_BIN}" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)
        current_version="v${current_version}"

        # Check: same major version and current >= target
        local current_major="${current_version#v}" target_major="${target_version#v}"
        current_major="${current_major%%.*}"
        target_major="${target_major%%.*}"

        if [[ "${current_major}" == "${target_major}" ]] && version_gte "${current_version}" "${target_version}"; then
            # Also verify the binary was built with a Go version >= project's Go version
            local binary_go
            binary_go=$(get_binary_go_version "${LINT_BIN}")
            if [[ -n "${binary_go}" ]] && version_gte "${binary_go}" "${go_version}"; then
                echo "golangci-lint ${current_version} already installed (>= ${target_version}, built with Go ${binary_go})"
                return 0
            fi
            echo "golangci-lint ${current_version} was built with Go ${binary_go:-unknown}, but project requires Go ${go_version}. Rebuilding..."
            rm -f "${LINT_BIN}"
        else
            echo "Current: ${current_version}, need: ${target_version}+. Upgrading..."
            rm -f "${LINT_BIN}"
        fi
    fi

    # Build from source using the local Go toolchain to ensure Go version compatibility
    echo "Installing golangci-lint ${target_version} from source (using Go $(go version | grep -oE 'go[0-9]+\.[0-9]+\.[0-9]*'))..."
    GOBIN="${LOCALBIN}" go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${target_version}"
}

# Main
main() {
    local go_version lint_version

    go_version=$(detect_go_version)

    if [[ -n "${GOLANGCI_LINT_VERSION:-}" ]]; then
        lint_version="${GOLANGCI_LINT_VERSION}"
        echo "Using override golangci-lint version: ${lint_version}"
    else
        lint_version=$(select_lint_version "${go_version}")
        echo "Go ${go_version} -> golangci-lint ${lint_version}"
    fi

    install_lint "${lint_version}" "${go_version}"

    echo ""
    echo "Running: ${LINT_BIN} run ./..."
    "${LINT_BIN}" run ./...
}

main "$@"
