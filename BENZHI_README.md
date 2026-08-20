# task126-reinsurance — 再保险分保与摊回引擎

## 业务问题

分出公司（cedant）把原保单风险按分保合约（treaty）分出给再保接受人（reinsurer）。赔案发生后，引擎按合约条款在分出公司自留与再保接受人之间摊回赔款：

- **比例型**（quota_share 配额分保 / surplus_share 溢额分保）：按固定/派生比例摊回每一笔保费与赔款；
- **非比例型**（per_risk_xl 险位超赔 / cat_xl 巨灾超赔）：按自留额 + 责任限额分层，仅当单笔或累积赔款穿透自留额时由再保摊回；
- **恢复条款**：超额合约每次摊回耗尽限额后可恢复，按恢复保费因子计收恢复保费；
- **巨灾累积**：同一巨灾事件下归属同一合约的多笔赔案先累积再适用自留额与限额；
- **季度账单（bordereaux）**：按合约汇总分出保费、摊回赔款、分保佣金与恢复保费，结算后锁定。

主要输入：合约、原保单、赔案。主要输出：摊回明细、恢复记录、季度账单净额、汇总报表。

## 本地命令

```bash
go build ./...                           # 编译
go run . --smoke-test                    # 进程内自检（隔离的内存 DB，跑全闭环后退出）
go run . --addr=:8080 --dsn=reinsurance.db   # 启动 HTTP 服务（默认 :8080）
go test ./...                            # 全量测试
```

启动后访问 `http://localhost:8080/` 即原生前端页面（合约/保单/赔案/账单/报表五个面板，覆盖登记→摊回→结算→报表全闭环）。

## Benzhi 评测镜像构建

`build_benzhi_docker.sh` 接收两个参数：

1. 镜像名（默认 `my-project`）
2. Docker 平台（默认 `linux/amd64`，亦可用 `linux/arm64`）

amd64 与 arm64 镜像构建命令：

```bash
bash ./build_benzhi_docker.sh go-task-benzhi:amd64 linux/amd64
bash ./build_benzhi_docker.sh go-task-benzhi:arm64 linux/arm64
```

构建后进入容器：

```bash
docker run -it go-task-benzhi:amd64
docker run -it go-task-benzhi:arm64
```

容器内可用：

```bash
go build ./...        # 确认编译
go run . --smoke-test # 跑全闭环自检
```

## 双架构运行时镜像（交付用 Dockerfile）

交付镜像（非 benzhi 评测镜像）通过 `Dockerfile` 多阶段构建：builder 用
`docker.m.daocloud.io/library/golang:1.26.3-bookworm`，runtime 用
`docker.m.daocloud.io/library/alpine:3.20`，须同时支持 `linux/amd64` 与
`linux/arm64`：

```bash
docker buildx build --platform linux/amd64 --load -t go-task-check:amd64 .
docker run --rm go-task-check:amd64 --smoke-test
docker buildx build --platform linux/arm64 --load -t go-task-check:arm64 .
docker run --rm go-task-check:arm64 --smoke-test
```

## 版本

- Go `1.26.3`（`/opt/homebrew/bin/go`，`go.mod` 的 `go` 指令等于该版本，不写 `toolchain`，`GOTOOLCHAIN=local`）
- SQLite `3.46.1`（`modernc.org/sqlite v1.52.0`，纯 Go driver，`CGO_ENABLED=0`）
- `GOPROXY=https://goproxy.cn,direct`，`GOSUMDB=sum.golang.google.cn`
