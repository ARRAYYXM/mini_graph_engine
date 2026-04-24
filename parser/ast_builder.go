package parser

import (
	"fmt"
	"mini_graph_engine/ast"
	"mini_graph_engine/cyphertree"
	"strconv"
	"strings"

	"github.com/antlr4-go/antlr/v4"
)

// ASTBuilder 将 ANTLR ParseTree 转换为自定义 AST。
type ASTBuilder struct {
	*cyphertree.BaseCypherParserVisitor
}

func NewASTBuilder() *ASTBuilder {
	return &ASTBuilder{BaseCypherParserVisitor: &cyphertree.BaseCypherParserVisitor{}}
}

// Build 是便捷方法：解析字符串并构建 AST。
func Build(input string) (*ast.Query, error) {
	tree, err := Parse(input)
	if err != nil {
		return nil, err
	}
	builder := NewASTBuilder()
	result := tree.Accept(builder)
	if q, ok := result.(*ast.Query); ok {
		return q, nil
	}
	return nil, fmt.Errorf("failed to build AST")
}

// -------------------- 辅助方法 --------------------

func (v *ASTBuilder) visitExpr(ctx antlr.ParserRuleContext) ast.Expression {
	if ctx == nil {
		return nil
	}
	r := ctx.Accept(v)
	if r == nil {
		return nil
	}
	if e, ok := r.(ast.Expression); ok {
		return e
	}
	return nil
}

func (v *ASTBuilder) visitClause(ctx antlr.ParserRuleContext) ast.Clause {
	if ctx == nil {
		return nil
	}
	r := ctx.Accept(v)
	if r == nil {
		return nil
	}
	if c, ok := r.(ast.Clause); ok {
		return c
	}
	return nil
}

func textOf(ctx antlr.ParserRuleContext) string {
	if ctx == nil {
		return ""
	}
	return strings.TrimSpace(ctx.GetText())
}

func textOfTerminal(t antlr.TerminalNode) string {
	if t == nil {
		return ""
	}
	return strings.TrimSpace(t.GetText())
}

// -------------------- Script / Query --------------------

func (v *ASTBuilder) VisitScript(ctx *cyphertree.ScriptContext) interface{} {
	if q := ctx.Query(); q != nil {
		return q.Accept(v)
	}
	return nil
}

func (v *ASTBuilder) VisitQuery(ctx *cyphertree.QueryContext) interface{} {
	if rq := ctx.RegularQuery(); rq != nil {
		return rq.Accept(v)
	}
	return nil
}

func (v *ASTBuilder) VisitRegularQuery(ctx *cyphertree.RegularQueryContext) interface{} {
	query := &ast.Query{}
	if sq := ctx.SingleQuery(); sq != nil {
		part := sq.Accept(v).(*ast.SingleQuery)
		query.Parts = append(query.Parts, part)
	}
	return query
}

func (v *ASTBuilder) VisitSingleQuery(ctx *cyphertree.SingleQueryContext) interface{} {
	if sp := ctx.SinglePartQ(); sp != nil {
		return sp.Accept(v)
	}
	if mp := ctx.MultiPartQ(); mp != nil {
		return mp.Accept(v)
	}
	return &ast.SingleQuery{}
}

func (v *ASTBuilder) VisitSinglePartQ(ctx *cyphertree.SinglePartQContext) interface{} {
	sq := &ast.SingleQuery{}
	for _, rs := range ctx.AllReadingStatement() {
		if c := v.visitClause(rs); c != nil {
			sq.Clauses = append(sq.Clauses, c)
		}
	}
	for _, us := range ctx.AllUpdatingStatement() {
		if c := v.visitClause(us); c != nil {
			sq.Clauses = append(sq.Clauses, c)
		}
	}
	if rs := ctx.ReturnSt(); rs != nil {
		if c := v.visitClause(rs); c != nil {
			sq.Clauses = append(sq.Clauses, c)
		}
	}
	return sq
}

func (v *ASTBuilder) VisitMultiPartQ(ctx *cyphertree.MultiPartQContext) interface{} {
	sq := &ast.SingleQuery{}
	for _, rs := range ctx.AllReadingStatement() {
		if c := v.visitClause(rs); c != nil {
			sq.Clauses = append(sq.Clauses, c)
		}
	}
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if prc, ok := child.(antlr.ParserRuleContext); ok {
			switch prc.GetRuleIndex() {
			case cyphertree.CypherParserRULE_updatingStatement,
				 cyphertree.CypherParserRULE_withSt:
				if c := v.visitClause(prc); c != nil {
					sq.Clauses = append(sq.Clauses, c)
				}
			}
		}
	}
	if sp := ctx.SinglePartQ(); sp != nil {
		part := sp.Accept(v).(*ast.SingleQuery)
		sq.Clauses = append(sq.Clauses, part.Clauses...)
	}
	return sq
}

// -------------------- Reading Statements --------------------

