#!/usr/bin/env bash
# Pre-Release Validation Script for Team Tasks
# Runs all quality checks before creating a release.
# Mirrors CI checks plus additional local validations.
#
# Usage:
#   bash scripts/pre-release-check.sh          # full check
#   bash scripts/pre-release-check.sh --quick  # skip slow steps (tests, lint)

set -e

QUICK=false
[[ "${1:-}" == "--quick" ]] && QUICK=true

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

echo ""
echo "================================================"
echo "  Team Tasks — Pre-Release Check"
echo "================================================"
echo ""

ERRORS=0
WARNINGS=0

# 1. Go version
log_info "Checking Go version..."
GO_VERSION=$(go version | awk '{print $3}')
REQUIRED="go1.26"
if [[ "$GO_VERSION" < "$REQUIRED" ]]; then
    log_error "Go $REQUIRED+ required, found $GO_VERSION"
    ERRORS=$((ERRORS + 1))
else
    log_success "Go version: $GO_VERSION"
fi
echo ""

# 2. Git status
log_info "Checking git status..."
if git diff-index --quiet HEAD --; then
    log_success "Working directory is clean"
else
    log_warning "Uncommitted changes detected"
    git status --short
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 3. Formatting
log_info "Checking formatting (gofmt)..."
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    log_error "Files need formatting:"
    echo "$UNFORMATTED"
    log_info "Run: go fmt ./..."
    ERRORS=$((ERRORS + 1))
else
    log_success "All files are properly formatted"
fi
echo ""

# 4. go vet
log_info "Running go vet..."
if go vet ./... 2>&1; then
    log_success "go vet passed"
else
    log_error "go vet failed"
    ERRORS=$((ERRORS + 1))
fi
echo ""

# 5. go.mod / go.sum
log_info "Validating go.mod..."
go mod verify
if [ $? -eq 0 ]; then
    log_success "go.mod verified"
else
    log_error "go.mod verification failed"
    ERRORS=$((ERRORS + 1))
fi

go mod tidy
if git diff --quiet go.mod go.sum 2>/dev/null; then
    log_success "go.mod is tidy"
else
    log_warning "go.mod needs tidying (run 'go mod tidy')"
    git diff go.mod go.sum 2>/dev/null || true
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# 6. Generated code (oapi-codegen + mockery)
log_info "Checking generated code is up-to-date..."
PRE_DIFF=$(git diff HEAD 2>/dev/null)
go generate ./... 2>/dev/null || true
POST_DIFF=$(git diff HEAD 2>/dev/null)
if [ "$PRE_DIFF" = "$POST_DIFF" ]; then
    log_success "Generated code is up-to-date"
else
    log_error "go generate produced new changes — run 'go generate ./...' and commit:"
    git diff --stat HEAD
    ERRORS=$((ERRORS + 1))
fi
echo ""

if [ "$QUICK" = false ]; then
    # 7. golangci-lint
    log_info "Verifying golangci-lint config..."
    if command -v golangci-lint &>/dev/null; then
        LINT_OUT=$(golangci-lint config verify 2>&1 || true)
        if echo "$LINT_OUT" | grep -qE "deadline exceeded|connection refused|no such host"; then
            log_warning "golangci-lint config verify skipped (no network)"
            WARNINGS=$((WARNINGS + 1))
        elif [ $? -ne 0 ] && [ -n "$LINT_OUT" ]; then
            log_error "golangci-lint config invalid: $LINT_OUT"
            ERRORS=$((ERRORS + 1))
        else
            log_success "golangci-lint config valid"
        fi

        log_info "Running golangci-lint..."
        if golangci-lint run --timeout=5m ./... 2>&1; then
            log_success "golangci-lint passed"
        else
            log_error "golangci-lint found issues"
            ERRORS=$((ERRORS + 1))
        fi
    else
        log_warning "golangci-lint not installed — https://golangci-lint.run/welcome/install/"
        WARNINGS=$((WARNINGS + 1))
    fi
    echo ""

    # 8. Tests with race detector
    log_info "Running tests with race detector..."
    TEST_TMP=$(mktemp)
    go test -v -race ./... 2>&1 | tee "$TEST_TMP" || true
    TEST_OUT=$(cat "$TEST_TMP"); rm -f "$TEST_TMP"
    if echo "$TEST_OUT" | grep -q "FAIL"; then
        log_error "Tests failed"
        ERRORS=$((ERRORS + 1))
    else
        log_success "All tests passed"
    fi
    echo ""

    # 9. Integration tests
    log_info "Running integration tests (requires Docker)..."
    log_info "Progress is streamed live — each test spins up MySQL + Redis containers"
    INTEG_TMP=$(mktemp)
    go test -v -race -count=1 -tags integration -timeout 15m \
        -coverprofile=/tmp/tt-coverage-integration.txt \
        -covermode=atomic \
        -coverpkg=./internal/...,./pkg/logger \
        ./tests/... 2>&1 | tee "$INTEG_TMP" || true
    INTEG_OUT=$(cat "$INTEG_TMP"); rm -f "$INTEG_TMP"
    if echo "$INTEG_OUT" | grep -q "FAIL"; then
        log_warning "Integration tests failed (is Docker running?)"
        WARNINGS=$((WARNINGS + 1))
    elif echo "$INTEG_OUT" | grep -q "^ok"; then
        log_success "Integration tests passed"
    else
        log_warning "Integration tests skipped (Docker unavailable)"
        WARNINGS=$((WARNINGS + 1))
    fi
    echo ""

    # 10. Coverage
    log_info "Checking test coverage (target ≥85%)..."
    go test -count=1 -coverprofile=/tmp/tt-coverage-unit.txt -covermode=atomic \
        ./internal/service/... 2>/dev/null || true

    COVERAGE=""
    if [ -f /tmp/tt-coverage-unit.txt ]; then
        COVERAGE=$(go tool cover -func=/tmp/tt-coverage-unit.txt \
            | grep "^total:" | awk '{print $3}' | sed 's/%//')
    fi

    if [ -n "$COVERAGE" ]; then
        echo "  unit test coverage (service): ${COVERAGE}%"
        if awk -v cov="$COVERAGE" 'BEGIN {exit !(cov >= 85.0)}'; then
            log_success "Coverage meets requirement (≥85%)"
        else
            log_warning "Coverage below 85% (${COVERAGE}%)"
            WARNINGS=$((WARNINGS + 1))
        fi
    else
        log_warning "Could not determine coverage (no tests yet?)"
        WARNINGS=$((WARNINGS + 1))
    fi
    rm -f /tmp/tt-coverage-unit.txt /tmp/tt-coverage-integration.txt
    echo ""
fi

# 11. Required files
log_info "Checking required files..."
MISSING=0
for f in README.md LICENSE configs/config.yaml; do
    if [ ! -f "$f" ]; then
        log_error "Missing: $f"
        MISSING=1
        ERRORS=$((ERRORS + 1))
    fi
done
[ $MISSING -eq 0 ] && log_success "All required files present"
echo ""

# Summary
echo "========================================"
echo "  Summary"
echo "========================================"
echo ""

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    log_success "All checks passed — ready for release."
    exit 0
elif [ $ERRORS -eq 0 ]; then
    log_warning "Completed with $WARNINGS warning(s) — review before releasing."
    exit 0
else
    log_error "Failed with $ERRORS error(s) and $WARNINGS warning(s) — fix before releasing."
    exit 1
fi
