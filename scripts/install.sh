#!/usr/bin/env bash
# ==============================================================================
# Crush + Harness² Local Build & Installation Script
# Compiles Crush from source and installs to a system or user binary directory.
# ==============================================================================

set -euo pipefail

# Text colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

INSTALL_DIR="${INSTALL_DIR:-}"
SKIP_BUILD="${SKIP_BUILD:-0}"

# Parse optional command-line flags
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir|-d)
      INSTALL_DIR="$2"
      shift 2
      ;;
    --skip-build)
      SKIP_BUILD=1
      shift
      ;;
    --help|-h)
      echo -e "${BOLD}Usage:${NC} $0 [options]"
      echo ""
      echo "Options:"
      echo "  -d, --dir <path>     Specify target installation directory"
      echo "                       (default: /usr/local/bin or ~/.local/bin)"
      echo "  --skip-build         Skip compilation, only install existing bin/crush"
      echo "  -h, --help           Show this help message"
      echo ""
      echo "Environment variables:"
      echo "  INSTALL_DIR=<path>   Override installation directory"
      echo "  GOPROXY=<url>        Go proxy URL (default: https://goproxy.cn,direct)"
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      echo "Run '$0 --help' for usage."
      exit 1
      ;;
  esac
done

echo -e "${CYAN}╔══════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║     Crush + Harness² RSI 智能体本地编译与安装程序                   ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# 1. Check Go environment
if ! command -v go >/dev/null 2>&1; then
  echo -e "${RED}❌ 错误: 未检测到 Go 编译器，请先安装 Go 1.22+：https://golang.org/${NC}"
  exit 1
fi

GO_VERSION="$(go version)"
echo -e "• Go 环境: ${GREEN}${GO_VERSION}${NC}"

# 2. Determine target install directory
if [[ -z "$INSTALL_DIR" ]]; then
  if [[ -w "/usr/local/bin" ]]; then
    INSTALL_DIR="/usr/local/bin"
  elif [[ -d "$HOME/.local/bin" && -w "$HOME/.local/bin" ]]; then
    INSTALL_DIR="$HOME/.local/bin"
  else
    mkdir -p "$HOME/.local/bin"
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi

# 3. Determine Version Tag
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "dev")"
GIT_TAG="$(git describe --tags --always 2>/dev/null || echo "v0.1.0-harness2")"
VERSION="${GIT_TAG}+${GIT_COMMIT}"

# 4. Compile binary
if [[ "$SKIP_BUILD" -eq 0 ]]; then
  echo -e "\n${BOLD}🔨 [1/3] 正在编译 Crush 二进制 (含 Harness² RSI 引擎)...${NC}"
  mkdir -p bin

  export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

  LDFLAGS="-s -w -X github.com/charmbracelet/crush/internal/version.Version=${VERSION}"

  START_TIME=$(date +%s)
  go build -ldflags "$LDFLAGS" -o bin/crush .
  END_TIME=$(date +%s)

  BUILD_DURATION=$((END_TIME - START_TIME))
  echo -e "   ${GREEN}✓ 编译成功！耗时: ${BUILD_DURATION}s -> bin/crush${NC}"
else
  if [[ ! -f "bin/crush" ]]; then
    echo -e "${RED}❌ 错误: bin/crush 不存在，请不要使用 --skip-build${NC}"
    exit 1
  fi
  echo -e "• ${YELLOW}跳过编译步骤，直接安装现有 bin/crush${NC}"
fi

# 5. Install to target directory
echo -e "\n${BOLD}📦 [2/3] 正在安装至目标目录: ${CYAN}${INSTALL_DIR}${NC}..."
mkdir -p "$INSTALL_DIR"

if [[ -w "$INSTALL_DIR" ]]; then
  install -m 755 bin/crush "${INSTALL_DIR}/crush"
else
  echo -e "   ${YELLOW}目标目录需要管理员权限，正在尝试 sudo 安装...${NC}"
  sudo install -m 755 bin/crush "${INSTALL_DIR}/crush"
fi
echo -e "   ${GREEN}✓ 成功安装: ${INSTALL_DIR}/crush${NC}"

# 6. Verify installation
echo -e "\n${BOLD}🔍 [3/3] 验证安装与环境配置...${NC}"
INSTALLED_BIN="${INSTALL_DIR}/crush"

if ! "$INSTALLED_BIN" --version >/dev/null 2>&1; then
  echo -e "   ${RED}⚠️ 警告: 安装的二进制执行异常${NC}"
else
  echo -e "   ${GREEN}✓ crush 运行正常！${NC}"
fi

# Check PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    echo -e "   ${GREEN}✓ 安装路径已在系统 \$PATH 中，可在任意终端直接执行 'crush'${NC}"
    ;;
  *)
    echo -e "   ${YELLOW}⚠️ 提示: ${INSTALL_DIR} 暂未包含在当前 \$PATH 中。${NC}"
    echo -e "   建议在你的 ~/.zshrc 或 ~/.bashrc 末尾添加："
    echo -e "   ${CYAN}export PATH=\"${INSTALL_DIR}:\$PATH\"${NC}"
    ;;
esac

# 7. Check config
CONFIG_FILE="$HOME/.config/crush/crushrc"
if [[ -f "$CONFIG_FILE" ]]; then
  echo -e "   ${GREEN}✓ 全局配置就绪: ${CONFIG_FILE}${NC}"
else
  echo -e "   ${YELLOW}ℹ️ 全局配置文件未检测到，建议创建 ~/.config/crush/crushrc${NC}"
fi

echo -e "\n${GREEN}╔══════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║               🎉 Crush + Harness² 安装完成！                         ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════════════════╝${NC}"
echo -e "直接在终端输入 ${BOLD}${CYAN}crush${NC} 即可开启智能编程助手！"
echo ""
