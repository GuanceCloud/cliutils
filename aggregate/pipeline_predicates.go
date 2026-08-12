package aggregate

import (
	fp "github.com/GuanceCloud/cliutils/filter"
)

// span 谓词编译：将 SamplingPipeline 的条件静态编译为基于 DataPacket
// span 谓词摘要（Pred* / *DurationUs 字段）的判定，使决策无需解压 payload。
// 无法被摘要覆盖的条件（回退到解压 walk）在编译期返回 ok=false。

type spanPredicateExpr func(*DataPacket) bool

// packetHasSpanPredicates 判断 packet 是否携带 PickTrace 组包时计算的谓词摘要。
// 谓词全零的包（旧版本 Datakit / 手工构造 / 无 duration 字段的异常数据）不信任摘要，
// 决策回退到解压 walk，保持与旧逻辑一致。
func packetHasSpanPredicates(packet *DataPacket) bool {
	if packet == nil {
		return false
	}

	return packet.PredError || packet.PredHttpError || packet.PredBizError ||
		packet.PredTraceKeep || packet.MaxSpanDurationUs > 0 ||
		packet.RootDurationUs > 0 || packet.MaxNonrootDurationUs > 0
}

const (
	predParentNone = iota
	predParentRoot
	predParentNonRoot
)

type spanPred struct {
	flag       spanPredicateExpr // 直接布尔谓词（error/5xx/biz/trace_keep 等）
	parentID   int               // predParentNone / predParentRoot / predParentNonRoot
	durationGT int64             // duration 阈值（微秒），0 表示无约束
}

// compilePipelinePredicate 编译单条 pipeline。
// match-all 概率采样（无条件）恒匹配；条件管道逐条分析，任一无法覆盖则失败。
func compilePipelinePredicate(pipeline *SamplingPipeline) (spanPredicateExpr, bool) {
	if pipeline == nil {
		return nil, false
	}

	if pipeline.isMatchAllSampling() {
		return func(*DataPacket) bool { return true }, true
	}

	if pipeline.conds == nil {
		return nil, false
	}

	var exprs []spanPredicateExpr
	for _, node := range pipeline.conds {
		wc, ok := node.(*fp.WhereCondition)
		if !ok {
			return nil, false
		}
		expr, ok2 := compileWhereCondition(wc)
		if !ok2 || expr == nil {
			return nil, false
		}
		exprs = append(exprs, expr)
	}

	// WhereConditions.Eval 语义：任一 WhereCondition 满足即命中。
	return func(packet *DataPacket) bool {
		for _, expr := range exprs {
			if expr(packet) {
				return true
			}
		}
		return false
	}, true
}

// compileWhereCondition 编译一个 {} 内的 AND 组合。
func compileWhereCondition(wc *fp.WhereCondition) (spanPredicateExpr, bool) {
	if wc == nil {
		return nil, false
	}

	combined := spanPred{}
	for _, node := range wc.Conds() {
		be, ok := node.(*fp.BinaryExpr)
		if !ok {
			return nil, false
		}
		part, ok2 := compilePred(be)
		if !ok2 {
			return nil, false
		}
		merged, ok3 := mergePreds(combined, part)
		if !ok3 {
			return nil, false
		}
		combined = merged
	}

	return combined.eval(), true
}

func compilePred(be *fp.BinaryExpr) (spanPred, bool) {
	if be == nil {
		return spanPred{}, false
	}

	switch be.Op {
	case fp.AND:
		l, ok1 := compilePred(asBinaryExpr(be.LHS))
		r, ok2 := compilePred(asBinaryExpr(be.RHS))
		if !ok1 || !ok2 {
			return spanPred{}, false
		}
		return mergePreds(l, r)
	case fp.OR:
		l, ok1 := compilePred(asBinaryExpr(be.LHS))
		r, ok2 := compilePred(asBinaryExpr(be.RHS))
		if !ok1 || !ok2 {
			return spanPred{}, false
		}
		le, re := l.eval(), r.eval()
		if le == nil || re == nil {
			return spanPred{}, false
		}
		return spanPred{flag: func(packet *DataPacket) bool { return le(packet) || re(packet) }}, true
	default:
		return compileAtomic(be)
	}
}

func asBinaryExpr(node fp.Node) *fp.BinaryExpr {
	switch e := node.(type) {
	case *fp.BinaryExpr:
		return e
	case *fp.ParenExpr:
		return asBinaryExpr(e.Param)
	default:
		return nil
	}
}

// mergePreds 按 AND 语义合并两个谓词片段。
// parent 约束冲突（root 与 nonroot 并存）或组合无法表达时返回 false。
func mergePreds(a, b spanPred) (spanPred, bool) {
	out := spanPred{}

	switch {
	case a.flag != nil && b.flag != nil:
		fa, fb := a.flag, b.flag
		out.flag = func(packet *DataPacket) bool { return fa(packet) && fb(packet) }
	case a.flag != nil:
		out.flag = a.flag
	case b.flag != nil:
		out.flag = b.flag
	}

	switch {
	case a.parentID != predParentNone && b.parentID != predParentNone && a.parentID != b.parentID:
		return spanPred{}, false
	case a.parentID != predParentNone:
		out.parentID = a.parentID
	case b.parentID != predParentNone:
		out.parentID = b.parentID
	}

	out.durationGT = max(a.durationGT, b.durationGT)

	return out, true
}

