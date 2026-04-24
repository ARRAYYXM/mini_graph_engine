package ast

// ============================================================
// Cypher AST 定义（适配 ANTLR grammars-v4/cypher）
// ============================================================
// 这个 AST 设计覆盖了 Cypher 的核心语法结构，包括：
//   - Query / SingleQuery / Union
//   - MATCH / WITH / RETURN / CREATE / UNWIND / DELETE / SET / REMOVE / MERGE
//   - Pattern（节点、关系、路径）
//   - 完整的 Expression 体系（二元运算、函数调用、字面量、属性访问等）
// ============================================================

// Statement 是所有 Cypher 语句的接口。
type Statement interface {
	stmtNode()
}

func (*Query) stmtNode() {}

// Query 是 Cypher 查询的根节点，支持 UNION。
type Query struct {
	Parts []*SingleQuery // 每个部分是一个 SingleQuery，多个部分用 UNION 连接
}

// SingleQuery 是一个不包含 UNION 的查询单元。
type SingleQuery struct {
	Clauses []Clause
}

// Clause 是所有查询子句的接口。
type Clause interface {
	clauseNode()
}

func (*MatchClause) clauseNode()   {}
func (*WithClause) clauseNode()    {}
func (*ReturnClause) clauseNode()  {}
func (*CreateClause) clauseNode()  {}
func (*UnwindClause) clauseNode()  {}
func (*DeleteClause) clauseNode()  {}
func (*SetClause) clauseNode()     {}
func (*RemoveClause) clauseNode()  {}
func (*MergeClause) clauseNode()   {}

// MatchClause: OPTIONAL? MATCH pattern WHERE?
type MatchClause struct {
	Optional bool
	Patterns []*Pattern
	Where    Expression
}

// WithClause: WITH projectionBody WHERE?
type WithClause struct {
	Distinct    bool
	Projections []*ProjectionItem
	OrderBy     []*OrderItem
	Skip        Expression
	Limit       Expression
	Where       Expression
}

// ReturnClause: RETURN projectionBody
type ReturnClause struct {
	Distinct    bool
	Projections []*ProjectionItem
	OrderBy     []*OrderItem
	Skip        Expression
	Limit       Expression
}

// CreateClause: CREATE pattern
type CreateClause struct {
	Patterns []*Pattern
}

// UnwindClause: UNWIND expression AS symbol
type UnwindClause struct {
	Expression Expression
	Alias      string
}

// DeleteClause: DETACH? DELETE expressionChain
type DeleteClause struct {
	Detach bool
	Exprs  []Expression
}

// SetClause: SET setItem (COMMA setItem)*
type SetClause struct {
	Items []*SetItem
}

type SetItem struct {
	Target     Expression // Variable or PropertyExpression
	Operator   string     // "=" or "+="
	Value      Expression
	NodeLabels []string   // 仅用于 SET n:Label
}

// RemoveClause: REMOVE removeItem (COMMA removeItem)*
type RemoveClause struct {
	Items []*RemoveItem
}

type RemoveItem struct {
	Target     Expression // Variable or PropertyExpression
	NodeLabels []string
}

// MergeClause: MERGE patternPart mergeAction*
type MergeClause struct {
	Pattern *PatternPart
	Actions []*MergeAction
}

type MergeAction struct {
	OnCreate bool       // true = ON CREATE, false = ON MATCH
	SetItems []*SetItem
}

// ProjectionItem: expression (AS symbol)?
type ProjectionItem struct {
	Expression Expression
	Alias      string // 空字符串表示没有显式别名
}

// OrderItem: expression (ASCENDING | ASC | DESCENDING | DESC)?
type OrderItem struct {
	Expression Expression
	Descending bool
}
