# 本质评测环境说明

## 项目

- 项目编号：`zgw-gowork-0010`
- 项目名称：志愿者排班协调
- 项目说明：志愿者排班协调本地服务

## 固定环境

- Go toolchain：`go1.26.5`
- go.mod language version：`go 1.21`
- GOTOOLCHAIN：`local`
- 支持平台：`linux/amd64`、`linux/arm64`
- Docker 基础镜像：`golang:1.26.5-bookworm`
- Docker manifest：`golang@sha256:53eeac89074db483fdf0ab3be1df32bf6e47562263d2d0d6baa7f26acb4957dd`

## 构建

```bash
./build_benzhi_docker.sh zgw-gowork-0010:benzhi-amd64 linux/amd64
./build_benzhi_docker.sh zgw-gowork-0010:benzhi-arm64 linux/arm64
```

## 运行

```bash
docker run --rm -it --network none zgw-gowork-0010:benzhi-amd64 bash
```

## 容器内验证

```bash
go version
go env GOTOOLCHAIN GOPROXY GOMODCACHE GOCACHE
go test ./...
go vet ./...
go build ./...
```

---

# 项目 README 同步内容

# 志愿者排班服务

本项目是一个基于 Go 标准库的志愿者排班服务示例，管理志愿者、技能、服务班次、报名和签到，支持发布、报名、确认、签到、结算，限制时间冲突和技能缺口，取消后按报名时间递补合格候选人，并提供按活动统计覆盖率与人员缺口。

## 功能特性

- 志愿者管理：注册志愿者并维护其技能列表。
- 班次管理：创建活动及班次，每个班次需指定所需人数和技能要求。
- 报名流程：志愿者报名班次，系统自动校验技能匹配和时间冲突。
- 状态流转：报名（registered）→ 确认（confirmed）→ 签到（checked_in）→ 结算（settled）。
- 取消递补：取消报名后，自动从待确认候选池中按报名时间优先递补合格志愿者。
- 活动统计：按活动统计覆盖率（已确认+已签到人数/所需人数）和人员缺口。

## 技术栈

- Go 1.21+（标准库）
- 线程安全内存存储（`sync.RWMutex`）
- 标准库 `net/http` JSON 接口

## 目录结构

```
├── cmd/server/            # 服务入口
├── internal/domain/       # 领域模型（志愿者、班次、报名等）
├── internal/store/        # 内存存储实现
├── internal/service/      # 核心业务逻辑
├── internal/httpapi/      # HTTP 接口层
├── go.mod
└── README.md
```

## 快速开始

### 运行服务

```bash
go run ./cmd/server
```

服务默认监听 `:8080`，可通过 `-addr` 参数修改。

### 构建并运行（无网络）

```bash
# 生成静态二进制（支持 linux/amd64 和 linux/arm64）
GOOS=linux GOARCH=amd64 go build -o bin/volunteer-scheduler-amd64 ./cmd/server
GOOS=linux GOARCH=arm64 go build -o bin/volunteer-scheduler-arm64 ./cmd/server
```

### Docker 部署

```bash
# 使用提供的 Dockerfile 构建镜像
docker build -f Dockerfile -t volunteer-scheduler:latest .
```

## API 接口

### 1. 创建志愿者

**POST /api/volunteers**

请求体：
```json
{
  "name": "张三",
  "skills": ["医疗", "翻译"]
}
```

响应：`201 Created`，返回创建的志愿者对象。

### 2. 创建活动（含班次）

**POST /api/activities**

请求体：
```json
{
  "name": "社区义诊",
  "shifts": [
    {"start_time": "2025-01-01T09:00:00Z", "end_time": "2025-01-01T12:00:00Z", "required_skills": ["医疗"], "required_count": 2}
  ]
}
```

响应：`201 Created`，包含活动ID和班次ID。

### 3. 报名班次

**POST /api/registrations**

请求体：
```json
{
  "volunteer_id": "v1",
  "shift_id": "s1"
}
```

响应：`201 Created`，返回报名对象（状态为 `registered`）。

### 4. 确认报名

**POST /api/registrations/{registration_id}/confirm**

响应：200，返回更新后的报名对象。

### 5. 签到

**POST /api/registrations/{registration_id}/checkin**

响应：200，返回更新后的报名对象。

### 6. 结算

**POST /api/registrations/{registration_id}/settle**

响应：200，返回更新后的报名对象。

### 7. 取消报名

**DELETE /api/registrations/{registration_id}**

响应：200，返回被取消的报名对象。取消后自动尝试递补。

### 8. 查询活动统计

**GET /api/activities/{activity_id}/statistics**

响应：
```json
{
  "activity_id": "a1",
  "shift_count": 2,
  "total_required": 4,
  "confirmed_count": 3,
  "checked_in_count": 2,
  "settled_count": 1,
  "coverage_rate": 0.75,
  "shortage": 1
}
```

## 测试

```bash
go test ./...
# 并发安全测试
go test -race ./...
```

## 核心规则

1. **技能匹配**：志愿者必须至少拥有班次要求的技能之一才能报名。
2. **时间冲突**：同一个志愿者不能同时报名两个时间重叠的班次。
3. **状态机**：报名状态只能按 `registered → confirmed → checked_in → settled` 顺序流转，不可跳跃。
4. **递补机制**：当已确认的报名被取消时，系统从处于 `registered` 状态的候选人中按报名时间最早者优先，递补为 `confirmed`（需满足技能和冲突约束）。
5. **统计口径**：覆盖率 = (确认+签到+结算人数) / 所需总人数；缺口 = 所需总人数 - (确认+签到+结算人数)。

## 说明

- 内存存储，重启数据丢失。
- 所有时间均使用 UNIX 时间戳（秒）表示，方便比较。
- 为演示简单，报名后自动进入 `registered` 状态，需要显式调用确认接口才能推进。
