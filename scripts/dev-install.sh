#!/bin/bash
# 开发模式安装脚本：编译插件并注册到本地 Core
# 用法: ./scripts/dev-install.sh [core_addr]
#
# 前置条件：airgate-core 后端已启动

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CORE_ADDR="${1:-http://localhost:9517}"
PLUGIN_NAME="gateway-openai"
CORE_DIR="$(cd "$PROJECT_DIR/../airgate-core/backend" && pwd)"
TARGET_DIR="$CORE_DIR/data/plugins/$PLUGIN_NAME"

echo "=== AirGate OpenAI 插件开发安装 ==="
echo "插件目录: $PROJECT_DIR"
echo "Core 目录: $CORE_DIR"
echo "目标路径: $TARGET_DIR"
echo ""

# 1. 构建前端
echo "[1/4] 构建前端..."
cd "$PROJECT_DIR/web"
npm run build --silent

# 2. 构建后端（含嵌入前端资源）
echo "[2/4] 构建后端..."
rm -rf "$PROJECT_DIR/backend/internal/gateway/webdist"
cp -r "$PROJECT_DIR/web/dist" "$PROJECT_DIR/backend/internal/gateway/webdist"
cd "$PROJECT_DIR/backend"
go build -o "$PROJECT_DIR/bin/$PLUGIN_NAME" .

# 3. 复制二进制到 Core 的 plugins 目录
echo "[3/4] 部署到 Core..."
mkdir -p "$TARGET_DIR"
cp "$PROJECT_DIR/bin/$PLUGIN_NAME" "$TARGET_DIR/$PLUGIN_NAME"
chmod +x "$TARGET_DIR/$PLUGIN_NAME"
echo "  二进制: $TARGET_DIR/$PLUGIN_NAME"

# 4. 通过 API 注册插件（如果 Core 在运行）
echo "[4/4] 注册插件..."
TOKEN=$(cat "$CORE_DIR/.admin_token" 2>/dev/null || echo "")

# 尝试获取 admin token（如果没有缓存的话需要手动登录）
if [ -z "$TOKEN" ]; then
    echo "  提示: 未找到 admin token，请手动注册："
    echo ""
    echo "  curl -X POST $CORE_ADDR/api/v1/admin/plugins/install \\"
    echo "    -H 'Authorization: Bearer <your_token>' \\"
    echo "    -H 'Content-Type: application/json' \\"
    echo "    -d '{\"name\": \"$PLUGIN_NAME\", \"version\": \"1.0.0\"}'"
    echo ""
    echo "  然后启用："
    echo "  curl -X POST $CORE_ADDR/api/v1/admin/plugins/<id>/enable \\"
    echo "    -H 'Authorization: Bearer <your_token>'"
else
    # 自动注册
    RESP=$(curl -s -X POST "$CORE_ADDR/api/v1/admin/plugins/install" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"$PLUGIN_NAME\", \"version\": \"1.0.0\"}" 2>&1)
    echo "  注册响应: $RESP"
fi

echo ""
echo "=== 完成 ==="
echo "下一步:"
echo "  1. 在 Core 管理后台 → 插件管理 中找到 $PLUGIN_NAME"
echo "  2. 点击「启用」按钮"
echo "  3. Core 会通过 gRPC 启动插件进程并提取前端资源"
