# task200-fiberorient · 纸张纤维取向统计校准服务

面向纸张研发人员的纤维取向统计校准后端服务：登记样品批次与切片方向，上传显微切片
纤维角度观测，服务执行圆周（轴向）统计、双峰检测与仪器切片偏差校准；用户可剔除
污染视野、锁定校准版本并发布统计摘要，新观测自动产生替代结果版本。

## 业务闭环

1. 研发人员登记**样品批次**（材料 + 切片方向）。
2. 登记 0°/90° 两个正交切片的**视野**，批量导入**纤维角度观测**（同提交重试幂等）。
3. 清洗：标记污染视野并剔除，仅**有效视野**参与统计。
4. 校准：用 0°/90° 切片观测估计**仪器偏差 δ**，生成校准版本（草稿→生效→废止，
   同批次至多一个生效版本）。
5. 统计：按切片方向校正后执行**圆周均值 / 双峰检测 / 置信区间**，生成结果版本
   （计算中→可发布/置信不足→冻结）。
6. 发布：冻结结果后发布批次；**新增观测产生替代版本**，可与旧版本比较。

## 状态机

| 实体 | 状态机 |
| --- | --- |
| 样品批次 | `registered` → `observing` → `analyzable` → `published` |
| 视野 | `pending` → `valid` / `polluted` → `excluded` |
| 校准版本 | `draft` → `active` → `revoked` |
| 统计结果 | `computing` → `publishable` / `insufficient_confidence` → `frozen` |

## 标准命令

```bash
# 构建 / 静态检查 / 测试
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...

# 冒烟测试（端到端 + 重启恢复，退出码 0 即通过）
go run ./cmd/fiberorient --smoke-test

# 启动服务
go run ./cmd/fiberorient --addr :8080 --db ./fiberorient.db
```

## API 一览

| 能力 | 入口 |
| --- | --- |
| 批次 | `POST/GET /api/batches`、`GET/PATCH /api/batches/{id}` |
| 批次流转/发布 | `POST /api/batches/{id}/transition`、`/publish` |
| 视野 | `POST/GET /api/batches/{id}/fields`、`GET /api/fields/{id}`、`/valid`、`/polluted`、`/exclude` |
| 观测导入/查询 | `POST /api/batches/{id}/observations/import`、`GET .../observations`、`/count`、`/units` |
| 校准 | `POST/GET /api/batches/{id}/calibrations`、`GET /api/calibrations/{id}`、`/activate`、`/revoke` |
| 统计与结果 | `POST /api/batches/{id}/compute`、`GET .../results`、`/latest`、`GET /api/results/{id}`、`/freeze`、`GET /api/results/compare` |
| 自检 | `GET /api/health`、`POST /api/selfcheck` |

## 关键不变量

- 角度为轴向数据（模 180°）；同提交（submission_id）重试幂等，同角度多纤维不误判。
- 校准需 0°/90° 切片各 ≥3 条有效观测；统计拒绝单位混用、空视野、未生效校准。
- 冻结结果不可直接编辑；结果版本号单调递增（UNIQUE(batch_id, version)）。
- 发布结果绑定视野快照与校准版本；重启后从未完成批次/最新版本恢复。