func (v *ASTBuilder) VisitReadingStatement(ctx *cyphertree.ReadingStatementContext) interface{} {
	if m := ctx.MatchSt(); m != nil {
		return m.Accept(v)
	}
	if u := ctx.UnwindSt(); u != nil {
		return u.Accept(v)
	}
	return nil
}

func (v *ASTBuilder) VisitMatchSt(ctx *cyphertree.MatchStContext) interface{} {
	mc := &ast.MatchClause{Optional: ctx.OPTIONAL() != nil}
	if pw := ctx.PatternWhere(); pw != nil {
		mc.Patterns = v.visitPatternWhere(pw)
		if w := pw.Where(); w != nil {
			wCtx, _ := w.(*cyphertree.WhereContext)
			if wCtx != nil && wCtx.Expression() != nil {
				mc.Where = v.visitExpr(wCtx.Expression())
			}
		}
	}
	return mc
}

func (v *ASTBuilder) VisitUnwindSt(ctx *cyphertree.UnwindStContext) interface{} {
	uc := &ast.UnwindClause{Expression: v.visitExpr(ctx.Expression())}
	if sym := ctx.Symbol(); sym != nil {
		uc.Alias = textOf(sym)
	}
	return uc
}

// -------------------- Updating Statements --------------------

func (v *ASTBuilder) VisitUpdatingStatement(ctx *cyphertree.UpdatingStatementContext) interface{} {
	if c := ctx.CreateSt(); c != nil {
		return c.Accept(v)
	}
	if m := ctx.MergeSt(); m != nil {
		return m.Accept(v)
	}
	if d := ctx.DeleteSt(); d != nil {
		return d.Accept(v)
	}
	if s := ctx.SetSt(); s != nil {
		return s.Accept(v)
	}
	if r := ctx.RemoveSt(); r != nil {
		return r.Accept(v)
	}
	return nil
}

func (v *ASTBuilder) VisitCreateSt(ctx *cyphertree.CreateStContext) interface{} {
	cc := &ast.CreateClause{}
	if p := ctx.Pattern(); p != nil {
		cc.Patterns = v.visitPattern(p)
	}
	return cc
}

func (v *ASTBuilder) VisitDeleteSt(ctx *cyphertree.DeleteStContext) interface{} {
	dc := &ast.DeleteClause{Detach: ctx.DETACH() != nil}
	if ec := ctx.ExpressionChain(); ec != nil {
		dc.Exprs = v.visitExpressionChain(ec)
	}
	return dc
}

func (v *ASTBuilder) VisitSetSt(ctx *cyphertree.SetStContext) interface{} {
	sc := &ast.SetClause{}
	for _, si := range ctx.AllSetItem() {
		if item := si.Accept(v); item != nil {
			sc.Items = append(sc.Items, item.(*ast.SetItem))
		}
	}
	return sc
}

func (v *ASTBuilder) VisitSetItem(ctx *cyphertree.SetItemContext) interface{} {
	item := &ast.SetItem{}
	if pe := ctx.PropertyExpression(); pe != nil {
		item.Target = v.visitExpr(pe)
		if ctx.ASSIGN() != nil {
			item.Operator = "="
		}
		item.Value = v.visitExpr(ctx.Expression())
	} else if sym := ctx.Symbol(); sym != nil {
		item.Target = &ast.VariableExpr{Name: textOf(sym)}
		if ctx.ASSIGN() != nil {
			item.Operator = "="
		} else if ctx.ADD_ASSIGN() != nil {
			item.Operator = "+="
		}
		item.Value = v.visitExpr(ctx.Expression())
	}
	return item
}

func (v *ASTBuilder) VisitRemoveSt(ctx *cyphertree.RemoveStContext) interface{} {
	rc := &ast.RemoveClause{}
	for _, ri := range ctx.AllRemoveItem() {
		if item := ri.Accept(v); item != nil {
			rc.Items = append(rc.Items, item.(*ast.RemoveItem))
		}
	}
	return rc
}

func (v *ASTBuilder) VisitRemoveItem(ctx *cyphertree.RemoveItemContext) interface{} {
	item := &ast.RemoveItem{}
	if pe := ctx.PropertyExpression(); pe != nil {
		item.Target = v.visitExpr(pe)
	} else if sym := ctx.Symbol(); sym != nil {
		item.Target = &ast.VariableExpr{Name: textOf(sym)}
		item.NodeLabels = v.visitNodeLabels(ctx.NodeLabels())
	}
	return item
}

func (v *ASTBuilder) VisitMergeSt(ctx *cyphertree.MergeStContext) interface{} {
	mc := &ast.MergeClause{}
	if pp := ctx.PatternPart(); pp != nil {
		mc.Pattern = v.visitPatternPart(pp)
	}
	for _, ma := range ctx.AllMergeAction() {
		if action := ma.Accept(v); action != nil {
			mc.Actions = append(mc.Actions, action.(*ast.MergeAction))
		}
	}
	return mc
}

