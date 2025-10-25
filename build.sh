#!/bin/bash

# 配置
APP_NAME="instrument-server"
VERSION="v1.0.0"
BUILD_DIR="build"
MAIN_FILE="cmd/server/main.go"  # 或 "." 如果在根目录

# 清理旧文件
rm -rf $BUILD_DIR
mkdir -p $BUILD_DIR

# 构建信息
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 编译选项
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

echo "🚀 开始编译 ${APP_NAME} ${VERSION}"
echo "📦 构建时间: ${BUILD_TIME}"
echo "📝 Git Commit: ${GIT_COMMIT}"
echo ""

# 编译函数
build() {
    local GOOS=$1
    local GOARCH=$2
    local OUTPUT="${BUILD_DIR}/${APP_NAME}-${GOOS}-${GOARCH}"
    
    if [ "$GOOS" = "windows" ]; then
        OUTPUT="${OUTPUT}.exe"
    fi
    
    echo "🔨 编译 ${GOOS}/${GOARCH}..."
    
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="${LDFLAGS}" -o "$OUTPUT" $MAIN_FILE
    
    if [ $? -eq 0 ]; then
        # 显示文件大小
        SIZE=$(ls -lh "$OUTPUT" | awk '{print $5}')
        echo "   ✅ 成功! 大小: ${SIZE}"
        
        # 压缩
        if command -v upx &> /dev/null; then
            echo "   📦 使用 UPX 压缩..."
            upx -q "$OUTPUT" 2>/dev/null
            SIZE_AFTER=$(ls -lh "$OUTPUT" | awk '{print $5}')
            echo "   ✅ 压缩后: ${SIZE_AFTER}"
        fi
    else
        echo "   ❌ 失败!"
        return 1
    fi
    echo ""
}

# 编译各平台
build linux amd64
build linux arm64
build windows amd64
build darwin amd64
build darwin arm64

# 生成校验和
cd $BUILD_DIR
echo "🔐 生成校验和..."
sha256sum * > checksums.txt
cd ..

echo "✅ 编译完成!"
echo "📁 输出目录: ${BUILD_DIR}/"
ls -lh ${BUILD_DIR}/
