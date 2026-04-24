package ast

// ============================================================
// Expression AST 节点（完整版）
// ============================================================
// 覆盖 Cypher grammar 中所有表达式类型：
//   - 二元运算（OR, XOR, AND, +, -, *, /, %, ^, 比较运算符）
//   - 一元运算（NOT, +, -）
//   - 属性访问和标签断言（a.name, a:Person）
//   - 函数调用（count(*), sum(x)）
//   - 字面量（整数、字符串、布尔、NULL、列表、Map）
//   - 参数（$param）
//   - CASE 表达式
//   - 模式表达式（EXISTS, 列表推导式, 模式推导式）
//   - IN 表达式、IS NULL 表达式、字符串匹配表达式
// ============================================================

type Expression interface {
	exprNode()
}

func (*VariableExpr) exprNode()               {}
func (*PropertyExpr) exprNode()               {}
func (*LabelExpr) exprNode()                  {}
func (*BinaryExpr) exprNode()                 {}
func (*UnaryExpr) exprNode()                  {}
func (*FunctionExpr) exprNode()               {}
func (*CountAllExpr) exprNode()               {}
func (*LiteralExpr) exprNode()                {}
func (*ListLiteralExpr) exprNode()            {}
func (*MapLiteralExpr) exprNode()             {}
func (*ParameterExpr) exprNode()              {}
func (*CaseExpr) exprNode()                   {}
func (*ListComprehensionExpr) exprNode()      {}
func (*PatternComprehensionExpr) exprNode()    {}
func (*ExistsExpr) exprNode()                 {}
func (*InExpr) exprNode()                     {}
func (*IsNullExpr) exprNode()                 {}
func (*StringMatchExpr) exprNode()            {}
func (*PatternElement) exprNode()             {}

// VariableExpr: 变量引用，如 a, b, friend_count
type VariableExpr struct {
	Name string
}

// PropertyExpr: 属性访问，如 a.age, b.name
// 支持链式访问：a.foo.bar（虽然 Cypher 中不常见）
type PropertyExpr struct {
	Expression Expression // 基础表达式
	Property   string
}

// LabelExpr: 标签断言，如 a:Person:Actor
// 在 Cypher grammar 中，标签断言和属性访问在同一层级。
type LabelExpr struct {
	Expression Expression
	Labels     []string
}

// BinaryExpr: 二元运算表达式。
// Op 包括：OR, XOR, AND, =, <>, <, >, <=, >=, +, -, *, /, %, ^, STARTS WITH, ENDS WITH, CONTAINS
type BinaryExpr struct {
	Op    string
	Left  Expression
	Right Expression
}

// UnaryExpr: 一元运算表达式。
// Op 包括：NOT, +, -
type UnaryExpr struct {
	Op   string
	Expr Expression
}

// FunctionExpr: 函数调用，如 sum(a.age), collect(b.name)
type FunctionExpr struct {
	Name     string
	Distinct bool
	Args     []Expression
}

// CountAllExpr: count(*) 特殊表达式。
type CountAllExpr struct{}

// LiteralExpr: 标量字面量。
// Value 可以是：int64, float64, string, bool, nil
type LiteralExpr struct {
	Value interface{}
}

// ListLiteralExpr: 列表字面量，如 [1, 2, 3]
type ListLiteralExpr struct {
	Elements []Expression
}

// MapLiteralExpr: Map 字面量，如 {name: 'Tom', age: 25}
type MapLiteralExpr struct {
	Pairs map[string]Expression
}

// ParameterExpr: 参数引用，如 $param, $1
type ParameterExpr struct {
	Name string
}

// CaseExpr: CASE 表达式。
type CaseExpr struct {
	Expression Expression   // 可选的测试表达式
	Whens      []*WhenThen
	ElseExpr   Expression // 可选
}

type WhenThen struct {
	When Expression
	Then Expression
}

// ListComprehensionExpr: 列表推导式，如 [x IN list WHERE x > 0 | x * 2]
type ListComprehensionExpr struct {
	Variable string
	InExpr   Expression
	Where    Expression // 可选
	Result   Expression // 可选（| 后面的表达式）
}

// PatternComprehensionExpr: 模式推导式，如 [(a)-[:KNOWS]->(b) WHERE a.age > 20 | b.name]
type PatternComprehensionExpr struct {
	Pattern *PatternElement
	Where   Expression // 可选
	Result  Expression
}

// ExistsExpr: EXISTS { pattern } 或 EXISTS { pattern WHERE ... }
type ExistsExpr struct {
	Pattern *PatternElement
	Where   Expression // 可选
}

// InExpr: IN 表达式，如 x IN [1, 2, 3]
type InExpr struct {
	Left  Expression
	Right Expression
}

// IsNullExpr: IS [NOT] NULL 表达式。
type IsNullExpr struct {
	Expr Expression
	Not  bool
}

// StringMatchExpr: STARTS WITH / ENDS WITH / CONTAINS 表达式。
type StringMatchExpr struct {
	Op     string // "STARTS WITH", "ENDS WITH", "CONTAINS"
	Left   Expression
	Right  Expression
}