// eval 生成最终判定函数；nil 表示该组合无法被谓词覆盖。
func (p *spanPred) eval() spanPredicateExpr {
	var dur spanPredicateExpr
	switch {
	case p.parentID == predParentRoot && p.durationGT > 0:
		threshold := p.durationGT
		dur = func(packet *DataPacket) bool { return packet.RootDurationUs > threshold }
	case p.parentID == predParentNonRoot && p.durationGT > 0:
		threshold := p.durationGT
		dur = func(packet *DataPacket) bool { return packet.MaxNonrootDurationUs > threshold }
	case p.parentID == predParentNone && p.durationGT > 0:
		threshold := p.durationGT
		dur = func(packet *DataPacket) bool { return packet.MaxSpanDurationUs > threshold }
	}

	switch {
	case p.flag != nil && dur != nil:
		flag := p.flag
		return func(packet *DataPacket) bool { return flag(packet) && dur(packet) }
	case p.flag != nil:
		return p.flag
	case dur != nil:
		return dur
	default:
		return nil
	}
}

// compileAtomic 编译原子比较条件。
func compileAtomic(be *fp.BinaryExpr) (spanPred, bool) {
	if be == nil {
		return spanPred{}, false
	}

	ident, ok := be.LHS.(*fp.Identifier)
	if !ok {
		return spanPred{}, false
	}

	switch ident.Name {
	case "status":
		if be.Op == fp.EQ {
			if s, ok := stringLiteralValue(be.RHS); ok && s == "error" {
				return spanPred{flag: func(packet *DataPacket) bool { return packet.PredError }}, true
			}
		}
	case "http_status_code":
		if be.Op == fp.MATCH && isHTTPErrorRegex(be.RHS) {
			return spanPred{flag: func(packet *DataPacket) bool { return packet.PredHttpError }}, true
		}
	case "body_code":
		if be.Op == fp.NOT_IN && isBizErrorList(be.RHS) {
			return spanPred{flag: func(packet *DataPacket) bool { return packet.PredBizError }}, true
		}
	case "trace_keep":
		if be.Op == fp.EQ {
			if b, ok := boolLiteralValue(be.RHS); ok && b {
				return spanPred{flag: func(packet *DataPacket) bool { return packet.PredTraceKeep }}, true
			}
		}
	case "parent_id":
		if s, ok := stringLiteralValue(be.RHS); ok && s == "0" {
			switch be.Op {
			case fp.EQ:
				return spanPred{parentID: predParentRoot}, true
			case fp.NEQ:
				return spanPred{parentID: predParentNonRoot}, true
			}
		}
	case "duration":
		if be.Op == fp.GT {
			if n, ok := numberLiteralInt(be.RHS); ok && n > 0 {
				return spanPred{durationGT: n}, true
			}
		}
	}

	return spanPred{}, false
}

func stringLiteralValue(node fp.Node) (string, bool) {
	s, ok := node.(*fp.StringLiteral)
	if !ok {
		return "", false
	}
	return s.Val, true
}

func boolLiteralValue(node fp.Node) (bool, bool) {
	b, ok := node.(*fp.BoolLiteral)
	if !ok {
		return false, false
	}
	return b.Val, true
}

func numberLiteralInt(node fp.Node) (int64, bool) {
	n, ok := node.(*fp.NumberLiteral)
	if !ok || !n.IsInt {
		return 0, false
	}
	return n.Int, true
}

// isHTTPErrorRegex 判断 MATCH 正则是否精确等价于 4xx/5xx 状态码匹配。
// 仅识别已知等价形式；其余正则回退解压 walk，保证语义不变。
func isHTTPErrorRegex(node fp.Node) bool {
	list, ok := node.(fp.NodeList)
	if !ok || len(list) != 1 {
		return false
	}

	re, ok := list[0].(*fp.Regex)
	if !ok {
		return false
	}

	switch re.Regex {
	case "^[45][0-9][0-9]$",
		"^4[0-9][0-9]$|^5[0-9][0-9]$",
		"^[45]\\d\\d$",
		"^4\\d\\d$|^5\\d\\d$",
		"^[45][0-9]{2}$",
		"^4[0-9]{2}$|^5[0-9]{2}$":
		return true
	default:
		return false
	}
}

// isBizErrorList 判断 NOT_IN 列表是否精确为 ["0", "200", null]。
func isBizErrorList(node fp.Node) bool {
	list, ok := node.(fp.NodeList)
	if !ok || len(list) != 3 {
		return false
	}

	hasZero, has200, hasNil := false, false, false
	for _, item := range list {
		switch v := item.(type) {
		case *fp.StringLiteral:
			switch v.Val {
			case "0":
				hasZero = true
			case "200":
				has200 = true
			}
		case *fp.NilLiteral:
			hasNil = true
		}
	}

	return hasZero && has200 && hasNil
}
