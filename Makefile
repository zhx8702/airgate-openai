# AirGate OpenAI 插件 Makefile

GO := GOTOOLCHAIN=local go

.PHONY: help build build-web build-backend ci pre-commit lint fmt test vet clean setup-hooks

help: ## 显示帮助信息
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ===================== 构建 =====================

build: build-web build-backend ## 完整构建：前端 → 复制 → 后端

build-web: ## 构建前端
	cd web && npm run build

build-backend: ## 构建后端（自动复制前端产物）
	rm -rf backend/internal/gateway/webdist
	cp -r web/dist backend/internal/gateway/webdist
	cd backend && $(GO) build -o ../bin/gateway-openai .

# ===================== 开发 =====================

dev: ## 启动开发服务器
	cd backend && $(GO) run ./cmd/devserver

# ===================== 质量检查 =====================

ci: lint test vet build-backend ## 本地运行与 CI 完全一致的检查

pre-commit: lint vet ## pre-commit hook 调用（跳过耗时的测试和构建）

lint: ## 代码检查（需要安装 golangci-lint）
	@if ! command -v golangci-lint > /dev/null 2>&1; then \
		echo "错误: 未安装 golangci-lint，请执行: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"; \
		exit 1; \
	fi
	@cd backend && golangci-lint run ./...
	@echo "代码检查通过"

fmt: ## 格式化代码
	@cd backend && \
	if command -v goimports > /dev/null 2>&1; then \
		goimports -w -local github.com/DouDOU-start .; \
	else \
		$(GO) fmt ./...; \
	fi
	@echo "代码格式化完成"

test: ## 运行测试
	@cd backend && $(GO) test ./...
	@echo "测试完成"

vet: ## 静态分析
	@cd backend && $(GO) vet ./...

# ===================== Git Hooks =====================

setup-hooks: ## 安装 Git pre-commit hook
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo 'make pre-commit' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "pre-commit hook 已安装"

# ===================== 清理 =====================

clean: ## 清理构建产物
	rm -rf backend/internal/gateway/webdist bin/
