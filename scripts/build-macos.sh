#!/bin/bash

# go-stock macOS 构建脚本
# 此脚本需要在 macOS 环境下运行
# 功能：检查依赖、构建应用、打包 DMG

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}======================================"
echo -e "  go-stock macOS 构建脚本"
echo -e "======================================${NC}"

# 检查是否在 macOS 环境下运行
if [ "$(uname -s)" != "Darwin" ]; then
    echo -e "\n${RED}错误：此脚本只能在 macOS 环境下运行${NC}"
    echo -e "macOS 应用必须在 macOS 上构建（Wails 依赖 Xcode 工具链）"
    echo -e "\n如需跨平台构建，请使用 GitHub Actions CI/CD"
    exit 1
fi

# 进入项目根目录
cd "$(dirname "$0")/.."
PROJECT_DIR="$(pwd)"

# 检测 CPU 架构
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    DEFAULT_PLATFORM="darwin/arm64"
    ARCH_LABEL="Apple Silicon (M1/M2/M3/M4)"
else
    DEFAULT_PLATFORM="darwin/amd64"
    ARCH_LABEL="Intel (x86_64)"
fi

echo -e "\n${GREEN}当前架构：${ARCH_LABEL}${NC}"

# 构建参数
PLATFORM="${1:-$DEFAULT_PLATFORM}"
BUILD_DMG="${2:-yes}"

# 显示可选项
echo -e "\n${CYAN}可用构建目标：${NC}"
echo -e "  darwin/amd64      - Intel 芯片"
echo -e "  darwin/arm64      - Apple Silicon (M1/M2/M3/M4)"
echo -e "  darwin/universal  - 通用二进制（同时支持 Intel 和 Apple Silicon）"
echo -e "\n${GREEN}当前构建目标：${PLATFORM}${NC}"

# ==================== 依赖检查 ====================

echo -e "\n${CYAN}[1/6] 检查依赖...${NC}"

# 检查 Xcode Command Line Tools
if ! xcode-select -p &> /dev/null; then
    echo -e "${YELLOW}  未找到 Xcode Command Line Tools，正在安装...${NC}"
    xcode-select --install
    echo -e "${YELLOW}  请等待 Xcode Command Line Tools 安装完成后重新运行此脚本${NC}"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Xcode Command Line Tools 已安装"

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo -e "${RED}  错误：未找到 Go 编译器，请先安装 Go 1.21+${NC}"
    echo -e "  下载地址：https://go.dev/dl/"
    echo -e "  或使用 Homebrew：brew install go"
    exit 1
fi
GO_VERSION=$(go version | awk '{print $3}')
echo -e "  ${GREEN}✓${NC} Go 版本：${GO_VERSION}"

# 检查 Node.js 是否安装
if ! command -v node &> /dev/null; then
    echo -e "${RED}  错误：未找到 Node.js，请先安装 Node.js 18+${NC}"
    echo -e "  下载地址：https://nodejs.org/"
    echo -e "  或使用 Homebrew：brew install node"
    exit 1
fi
NODE_VERSION=$(node --version)
echo -e "  ${GREEN}✓${NC} Node.js 版本：${NODE_VERSION}"

# 检查 npm 是否可用
if ! command -v npm &> /dev/null; then
    echo -e "${RED}  错误：未找到 npm${NC}"
    exit 1
fi
NPM_VERSION=$(npm --version)
echo -e "  ${GREEN}✓${NC} npm 版本：${NPM_VERSION}"

# 检查 Wails 是否安装
if ! command -v wails &> /dev/null; then
    echo -e "${YELLOW}  正在安装 Wails CLI...${NC}"
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    if ! command -v wails &> /dev/null; then
        echo -e "${RED}  错误：Wails 安装失败${NC}"
        echo -e "  请手动执行：go install github.com/wailsapp/wails/v2/cmd/wails@latest"
        exit 1
    fi
fi
WAILS_VERSION=$(wails version 2>/dev/null | head -1)
echo -e "  ${GREEN}✓${NC} Wails 版本：${WAILS_VERSION}"

# ==================== 清理旧构建 ====================

echo -e "\n${CYAN}[2/6] 清理旧的构建文件...${NC}"
rm -rf build/bin
echo -e "  ${GREEN}✓${NC} 已清理"

# ==================== 安装前端依赖 ====================

echo -e "\n${CYAN}[3/6] 安装前端依赖...${NC}"
cd frontend
npm install
cd ..
echo -e "  ${GREEN}✓${NC} 前端依赖安装完成"

# ==================== 构建前端 ====================

echo -e "\n${CYAN}[4/6] 构建前端...${NC}"
cd frontend
npm run build
cd ..
echo -e "  ${GREEN}✓${NC} 前端构建完成"

# ==================== 构建 macOS 应用 ====================

echo -e "\n${CYAN}[5/6] 构建 macOS 应用 (${PLATFORM})...${NC}"
wails build --clean --platform "${PLATFORM}"
echo -e "  ${GREEN}✓${NC} 应用构建完成"

# 确认产物
APP_PATH="build/bin/go-stock.app"
if [ ! -d "$APP_PATH" ]; then
    echo -e "${RED}  错误：未找到构建产物 ${APP_PATH}${NC}"
    # 尝试在子目录中查找
    APP_PATH=$(find build/bin -name "go-stock.app" -type d 2>/dev/null | head -1)
    if [ -z "$APP_PATH" ]; then
        echo -e "${RED}  构建失败，未生成 .app 包${NC}"
        exit 1
    fi
fi
echo -e "  ${GREEN}✓${NC} 应用路径：${APP_PATH}"

# ==================== 签名处理 ====================

echo -e "\n${CYAN}[5.5/6] 签名处理...${NC}"

