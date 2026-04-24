package main

import (
	"fmt"
	"mini_graph_engine/ast"
	"mini_graph_engine/parser"
	"mini_graph_engine/semantic"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     Cypher 查询引擎 —— ANTLR Parser + Semantic Analysis            ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	examples := []struct {
		name  string
		query string
	}{
		{
			name: "【示例 1】完整复杂查询",
			query: `MATCH (a:Person)-[:KNOWS]->(b:Person)
WHERE a.age > 25
WITH b, count(*) AS friend_count
WHERE friend_count > 2
RETURN b.name, friend_count
ORDER BY friend_count DESC`,
		},
		{
			name: "【示例 2】引用未定义变量",
			query: `MATCH (a:Person)-[:KNOWS]->(b:Person)
WHERE x.age > 25
RETURN b.name`,
		},
		{
			name: "【示例 3】WITH 后访问已消失的变量",
			query: `MATCH (a:Person)-[:KNOWS]->(b:Person)
WITH b
RETURN a.name`,
		},
		{
			name: "【示例 4】CREATE + SET + RETURN",
			query: `CREATE (n:Person {name: "Alice", age: 30})
SET n.city = "Beijing"
RETURN n.name, n.age, n.city`,
		},
		{
			name: "【示例 5】UNWIND + 聚合",
			query: `UNWIND [1, 2, 3] AS num
MATCH (n:Person)
WHERE n.age > num
RETURN num, count(n) AS cnt`,
		},
		{
			name: "【示例 6】属性访问在非图变量上",
			query: `MATCH (a:Person)-[:KNOWS]->(b:Person)
WITH b, count(*) AS friend_count
RETURN friend_count.name`,
		},
	}

	for _, ex := range examples {
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println(ex.name)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		runDemo(ex.query)
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("【项目说明】")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(`
项目结构：
  cyphertree/           ← ANTLR4 自动生成的 Parser/Lexer/Visitor
  ast/                  ← 自定义 AST 定义
  parser/
    antlr_parser.go     ← 调用 ANTLR 解析字符串
    ast_builder.go      ← 实现 Visitor，将 ParseTree 转为 AST
  semantic/
    semantic.go         ← Semantic Analyzer 核心
  main.go               ← 演示入口

Parser 生成命令：
  java -jar antlr-4.13.2-complete.jar \
    -Dlanguage=Go -visitor -package cyphertree \
    CypherLexer.g4 CypherParser.g4

Semantic Analysis 核心产出：
  • SymbolTable      — 每个变量的类型和声明位置
  • ScopeChain       — 每个 clause 的可见变量集合
  • AggregationInfo  — 分组键 vs 聚合值
  • TypeMap          — 每个表达式的逻辑类型
  • Validation       — 语义合法性检查
`)
}

func runDemo(queryStr string) {
	fmt.Println("Query:")
	fmt.Println("  " + queryStr)
	fmt.Println()

	// Step 1: Parsing with ANTLR
	fmt.Println("── Step 1: ANTLR Parsing ─────────────────────────────────────────────")
	q, err := parser.Build(queryStr)
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}
	printAST(q)
	fmt.Println()

	// Step 2: Semantic Analysis
	fmt.Println("── Step 2: Semantic Analysis ─────────────────────────────────────────")
	analyzer := semantic.NewAnalyzer()
	errors := analyzer.Analyze(q)
	if len(errors) == 0 {
		fmt.Println("✅ No semantic errors.")
	} else {
		fmt.Printf("❌ Found %d error(s):\n", len(errors))
		for _, e := range errors {
			fmt.Printf("   • %s\n", e.Message)
		}
	}
	fmt.Println()

	// Step 3: Results
	fmt.Println("── Step 3: Analysis Results ──────────────────────────────────────────")
	analyzer.PrintResults()
}

