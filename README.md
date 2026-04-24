# Cypher 查询引擎 —— ANTLR Parser + Semantic Analysis

> 语言：Go  
> 目标：基于 ANTLR4 生成完整 Cypher Parser，实现 Semantic Analysis

---

## 项目结构

```
mini_graph_engine/
├── cyphertree/              ← ANTLR4 自动生成的 Parser/Lexer/Visitor/BaseVisitor
│   ├── cypher_lexer.go
│   ├── cypher_parser.go
│   ├── cypherparser_visitor.go
│   └── cypherparser_base_visitor.go
├── ast/
│   ├── ast.go               ← Query, SingleQuery, Clause, ProjectionItem, OrderItem
│   ├── pattern.go           ← Pattern, NodePattern, RelPattern, RangeLiteral
│   └── expression.go        ← 完整 Expression 体系（BinaryExpr, FunctionExpr, CaseExpr, ExistsExpr, ...）
├── parser/
│   ├── antlr_parser.go      ← 调用 ANTLR 解析字符串，返回 ParseTree
│   └── ast_builder.go       ← 实现 Visitor，将 ParseTree 转换为自定义 AST
├── semantic/
│   └── semantic.go          ← Semantic Analyzer 核心（符号表、作用域、类型推断、聚合检查）
├── main.go                  ← 6 个示例演示
├── tools/
│   └── antlr-4.13.2-complete.jar
├── grammars-v4/             ← Cypher Grammar 源码（含修复后的 Lexer）
│   └── cypher/
│       ├── CypherLexer.g4
│       └── CypherParser.g4
└── README.md
```

---

## 快速开始

```bash
cd /mnt/workspace/go_space/mini_graph_engine
go run main.go
```

---

## ANTLR Parser 生成步骤

本项目已经包含了生成好的 Go Parser。如果你需要重新生成：

```bash
cd /mnt/workspace/go_space/mini_graph_engine/cyphertree

# 生成 Lexer
java -jar ../tools/antlr-4.13.2-complete.jar \
  -Dlanguage=Go -visitor -package cyphertree -no-listener \
  ../grammars-v4/cypher/CypherLexer.g4

# 生成 Parser
java -jar ../tools/antlr-4.13.2-complete.jar \
  -Dlanguage=Go -visitor -package cyphertree -no-listener \
  ../grammars-v4/cypher/CypherParser.g4
```

> **注意**：我们对 `grammars-v4/cypher/CypherLexer.g4` 做了两处关键修复：
> 1. `ID` 规则改为 `Letter LetterOrDigit*`（不能以数字开头），避免数字被识别为变量名
> 2. `DIGIT` 规则使用 `HexDigits` 而非 `HexDigit`，避免 `a`~`f` 被识别为数字
> 3. `STRING_LITERAL` 同时支持双引号和单引号字符串

---

## Semantic Analysis 一步一步讲解

以这条语句为例：

```cypher
MATCH (a:Person)-[:KNOWS]->(b:Person)
WHERE a.age > 25
WITH b, count(*) AS friend_count
WHERE friend_count > 2
RETURN b.name, friend_count
ORDER BY friend_count DESC
```

### Step 1: 变量收集（Variable Declaration）

在 Cypher 中，变量不是显式声明的，而是在模式匹配中隐式引入：
- `MATCH (a:Person)` → 声明 `a` 为 `Node`
- `WITH b, count(*) AS friend_count` → 声明 `friend_count` 为 `Aggregate`

对应代码：`scope.Declare(name, kind, clause)`

### Step 2: 变量解析（Variable Resolution）

每个表达式中的变量引用都要检查是否已定义：
- `WHERE a.age > 25` → `Resolve("a")` 必须在符号表中存在
- 如果不存在，报 `undefined variable`

对应代码：`scope.Resolve(name)` 沿作用域链向上查找

### Step 3: 作用域管理（Scope Management）

Cypher 的核心规则：**WITH 是作用域边界**。

```
MATCH (a:Person)        -- a 进入作用域
WITH b, count(*) AS c   -- 创建新作用域，只保留 b 和 c
RETURN a.name           -- ❌ a 已不可见！
```

代码实现的关键设计：**WITH 后的新作用域是隔离的（Parent = nil）**。

