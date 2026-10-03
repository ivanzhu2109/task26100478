# task26100478

季度效益核算库（Go），供项目组合治理报告使用。

## 特性

- 不可变修订历史：每次成功修改产生从 1 开始连续递增的全局修订号，
  任何历史修订号下的报告都可精确重现；迟到、更正、撤回不改写旧报告。
- `Put(benefit, quarter, actual)`：写入季度快照，`actual` 为截至该季度的
  累计已实现值（整数分）；同效益同季度的新 Put 替换当前快照。
- `Retract(benefit, quarter)`：撤回当前该季度快照，不恢复更早的 Put。
- `Report(portfolio, quarter, revision)`：返回累计已实现总额与本季度
  净实现额（本季度累计减上一季度累计，同一修订号；第 0 季度上一季度按 0）。
- 缺季沿用此前值，未来快照不反向填补；累计值允许环比下降（净额可为负）。

## 目录

- `benefit/ledger.go` — 库源码（包名 `benefit`）
- `benefit/ledger_test.go` — 单元测试

## 运行

```sh
go build ./...   # 构建
go test ./...    # 运行全部测试
go test ./... -v # 查看测试明细
```

## 用法示例

```go
l := benefit.NewLedger(map[string]string{"b1": "pA"})
rev1, _ := l.Put("b1", 1, 10)
rev2, _ := l.Put("b1", 3, 60)
cum, net, _ := l.Report("pA", 3, rev2) // cum=60, net=50
```
