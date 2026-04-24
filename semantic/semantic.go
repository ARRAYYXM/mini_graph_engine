package semantic

import (
	"fmt"
	"mini_graph_engine/ast"
)

// ============================================================
// Semantic Analyzer（完整版）
// ============================================================
// 基于 ANTLR 生成的 Parser 构建的完整 AST 进行语义分析。
// 覆盖 Cypher 的核心语义检查：
//   1. 变量声明与解析
//   2. 作用域管理（WITH 是作用域边界）
//   3. 聚合上下文检查
//   4. 类型推断
//   5. 属性访问合法性检查
// ============================================================

// -------------------- 1. 符号与类型系统 --------------------

type SymbolKind int

const (
	KindUnknown   SymbolKind = iota
	KindNode
	KindEdge
	KindScalar
	KindAggregate
	KindList
	KindMap
	KindPattern
)

func (k SymbolKind) String() string {
	switch k {
	case KindNode:
		return "Node"
	case KindEdge:
		return "Edge"
	case KindScalar:
		return "Scalar"
	case KindAggregate:
		return "Aggregate"
	case KindList:
		return "List"
	case KindMap:
		return "Map"
	case KindPattern:
		return "Pattern"
	default:
		return "Unknown"
	}
}

type Symbol struct {
	Name       string
	Kind       SymbolKind
	Introduced ast.Clause
	IsVisible  bool
}

// SymbolTable 是 Semantic Analysis 的显式产物，供外部（如 Planner）查询。
// 它记录了查询中所有变量的声明信息，以及每个 clause 执行后暴露的变量集合。
type SymbolTable struct {
	// All 是所有在查询中声明过的变量的全局索引。
	// 即使某个变量在后续 clause 中不可见，它仍然在这里有记录。
	All map[string]*Symbol

	// ClauseResult 记录每个 clause 执行后"暴露给下游"的变量集合。
	// Planner 用这个信息来决定算子之间的数据流（pipeline）。
	// 例如：
	//   MatchClause -> {a: Node, b: Node}
	//   WithClause  -> {b: Node, friend_count: Aggregate}
	ClauseResult map[ast.Clause]map[string]*Symbol
}

func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		All:          make(map[string]*Symbol),
		ClauseResult: make(map[ast.Clause]map[string]*Symbol),
	}
}

// Lookup 按变量名查找符号。
func (st *SymbolTable) Lookup(name string) *Symbol {
	return st.All[name]
}

// ResultOf 获取某个 clause 执行后暴露的变量集合。
func (st *SymbolTable) ResultOf(clause ast.Clause) map[string]*Symbol {
	return st.ClauseResult[clause]
}

// ListByKind 按类型列出所有符号。
func (st *SymbolTable) ListByKind(kind SymbolKind) []*Symbol {
	var result []*Symbol
	for _, sym := range st.All {
		if sym.Kind == kind {
			result = append(result, sym)
		}
	}
	return result
}

// -------------------- 2. 作用域 --------------------
//
// Scope 是 SymbolTable 的"运行时"实现。
// 在分析过程中，Scope 负责管理变量在当前作用域中的可见性。
// 分析完成后，所有符号信息被汇总到 SymbolTable 中，供后续阶段使用。

type Scope struct {
	Parent  *Scope
	Symbols map[string]*Symbol
}

func NewScope(parent *Scope) *Scope {
	return &Scope{Parent: parent, Symbols: make(map[string]*Symbol)}
}

func (s *Scope) Declare(name string, kind SymbolKind, at ast.Clause) error {
	if _, exists := s.Symbols[name]; exists {
		return fmt.Errorf("variable '%s' already declared in current scope", name)
	}
	s.Symbols[name] = &Symbol{Name: name, Kind: kind, Introduced: at, IsVisible: true}
	return nil
}

func (s *Scope) Resolve(name string) *Symbol {
	if sym, ok := s.Symbols[name]; ok {
		return sym
	}
	if s.Parent != nil {
		return s.Parent.Resolve(name)
	}
	return nil
}

// -------------------- 3. 语义错误 --------------------

type SemanticError struct {
	Message string
	Clause  ast.Clause
}

func (e SemanticError) Error() string {
	return e.Message
}

// -------------------- 4. 分析器主体 --------------------

type SemanticAnalyzer struct {
	currentScope *Scope
	scopes       []*Scope
	errors       []SemanticError

	// SymbolTable 是 Semantic Analysis 的显式产物。
	// 分析完成后，外部（如 Planner）可以直接读取这个表。
	SymbolTable *SymbolTable
}