func (v *ASTBuilder) VisitMergeAction(ctx *cyphertree.MergeActionContext) interface{} {
	action := &ast.MergeAction{OnCreate: ctx.CREATE() != nil}
	if s := ctx.SetSt(); s != nil {
		setClause := s.Accept(v).(*ast.SetClause)
		action.SetItems = setClause.Items
	}
	return action
}

// -------------------- WITH / RETURN --------------------

func (v *ASTBuilder) VisitWithSt(ctx *cyphertree.WithStContext) interface{} {
	wc := &ast.WithClause{}
	if pb := ctx.ProjectionBody(); pb != nil {
		v.visitProjectionBody(pb, wc)
	}
	if w := ctx.Where(); w != nil {
		wCtx, _ := w.(*cyphertree.WhereContext)
		if wCtx != nil && wCtx.Expression() != nil {
			wc.Where = v.visitExpr(wCtx.Expression())
		}
	}
	return wc
}

func (v *ASTBuilder) VisitReturnSt(ctx *cyphertree.ReturnStContext) interface{} {
	rc := &ast.ReturnClause{}
	if pb := ctx.ProjectionBody(); pb != nil {
		v.visitProjectionBody(pb, rc)
	}
	return rc
}

func (v *ASTBuilder) visitProjectionBody(ctx cyphertree.IProjectionBodyContext, target interface{}) {
	pb, _ := ctx.(*cyphertree.ProjectionBodyContext)
	if pb == nil {
		return
	}
	switch t := target.(type) {
	case *ast.WithClause:
		t.Distinct = pb.DISTINCT() != nil
	case *ast.ReturnClause:
		t.Distinct = pb.DISTINCT() != nil
	}
	if pi := pb.ProjectionItems(); pi != nil {
		items := v.visitProjectionItems(pi)
		switch t := target.(type) {
		case *ast.WithClause:
			t.Projections = items
		case *ast.ReturnClause:
			t.Projections = items
		}
	}
	if o := pb.OrderSt(); o != nil {
		order := v.visitOrderSt(o)
		switch t := target.(type) {
		case *ast.WithClause:
			t.OrderBy = order
		case *ast.ReturnClause:
			t.OrderBy = order
		}
	}
	if s := pb.SkipSt(); s != nil {
		skip := v.visitExpr(s.Expression())
		switch t := target.(type) {
		case *ast.WithClause:
			t.Skip = skip
		case *ast.ReturnClause:
			t.Skip = skip
		}
	}
	if l := pb.LimitSt(); l != nil {
		limit := v.visitExpr(l.Expression())
		switch t := target.(type) {
		case *ast.WithClause:
			t.Limit = limit
		case *ast.ReturnClause:
			t.Limit = limit
		}
	}
}

func (v *ASTBuilder) visitProjectionItems(ctx cyphertree.IProjectionItemsContext) []*ast.ProjectionItem {
	pi, _ := ctx.(*cyphertree.ProjectionItemsContext)
	if pi == nil {
		return nil
	}
	var items []*ast.ProjectionItem
	if pi.MULT() != nil {
		items = append(items, &ast.ProjectionItem{Expression: &ast.VariableExpr{Name: "*"}})
	}
	for _, item := range pi.AllProjectionItem() {
		if it := item.Accept(v); it != nil {
			items = append(items, it.(*ast.ProjectionItem))
		}
	}
	return items
}

func (v *ASTBuilder) VisitProjectionItem(ctx *cyphertree.ProjectionItemContext) interface{} {
	item := &ast.ProjectionItem{Expression: v.visitExpr(ctx.Expression())}
	if sym := ctx.Symbol(); sym != nil {
		item.Alias = textOf(sym)
	}
	return item
}

func (v *ASTBuilder) visitOrderSt(ctx cyphertree.IOrderStContext) []*ast.OrderItem {
	o, _ := ctx.(*cyphertree.OrderStContext)
	if o == nil {
		return nil
	}
	var items []*ast.OrderItem
	for _, oi := range o.AllOrderItem() {
		if it := oi.Accept(v); it != nil {
			items = append(items, it.(*ast.OrderItem))
		}
	}
	return items
}

func (v *ASTBuilder) VisitOrderItem(ctx *cyphertree.OrderItemContext) interface{} {
	item := &ast.OrderItem{Expression: v.visitExpr(ctx.Expression())}
	if ctx.DESC() != nil || ctx.DESCENDING() != nil {
		item.Descending = true
	}
	return item
}

// -------------------- Pattern --------------------

func (v *ASTBuilder) visitPatternWhere(ctx cyphertree.IPatternWhereContext) []*ast.Pattern {
	pw, _ := ctx.(*cyphertree.PatternWhereContext)
	if pw == nil {
		return nil
	}
	if p := pw.Pattern(); p != nil {
		return v.visitPattern(p)
	}
	return nil
}

