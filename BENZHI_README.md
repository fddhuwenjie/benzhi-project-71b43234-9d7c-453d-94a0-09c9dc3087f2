# BENZHI_README

## 项目说明
- 项目：benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2
- 项目用途：已完整实现古树迁移保护方案核验台，提供从草稿修订、确定性规则核查、方案锁定、现场证据、专家退回、逐项整改重提到批准归档的单流程浏览器工作台，并以本地摘要快照、追加事件日志、乐观并发和请求幂等保证业务记录可追溯。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：古树迁移保护方案核验台
- 项目概述：面向城市园林保护部门的单流程核验应用，将古树迁移保护方案从草稿申报、规则核查、现场证据采集、专家复核、整改重提推进到批准归档，并保留完整、可追溯的状态变更记录。
- 核心工作流：迁移申请以草稿创建，完成保护规则核查后提交并锁定版本，现场人员登记定位照片与保护措施证据，专家复核可退回整改，编制人员补正后重新提交，复核通过则生成归档摘要并将申请关闭。
- 对外接口：由 Go HTTP 服务提供一个原生 HTML、CSS 和 JavaScript 工作台，包含申请编辑、核查结果、现场证据、复核意见、整改对照和归档摘要视图；监听地址支持 -addr=127.0.0.1:<port>，并在未传参数时读取 PORT 后绑定 127.0.0.1:<PORT>，默认使用 127.0.0.1:19081，禁止默认绑定 8080、80、3000 或 0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selftest -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2-arm64 linux/arm64
docker run -it benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selftest -addr=127.0.0.1:19081`
