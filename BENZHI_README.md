# BENZHI_README

这是一个面向家庭物联网运营的 Go 后端服务，负责家庭与成员接入、设备生命周期、遥测采集、能源计划、自动化执行、告警审计及可靠消息投递。

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
./build_benzhi_docker.sh benzhi-task-276-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-task-276-arm64 linux/arm64
docker run -it benzhi-task-276-amd64:latest
docker run -it --platform linux/arm64 benzhi-task-276-arm64:latest
```