func (v *ASTBuilder) visitPattern(ctx cyphertree.IPatternContext) []*ast.Pattern {
	p, _ := ctx.(*cyphertree.PatternContext)
	if p == nil {
		return nil
	}
	pat := &ast.Pattern{}
	for _, pp := range p.AllPatternPart() {
		pat.Parts = append(pat.Parts, v.visitPatternPart(pp))
	}
	return []*ast.Pattern{pat}
}

func (v *ASTBuilder) visitPatternPart(ctx cyphertree.IPatternPartContext) *ast.PatternPart {
	pp, _ := ctx.(*cyphertree.PatternPartContext)
	if pp == nil {
		return nil
	}
	part := &ast.PatternPart{}
	if sym := pp.Symbol(); sym != nil {
		part.Variable = textOf(sym)
	}
	if pe := pp.PatternElem(); pe != nil {
		part.Element = v.visitPatternElem(pe)
	}
	return part
}

func (v *ASTBuilder) visitPatternElem(ctx cyphertree.IPatternElemContext) *ast.PatternElement {
	pe, _ := ctx.(*cyphertree.PatternElemContext)
	if pe == nil {
		return nil
	}
	elem := &ast.PatternElement{}
	if np := pe.NodePattern(); np != nil {
		elem.Nodes = append(elem.Nodes, v.visitNodePattern(np))
	}
	for _, pec := range pe.AllPatternElemChain() {
		c := pec.Accept(v)
		if c != nil {
			chain := c.(*patternElemChainResult)
			elem.Rels = append(elem.Rels, chain.Rel)
			elem.Nodes = append(elem.Nodes, chain.Node)
		}
	}
	return elem
}

type patternElemChainResult struct {
	Rel  *ast.RelPattern
	Node *ast.NodePattern
}

func (v *ASTBuilder) VisitPatternElemChain(ctx *cyphertree.PatternElemChainContext) interface{} {
	result := &patternElemChainResult{}
	if rp := ctx.RelationshipPattern(); rp != nil {
		result.Rel = v.visitRelationshipPattern(rp)
	}
	if np := ctx.NodePattern(); np != nil {
		result.Node = v.visitNodePattern(np)
	}
	return result
}

func (v *ASTBuilder) visitNodePattern(ctx cyphertree.INodePatternContext) *ast.NodePattern {
	np, _ := ctx.(*cyphertree.NodePatternContext)
	if np == nil {
		return nil
	}
	node := &ast.NodePattern{}
	if sym := np.Symbol(); sym != nil {
		node.Variable = textOf(sym)
	}
	node.Labels = v.visitNodeLabels(np.NodeLabels())
	if props := np.Properties(); props != nil {
		node.Properties = v.visitProperties(props)
	}
	return node
}

func (v *ASTBuilder) visitRelationshipPattern(ctx cyphertree.IRelationshipPatternContext) *ast.RelPattern {
	rp, _ := ctx.(*cyphertree.RelationshipPatternContext)
	if rp == nil {
		return nil
	}
	rel := &ast.RelPattern{}
	hasGT := false
	for i := 0; i < rp.GetChildCount(); i++ {
		if term, ok := rp.GetChild(i).(antlr.TerminalNode); ok {
			if term.GetSymbol().GetTokenType() == cyphertree.CypherParserGT {
				hasGT = true
				break
			}
		}
	}
	if rp.LT() != nil {
		rel.Direction = ast.DirLeft
	} else if hasGT {
		rel.Direction = ast.DirRight
	} else {
		rel.Direction = ast.DirBoth
	}
	if rd := rp.RelationDetail(); rd != nil {
		detail := v.visitRelationDetail(rd)
		rel.Variable = detail.Variable
		rel.Types = detail.Types
		rel.Properties = detail.Properties
		rel.Range = detail.Range
	}
	return rel
}

type relDetailResult struct {
	Variable   string
	Types      []string
	Properties ast.Expression
	Range      *ast.RangeLiteral
}

func (v *ASTBuilder) visitRelationDetail(ctx cyphertree.IRelationDetailContext) *relDetailResult {
	rd, _ := ctx.(*cyphertree.RelationDetailContext)
	if rd == nil {
		return nil
	}
	result := &relDetailResult{}
	if sym := rd.Symbol(); sym != nil {
		result.Variable = textOf(sym)
	}
	if rt := rd.RelationshipTypes(); rt != nil {
		result.Types = v.visitRelationshipTypes(rt)
	}
	if rl := rd.RangeLit(); rl != nil {
		result.Range = v.visitRangeLit(rl)
	}
	if props := rd.Properties(); props != nil {
		result.Properties = v.visitProperties(props)
	}
	return result
}

func (v *ASTBuilder) visitRelationshipTypes(ctx cyphertree.IRelationshipTypesContext) []string {
	rt, _ := ctx.(*cyphertree.RelationshipTypesContext)
	if rt == nil {
		return nil
	}
	var types []string
	for _, n := range rt.AllName() {
		types = append(types, v.visitName(n))
	}
	return types
}