```go
// 先在旧作用域中分析投影表达式
for _, proj := range wc.Projections {
    sa.analyzeExpression(proj.Expression, wc)
}

// 创建隔离的新作用域
isolatedScope := NewScope(nil)
sa.currentScope = isolatedScope

// 只把投影变量声明进新作用域
sa.currentScope.Declare("b", KindNode, wc)
sa.currentScope.Declare("friend_count", KindAggregate, wc)
```

### Step 4: 聚合上下文检查（Aggregation Context）

Cypher 规则：如果 `WITH` / `RETURN` 中包含聚合函数（`count`, `sum`, `avg`, `min`, `max`, `collect`...），所有**非聚合表达式必须是分组键（grouping key）**。

```cypher
WITH b, count(*) AS friend_count
-- b 是分组键 ✓
-- count(*) 是聚合 ✓
```

### Step 5: 类型推断（Type Inference）

| 表达式 | 推断类型 | 理由 |
|--------|---------|------|
| `a` | `Node` | MATCH 声明 |
| `a.age` | `Scalar` | 属性访问返回标量 |
| `count(*)` | `Aggregate` | 聚合函数 |
| `a.age > 25` | `Scalar` | 比较返回布尔标量 |
| `[1, 2, 3]` | `List` | 列表字面量 |
| `{name: "Alice"}` | `Map` | Map 字面量 |

---

## Semantic Analysis 为查询计划生成提供了什么？

| 产物 | 内容 | 计划器如何使用 |
|------|------|--------------|
| **SymbolTable** | 每个变量的名字、类型、声明位置 | 决定扫描/投影需要哪些列 |
| **ScopeChain** | 每个 clause 的可见变量集合 | 决定算子间的数据流 |
| **AggregationInfo** | 分组键 vs 聚合值 | 生成 `HashAggregate` / `SortAggregate` |
| **TypeMap** | 每个表达式的返回类型 | 类型驱动的算子选择、内存布局 |
| **Validation** | 语句是否语义合法 | 计划器只处理合法输入 |

---

## 支持的 Cypher 语法子集

| 特性 | 支持状态 |
|------|---------|
| MATCH (含 OPTIONAL) | ✅ |
| WHERE | ✅ |
| WITH / RETURN (含 DISTINCT, ORDER BY, SKIP, LIMIT) | ✅ |
| CREATE | ✅ |
| UNWIND | ✅ |
| DELETE (含 DETACH) | ✅ |
| SET / REMOVE (含标签操作) | ✅ |
| MERGE (含 ON CREATE/MATCH) | ✅ |
| 节点模式 `(a:Person {name: "Tom"})` | ✅ |
| 关系模式 `-[:KNOWS]->` | ✅ |
| 属性访问 `a.name` | ✅ |
| 标签过滤 `a:Person` | ✅ |
| 二元运算 `+, -, *, /, %, ^, =, <>, <, >, <=, >=, AND, OR, XOR` | ✅ |
| 一元运算 `NOT, +, -` | ✅ |
| 函数调用 `sum(x), collect(y)` | ✅ |
| `count(*)` | ✅ |
| 字面量（整数、浮点、字符串、布尔、NULL） | ✅ |
| 列表字面量 `[1, 2, 3]` | ✅ |
| Map 字面量 `{name: "Alice"}` | ✅ |
| 参数 `$param` | ✅ |
| CASE 表达式 | ✅ |
| EXISTS { pattern } | ✅ |
| IN 表达式 | ✅ |
| IS [NOT] NULL | ✅ |
| STARTS WITH / ENDS WITH / CONTAINS | ✅ |
| 列表推导式 | ✅ |
| 模式推导式 | ✅ |
| UNION | ❌（暂不实现） |
| 子查询 `CALL {}` | ❌（暂不实现） |
| 路径变量 `p = (a)-[]->(b)` | ✅（解析支持，语义分析简化） |
| 变长关系 `*1..5` | ❌（暂不实现） |

---

## 下一步学习建议

1. **Schema Binding**：把变量和真实的数据库 schema 绑定，检查属性名是否存在
2. **子查询 `CALL {}`**：子查询的作用域更复杂，内部可见外部变量（闭包）
3. **查询计划生成**：基于 Semantic Analysis 的结果，构建逻辑计划树
4. **计划优化**：谓词下推、连接重排、索引选择
5. **物理执行**：算子实现（NodeScan, Expand, Filter, Aggregate, Project...）
