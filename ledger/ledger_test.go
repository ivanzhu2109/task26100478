package ledger

import "testing"

// 复现题目场景：先有第 1、3 季度累计值 10、60，随后补录第 2 季度的 35，
// 再撤回第 2 季度。期间穿插其他组合的修改，验证旧修订号报告始终不变。
func TestBackfillAndRetract(t *testing.T) {
	l := New(map[string]string{
		"b1": "P1",
		"x1": "P2", // 其他组合，用于穿插修改
	})

	r1 := l.Put("b1", 1, 10)
	r2 := l.Put("b1", 3, 60)

	assertReport(t, l, "P1", 3, r1, 10, 0)  // 缺季沿用：q3 时只有 q1=10，净额 10-10=0... 见下方注释
	assertReport(t, l, "P1", 3, r2, 60, 50) // q3 净额 = 60 - q2 沿用的 10 = 50

	// 穿插：其他组合的修改产生中间修订号
	x1 := l.Put("x1", 0, 100)
	assertReport(t, l, "P1", 3, x1, 60, 50) // 用其他组合的修订号查 P1，结果不变
	assertReport(t, l, "P2", 0, x1, 100, 100)

	r3 := l.Put("b1", 2, 35) // 补录第 2 季度
	assertReport(t, l, "P1", 2, r3, 35, 25) // 35 - 10 = 25
	assertReport(t, l, "P1", 3, r3, 60, 25) // 60 - 35 = 25

	// 旧修订号报告不受补录影响
	assertReport(t, l, "P1", 3, r2, 60, 50)
	assertReport(t, l, "P1", 3, r1, 10, 0)

	// 再穿插其他组合的修改
	x2 := l.Retract("x1", 0)
	assertReport(t, l, "P2", 0, x2, 0, 0)

	r4 := l.Retract("b1", 2) // 撤回补录
	assertReport(t, l, "P1", 3, r4, 60, 50) // 新修订号下恢复原结果
	assertReport(t, l, "P1", 2, r4, 10, 0)  // q2 不再有快照，沿用 q1=10

	// 撤回之后，所有旧修订号报告仍不变
	assertReport(t, l, "P1", 3, r3, 60, 25)
	assertReport(t, l, "P1", 3, r2, 60, 50)
	assertReport(t, l, "P2", 0, x1, 100, 100)

	// 修订号 0 为空快照集
	assertReport(t, l, "P1", 3, 0, 0, 0)
}

func assertReport(t *testing.T, l *Ledger, portfolio string, quarter, revision int, wantCum, wantNet int64) {
	t.Helper()
	cum, net := l.Report(portfolio, quarter, revision)
	if cum != wantCum || net != wantNet {
		t.Errorf("Report(%s, q%d, rev%d) = (%d, %d), want (%d, %d)",
			portfolio, quarter, revision, cum, net, wantCum, wantNet)
	}
}

// 累计值允许下调，下调后单季净额为负。
func TestCumulativeDecreaseNegativeNet(t *testing.T) {
	l := New(map[string]string{"b1": "P1"})

	r1 := l.Put("b1", 0, 100)
	r2 := l.Put("b1", 1, 40) // 累计值下调

	assertReport(t, l, "P1", 0, r2, 100, 100)
	assertReport(t, l, "P1", 1, r2, 40, -60) // 40 - 100 = -60
	assertReport(t, l, "P1", 1, r1, 100, 0) // 旧修订号下 q1 沿用 q0=100，净额 0
}

// 同一季度反复覆盖后撤回：撤回不恢复更早的 Put，该季度在新版本中不存在。
func TestOverwriteThenRetract(t *testing.T) {
	l := New(map[string]string{"b1": "P1"})

	r1 := l.Put("b1", 0, 10)
	r2 := l.Put("b1", 0, 20) // 覆盖
	r3 := l.Put("b1", 0, 30) // 再覆盖
	r4 := l.Retract("b1", 0)

	assertReport(t, l, "P1", 0, r1, 10, 10)
	assertReport(t, l, "P1", 0, r2, 20, 20)
	assertReport(t, l, "P1", 0, r3, 30, 30)
	assertReport(t, l, "P1", 0, r4, 0, 0) // 撤回后不存在，不恢复 10/20
	assertReport(t, l, "P1", 5, r4, 0, 0) // 后续季度也无值可沿用

	// 撤回后旧修订号仍不变
	assertReport(t, l, "P1", 0, r3, 30, 30)
	assertReport(t, l, "P1", 0, r1, 10, 10)
}
