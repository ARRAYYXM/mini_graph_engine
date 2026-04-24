package parser

import (
	"fmt"
	"mini_graph_engine/cyphertree"

	"github.com/antlr4-go/antlr/v4"
)

// Parse 使用 ANTLR4 生成的 Parser 将 Cypher 查询字符串解析为 ParseTree。
// 调用方需要用 ASTBuilder 将 ParseTree 转换为自定义 AST。
func Parse(input string) (cyphertree.IScriptContext, error) {
	is := antlr.NewInputStream(input)
	lexer := cyphertree.NewCypherLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := cyphertree.NewCypherParser(stream)
	p.RemoveErrorListeners()
	p.AddErrorListener(&errorListener{})
	tree := p.Script()
	return tree, nil
}

type errorListener struct {
	errors []string
}

func (e *errorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, ex antlr.RecognitionException) {
	e.errors = append(e.errors, fmt.Sprintf("line %d:%d %s", line, column, msg))
}

func (e *errorListener) ReportAmbiguity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, exact bool, ambigAlts *antlr.BitSet, configs *antlr.ATNConfigSet) {}
func (e *errorListener) ReportAttemptingFullContext(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, conflictingAlts *antlr.BitSet, configs *antlr.ATNConfigSet) {}
func (e *errorListener) ReportContextSensitivity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, prediction int, configs *antlr.ATNConfigSet) {}
