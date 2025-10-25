APP_NAME=instrument-server
VERSION=1.0.0
BUILD_TIME=$(shell date +%Y-%m-%d_%H:%M:%S)
BUILD_DIR=build
MAIN_FILE=cmd/server/main.go

GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: all build clean test run deps lint docker help

all: clean build

## build: 编译项目
build:
	@echo "编译 $(APP_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_FILE)
	@echo "编译完成: $(BUILD_DIR)/$(APP_NAME)"

## run: 运行项目
run:
	$(GOCMD) run $(MAIN_FILE) -config configs/config.yaml

## test: 运行测试
test:
	$(GOTEST) -v -cover ./...

## clean: 清理构建文件
clean:
	@echo "清理..."
	@rm -rf $(BUILD_DIR)
	@$(GOCMD) clean

## deps: 安装/更新依赖
deps:
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "依赖安装完成"

## lint: 代码检查
lint:
	@which golangci-lint > /dev/null || (echo "安装 golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

## docker: 构建Docker镜像
docker:
	docker build -t $(APP_NAME):$(VERSION) .
	docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest

## docker-run: 运行Docker容器
docker-run:
	docker run -d \
		--name $(APP_NAME) \
		-p 8888:8888 \
		-p 9090:9090 \
		--restart always \
		$(APP_NAME):$(VERSION)

## build-all: 交叉编译所有平台
build-all:
	@echo "编译多平台版本..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(MAIN_FILE)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 $(MAIN_FILE)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe $(MAIN_FILE)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 $(MAIN_FILE)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 $(MAIN_FILE)
	@echo "所有平台编译完成"

## install: 安装到系统
install: build
	@echo "安装到 /usr/local/bin/..."
	@sudo cp $(BUILD_DIR)/$(APP_NAME) /usr/local/bin/
	@echo "安装完成"

## help: 显示帮助信息
help:
	@echo "可用命令:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
