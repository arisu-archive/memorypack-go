# Change these variables as necessary.
TMP_DIR := ./tmp

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

.PHONY: prepare
prepare:
	mkdir -p ${TMP_DIR}
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5
	go env -w CGO_ENABLED=1

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

.PHONY: no-dirty
no-dirty:
	git diff --exit-code

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy -v

## audit: run quality control checks
.PHONY: audit
audit:
	go mod verify
	go vet ./...
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5 run ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## test: run all tests
.PHONY: test
test:
	go test -v -race -buildvcs ./...

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	go test -v -race -buildvcs -coverprofile=${TMP_DIR}/coverage.out.tmp ./...
	cat ${TMP_DIR}/coverage.out.tmp | grep -Ev "mock_|mocks/|cmd/|.pb.go" > ${TMP_DIR}/coverage.out
	go tool cover -html=${TMP_DIR}/coverage.out

# ==================================================================================== #
# OPERATIONS
# ==================================================================================== #

## push: push changes to the remote Git repository
.PHONY: push
push: tidy audit no-dirty
	git push