func (v *ASTBuilder) visitRangeLit(ctx cyphertree.IRangeLitContext) *ast.RangeLiteral {
	if ctx == nil {
		return nil
	}
	return &ast.RangeLiteral{} // 简化：暂不解析具体数值
}

func (v *ASTBuilder) visitProperties(ctx cyphertree.IPropertiesContext) ast.Expression {
	p, _ := ctx.(*cyphertree.PropertiesContext)
	if p == nil {
		return nil
	}
	if ml := p.MapLit(); ml != nil {
		return v.visitExpr(ml)
	}
	if param := p.Parameter(); param != nil {
		return v.visitExpr(param)
	}
	return nil
}

func (v *ASTBuilder) visitNodeLabels(ctx cyphertree.INodeLabelsContext) []string {
	nl, _ := ctx.(*cyphertree.NodeLabelsContext)
	if nl == nil {
		return nil
	}
	var labels []string
	for _, n := range nl.AllName() {
		labels = append(labels, v.visitName(n))
	}
	return labels
}

// -------------------- Expression (优先级链) --------------------

func (v *ASTBuilder) VisitExpression(ctx *cyphertree.ExpressionContext) interface{} {
	exprs := ctx.AllXorExpression()
	if len(exprs) == 0 {
		return nil
	}
	result := v.visitExpr(exprs[0])
	for i := 1; i < len(exprs); i++ {
		result = &ast.BinaryExpr{Op: "OR", Left: result, Right: v.visitExpr(exprs[i])}
	}
	return result
}

func (v *ASTBuilder) VisitXorExpression(ctx *cyphertree.XorExpressionContext) interface{} {
	exprs := ctx.AllAndExpression()
	if len(exprs) == 0 {
		return nil
	}
	result := v.visitExpr(exprs[0])
	for i := 1; i < len(exprs); i++ {
		result = &ast.BinaryExpr{Op: "XOR", Left: result, Right: v.visitExpr(exprs[i])}
	}
	return result
}

func (v *ASTBuilder) VisitAndExpression(ctx *cyphertree.AndExpressionContext) interface{} {
	exprs := ctx.AllNotExpression()
	if len(exprs) == 0 {
		return nil
	}
	result := v.visitExpr(exprs[0])
	for i := 1; i < len(exprs); i++ {
		result = &ast.BinaryExpr{Op: "AND", Left: result, Right: v.visitExpr(exprs[i])}
	}
	return result
}

func (v *ASTBuilder) VisitNotExpression(ctx *cyphertree.NotExpressionContext) interface{} {
	expr := v.visitExpr(ctx.ComparisonExpression())
	if ctx.NOT() != nil {
		return &ast.UnaryExpr{Op: "NOT", Expr: expr}
	}
	return expr
}

func (v *ASTBuilder) VisitComparisonExpression(ctx *cyphertree.ComparisonExpressionContext) interface{} {
	left := v.visitExpr(ctx.AddSubExpression(0))
	comps := ctx.AllComparisonSigns()
	rights := ctx.AllAddSubExpression()
	for i, cs := range comps {
		sign := cs.Accept(v).(string)
		right := v.visitExpr(rights[i+1])
		left = &ast.BinaryExpr{Op: sign, Left: left, Right: right}
	}
	return left
}

func (v *ASTBuilder) VisitComparisonSigns(ctx *cyphertree.ComparisonSignsContext) interface{} {
	switch {
	case ctx.ASSIGN() != nil:
		return "="
	case ctx.LE() != nil:
		return "<="
	case ctx.GE() != nil:
		return ">="
	case ctx.GT() != nil:
		return ">"
	case ctx.LT() != nil:
		return "<"
	case ctx.NOT_EQUAL() != nil:
		return "<>"
	default:
		return "="
	}
}

func (v *ASTBuilder) VisitAddSubExpression(ctx *cyphertree.AddSubExpressionContext) interface{} {
	exprs := ctx.AllMultDivExpression()
	if len(exprs) == 0 {
		return nil
	}
	result := v.visitExpr(exprs[0])
	for i := 1; i < len(exprs); i++ {
		op := "-"
		if ctx.PLUS(i-1) != nil {
			op = "+"
		}
		result = &ast.BinaryExpr{Op: op, Left: result, Right: v.visitExpr(exprs[i])}
	}
	return result
}

func (v *ASTBuilder) VisitMultDivExpression(ctx *cyphertree.MultDivExpressionContext) interface{} {
	exprs := ctx.AllPowerExpression()
	if len(exprs) == 0 {
		return nil
	}
	result := v.visitExpr(exprs[0])
	for i := 1; i < len(exprs); i++ {
		op := "%"
		if ctx.MULT(i-1) != nil {
			op = "*"
		} else if ctx.DIV(i-1) != nil {
			op = "/"
		}
		result = &ast.BinaryExpr{Op: op, Left: result, Right: v.visitExpr(exprs[i])}
	}
	return result
}

