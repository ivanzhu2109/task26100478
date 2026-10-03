package benefit

import "testing"

// mustPut 写入快照并校验不报错，返回新修订号。
func mustPut(t *testing.T, l *Ledger, benefit string, quarter int, actual int64) int64 {
	t.Helper()
	rev, err := l.Put(benefit, quarter, actual)
	if err != nil {
		t.Fatalf("Put(%s, %d, %d) 出错: %v", benefit, quarter, actual, err)
	}
	return rev
}

// mustRetract 撤回快照并校验不报错，返回新修订号。
func mustRetract(t *testing.T, l *Ledger, benefit string, quarter int) int64 {
	t.Helper()
	rev, err := l.Retract(benefit, quarter)
	if err != nil {
		t.Fatalf("Retract(%s, %d) 出错: %v", benefit, quarter, err)
	}
	return rev
}

// checkReport 校验报告结果。
func checkReport(t *testing.T, l *Ledger, portfolio string, quarter int, revision int64, wantCum, wantNet int64) {
	t.Helper()
	cum, net, err := l.Report(portfolio, quarter, revision)
	if err != nil {
		t.Fatalf("Report(%s, %d, %d) 出错: %v", portfolio, quarter, revision, err)
	}
	if cum != wantCum || net != wantNet {
		t.Fatalf("Report(%s, %d, %d) = (累计 %d, 净额 %d), 期望 (累计 %d, 净额 %d)",
			portfolio, quarter, revision, cum, net, wantCum, wantNet)
	}
}

// TestBackfillAndRetract 复现题目示例：补录历史季度与撤回，
// 并穿插其他组合的修改，验证旧修订号下的报告始终不变。
func TestBackfillAndRetract(t *testing.T) {
	l := NewLedger(map[string]string{
		"b1": "pA",
		"b2": "pB", // 另一个组合，用于穿插修改
	})

	// 第1、3季度累计值 10、60。
	rev1 := mustPut(t, l, "b1", 1, 10)
	rev2 := mustPut(t, l, "b1", 3, 60)
	if rev1 != 1 || rev2 != 2 {
		t.Fatalf("修订号应从 1 连续递增, 得到 %d, %d", rev1, rev2)
	}

	// 穿插其他组合的修改。
	revOther := mustPut(t, l, "b2", 0, 999)

	// 补录第2季度的 35。
	rev3 := mustPut(t, l, "b1", 2, 35)

	// 旧修订号下第3季度净额仍为 50（缺季沿用此前值）。
	checkReport(t, l, "pA", 3, rev2, 60, 50)
	// 新修订号下第2、3季度净额分别为 25、25。
	checkReport(t, l, "pA", 2, rev3, 35, 25)
	checkReport(t, l, "pA", 3, rev3, 60, 25)
	// 穿插的其他组合修改不影响 pA 的报告，也不被 pA 的修改影响。
	checkReport(t, l, "pA", 3, revOther, 60, 50)
	checkReport(t, l, "pB", 0, revOther, 999, 999)
	checkReport(t, l, "pB", 0, rev3, 999, 999)

	// 撤回第2季度后，只在新的修订号下恢复原结果。
	rev4 := mustRetract(t, l, "b1", 2)
	checkReport(t, l, "pA", 2, rev4, 10, 0)  // 第2季度缺季沿用第1季度的10
	checkReport(t, l, "pA", 3, rev4, 60, 50) // 第3季度净额恢复为 50
	// 补录后的修订号报告不变。
	checkReport(t, l, "pA", 2, rev3, 35, 25)
	checkReport(t, l, "pA", 3, rev3, 60, 25)
	// 最早的修订号报告也不变。
	checkReport(t, l, "pA", 3, rev2, 60, 50)
}

// TestNegativeNetFromDownwardAdjustment 累计值下调导致单季净额为负。
func TestNegativeNetFromDownwardAdjustment(t *testing.T) {
	l := NewLedger(map[string]string{"b1": "pA"})

	mustPut(t, l, "b1", 0, 100)
	rev := mustPut(t, l, "b1", 1, 40) // 后一季度累计值允许低于前一季度

	checkReport(t, l, "pA", 0, rev, 100, 100)
	checkReport(t, l, "pA", 1, rev, 40, -60) // 净额为负
}

