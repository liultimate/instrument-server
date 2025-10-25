# 配置
APP_NAME := instrument-server
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 编译选项
LDFLAGS := -s -w \
	-X main.Version=$(VERSION) \
	-X main.BuildTime=$(BUILD_TIME) \
	-X main.GitCommit=$(GIT_COMMIT)

# 目录
BUILD_DIR := dist
SRC := .

# 平台列表
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	windows/amd64 \
	darwin/amd64 \
	darwin/arm64

.PHONY: all clean build-all $(PLATFORMS)

# 默认构建所有平台
all: clean build-all

# 构建所有平台
build-all: $(PLATFORMS)

# 构建单个平台
$(PLATFORMS):
	$(eval GOOS := $(word 1,$(subst /, ,$@)))
	$(eval GOARCH := $(word 2,$(subst /, ,$@)))
	$(eval OUTPUT := $(BUILD_DIR)/$(APP_NAME)-$(GOOS)-$(GOARCH))
	$(eval EXT := $(if $(filter windows,$(GOOS)),.exe,))
	@echo "🔨 编译 $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS)" -o $(OUTPUT)$(EXT) $(SRC)
	@echo "   ✅ $(OUTPUT)$(EXT)"

# 编译当前平台
build:
	@echo "🔨 编译当前平台..."
	@go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) $(SRC)
	@echo "✅ 完成!"

# 编译并压缩
build-compressed: build-all
	@echo "📦 压缩二进制文件..."
	@command -v upx >/dev/null 2>&1 && upx -q $(BUILD_DIR)/* || echo "⚠️  UPX 未安装，跳过压缩"

# 生成校验和
checksums:
	@echo "🔐 生成校验和..."
	@cd $(BUILD_DIR) && sha256sum * > checksums.txt
	@echo "✅ 校验和已保存到 $(BUILD_DIR)/checksums.txt"

# 清理
clean:
	@echo "🧹 清理构建文件..."
	@rm -rf $(BUILD_DIR)
	@echo "✅ 清理完成!"

# 运行
run:
	@go run $(SRC)

# 测试
test:
	@go test -v ./...

# 安装依赖
deps:
	@go mod download
	@go mod tidy

# 显示信息
info:
	@echo "应用名称: $(APP_NAME)"
	@echo "版本:     $(VERSION)"
	@echo "构建时间: $(BUILD_TIME)"
	@echo "Git提交:  $(GIT_COMMIT)"

# 帮助
help:
	@echo "可用命令:"
	@echo "  make all              - 清理并构建所有平台"
	@echo "  make build            - 构建当前平台"
	@echo "  make build-all        - 构建所有平台"
	@echo "  make build-compressed - 构建并压缩"
	@echo "  make linux/amd64      - 构建指定平台"
	@echo "  make clean            - 清理构建文件"
	@echo "  make run              - 运行程序"
	@echo "  make test             - 运行测试"
	@echo "  make checksums        - 生成校验和"
	@echo "  make info             - 显示构建信息"
