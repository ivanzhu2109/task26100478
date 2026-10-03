// Package benefit 提供季度效益核算能力，供项目组合治理报告使用。
//
// 核算基于不可变修订历史：每次成功修改产生一个从 1 开始连续递增的
// 全局修订号，任何历史修订号下的报告都可以精确重现，迟到、更正或
// 撤回的快照不会改写旧报告。
package benefit

import (
	"errors"
	"sort"
)

// 错误定义。
var (
	ErrUnknownBenefit   = errors.New("benefit: 未知的效益ID")
	ErrUnknownPortfolio = errors.New("benefit: 未知的组合ID")
	ErrInvalidQuarter   = errors.New("benefit: 季度必须为非负整数")
	ErrNegativeActual   = errors.New("benefit: 累计已实现值必须非负")
	ErrNoSnapshot       = errors.New("benefit: 该季度当前不存在可撤回的快照")
	ErrInvalidRevision  = errors.New("benefit: 修订号尚未生成或非法")
)

// snapshot 是某效益在某个修订号下的全部季度快照表。
// 键为季度（从 0 开始），值为截至该季度的累计已实现金额（整数分）。
type snapshot map[int]int64

// clone 返回快照的深拷贝，保证历史版本不可变。
func (s snapshot) clone() snapshot {
	c := make(snapshot, len(s))
	for q, v := range s {
		c[q] = v
	}
	return c
}

// valueAt 返回截至季度 quarter 的累计已实现值：
// 取季度不晚于 quarter 的最后一个快照；没有快照则为 0。
// 缺季沿用此前值，未来快照不反向填补空缺。
func (s snapshot) valueAt(quarter int) int64 {
	best := -1
	for q := range s {
		if q <= quarter && q > best {
			best = q
		}
	}
	if best < 0 {
		return 0
	}
	return s[best]
}

// version 记录一次修改后某效益的完整快照状态。
type version struct {
	revision int64
	state    snapshot
}

// benefitHistory 保存单个效益的版本链，revision 严格递增。
type benefitHistory struct {
	versions []version
}

// current 返回最新版本状态；无版本时返回空快照。
func (h *benefitHistory) current() snapshot {
	if len(h.versions) == 0 {
		return snapshot{}
	}
	return h.versions[len(h.versions)-1].state
}

// at 返回截至 revision（含）仍存在的快照状态；无版本时返回空快照。
func (h *benefitHistory) at(revision int64) snapshot {
	i := sort.Search(len(h.versions), func(i int) bool {
		return h.versions[i].revision > revision
	})
	if i == 0 {
		return snapshot{}
	}
	return h.versions[i-1].state
}

// Ledger 是季度效益核算账本。Ledger 不是并发安全的。
type Ledger struct {
	ownership   map[string]string   // 效益ID -> 组合ID（初始化后固定）
	byPortfolio map[string][]string // 组合ID -> 效益ID 列表
	histories   map[string]*benefitHistory
	revision    int64 // 当前全局修订号，0 表示空快照集
}

// NewLedger 以固定的效益归属关系创建账本。
// ownership 的键为效益ID，值为组合ID，初始化后不可变更。
func NewLedger(ownership map[string]string) *Ledger {
	l := &Ledger{
		ownership:   make(map[string]string, len(ownership)),
		byPortfolio: make(map[string][]string),
		histories:   make(map[string]*benefitHistory, len(ownership)),
	}
	for b, p := range ownership {
		l.ownership[b] = p
		l.byPortfolio[p] = append(l.byPortfolio[p], b)
		l.histories[b] = &benefitHistory{}
	}
	return l
}

// Revision 返回当前全局修订号。
func (l *Ledger) Revision() int64 {
	return l.revision
}

// Put 写入一个季度快照，actual 为截至该季度的累计已实现值（整数分）。
// 同一效益、同一季度的新 Put 替换当前快照。
// 成功时返回新产生的全局修订号。
func (l *Ledger) Put(benefit string, quarter int, actual int64) (int64, error) {
	h, ok := l.histories[benefit]
	if !ok {
		return 0, ErrUnknownBenefit
	}
	if quarter < 0 {
		return 0, ErrInvalidQuarter
	}
	if actual < 0 {
		return 0, ErrNegativeActual
	}
	next := h.current().clone()
	next[quarter] = actual
	l.revision++
	h.versions = append(h.versions, version{revision: l.revision, state: next})
	return l.revision, nil
}

// Retract 撤回该效益当前该季度的快照。
// 撤回使该季度在新版本中不存在，不会恢复该季度更早的 Put。
// 成功时返回新产生的全局修订号；该季度当前无快照时返回 ErrNoSnapshot。
func (l *Ledger) Retract(benefit string, quarter int) (int64, error) {
	h, ok := l.histories[benefit]
	if !ok {
		return 0, ErrUnknownBenefit
	}
	if quarter < 0 {
		return 0, ErrInvalidQuarter
	}
	cur := h.current()
	if _, ok := cur[quarter]; !ok {
		return 0, ErrNoSnapshot
	}
	next := cur.clone()
	delete(next, quarter)
	l.revision++
	h.versions = append(h.versions, version{revision: l.revision, state: next})
	return l.revision, nil
}

// Report 返回指定修订号下某组合的累计已实现总额与本季度净实现额。
//
// 对每个效益，取截至 revision 仍存在、且季度不晚于 quarter 的最后一个
// 快照作为其累计已实现值；没有快照则为 0。净实现额等于本季度累计总额
// 减上一季度累计总额（两者使用同一修订号），第 0 季度的上一季度按 0 算。
func (l *Ledger) Report(portfolio string, quarter int, revision int64) (cumulative, net int64, err error) {
	benefits, ok := l.byPortfolio[portfolio]
	if !ok {
		return 0, 0, ErrUnknownPortfolio
	}
	if quarter < 0 {
		return 0, 0, ErrInvalidQuarter
	}
	if revision < 0 || revision > l.revision {
		return 0, 0, ErrInvalidRevision
	}
	cumulative = l.totalAt(benefits, quarter, revision)
	var prev int64
	if quarter > 0 {
		prev = l.totalAt(benefits, quarter-1, revision)
	}
	return cumulative, cumulative - prev, nil
}

// totalAt 计算一组效益在指定修订号、指定季度的累计已实现总额。
func (l *Ledger) totalAt(benefits []string, quarter int, revision int64) int64 {
	var total int64
	for _, b := range benefits {
		total += l.histories[b].at(revision).valueAt(quarter)
	}
	return total
}
