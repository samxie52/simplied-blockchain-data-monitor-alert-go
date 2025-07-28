#!/bin/bash
# 创建项目目录结构

# 应用程序入口
mkdir -p cmd/server
mkdir -p cmd/worker
mkdir -p cmd/migrator

# 内部业务逻辑
mkdir -p internal/config
mkdir -p internal/models
mkdir -p internal/services/ethereum
mkdir -p internal/services/alert
mkdir -p internal/services/prediction
mkdir -p internal/services/telegram
mkdir -p internal/repositories
mkdir -p internal/handlers/api
mkdir -p internal/handlers/websocket
mkdir -p internal/middleware
mkdir -p internal/utils

# 可复用包
mkdir -p pkg/database
mkdir -p pkg/logger
mkdir -p pkg/metrics

# 前端资源
mkdir -p web/static/css
mkdir -p web/static/js
mkdir -p web/static/images
mkdir -p web/templates

# 脚本和部署
mkdir -p scripts
mkdir -p deployments/docker
mkdir -p deployments/kubernetes
mkdir -p deployments/monitoring

# 文档和测试
mkdir -p docs
mkdir -p tests/unit
mkdir -p tests/integration
mkdir -p tests/e2e

# GitHub 配置
mkdir -p .github/workflows

echo "项目目录结构创建完成！"