func NewAnalyzer() *SemanticAnalyzer {
	root := NewScope(nil)
	return &SemanticAnalyzer{
		currentScope: root,
		scopes:       []*Scope{root},
		errors:       []SemanticError{},
		SymbolTable:  NewSymbolTable(),
	}
}

func (sa *SemanticAnalyzer) Analyze(q *ast.Query) []SemanticError {
	for _, part := range q.Parts {
		for _, clause := range part.Clauses {
			sa.analyzeClause(clause)
		}
	}
	return sa.errors
}

// declare 封装了 Scope.Declare，同时把符号记录到全局 SymbolTable。
// SymbolTable.All 只记录变量的"首次声明"位置。如果同一个变量名在后续 clause
// 中重新声明（如 WITH 投影中的 b），不会覆盖首次声明的记录。
func (sa *SemanticAnalyzer) declare(name string, kind SymbolKind, at ast.Clause) error {
	err := sa.currentScope.Declare(name, kind, at)
	if err == nil {
		// 只记录首次声明，不覆盖（WITH 中的投影变量只是传递，不是重新定义）
		if _, exists := sa.SymbolTable.All[name]; !exists {
			sa.SymbolTable.All[name] = sa.currentScope.Resolve(name)
		}
	}
	return err
}

func (sa *SemanticAnalyzer) pushScope() {
	newScope := NewScope(sa.currentScope)
	sa.scopes = append(sa.scopes, newScope)
	sa.currentScope = newScope
}

func (sa *SemanticAnalyzer) popScope() {
	if len(sa.scopes) <= 1 {
		return
	}
	sa.scopes = sa.scopes[:len(sa.scopes)-1]
	sa.currentScope = sa.scopes[len(sa.scopes)-1]
}

func (sa *SemanticAnalyzer) addError(msg string, clause ast.Clause) {
	sa.errors = append(sa.errors, SemanticError{Message: msg, Clause: clause})
}

// -------------------- 5. Clause 分析 --------------------

func (sa *SemanticAnalyzer) analyzeClause(c ast.Clause) {
	switch cl := c.(type) {
	case *ast.MatchClause:
		sa.analyzeMatch(cl)
	case *ast.WithClause:
		sa.analyzeWith(cl)
	case *ast.ReturnClause:
		sa.analyzeReturn(cl)
	case *ast.CreateClause:
		sa.analyzeCreate(cl)
	case *ast.UnwindClause:
		sa.analyzeUnwind(cl)
	case *ast.DeleteClause:
		sa.analyzeDelete(cl)
	case *ast.SetClause:
		sa.analyzeSet(cl)
	case *ast.RemoveClause:
		sa.analyzeRemove(cl)
	case *ast.MergeClause:
		sa.analyzeMerge(cl)
	}
}

func (sa *SemanticAnalyzer) analyzeMatch(mc *ast.MatchClause) {
	for _, pat := range mc.Patterns {
		sa.analyzePattern(pat, mc)
	}
	if mc.Where != nil {
		sa.analyzeExpression(mc.Where, mc)
	}
	sa.recordResult(mc)
}

func (sa *SemanticAnalyzer) analyzeCreate(cc *ast.CreateClause) {
	for _, pat := range cc.Patterns {
		sa.analyzePattern(pat, cc)
	}
	sa.recordResult(cc)
}

func (sa *SemanticAnalyzer) analyzeUnwind(uc *ast.UnwindClause) {
	sa.analyzeExpression(uc.Expression, uc)
	if err := sa.declare(uc.Alias, KindList, uc); err != nil {
		sa.addError(err.Error(), uc)
	}
	sa.recordResult(uc)
}

func (sa *SemanticAnalyzer) analyzeDelete(dc *ast.DeleteClause) {
	for _, expr := range dc.Exprs {
		sa.analyzeExpression(expr, dc)
	}
}

func (sa *SemanticAnalyzer) analyzeSet(sc *ast.SetClause) {
	for _, item := range sc.Items {
		sa.analyzeExpression(item.Target, sc)
		sa.analyzeExpression(item.Value, sc)
	}
}

func (sa *SemanticAnalyzer) analyzeRemove(rc *ast.RemoveClause) {
	for _, item := range rc.Items {
		sa.analyzeExpression(item.Target, rc)
	}
}

