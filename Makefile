.PHONY: build build-web build-backend clean

# 完整构建：前端 → 复制 → 后端
build: build-web build-backend

# 构建前端
build-web:
	cd web && npm run build

# 构建后端（自动复制前端产物）
build-backend:
	rm -rf backend/internal/gateway/webdist
	cp -r web/dist backend/internal/gateway/webdist
	cd backend && go build -o ../bin/gateway-openai .

# 清理构建产物
clean:
	rm -rf backend/internal/gateway/webdist bin/
