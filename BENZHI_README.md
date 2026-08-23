# BENZHI 评测说明 · task200-fiberorient

纸张纤维取向统计校准服务（纯后端，Go + SQLite 持久化）。

## 容器契约

- **镜像**：单阶段构建，`golang:1.26.3-bookworm` 基础镜像，`CGO_ENABLED=0`。
- **入口**：`ENTRYPOINT ["/app/fiberorient"]`，`CMD ["--smoke-test"]`。
  - `docker run --rm <image> --smoke-test`：执行端到端冒烟（含关闭重开数据库的
    重启恢复验证），退出码 0 即通过。
  - `docker run --rm -p 8080:8080 <image> --addr :8080 --db /app/data.db`：启动服务。
- **双架构**：支持 `linux/amd64` 与 `linux/arm64`，均需通过 `--smoke-test`。
- **构建**：`bash build_benzhi_docker.sh <镜像名> <平台>`（默认 `my-project` /
  `linux/amd64`）。

## 业务契约

- 登记批次 → 登记 0°/90° 切片视野 → 导入角度观测（submission_id 幂等）→ 清洗
  （剔除污染视野）→ 生成并激活校准 → 计算统计结果（圆周均值/双峰/置信区间）→
  冻结结果 → 发布批次。
- 数据持久化于 SQLite（`modernc.org/sqlite`，纯 Go，无 CGO）；`--smoke-test`
  验证重启恢复路径。

## 测试命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/fiberorient --smoke-test
```

## 关键限制（评测时应验证）

- 统计拒绝：单位混用、空视野、校准样本不足、冻结结果直接编辑。
- 同一批次至多一个生效校准版本；激活新校准自动废止旧生效版本。
- 同提交重试幂等：重复导入同一 `submission_id` 不产生重复观测。