// TestOverwriteThenRetract 同季度反复覆盖后撤回：
// 撤回后该季度不存在，不恢复更早的 Put。
func TestOverwriteThenRetract(t *testing.T) {
	l := NewLedger(map[string]string{"b1": "pA"})

	rev1 := mustPut(t, l, "b1", 2, 10)
	rev2 := mustPut(t, l, "b1", 2, 20) // 覆盖
	rev3 := mustPut(t, l, "b1", 2, 30) // 再覆盖

	checkReport(t, l, "pA", 2, rev1, 10, 10)
	checkReport(t, l, "pA", 2, rev2, 20, 20)
	checkReport(t, l, "pA", 2, rev3, 30, 30)

	// 撤回后该季度不存在，不恢复 10 或 20。
	rev4 := mustRetract(t, l, "b1", 2)
	checkReport(t, l, "pA", 2, rev4, 0, 0)
	// 旧修订号报告不变。
	checkReport(t, l, "pA", 2, rev3, 30, 30)
}

// TestEmptyAndCarryForward 修订号 0 为空快照集；缺季沿用、未来快照不反向填补。
func TestEmptyAndCarryForward(t *testing.T) {
	l := NewLedger(map[string]string{"b1": "pA"})

	// 修订号 0：空快照集。
	checkReport(t, l, "pA", 5, 0, 0, 0)

	rev := mustPut(t, l, "b1", 2, 70)

	// 快照之前的季度没有值，未来快照不反向填补。
	checkReport(t, l, "pA", 0, rev, 0, 0)
	checkReport(t, l, "pA", 1, rev, 0, 0)
	// 缺季沿用此前值。
	checkReport(t, l, "pA", 2, rev, 70, 70)
	checkReport(t, l, "pA", 3, rev, 70, 0)
	checkReport(t, l, "pA", 9, rev, 70, 0)
}

// TestPortfolioAggregation 组合总额为各效益对应值之和。
func TestPortfolioAggregation(t *testing.T) {
	l := NewLedger(map[string]string{
		"b1": "pA",
		"b2": "pA",
		"b3": "pB",
	})

	mustPut(t, l, "b1", 0, 100)
	mustPut(t, l, "b2", 1, 200)
	rev := mustPut(t, l, "b3", 0, 5)

	checkReport(t, l, "pA", 0, rev, 100, 100)
	checkReport(t, l, "pA", 1, rev, 300, 200)
	checkReport(t, l, "pB", 1, rev, 5, 0)
}

// TestErrors 非法输入不产生修订号。
func TestErrors(t *testing.T) {
	l := NewLedger(map[string]string{"b1": "pA"})
	rev := mustPut(t, l, "b1", 0, 10)

	cases := []struct {
		name string
		fn   func() error
		want error
	}{
		{"Put未知效益", func() error { _, err := l.Put("nope", 0, 1); return err }, ErrUnknownBenefit},
		{"Put负季度", func() error { _, err := l.Put("b1", -1, 1); return err }, ErrInvalidQuarter},
		{"Put负金额", func() error { _, err := l.Put("b1", 0, -1); return err }, ErrNegativeActual},
		{"Retract未知效益", func() error { _, err := l.Retract("nope", 0); return err }, ErrUnknownBenefit},
		{"Retract负季度", func() error { _, err := l.Retract("b1", -1); return err }, ErrInvalidQuarter},
		{"Retract不存在的快照", func() error { _, err := l.Retract("b1", 5); return err }, ErrNoSnapshot},
		{"Report未知组合", func() error { _, _, err := l.Report("nope", 0, rev); return err }, ErrUnknownPortfolio},
		{"Report负季度", func() error { _, _, err := l.Report("pA", -1, rev); return err }, ErrInvalidQuarter},
		{"Report负修订号", func() error { _, _, err := l.Report("pA", 0, -1); return err }, ErrInvalidRevision},
		{"Report未来修订号", func() error { _, _, err := l.Report("pA", 0, rev+1); return err }, ErrInvalidRevision},
	}
	for _, c := range cases {
		if err := c.fn(); err != c.want {
			t.Fatalf("%s: 得到错误 %v, 期望 %v", c.name, err, c.want)
		}
	}
	if l.Revision() != rev {
		t.Fatalf("失败的操作不应产生修订号: 当前 %d, 期望 %d", l.Revision(), rev)
	}
}
