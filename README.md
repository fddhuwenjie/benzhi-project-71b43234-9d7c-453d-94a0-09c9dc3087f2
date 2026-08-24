# 古树迁移保护方案核验台

古树迁移保护方案核验台面向城市园林保护审查、现场核验和方案编制人员，提供一条可追溯的迁移申请核验流程。系统支持活跃重复申报识别、草稿修订、警示项处置、不可变方案锁定、现场证据分项暂存、专家逐项复核、整改闭环对照和批准归档。归档后会冻结规则结果、警示处置、复核矩阵、状态时间线和证据清单，并通过完整性核验回执控制正常打印。

项目采用单进程 Go HTTP 服务，无外部数据库或前端构建链。申请以带摘要校验的本地 JSON 快照保存，状态操作写入追加事件日志；写命令通过 `revision` 做乐观并发控制，通过 `request_id` 保证重复请求幂等。

## 环境要求

- Go 1.22 或更高版本
- 可写的本地数据目录

## 构建

```bash
go build ./cmd/server
```

## 运行

```bash
go run ./cmd/server -addr=127.0.0.1:19081 -data=./data
```

浏览器访问 `http://127.0.0.1:19081/`。监听地址按以下顺序确定：

1. 显式传入的 `-addr`
2. `PORT` 环境变量对应的 `127.0.0.1:<PORT>`
3. 默认值 `127.0.0.1:19081`

服务拒绝 `0.0.0.0` 等非回环监听地址。`-data` 指定快照和事件日志目录，默认使用 `./data`。

## 测试

运行全部自动化测试：

```bash
go test ./...
```

运行有界端到端自检：

```bash
go run ./cmd/server -selftest -addr=127.0.0.1:19081
```

自检使用临时数据目录启动真实 HTTP 服务，依次完成草稿创建、规则核查、幂等提交、现场证据登记、专家退回、逐项补正、重新核查、整改重提和批准归档，然后自动关闭并删除临时数据。

## 数据文件

- `<data>/snapshots/*.json`：申请聚合快照、幂等命令结果和摘要封装
- `<data>/events.jsonl`：追加式持久化状态事件
- `<data>/receipts/*.json`：不改变申请修订号的归档完整性核验回执

启动时会检查全部快照格式和内容摘要。快照写入采用同目录临时文件、文件同步、原子替换和目录同步，单进程内的比较后写入由互斥区保护。

## HTTP 入口

- `GET /`：浏览器工作台
- `GET /archive/{id}`：打印友好的归档页
- `GET /api/health`：健康检查
- `GET|POST /api/applications`：申请列表与草稿创建
- `GET /api/applications/{id}`：完整申请查询模型
- `PUT /api/applications/{id}/draft`：保存草稿或整改中的方案
- `POST /api/applications/{id}/assess`：执行保护规则核查
- `POST /api/applications/{id}/submit`：提交并锁定草稿版本
- `PUT /api/applications/{id}/warning-dispositions`：保存当前规则警示处置
- `PUT /api/applications/{id}/site-evidence/draft`：分项暂存现场证据
- `DELETE /api/applications/{id}/site-evidence/photos/{photoID}`：删除误录照片元数据
- `POST /api/applications/{id}/site-evidence/confirm`：冻结齐套证据并进入专家复核
- `POST /api/applications/{id}/site-evidence`：兼容的一次性完整证据入口，同样执行齐套校验
- `POST /api/applications/{id}/review`：专家批准或退回整改
- `POST /api/applications/{id}/rectifications`：逐项提交补正
- `POST /api/applications/{id}/resubmit`：核查通过后重提专家复核
- `POST /api/applications/{id}/archive-integrity`：复算归档组成并生成只读核验回执