func (v *ASTBuilder) VisitPowerExpression(ctx *cyphertree.PowerExpressionContext) interface{} {
	exprs := ctx.AllUnaryAddSubExpression()
	if len(exprs) == 0 {
		return nil
	}
	result := v.visitExpr(exprs[0])
	for i := 1; i < len(exprs); i++ {
		result = &ast.BinaryExpr{Op: "^", Left: result, Right: v.visitExpr(exprs[i])}
	}
	return result
}

func (v *ASTBuilder) VisitUnaryAddSubExpression(ctx *cyphertree.UnaryAddSubExpressionContext) interface{} {
	expr := v.visitExpr(ctx.AtomicExpression())
	if ctx.PLUS() != nil {
		return &ast.UnaryExpr{Op: "+", Expr: expr}
	}
	if ctx.SUB() != nil {
		return &ast.UnaryExpr{Op: "-", Expr: expr}
	}
	return expr
}

func (v *ASTBuilder) VisitAtomicExpression(ctx *cyphertree.AtomicExpressionContext) interface{} {
	base := v.visitExpr(ctx.PropertyOrLabelExpression())
	for i := 1; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if prc, ok := child.(antlr.ParserRuleContext); ok {
			switch prc.GetRuleIndex() {
			case cyphertree.CypherParserRULE_listExpression:
				le := prc.(*cyphertree.ListExpressionContext)
				if le.IN() != nil {
					base = &ast.InExpr{Left: base, Right: v.visitExpr(le.PropertyOrLabelExpression())}
				}
			case cyphertree.CypherParserRULE_stringExpression:
				se := prc.(*cyphertree.StringExpressionContext)
				prefix := v.visitStringExpPrefix(se.StringExpPrefix())
				base = &ast.StringMatchExpr{Op: prefix, Left: base, Right: v.visitExpr(se.PropertyOrLabelExpression())}
			case cyphertree.CypherParserRULE_nullExpression:
				ne := prc.(*cyphertree.NullExpressionContext)
				base = &ast.IsNullExpr{Expr: base, Not: ne.NOT() != nil}
			}
		}
	}
	return base
}

func (v *ASTBuilder) visitStringExpPrefix(ctx cyphertree.IStringExpPrefixContext) string {
	sp, _ := ctx.(*cyphertree.StringExpPrefixContext)
	if sp == nil {
		return ""
	}
	if sp.STARTS() != nil {
		return "STARTS WITH"
	}
	if sp.ENDS() != nil {
		return "ENDS WITH"
	}
	return "CONTAINS"
}

func (v *ASTBuilder) VisitPropertyOrLabelExpression(ctx *cyphertree.PropertyOrLabelExpressionContext) interface{} {
	base := v.visitExpr(ctx.PropertyExpression())
	if nl := ctx.NodeLabels(); nl != nil {
		labels := v.visitNodeLabels(nl)
		base = &ast.LabelExpr{Expression: base, Labels: labels}
	}
	return base
}

func (v *ASTBuilder) VisitPropertyExpression(ctx *cyphertree.PropertyExpressionContext) interface{} {
	base := v.visitExpr(ctx.Atom())
	for _, n := range ctx.AllName() {
		base = &ast.PropertyExpr{Expression: base, Property: v.visitName(n)}
	}
	return base
}

func (v *ASTBuilder) VisitAtom(ctx *cyphertree.AtomContext) interface{} {
	switch {
	case ctx.Literal() != nil:
		return ctx.Literal().Accept(v)
	case ctx.Parameter() != nil:
		return ctx.Parameter().Accept(v)
	case ctx.CaseExpression() != nil:
		return ctx.CaseExpression().Accept(v)
	case ctx.CountAll() != nil:
		return ctx.CountAll().Accept(v)
	case ctx.ListComprehension() != nil:
		return ctx.ListComprehension().Accept(v)
	case ctx.PatternComprehension() != nil:
		return ctx.PatternComprehension().Accept(v)
	case ctx.FilterWith() != nil:
		return ctx.FilterWith().Accept(v)
	case ctx.RelationshipsChainPattern() != nil:
		return v.visitRelationshipsChainPattern(ctx.RelationshipsChainPattern())
	case ctx.ParenthesizedExpression() != nil:
		return ctx.ParenthesizedExpression().Accept(v)
	case ctx.FunctionInvocation() != nil:
		return ctx.FunctionInvocation().Accept(v)
	case ctx.Symbol() != nil:
		return &ast.VariableExpr{Name: textOf(ctx.Symbol())}
	case ctx.SubqueryExist() != nil:
		return ctx.SubqueryExist().Accept(v)
	}
	return nil
}

func (v *ASTBuilder) visitRelationshipsChainPattern(ctx cyphertree.IRelationshipsChainPatternContext) *ast.PatternElement {
	rcp, _ := ctx.(*cyphertree.RelationshipsChainPatternContext)
	if rcp == nil {
		return nil
	}
	elem := &ast.PatternElement{}
	if np := rcp.NodePattern(); np != nil {
		elem.Nodes = append(elem.Nodes, v.visitNodePattern(np))
	}
	for _, pec := range rcp.AllPatternElemChain() {
		c := pec.Accept(v)
		if c != nil {
			chain := c.(*patternElemChainResult)
			elem.Rels = append(elem.Rels, chain.Rel)
			elem.Nodes = append(elem.Nodes, chain.Node)
		}
	}
	return elem
}