func printAST(q *ast.Query) {
	for pi, part := range q.Parts {
		if len(q.Parts) > 1 {
			fmt.Printf("  Union Part %d:\n", pi)
		}
		for ci, clause := range part.Clauses {
			switch c := clause.(type) {
			case *ast.MatchClause:
				fmt.Printf("  [%d] MATCH", ci)
				if c.Optional {
					fmt.Print(" OPTIONAL")
				}
				fmt.Println()
				for _, pat := range c.Patterns {
					for _, pp := range pat.Parts {
						printPatternPart(pp)
					}
				}
				if c.Where != nil {
					fmt.Printf("      WHERE: %s\n", exprToString(c.Where))
				}
			case *ast.WithClause:
				fmt.Printf("  [%d] WITH", ci)
				if c.Distinct {
					fmt.Print(" DISTINCT")
				}
				fmt.Println()
				for _, p := range c.Projections {
					alias := p.Alias
					if alias == "" {
						alias = "<no alias>"
					}
					fmt.Printf("      %s AS %s\n", exprToString(p.Expression), alias)
				}
				if c.Where != nil {
					fmt.Printf("      WHERE: %s\n", exprToString(c.Where))
				}
				if len(c.OrderBy) > 0 {
					fmt.Println("      ORDER BY:")
					for _, ob := range c.OrderBy {
						dir := "ASC"
						if ob.Descending {
							dir = "DESC"
						}
						fmt.Printf("        %s %s\n", exprToString(ob.Expression), dir)
					}
				}
			case *ast.ReturnClause:
				fmt.Printf("  [%d] RETURN", ci)
				if c.Distinct {
					fmt.Print(" DISTINCT")
				}
				fmt.Println()
				for _, p := range c.Projections {
					alias := p.Alias
					if alias == "" {
						alias = "<no alias>"
					}
					fmt.Printf("      %s AS %s\n", exprToString(p.Expression), alias)
				}
				if len(c.OrderBy) > 0 {
					fmt.Println("      ORDER BY:")
					for _, ob := range c.OrderBy {
						dir := "ASC"
						if ob.Descending {
							dir = "DESC"
						}
						fmt.Printf("        %s %s\n", exprToString(ob.Expression), dir)
					}
				}
			case *ast.CreateClause:
				fmt.Printf("  [%d] CREATE\n", ci)
				for _, pat := range c.Patterns {
					for _, pp := range pat.Parts {
						printPatternPart(pp)
					}
				}
			case *ast.UnwindClause:
				fmt.Printf("  [%d] UNWIND %s AS %s\n", ci, exprToString(c.Expression), c.Alias)
			case *ast.DeleteClause:
				fmt.Printf("  [%d] DELETE", ci)
				if c.Detach {
					fmt.Print(" DETACH")
				}
				fmt.Println()
				for _, e := range c.Exprs {
					fmt.Printf("      %s\n", exprToString(e))
				}
			case *ast.SetClause:
				fmt.Printf("  [%d] SET\n", ci)
				for _, item := range c.Items {
					if item.Operator != "" {
						fmt.Printf("      %s %s %s\n", exprToString(item.Target), item.Operator, exprToString(item.Value))
					} else if len(item.NodeLabels) > 0 {
						fmt.Printf("      %s :%s\n", exprToString(item.Target), item.NodeLabels[0])
					}
				}
			case *ast.MergeClause:
				fmt.Printf("  [%d] MERGE\n", ci)
				if c.Pattern != nil {
					printPatternPart(c.Pattern)
				}
			default:
				fmt.Printf("  [%d] <unknown clause>\n", ci)
			}
		}
	}
}

func printPatternPart(pp *ast.PatternPart) {
	if pp.Variable != "" {
		fmt.Printf("      %s = ", pp.Variable)
	} else {
		fmt.Print("      ")
	}
	if elem := pp.Element; elem != nil {
		for i, node := range elem.Nodes {
			fmt.Print("(")
			if node.Variable != "" {
				fmt.Print(node.Variable)
			}
			for _, label := range node.Labels {
				fmt.Printf(":%s", label)
			}
			if node.Properties != nil {
				fmt.Printf(" %s", exprToString(node.Properties))
			}
			fmt.Print(")")
			if i < len(elem.Rels) {
				rel := elem.Rels[i]
				fmt.Print("-")
				if rel.Direction == ast.DirLeft {
					fmt.Print("<")
				}
				fmt.Print("[")
				if len(rel.Types) > 0 {
					fmt.Printf(":%s", rel.Types[0])
				}
				fmt.Print("]")
				if rel.Direction == ast.DirRight {
					fmt.Print(">")
				}
				fmt.Print("-")
			}
		}
	}
	fmt.Println()
}

func exprToString(e ast.Expression) string {
	if e == nil {
		return "nil"
	}
	switch ex := e.(type) {
	case *ast.VariableExpr:
		return ex.Name
	case *ast.PropertyExpr:
		return exprToString(ex.Expression) + "." + ex.Property
	case *ast.LabelExpr:
		labels := ""
		for _, l := range ex.Labels {
			labels += ":" + l
		}
		return exprToString(ex.Expression) + labels
	case *ast.BinaryExpr:
		return exprToString(ex.Left) + " " + ex.Op + " " + exprToString(ex.Right)
	case *ast.UnaryExpr:
		return ex.Op + " " + exprToString(ex.Expr)
	case *ast.FunctionExpr:
		args := ""
		for i, a := range ex.Args {
			if i > 0 {
				args += ", "
			}
			args += exprToString(a)
		}
		return ex.Name + "(" + args + ")"
	case *ast.CountAllExpr:
		return "count(*)"
	case *ast.LiteralExpr:
		if ex.Value == nil {
			return "NULL"
		}
		return fmt.Sprintf("%v", ex.Value)
	case *ast.ListLiteralExpr:
		 elems := ""
		for i, elem := range ex.Elements {
			if i > 0 {
				elems += ", "
			}
			elems += exprToString(elem)
		}
		return "[" + elems + "]"
	case *ast.MapLiteralExpr:
		pairs := ""
		first := true
		for k, v := range ex.Pairs {
			if !first {
				pairs += ", "
			}
			first = false
			pairs += k + ": " + exprToString(v)
		}
		return "{" + pairs + "}"
	case *ast.ParameterExpr:
		return "$" + ex.Name
	case *ast.CaseExpr:
		return "CASE ... END"
	case *ast.ExistsExpr:
		return "EXISTS {...}"
	case *ast.InExpr:
		return exprToString(ex.Left) + " IN " + exprToString(ex.Right)
	case *ast.IsNullExpr:
		if ex.Not {
			return exprToString(ex.Expr) + " IS NOT NULL"
		}
		return exprToString(ex.Expr) + " IS NULL"
	case *ast.StringMatchExpr:
		return exprToString(ex.Left) + " " + ex.Op + " " + exprToString(ex.Right)
	case *ast.ListComprehensionExpr:
		return "[...]"
	case *ast.PatternComprehensionExpr:
		return "[(...)]"
	}
	return "<?>"
}
