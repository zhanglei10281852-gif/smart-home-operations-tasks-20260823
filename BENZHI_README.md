# BENZHI_README

## 项目说明

- 项目：zhanglei10281852-gif/smart-home-operations-tasks-20260823
- 项目用途：Smart Home Operations is a Go backend for household onboarding, device lifecycle management, telemetry, energy plans, automation execution, alerts, audit, and reliable outbound delivery. PostgreSQL is the only production data store.
- Go 工具链：`golang:1.26`
- 前端工具链：无

## 标准构建、运行和测试命令

进入容器后执行：

```bash
# 编译
cd '/app' && GOTOOLCHAIN=local go build ./...

# 启动
cd '/app' && GOTOOLCHAIN=local go run ./cmd/server

# 测试
cd '/app' && GOTOOLCHAIN=local go test ./...
```

## Docker 构建和进入容器

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-task-272-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-task-272-arm64 linux/arm64
docker run -it benzhi-task-272-amd64:latest
docker run -it --platform linux/arm64 benzhi-task-272-arm64:latest
```

## 题目验证命令

1. 预期退出码 1：`go test ./internal/repo -run '^TestPlanDraftFailureRollsBackAllRows$' -count=1`