# 移除扩展属性 (com.apple.quarantine) - 允许未签名应用运行
xattr -cr "${APP_PATH}" 2>/dev/null || true
echo -e "  ${GREEN}✓${NC} 已移除 quarantine 扩展属性"

# 尝试 Ad-hoc 签名（如果有钥匙串中的签名证书）
ADHOC_SIGNED=false
if command -v codesign &> /dev/null; then
    # 尝试使用开发者证书签名
    if security find-identity -v -p codesigning | grep -q "Developer ID"; then
        echo -e "  正在使用 Developer ID 签名..."
        codesign --force --deep --sign "Developer ID Application: sparkmemory (TEAMID)" "${APP_PATH}" 2>/dev/null && ADHOC_SIGNED=true
    fi
    
    # 如果没有开发者证书，尝试 ad-hoc 签名（仅移除"来自不明开发者"警告）
    if [ "$ADHOC_SIGNED" = false ]; then
        echo -e "  正在执行 Ad-hoc 签名..."
        codesign --force --deep --sign - "${APP_PATH}" 2>/dev/null && ADHOC_SIGNED=true || true
    fi
fi

if [ "$ADHOC_SIGNED" = true ]; then
    echo -e "  ${GREEN}✓${NC} 应用已签名"
else
    echo -e "  ${YELLOW}⚠${NC} 未签名（可通过 xattr -cr 或系统设置绕过）"
fi

# ==================== 打包 DMG ====================

echo -e "\n${CYAN}[6/6] 打包 DMG...${NC}"

# 确定输出文件名
VERSION=$(grep -o '"productVersion"[[:space:]]*:[[:space:]]*"[^"]*"' wails.json | head -1 | grep -o '"[^"]*"$' | tr -d '"')
if [ -z "$VERSION" ]; then
    VERSION="1.0.0"
fi

if [ "$PLATFORM" = "darwin/universal" ]; then
    SUFFIX="universal"
elif [ "$PLATFORM" = "darwin/arm64" ]; then
    SUFFIX="arm64"
else
    SUFFIX="amd64"
fi

DMG_NAME="go-stock_${VERSION}_macos_${SUFFIX}.dmg"
DMG_PATH="build/bin/${DMG_NAME}"

# 创建临时目录用于 DMG 内容
DMG_TMP_DIR=$(mktemp -d)
DMG_APP_DIR="${DMG_TMP_DIR}/go-stock"
mkdir -p "${DMG_APP_DIR}"

# 复制 .app 到临时目录
cp -R "${APP_PATH}" "${DMG_APP_DIR}/"

# 移除复制后的 quarantine 属性（避免用户从 DMG 安装后无法运行）
xattr -cr "${DMG_APP_DIR}/go-stock.app" 2>/dev/null || true

# 创建 Applications 快捷方式
ln -s /Applications "${DMG_APP_DIR}/Applications"

# 创建 DMG
echo -e "  正在创建 DMG: ${DMG_NAME}"
hdiutil create \
    -volname "go-stock" \
    -srcfolder "${DMG_TMP_DIR}" \
    -ov \
    -format UDZO \
    "${DMG_PATH}"

# 清理临时目录
rm -rf "${DMG_TMP_DIR}"

if [ -f "$DMG_PATH" ]; then
    DMG_SIZE=$(du -h "$DMG_PATH" | awk '{print $1}')
    echo -e "  ${GREEN}✓${NC} DMG 创建完成：${DMG_PATH} (${DMG_SIZE})"
else
    echo -e "  ${YELLOW}⚠ DMG 打包失败，但 .app 已构建成功${NC}"
fi

# ==================== 完成 ====================

echo -e "\n${CYAN}======================================"
echo -e "  构建完成！"
echo -e "======================================${NC}"
echo -e ""
echo -e "  ${GREEN}应用路径：${NC}${APP_PATH}"
if [ -f "$DMG_PATH" ]; then
    echo -e "  ${GREEN}DMG 路径：${NC}${DMG_PATH}"
fi
echo -e ""
echo -e "${CYAN}运行方式：${NC}"
echo -e "  方式1：直接运行"
echo -e "    open ${APP_PATH}"
echo -e ""
echo -e "  方式2：从 DMG 安装"
echo -e "    open ${DMG_PATH}"
echo -e "    将 go-stock.app 拖到 Applications 文件夹"
echo -e ""
echo -e "${CYAN}签名说明：${NC}"
echo -e "  构建脚本已执行以下操作："
echo -e "  1. 移除了 com.apple.quarantine 扩展属性"
echo -e "  2. 尝试了 Ad-hoc 签名"
echo -e ""
echo -e "  如果仍有「无法打开，因为无法确认开发者」提示，执行："
echo -e "    xattr -cr \"${APP_PATH}\""
echo -e ""
echo -e "  或在系统设置中允许："
echo -e "    系统设置 > 隐私与安全性 > 仍要打开"
echo -e ""
echo -e "  如需正式分发（App Store 或跳过 Gatekeeper），需要："
echo -e "  1. 购买 Apple Developer Program ($99/年)"
echo -e "  2. 使用 Developer ID 证书签名"
echo -e "  3. 将 TEAMID 填入脚本第 164 行"
echo -e ""
echo -e "${CYAN}自定义构建：${NC}"
echo -e "  ./scripts/build-macos.sh [平台] [是否打包DMG]"
echo -e "  示例："
echo -e "    ./scripts/build-macos.sh darwin/arm64 yes    # Apple Silicon + DMG"
echo -e "    ./scripts/build-macos.sh darwin/universal no # 通用二进制，不打包 DMG"
echo -e "    ./scripts/build-macos.sh darwin/amd64        # Intel 芯片"
echo -e ""
