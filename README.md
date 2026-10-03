# task26100478 — 季度效益核算库

供项目组合治理报告使用的 Go 库（`ledger` 包）。基于不可变修订历史：
每次成功修改产生从 1 开始连续递增的全局修订号（0 为空快照集），
历史修订号下的报告结果永远不变——迟到、更正、撤回只影响新修订号。

## API

- `New(benefitToPortfolio map[string]string) *Ledger` — 传入效益ID到组合ID的固定归属
- `Put(benefit string, quarter int, actual int64) int` — 写入季度快照（actual 为累计已实现值，整数分），返回新修订号
- `Retract(benefit string, quarter int) int` — 撤回当前该季度快照，返回新修订号
- `Report(portfolio string, quarter int, revision int) (cumulative, net int64)` — 返回指定修订号下的累计已实现总额与本季度净实现额

## 运行

```sh
go build ./...   # 编译
go test ./...    # 运行全部测试
go vet ./...     # 静态检查
```

无外部依赖，仅使用 Go 标准库（建议 Go 1.22+）。
