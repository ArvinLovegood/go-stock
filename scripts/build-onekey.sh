#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# 默认构建 macOS 通用包，可通过第一个参数覆盖
PLATFORM="${1:-darwin/universal}"

echo "======================================"
echo "  go-stock 一键构建脚本"
echo "======================================"
echo "目标平台: ${PLATFORM}"

require_cmd() {
  local cmd="$1"
  local tip="$2"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "错误：未找到 ${cmd}。${tip}"
    exit 1
  fi
}

echo "[1/5] 检查基础依赖..."
require_cmd go "请先安装 Go 1.26+"
require_cmd node "请先安装 Node.js"
require_cmd npm "请先安装 npm"

echo "  Go:   $(go version)"
echo "  Node: $(node -v)"
echo "  npm:  $(npm -v)"

echo "[2/5] 检查并补齐 Go bin 到 PATH..."
GO_BIN="$(go env GOPATH)/bin"
if [[ ":${PATH}:" != *":${GO_BIN}:"* ]]; then
  export PATH="${GO_BIN}:${PATH}"
  echo "  已临时加入 PATH: ${GO_BIN}"
else
  echo "  PATH 已包含: ${GO_BIN}"
fi

echo "[3/5] 检查 Go 代理配置..."
CURRENT_GOPROXY="$(go env GOPROXY 2>/dev/null || true)"
TARGET_GOPROXY="https://goproxy.cn,direct"
if [[ -z "${CURRENT_GOPROXY}" || "${CURRENT_GOPROXY}" == "https://proxy.golang.org,direct" ]]; then
  echo "  当前 GOPROXY=${CURRENT_GOPROXY:-<empty>}"
  echo "  自动设置 GOPROXY=${TARGET_GOPROXY}"
  go env -w GOPROXY="${TARGET_GOPROXY}"
else
  echo "  当前 GOPROXY=${CURRENT_GOPROXY}"
fi

echo "[4/5] 检查 Wails CLI..."
WAILS_BIN="$(command -v wails || true)"
if [[ -z "${WAILS_BIN}" && -x "${GO_BIN}/wails" ]]; then
  WAILS_BIN="${GO_BIN}/wails"
fi

if [[ -z "${WAILS_BIN}" ]]; then
  echo "  未检测到 wails，开始安装..."
  GOPROXY="$(go env GOPROXY)" go install github.com/wailsapp/wails/v2/cmd/wails@latest
  WAILS_BIN="$(command -v wails || true)"
  if [[ -z "${WAILS_BIN}" && -x "${GO_BIN}/wails" ]]; then
    WAILS_BIN="${GO_BIN}/wails"
  fi
fi

if [[ -z "${WAILS_BIN}" ]]; then
  echo "错误：Wails 安装后仍未找到可执行文件。"
  exit 1
fi

echo "  Wails: $(${WAILS_BIN} version | head -n 1)"

echo "[5/5] 执行构建..."
cd "${ROOT_DIR}"
echo "  工作目录: ${ROOT_DIR}"
echo "  执行命令: ${WAILS_BIN} build --clean --platform ${PLATFORM}"

"${WAILS_BIN}" build --clean --platform "${PLATFORM}"

echo "======================================"
echo "  构建完成"
echo "======================================"
echo "产物目录: ${ROOT_DIR}/build/bin"
