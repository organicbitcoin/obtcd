#!/usr/bin/env bash

set -euo pipefail

readonly MAIN_WORKFLOW_FILE=".github/workflows/main.yml"
readonly RELEASE_WORKFLOW_FILE=".github/workflows/dimagespub.yml"
readonly RELEASE_DOCKERFILE=".github/workflows/Dockerfile"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${BLUE}[CI]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

print_section() {
    echo ""
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW} $1${NC}"
    echo -e "${YELLOW}========================================${NC}"
}

usage() {
    cat <<'EOF'
OBTC local GitHub Actions runner

Usage:
  scripts/ci-validate.sh [--quick|--full] [--release] [--docker-only] [--help]

Options:
  --quick        Run the fast local profile (build + OBTC smoke + quality).
  --full         Run the full main workflow simulation (default).
  --release      Include the release/tag workflow local simulation.
  --docker-only  Run only the release/tag workflow local simulation.
  --help         Show this help text.

Behavior:
  - Default run mirrors jobs in .github/workflows/main.yml.
  - --quick skips unit-cover, unit-race, rpctest, and build-matrix jobs.
  - --release additionally simulates .github/workflows/dimagespub.yml.
  - Coveralls upload and Docker push are replaced with local-only validation.
EOF
}

require_command() {
    local command_name="$1"

    if ! command -v "$command_name" >/dev/null 2>&1; then
        print_error "Missing required command: ${command_name}"
        return 1
    fi
}

run_cmd() {
    local description="$1"
    shift

    print_status "${description}"
    "$@" || return 1
}

workflow_go_version() {
    awk -F': ' '/GO_VERSION:/ {print $2; exit}' "${MAIN_WORKFLOW_FILE}"
}

release_platforms() {
    awk -F': ' '/TPLATFORMS:/ {print $2; exit}' "${RELEASE_WORKFLOW_FILE}"
}

repo_root="$(git rev-parse --show-toplevel)"
cd "${repo_root}"

run_main_workflow=1
run_release_workflow=0
validation_profile="full"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --quick)
            validation_profile="quick"
            ;;
        --full)
            validation_profile="full"
            ;;
        --release)
            run_release_workflow=1
            ;;
        --docker-only)
            run_main_workflow=0
            run_release_workflow=1
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Run 'scripts/ci-validate.sh --help' for usage."
            exit 1
            ;;
    esac
    shift
done

if [[ "${run_main_workflow}" -eq 0 && "${run_release_workflow}" -eq 0 ]]; then
    print_error "Nothing to run."
    exit 1
fi

require_command go
require_command make
require_command bash

tmp_root="$(mktemp -d "${TMPDIR:-/tmp}/obtc-ci.XXXXXX")"
go_cache_root="${OBTC_GO_CACHE_ROOT:-${TMPDIR:-/tmp}/obtc-go-cache}"
validate_file="${repo_root}/validate_obtc_local.go"
btcec_coverage_backup_existed=0

export GOCACHE="${GOCACHE:-${go_cache_root}/build}"
export GOTMPDIR="${GOTMPDIR:-${go_cache_root}/tmp}"
mkdir -p "${GOCACHE}" "${GOTMPDIR}"

if [[ -e btcec/coverage.txt.bak ]]; then
    btcec_coverage_backup_existed=1
fi

cleanup() {
    rm -rf "${tmp_root}"
    rm -f "${validate_file}"
    if [[ "${btcec_coverage_backup_existed}" -eq 0 ]]; then
        rm -f btcec/coverage.txt.bak
    fi
}
trap cleanup EXIT

check_go_version() {
    local expected current

    expected="$(workflow_go_version)"
    current="$(go version | awk '{print $3}' | sed 's/^go//')"

    if [[ -z "${expected}" ]]; then
        print_warn "Unable to read Go version from ${MAIN_WORKFLOW_FILE}"
        return 0
    fi

    if [[ "${current}" != "${expected}" ]]; then
        print_warn "Workflow uses Go ${expected}, local environment is Go ${current}"
        return 0
    fi

    print_status "Go version matches workflow: ${current}"
}

job_build() {
    run_cmd "Running make build" make build
}

job_unit_cover() {
    run_cmd "Running make unit-cover" make unit-cover
    print_warn "Coveralls upload is a GitHub-hosted step; local runner only produces coverage artifacts."
}

job_unit_race() {
    run_cmd "Running make unit-race" make unit-race
}

job_obtc_tests() {
    local script_files=(
        scripts/devnet-up.sh
        scripts/phase6/run_testnet_node.sh
        scripts/phase6/collect_validation_snapshot.sh
        scripts/phase6/seed_preflight.sh
        scripts/phase6/gen_testnet_conf.sh
        scripts/validation/testnet_smoke.sh
    )

    run_cmd "Running OBTC network tests" go test ./chaincfg ./wire -v -run "OBTC"

    run_cmd "Running OBTC fork height tests" go test ./chaincfg -v -run "Fork"

    print_status "Validating OBTC parameters"
    cat > "${validate_file}" <<'EOF'
package main

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
)