func (sa *SemanticAnalyzer) analyzeMerge(mc *ast.MergeClause) {
	if mc.Pattern != nil && mc.Pattern.Element != nil {
		sa.analyzePatternElement(mc.Pattern.Element, mc)
	}
	for _, action := range mc.Actions {
		for _, item := range action.SetItems {
			sa.analyzeExpression(item.Target, mc)
			sa.analyzeExpression(item.Value, mc)
		}
	}
}

func (sa *SemanticAnalyzer) analyzePattern(pat *ast.Pattern, clause ast.Clause) {
	for _, part := range pat.Parts {
		if part.Element != nil {
			sa.analyzePatternElement(part.Element, clause)
		}
	}
}

func (sa *SemanticAnalyzer) analyzePatternElement(elem *ast.PatternElement, clause ast.Clause) {
	for i, node := range elem.Nodes {
		if node.Variable != "" {
			if err := sa.declare(node.Variable, KindNode, clause); err != nil {
				sa.addError(err.Error(), clause)
			}
		}
		if i < len(elem.Rels) {
			rel := elem.Rels[i]
			if rel.Variable != "" {
				if err := sa.declare(rel.Variable, KindEdge, clause); err != nil {
					sa.addError(err.Error(), clause)
				}
			}
		}
	}
}

func (sa *SemanticAnalyzer) analyzeWith(wc *ast.WithClause) {
	// 聚合检查
	aggSeen := false
	for _, proj := range wc.Projections {
		if sa.hasAggregation(proj.Expression) {
			aggSeen = true
		}
	}
	if aggSeen {
		for _, proj := range wc.Projections {
			if !sa.hasAggregation(proj.Expression) {
				if _, ok := proj.Expression.(*ast.VariableExpr); !ok {
					sa.addError("non-aggregated expression must be a grouping key in WITH", wc)
				}
			}
		}
	}

	// 在旧作用域中分析投影表达式并推断类型
	type projMeta struct {
		name string
		kind SymbolKind
	}
	var metas []projMeta
	for _, proj := range wc.Projections {
		sa.analyzeExpression(proj.Expression, wc)
		varName := proj.Alias
		if varName == "" {
			varName = sa.defaultName(proj.Expression)
		}
		kind := sa.inferType(proj.Expression)
		metas = append(metas, projMeta{name: varName, kind: kind})
	}

	// 创建隔离的新作用域
	isolatedScope := NewScope(nil)
	sa.scopes = append(sa.scopes, isolatedScope)
	sa.currentScope = isolatedScope

	for _, meta := range metas {
		if meta.name == "" {
			sa.addError("projection must have an alias or be a simple variable", wc)
			continue
		}
		if err := sa.declare(meta.name, meta.kind, wc); err != nil {
			sa.addError(err.Error(), wc)
		}
	}

	if wc.Where != nil {
		sa.analyzeExpression(wc.Where, wc)
	}

	sa.recordResult(wc)
}

func (sa *SemanticAnalyzer) analyzeReturn(rc *ast.ReturnClause) {
	for _, proj := range rc.Projections {
		sa.analyzeExpression(proj.Expression, rc)
	}
	for _, ob := range rc.OrderBy {
		sa.analyzeExpression(ob.Expression, rc)
	}
	if rc.Skip != nil {
		sa.analyzeExpression(rc.Skip, rc)
	}
	if rc.Limit != nil {
		sa.analyzeExpression(rc.Limit, rc)
	}
	sa.recordResult(rc)
}

// -------------------- 6. 表达式分析 --------------------