func (v *ASTBuilder) VisitParenthesizedExpression(ctx *cyphertree.ParenthesizedExpressionContext) interface{} {
	return v.visitExpr(ctx.Expression())
}

func (v *ASTBuilder) VisitFunctionInvocation(ctx *cyphertree.FunctionInvocationContext) interface{} {
	name := v.visitInvocationName(ctx.InvocationName())
	fe := &ast.FunctionExpr{Name: name}
	if ctx.DISTINCT() != nil {
		fe.Distinct = true
	}
	if ec := ctx.ExpressionChain(); ec != nil {
		fe.Args = v.visitExpressionChain(ec)
	}
	return fe
}

func (v *ASTBuilder) VisitCountAll(ctx *cyphertree.CountAllContext) interface{} {
	return &ast.CountAllExpr{}
}

func (v *ASTBuilder) VisitSubqueryExist(ctx *cyphertree.SubqueryExistContext) interface{} {
	ex := &ast.ExistsExpr{}
	if pw := ctx.PatternWhere(); pw != nil {
		patterns := v.visitPatternWhere(pw)
		if len(patterns) > 0 && len(patterns[0].Parts) > 0 {
			ex.Pattern = patterns[0].Parts[0].Element
		}
		if w := pw.Where(); w != nil {
			ex.Where = v.visitExpr(w)
		}
	}
	return ex
}

func (v *ASTBuilder) VisitFilterWith(ctx *cyphertree.FilterWithContext) interface{} {
	funcName := "single"
	if ctx.ALL() != nil {
		funcName = "all"
	} else if ctx.ANY() != nil {
		funcName = "any"
	} else if ctx.NONE() != nil {
		funcName = "none"
	}
	fe := &ast.FunctionExpr{Name: funcName}
	if fexpr := ctx.FilterExpression(); fexpr != nil {
		fe.Args = append(fe.Args, v.visitExpr(fexpr.Expression()))
		if w := fexpr.Where(); w != nil {
			fe.Args = append(fe.Args, v.visitExpr(w))
		}
	}
	return fe
}

func (v *ASTBuilder) VisitListComprehension(ctx *cyphertree.ListComprehensionContext) interface{} {
	lc := &ast.ListComprehensionExpr{}
	if fexpr := ctx.FilterExpression(); fexpr != nil {
		lc.Variable = textOf(fexpr.Symbol())
		lc.InExpr = v.visitExpr(fexpr.Expression())
		if w := fexpr.Where(); w != nil {
			lc.Where = v.visitExpr(w)
		}
	}
	for i := 0; i < ctx.GetChildCount(); i++ {
		if term, ok := ctx.GetChild(i).(antlr.TerminalNode); ok {
			if term.GetSymbol().GetTokenType() == cyphertree.CypherParserSTICK {
				if i+1 < ctx.GetChildCount() {
					if expr, ok := ctx.GetChild(i+1).(*cyphertree.ExpressionContext); ok {
						lc.Result = v.visitExpr(expr)
					}
				}
			}
		}
	}
	return lc
}

func (v *ASTBuilder) VisitPatternComprehension(ctx *cyphertree.PatternComprehensionContext) interface{} {
	pc := &ast.PatternComprehensionExpr{}
	if rcp := ctx.RelationshipsChainPattern(); rcp != nil {
		pc.Pattern = v.visitRelationshipsChainPattern(rcp)
	}
	if w := ctx.Where(); w != nil {
		pc.Where = v.visitExpr(w)
	}
	for i := 0; i < ctx.GetChildCount(); i++ {
		if term, ok := ctx.GetChild(i).(antlr.TerminalNode); ok {
			if term.GetSymbol().GetTokenType() == cyphertree.CypherParserSTICK {
				if i+1 < ctx.GetChildCount() {
					if expr, ok := ctx.GetChild(i+1).(*cyphertree.ExpressionContext); ok {
						pc.Result = v.visitExpr(expr)
					}
				}
			}
		}
	}
	return pc
}

// -------------------- Literals --------------------

func (v *ASTBuilder) VisitLiteral(ctx *cyphertree.LiteralContext) interface{} {
	switch {
	case ctx.BoolLit() != nil:
		return ctx.BoolLit().Accept(v)
	case ctx.NumLit() != nil:
		return ctx.NumLit().Accept(v)
	case ctx.NULL_W() != nil:
		return &ast.LiteralExpr{Value: nil}
	case ctx.StringLit() != nil:
		return ctx.StringLit().Accept(v)
	case ctx.CharLit() != nil:
		return ctx.CharLit().Accept(v)
	case ctx.ListLit() != nil:
		return ctx.ListLit().Accept(v)
	case ctx.MapLit() != nil:
		return ctx.MapLit().Accept(v)
	}
	return nil
}

