package ast

// ============================================================
// Pattern AST 节点
// ============================================================
// Cypher 的图模式由 Pattern -> PatternPart -> PatternElement 组成。
// PatternElement 是交替的 NodePattern 和 RelPattern 序列。
// ============================================================

// Pattern: patternPart (COMMA patternPart)*
type Pattern struct {
	Parts []*PatternPart
}

// PatternPart: (symbol ASSIGN)? patternElem
type PatternPart struct {
	Variable string // 可选的路径变量名，如 p = (a)-[]->(b)
	Element  *PatternElement
}

// PatternElement 是路径的核心：节点和关系交替出现。
// 例如：(a)-[:KNOWS]->(b)<-[:WORKS_WITH]-(c)
// Nodes: [a, b, c]
// Rels:  [KNOWS->, <-WORKS_WITH]
type PatternElement struct {
	Nodes []*NodePattern
	Rels  []*RelPattern
}

// NodePattern: LPAREN symbol? nodeLabels? properties? RPAREN
type NodePattern struct {
	Variable   string
	Labels     []string
	Properties Expression // MapLiteral or Parameter
}

// RelDirection 表示关系方向。
type RelDirection int

const (
	DirRight RelDirection = iota // ->
	DirLeft                      // <-
	DirBoth                      // -
)

// RelPattern: relationshipPattern
type RelPattern struct {
	Direction  RelDirection
	Variable   string
	Types      []string
	Properties Expression // MapLiteral or Parameter
	Range      *RangeLiteral
}

// RangeLiteral: MULT numLit? (RANGE numLit?)?
// 例如：*1..5 表示长度1到5的路径。
type RangeLiteral struct {
	Min *int // nil 表示无下限
	Max *int // nil 表示无上限
}