func (sa *SemanticAnalyzer) analyzeExpression(expr ast.Expression, clause ast.Clause) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.VariableExpr:
		if sym := sa.currentScope.Resolve(e.Name); sym == nil {
			sa.addError(fmt.Sprintf("undefined variable '%s'", e.Name), clause)
		}
	case *ast.PropertyExpr:
		sa.analyzeExpression(e.Expression, clause)
		if base := sa.inferType(e.Expression); base != KindNode && base != KindEdge && base != KindMap {
			// 属性访问允许在 Map 上（如 {a:1}.a）
			if base != KindUnknown {
				sa.addError(fmt.Sprintf("cannot access property on variable of kind %s", base), clause)
			}
		}
	case *ast.LabelExpr:
		sa.analyzeExpression(e.Expression, clause)
		if base := sa.inferType(e.Expression); base != KindNode && base != KindEdge {
			if base != KindUnknown {
				sa.addError(fmt.Sprintf("cannot apply label filter on variable of kind %s", base), clause)
			}
		}
	case *ast.BinaryExpr:
		sa.analyzeExpression(e.Left, clause)
		sa.analyzeExpression(e.Right, clause)
	case *ast.UnaryExpr:
		sa.analyzeExpression(e.Expr, clause)
	case *ast.FunctionExpr:
		for _, arg := range e.Args {
			sa.analyzeExpression(arg, clause)
		}
	case *ast.CountAllExpr:
		// 合法，无需检查
	case *ast.LiteralExpr:
		// 合法
	case *ast.ListLiteralExpr:
		for _, elem := range e.Elements {
			sa.analyzeExpression(elem, clause)
		}
	case *ast.MapLiteralExpr:
		for _, val := range e.Pairs {
			sa.analyzeExpression(val, clause)
		}
	case *ast.ParameterExpr:
		// 合法
	case *ast.CaseExpr:
		if e.Expression != nil {
			sa.analyzeExpression(e.Expression, clause)
		}
		for _, wt := range e.Whens {
			sa.analyzeExpression(wt.When, clause)
			sa.analyzeExpression(wt.Then, clause)
		}
		if e.ElseExpr != nil {
			sa.analyzeExpression(e.ElseExpr, clause)
		}
	case *ast.ListComprehensionExpr:
		if err := sa.declare(e.Variable, KindScalar, clause); err != nil {
			sa.addError(err.Error(), clause)
		}
		sa.analyzeExpression(e.InExpr, clause)
		if e.Where != nil {
			sa.analyzeExpression(e.Where, clause)
		}
		if e.Result != nil {
			sa.analyzeExpression(e.Result, clause)
		}
	case *ast.PatternComprehensionExpr:
		if e.Pattern != nil {
			sa.analyzePatternElement(e.Pattern, clause)
		}
		if e.Where != nil {
			sa.analyzeExpression(e.Where, clause)
		}
		sa.analyzeExpression(e.Result, clause)
	case *ast.ExistsExpr:
		if e.Pattern != nil {
			sa.analyzePatternElement(e.Pattern, clause)
		}
		if e.Where != nil {
			sa.analyzeExpression(e.Where, clause)
		}
	case *ast.InExpr:
		sa.analyzeExpression(e.Left, clause)
		sa.analyzeExpression(e.Right, clause)
	case *ast.IsNullExpr:
		sa.analyzeExpression(e.Expr, clause)
	case *ast.StringMatchExpr:
		sa.analyzeExpression(e.Left, clause)
		sa.analyzeExpression(e.Right, clause)
	case *ast.PatternElement:
		sa.analyzePatternElement(e, clause)
	}
}

// -------------------- 7. 辅助方法 --------------------

func (sa *SemanticAnalyzer) hasAggregation(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.FunctionExpr:
		// 简化：假设所有函数都是聚合
		// 生产环境需要维护白名单
		lower := e.Name
		if lower == "count" || lower == "sum" || lower == "avg" || lower == "min" || lower == "max" || lower == "collect" {
			return true
		}
		return false
	case *ast.BinaryExpr:
		return sa.hasAggregation(e.Left) || sa.hasAggregation(e.Right)
	case *ast.UnaryExpr:
		return sa.hasAggregation(e.Expr)
	case *ast.CountAllExpr:
		return true
	default:
		return false
	}
}

func (sa *SemanticAnalyzer) inferType(expr ast.Expression) SymbolKind {
	if expr == nil {
		return KindUnknown
	}
	switch e := expr.(type) {
	case *ast.VariableExpr:
		if sym := sa.currentScope.Resolve(e.Name); sym != nil {
			return sym.Kind
		}
		return KindUnknown
	case *ast.PropertyExpr:
		return KindScalar
	case *ast.LabelExpr:
		return KindScalar // 布尔结果
	case *ast.FunctionExpr:
		if e.Name == "count" || e.Name == "sum" || e.Name == "avg" || e.Name == "min" || e.Name == "max" || e.Name == "collect" {
			return KindAggregate
		}
		return KindScalar
	case *ast.CountAllExpr:
		return KindAggregate
	case *ast.LiteralExpr:
		switch e.Value.(type) {
		case []interface{}:
			return KindList
		case map[string]interface{}:
			return KindMap
		default:
			return KindScalar
		}
	case *ast.ListLiteralExpr:
		return KindList
	case *ast.MapLiteralExpr:
		return KindMap
	case *ast.ParameterExpr:
		return KindUnknown
	case *ast.BinaryExpr:
		return KindScalar
	case *ast.UnaryExpr:
		return KindScalar
	case *ast.InExpr:
		return KindScalar // 布尔
	case *ast.IsNullExpr:
		return KindScalar // 布尔
	case *ast.StringMatchExpr:
		return KindScalar // 布尔
	case *ast.CaseExpr:
		return KindScalar
	case *ast.ExistsExpr:
		return KindScalar // 布尔
	case *ast.ListComprehensionExpr:
		return KindList
	case *ast.PatternComprehensionExpr:
		return KindList
	}
	return KindUnknown
}