func (v *ASTBuilder) VisitBoolLit(ctx *cyphertree.BoolLitContext) interface{} {
	return &ast.LiteralExpr{Value: ctx.TRUE() != nil}
}

func (v *ASTBuilder) VisitNumLit(ctx *cyphertree.NumLitContext) interface{} {
	text := ctx.GetText()
	if i, err := strconv.ParseInt(text, 10, 64); err == nil {
		return &ast.LiteralExpr{Value: i}
	}
	if f, err := strconv.ParseFloat(text, 64); err == nil {
		return &ast.LiteralExpr{Value: f}
	}
	return &ast.LiteralExpr{Value: text}
}

func (v *ASTBuilder) VisitStringLit(ctx *cyphertree.StringLitContext) interface{} {
	text := ctx.GetText()
	if len(text) >= 2 && (text[0] == '"' || text[0] == '\'') {
		text = text[1 : len(text)-1]
	}
	return &ast.LiteralExpr{Value: text}
}

func (v *ASTBuilder) VisitCharLit(ctx *cyphertree.CharLitContext) interface{} {
	return v.VisitStringLit(nil)
}

func (v *ASTBuilder) VisitListLit(ctx *cyphertree.ListLitContext) interface{} {
	ll := &ast.ListLiteralExpr{}
	if ec := ctx.ExpressionChain(); ec != nil {
		ll.Elements = v.visitExpressionChain(ec)
	}
	return ll
}

func (v *ASTBuilder) VisitMapLit(ctx *cyphertree.MapLitContext) interface{} {
	ml := &ast.MapLiteralExpr{Pairs: make(map[string]ast.Expression)}
	for _, mp := range ctx.AllMapPair() {
		if pair := mp.Accept(v); pair != nil {
			p := pair.(*mapPairResult)
			ml.Pairs[p.Key] = p.Value
		}
	}
	return ml
}

type mapPairResult struct {
	Key   string
	Value ast.Expression
}

func (v *ASTBuilder) VisitMapPair(ctx *cyphertree.MapPairContext) interface{} {
	return &mapPairResult{
		Key:   v.visitName(ctx.Name()),
		Value: v.visitExpr(ctx.Expression()),
	}
}

func (v *ASTBuilder) VisitParameter(ctx *cyphertree.ParameterContext) interface{} {
	if sym := ctx.Symbol(); sym != nil {
		return &ast.ParameterExpr{Name: textOf(sym)}
	}
	if n := ctx.NumLit(); n != nil {
		return &ast.ParameterExpr{Name: n.GetText()}
	}
	return &ast.ParameterExpr{Name: ctx.GetText()}
}

// -------------------- CASE Expression --------------------

func (v *ASTBuilder) VisitCaseExpression(ctx *cyphertree.CaseExpressionContext) interface{} {
	ce := &ast.CaseExpr{}
	exprs := ctx.AllExpression()
	if len(exprs) > 0 && ctx.WHEN(0) == nil {
		// CASE expr WHEN ...
		ce.Expression = v.visitExpr(exprs[0])
	}
	whenIdx := 0
	if ce.Expression != nil {
		whenIdx = 1
	}
	for i := 0; i < len(ctx.AllWHEN()); i++ {
		wt := &ast.WhenThen{
			When: v.visitExpr(exprs[whenIdx+i*2]),
			Then: v.visitExpr(exprs[whenIdx+i*2+1]),
		}
		ce.Whens = append(ce.Whens, wt)
	}
	if ctx.ELSE() != nil {
		ce.ElseExpr = v.visitExpr(exprs[len(exprs)-1])
	}
	return ce
}

// -------------------- Names & Symbols --------------------

func (v *ASTBuilder) visitName(ctx cyphertree.INameContext) string {
	if ctx == nil {
		return ""
	}
	if sym := ctx.Symbol(); sym != nil {
		return textOf(sym)
	}
	if rw := ctx.ReservedWord(); rw != nil {
		return strings.ToLower(textOf(rw))
	}
	return textOf(ctx)
}

func (v *ASTBuilder) visitInvocationName(ctx cyphertree.IInvocationNameContext) string {
	if ctx == nil {
		return ""
	}
	var parts []string
	for _, sym := range ctx.AllSymbol() {
		parts = append(parts, textOf(sym))
	}
	return strings.Join(parts, ".")
}

func (v *ASTBuilder) visitExpressionChain(ctx cyphertree.IExpressionChainContext) []ast.Expression {
	if ctx == nil {
		return nil
	}
	ec, _ := ctx.(*cyphertree.ExpressionChainContext)
	if ec == nil {
		return nil
	}
	var exprs []ast.Expression
	for _, e := range ec.AllExpression() {
		if expr := v.visitExpr(e); expr != nil {
			exprs = append(exprs, expr)
		}
	}
	return exprs
}