func main() {
	if !chaincfg.IsOBTC(&chaincfg.ObtcMainNetParams) {
		panic("OBTC MainNet not properly registered")
	}
	if !chaincfg.IsOBTC(&chaincfg.ObtcTestNetParams) {
		panic("OBTC TestNet not properly registered")
	}
	if !chaincfg.IsOBTC(&chaincfg.ObtcRegTestParams) {
		panic("OBTC RegTest not properly registered")
	}

	mainnetFork := chaincfg.GetOBTCForkHeight(&chaincfg.ObtcMainNetParams)
	testnetFork := chaincfg.GetOBTCForkHeight(&chaincfg.ObtcTestNetParams)
	regtestFork := chaincfg.GetOBTCForkHeight(&chaincfg.ObtcRegTestParams)

	if mainnetFork <= 0 || testnetFork <= 0 || regtestFork <= 0 {
		panic("Invalid fork heights detected")
	}

	fmt.Println("OBTC parameters validated successfully")
	fmt.Printf(
		"Fork heights - MainNet: %d, TestNet: %d, RegTest: %d\n",
		mainnetFork,
		testnetFork,
		regtestFork,
	)
}
EOF
    go run "${validate_file}" || return 1
    rm -f "${validate_file}"

    run_cmd "Listing scripts directory" ls -la scripts/

    run_cmd "Normalizing script execute bits" chmod +x "${script_files[@]}"

    run_cmd "Checking shell syntax" bash -n "${script_files[@]}"

    print_status "Checking help outputs"
    ./scripts/devnet-up.sh help >/dev/null || return 1
    ./scripts/phase6/run_testnet_node.sh --help >/dev/null || return 1
    ./scripts/phase6/collect_validation_snapshot.sh --help >/dev/null || return 1
    ./scripts/phase6/seed_preflight.sh --help >/dev/null || return 1
    ./scripts/phase6/gen_testnet_conf.sh --help >/dev/null || return 1
    ./scripts/validation/testnet_smoke.sh --help >/dev/null || return 1
}

job_rpctest() {
    run_cmd "Building btcd for rpctest harness" go build -o btcd .

    run_cmd "Running rpctest integration suite" go test -p 1 -tags=rpctest ./integration/... -count=1 -v
}

job_quality() {
    local unformatted

    run_cmd "Running go vet ./..." go vet ./...

    print_status "Checking gofmt -s -l ."
    unformatted="$(gofmt -s -l .)"
    if [[ -n "${unformatted}" ]]; then
        print_error "Code is not properly formatted. Run 'gofmt -s -w .'"
        echo "${unformatted}"
        return 1
    fi
}

job_build_matrix() {
    local build_dir os suffix
    build_dir="${tmp_root}/build-matrix"
    mkdir -p "${build_dir}"

    for os in linux windows darwin; do
        suffix=""
        if [[ "${os}" == "windows" ]]; then
            suffix=".exe"
        fi

        print_status "Cross-building btcd for ${os}/amd64"
        GOOS="${os}" GOARCH=amd64 go build -o "${build_dir}/btcd-${os}-amd64${suffix}" . || return 1

        print_status "Cross-building btcctl for ${os}/amd64"
        GOOS="${os}" GOARCH=amd64 go build -o "${build_dir}/btcctl-${os}-amd64${suffix}" ./cmd/btcctl || return 1
    done
}

job_release_docker() {
    local platforms image_tag

    require_command docker || return 1

    if ! docker buildx version >/dev/null 2>&1; then
        print_error "docker buildx is required for release workflow simulation"
        return 1
    fi

    platforms="$(release_platforms)"
    if [[ -z "${platforms}" ]]; then
        print_error "Unable to read TPLATFORMS from ${RELEASE_WORKFLOW_FILE}"
        return 1
    fi

    image_tag="obtcd-local:$(git describe --tags --always --dirty 2>/dev/null || git rev-parse --short HEAD)"

    print_warn "Docker registry login, metadata extraction, and push are replaced by a local build-only check."
    print_status "Running docker buildx build for platforms: ${platforms}"
    docker buildx build \
        --platform "${platforms}" \
        --file "${RELEASE_DOCKERFILE}" \
        --tag "${image_tag}" \
        . || return 1
}

run_job() {
    local job_name="$1"
    local job_func="$2"

    print_section "${job_name}"
    if "${job_func}"; then
        print_success "${job_name} passed"
    else
        failed_jobs+=("${job_name}")
        print_error "${job_name} failed"
    fi
}

failed_jobs=()

main() {
    print_section "OBTC local Actions runner"
    check_go_version

    if [[ "${run_main_workflow}" -eq 1 ]]; then
        if [[ "${validation_profile}" == "quick" ]]; then
            print_warn "Quick profile enabled: skipping unit-cover, unit-race, rpctest, and build-matrix."
            run_job "Build" job_build
            run_job "OBTC smoke" job_obtc_tests
            run_job "Code quality" job_quality
        else
            run_job "Build" job_build
            run_job "Unit coverage" job_unit_cover
            run_job "Unit race" job_unit_race
            run_job "OBTC integration" job_obtc_tests
            run_job "RPC integration (rpctest)" job_rpctest
            run_job "Code quality" job_quality
            run_job "Build matrix" job_build_matrix
        fi
    fi

    if [[ "${run_release_workflow}" -eq 1 ]]; then
        run_job "Docker release build" job_release_docker
    fi

    print_section "Summary"
    if [[ "${#failed_jobs[@]}" -eq 0 ]]; then
        print_success "All selected local workflow simulations passed"
        return 0
    fi

    print_error "The following workflow groups failed:"
    for job_name in "${failed_jobs[@]}"; do
        echo "  - ${job_name}"
    done
    return 1
}

main
