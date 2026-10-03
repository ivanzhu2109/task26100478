// Package ledger 提供季度效益核算能力，供项目组合治理报告使用。
//
// 每次成功修改产生从 1 开始连续递增的全局修订号，修订号 0 表示空快照集。
// 历史修订号下的报告结果不可变：迟到、更正或撤回的快照只影响新修订号。
package ledger

// state 是某一修订号下的完整快照集：效益ID -> (季度 -> 累计已实现值)。
// 一旦写入 Ledger.states 即视为不可变，任何修改都通过复制产生新状态。
type state map[string]map[int]int64

// clone 深拷贝状态，供写时复制使用。
func (s state) clone() state {
	out := make(state, len(s))
	for benefit, quarters := range s {
		q := make(map[int]int64, len(quarters))
		for quarter, actual := range quarters {
			q[quarter] = actual
		}
		out[benefit] = q
	}
	return out
}

// Ledger 保存全部修订历史。修订号 i 对应状态 states[i]，
// states[0] 为空快照集。每个状态一经写入即不可变。
type Ledger struct {
	owner  map[string]string // 效益ID -> 组合ID（初始化后固定）
	states []state           // states[rev] 为修订号 rev 对应的快照集
}

// New 以固定的效益归属关系创建核算账本。
func New(benefitToPortfolio map[string]string) *Ledger {
	owner := make(map[string]string, len(benefitToPortfolio))
	for benefit, portfolio := range benefitToPortfolio {
		owner[benefit] = portfolio
	}
	return &Ledger{
		owner:  owner,
		states: []state{{}},
	}
}

// Put 写入一个季度快照，actual 为截至该季度的累计已实现值（整数分）。
// 同一效益、同一季度的新 Put 替换当前快照。返回新的全局修订号。
func (l *Ledger) Put(benefit string, quarter int, actual int64) int {
	next := l.states[len(l.states)-1].clone()
	if next[benefit] == nil {
		next[benefit] = make(map[int]int64)
	}
	next[benefit][quarter] = actual
	l.states = append(l.states, next)
	return len(l.states) - 1
}

// Retract 撤回当前该季度快照，使该季度在新版本中不存在。
// 不恢复该季度更早的 Put。返回新的全局修订号。
func (l *Ledger) Retract(benefit string, quarter int) int {
	next := l.states[len(l.states)-1].clone()
	delete(next[benefit], quarter)
	if len(next[benefit]) == 0 {
		delete(next, benefit)
	}
	l.states = append(l.states, next)
	return len(l.states) - 1
}

// Report 返回指定修订号下，组合截至 quarter 的累计已实现总额
// 以及本季度净实现额（本季度累计总额减上一季度累计总额，同修订号）。
func (l *Ledger) Report(portfolio string, quarter int, revision int) (cumulative int64, net int64) {
	cumulative = l.cumulativeAt(portfolio, quarter, revision)
	var prev int64
	if quarter > 0 {
		prev = l.cumulativeAt(portfolio, quarter-1, revision)
	}
	return cumulative, cumulative - prev
}

// cumulativeAt 返回指定修订号下，组合截至 quarter 的累计已实现总额。
// 对每个效益取季度不晚于 quarter 的最后一个快照；没有则为 0。
// 缺季沿用此前值，未来快照不反向填补。
func (l *Ledger) cumulativeAt(portfolio string, quarter, revision int) int64 {
	s := l.states[revision]
	var total int64
	for benefit, owner := range l.owner {
		if owner != portfolio {
			continue
		}
		total += lastAtOrBefore(s[benefit], quarter)
	}
	return total
}

// lastAtOrBefore 返回季度不晚于 quarter 的最后一个快照值；没有则返回 0。
func lastAtOrBefore(quarters map[int]int64, quarter int) int64 {
	best := -1
	var value int64
	for q, actual := range quarters {
		if q <= quarter && q > best {
			best, value = q, actual
		}
	}
	return value
}