func (sa *SemanticAnalyzer) defaultName(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.VariableExpr:
		return e.Name
	case *ast.PropertyExpr:
		return sa.defaultName(e.Expression) + "." + e.Property
	case *ast.FunctionExpr:
		return e.Name + "(...)"
	case *ast.CountAllExpr:
		return "count(*)"
	}
	return ""
}

func (sa *SemanticAnalyzer) recordResult(clause ast.Clause) {
	exported := make(map[string]*Symbol)
	for name, sym := range sa.currentScope.Symbols {
		exported[name] = sym
	}
	sa.SymbolTable.ClauseResult[clause] = exported
}

// -------------------- 8. 结果打印 --------------------

func (sa *SemanticAnalyzer) PrintResults() {
	fmt.Println("\n========== Semantic Analysis Results ==========")
	if len(sa.errors) == 0 {
		fmt.Println("✅ No semantic errors found.")
	} else {
		fmt.Printf("❌ Found %d semantic error(s):\n", len(sa.errors))
		for _, err := range sa.errors {
			fmt.Printf("   - %s\n", err.Message)
		}
	}

	fmt.Println("\n--- SymbolTable.All (all declared variables) ---")
	if len(sa.SymbolTable.All) == 0 {
		fmt.Println("  (empty)")
	}
	for name, sym := range sa.SymbolTable.All {
		fmt.Printf("   %-15s -> %s (introduced in %s)\n", name, sym.Kind, clauseName(sym.Introduced))
	}

	fmt.Println("\n--- SymbolTable.ClauseResult (exported symbols per clause) ---")
	for clause, syms := range sa.SymbolTable.ClauseResult {
		var name string
		switch clause.(type) {
		case *ast.MatchClause:
			name = "MATCH"
		case *ast.WithClause:
			name = "WITH"
		case *ast.ReturnClause:
			name = "RETURN"
		case *ast.CreateClause:
			name = "CREATE"
		case *ast.UnwindClause:
			name = "UNWIND"
		case *ast.DeleteClause:
			name = "DELETE"
		case *ast.SetClause:
			name = "SET"
		case *ast.RemoveClause:
			name = "REMOVE"
		case *ast.MergeClause:
			name = "MERGE"
		}
		fmt.Printf("\n[%s] exposes:\n", name)
		for n, sym := range syms {
			fmt.Printf("   %-15s -> %s\n", n, sym.Kind)
		}
	}

	fmt.Println("\n--- Final Scope Chain (from root to current) ---")
	var chain []*Scope
	for s := sa.currentScope; s != nil; s = s.Parent {
		chain = append(chain, s)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		printScope(chain[i], len(chain)-1-i)
	}
}

func clauseName(c ast.Clause) string {
	switch c.(type) {
	case *ast.MatchClause:
		return "MATCH"
	case *ast.WithClause:
		return "WITH"
	case *ast.ReturnClause:
		return "RETURN"
	case *ast.CreateClause:
		return "CREATE"
	case *ast.UnwindClause:
		return "UNWIND"
	case *ast.DeleteClause:
		return "DELETE"
	case *ast.SetClause:
		return "SET"
	case *ast.RemoveClause:
		return "REMOVE"
	case *ast.MergeClause:
		return "MERGE"
	default:
		return "?"
	}
}

func printScope(s *Scope, depth int) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	fmt.Printf("%sScope (level %d):\n", indent, depth)
	if len(s.Symbols) == 0 {
		fmt.Printf("%s  (empty)\n", indent)
	}
	for name, sym := range s.Symbols {
		fmt.Printf("%s  %s: %s\n", indent, name, sym.Kind)
	}
}
