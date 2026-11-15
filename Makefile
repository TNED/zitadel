# Variables
BINARY_NAME ?= zitadel
BUILD_DIR ?= .artifacts/bin
COMMIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")

GO ?= go
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
CGO_ENABLED ?= 0

LDFLAGS := -s -w \
	-X 'github.com/zitadel/zitadel/cmd/build.commit=$(COMMIT_SHA)' \
	-X 'github.com/zitadel/zitadel/cmd/build.date=$(BUILD_DATE)' \
	-X 'github.com/zitadel/zitadel/cmd/build.version=$(VERSION)'

.PHONY: all
all: build

## 编译
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)/$(GOOS)/$(GOARCH)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(GOOS)/$(GOARCH)/$(BINARY_NAME) .

.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 $(MAKE) build

## 测试
.PHONY: test
test: test-unit

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	$(GO) test -race -coverprofile=profile.cov -coverpkg=./internal/... ./...

.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	$(GO) test -race -tags=integration -timeout=30m ./...

## 代码检查与格式化
.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run --timeout=15m

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix

## 代码生成
.PHONY: generate
generate: generate-statik

.PHONY: generate-statik
generate-statik:
	@echo "Generating statik assets..."
	$(GO) generate internal/statik/generate.go
	$(GO) generate internal/notification/statik/generate.go

## Docker 镜像打包
.PHONY: docker
docker:
	@echo "Building Docker image $(BINARY_NAME):local..."
	docker build -f Dockerfile -t $(BINARY_NAME):local .

## 清理产物
.PHONY: clean
clean:
	@echo "Cleaning artifacts..."
	rm -rf .artifacts $(BUILD_DIR) profile*.cov tmp
