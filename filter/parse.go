// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package filter

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/GuanceCloud/cliutils/logger"
	"github.com/prometheus/prometheus/util/strutil"
)

var (
	parserPool = sync.Pool{
		New: func() interface{} {
			return &parser{}
		},
	}

	log = logger.DefaultSLogger("filter-parser")
)

type parser struct {
	lex      Lexer
	yyParser yyParserImpl

	parseResult interface{}
	lastClosing Pos
	errs        ParseErrors
	warns       ParseErrors

	inject    ItemType
	injecting bool
}

func GetConds(input string) (WhereConditions, error) {
	log.Debugf("parse %s", input)

	var err error
	p := newParser(input)
	defer parserPool.Put(p)
	defer p.recover(&err)

	p.doParse()

	if len(p.errs) > 0 {
		return nil, &p.errs[0]
	}

	return p.parseResult.(WhereConditions), nil
}

func newParser(input string) *parser {
	p, ok := parserPool.Get().(*parser)
	if !ok {
		log.Fatal("parserPool: should not been here")
	}

	// reset parser fields
	p.injecting = false
	p.errs = nil
	p.warns = nil
	p.parseResult = nil
	p.lex = Lexer{
		input: input,
		state: lexStatements,
	}

	return p
}

var errUnexpected = errors.New("unexpected error")

func (p *parser) unexpected(context string, expected string) {
	var errMsg strings.Builder

	if p.yyParser.lval.item.Typ == ERROR { // do not report lex error twice
		return
	}

	errMsg.WriteString("unexpected: ")
	errMsg.WriteString(p.yyParser.lval.item.desc())

	if context != "" {
		errMsg.WriteString(" in: ")
		errMsg.WriteString(context)
	}

	if expected != "" {
		errMsg.WriteString(", expected: ")
		errMsg.WriteString(expected)
	}

	p.addParseErr(p.yyParser.lval.item.PositionRange(), errors.New(errMsg.String()))
}

func (p *parser) recover(errp *error) {
	e := recover() //nolint:ifshort
	if _, ok := e.(runtime.Error); ok {
		buf := make([]byte, 64<<10) // 64k
		buf = buf[:runtime.Stack(buf, false)]
		fmt.Fprintf(os.Stderr, "parser panic: %v\n%s", e, buf)
		*errp = errUnexpected
	} else if e != nil {
		*errp = e.(error) //nolint:forcetypeassert
	}
}

func (p *parser) addParseErr(pr *PositionRange, err error) {
	p.errs = append(p.errs, ParseError{
		Pos:   pr,
		Err:   err,
		Query: p.lex.input,
	})
}

func (p *parser) addParseWarn(pr *PositionRange, err error) {
	p.warns = append(p.warns, ParseError{
		Pos:   pr,
		Err:   err,
		Query: p.lex.input,
	})
}

func (p *parser) addParseErrf(pr *PositionRange, format string, args ...interface{}) {
	p.addParseErr(pr, fmt.Errorf(format, args...))
}

func (p *parser) addParseWarnf(pr *PositionRange, format string, args ...interface{}) {
	p.addParseWarn(pr, fmt.Errorf(format, args...))
}

// impl Lex interface.
func (p *parser) Lex(lval *yySymType) int {
	var typ ItemType

	if p.injecting {
		p.injecting = false
		return int(p.inject)
	}

	for { // skip comment
		p.lex.NextItem(&lval.item)
		typ = lval.item.Typ
		if typ != COMMENT {
			break
		}
	}

	switch typ {
	case ERROR:
		pos := PositionRange{
			Start: p.lex.start,
			End:   Pos(len(p.lex.input)),
		}

		p.addParseErr(&pos, errors.New(p.yyParser.lval.item.Val))
		return 0 // tell yacc it's the end of input

	case EOF:
		lval.item.Typ = EOF
		p.InjectItem(0)
	case RIGHT_BRACE, RIGHT_PAREN, RIGHT_BRACKET, DURATION:
		p.lastClosing = lval.item.Pos + Pos(len(lval.item.Val))
	}
	return int(typ)
}

func (p *parser) Error(e string) {}

func (p *parser) unquoteString(s string) string {
	unq, err := strutil.Unquote(s)
	if err != nil {
		p.addParseErrf(p.yyParser.lval.item.PositionRange(),
			"error unquoting string %q: %s", s, err)
	}
	return unq
}

func (p *parser) doParse() {
	p.InjectItem(START_WHERE_CONDITION)
	p.yyParser.Parse(p)
}

func (p *parser) InjectItem(typ ItemType) {
	if p.injecting {
		log.Warnf("current inject is %v, new inject is %v", p.inject, typ)
		panic("cannot inject multiple Items into the token stream")
	}

	if typ != 0 && (typ <= startSymbolsStart || typ >= startSymbolsEnd) {
		log.Warnf("current inject is %v", typ)
		panic("cannot inject symbol that isn't start symbol")
	}
	p.inject = typ
	p.injecting = true
}

func (p *parser) number(v string) *NumberLiteral {
	nl := &NumberLiteral{}

	n, err := strconv.ParseInt(v, 0, 64)
	if err != nil {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			p.addParseErrf(p.yyParser.lval.item.PositionRange(),
				"error parsing number: %s", err)
		}
		nl.Float = f
	} else {
		nl.IsInt = true
		nl.Int = n
	}

	return nl
}

func doNewRegex(s string) (*Regex, error) {
	re, err := regexp.Compile(s)
	if err != nil {
		return nil, err
	}
	return &Regex{
		Regex: s,
		Re:    re,
	}, nil
}

func (p *parser) newRegex(s string) *Regex {
	if x, err := doNewRegex(s); err != nil {
		p.addParseWarnf(p.yyParser.lval.item.PositionRange(),
			"invalid regex: %s: %s, ignored", err.Error(), s)
		return nil
	} else {
		return x
	}
}

func (p *parser) newBinExpr(l, r Node, op Item) *BinaryExpr {
	switch op.Typ {
	case DIV, MOD:
		rightNumber, ok := r.(*NumberLiteral)
		if ok {
			if rightNumber.IsInt && rightNumber.Int == 0 ||
				!rightNumber.IsInt && rightNumber.Float == 0 {
				p.addParseErrf(p.yyParser.lval.item.PositionRange(), "division or modulo by zero")
				return nil
			}
		}

	case MATCH, NOT_MATCH: // convert rhs into regex list
		switch nl := r.(type) {
		case NodeList:
			// convert elems in @n into Regex node, used in CONTAIN/NOTCONTAIN
			var regexArr NodeList
			for _, elem := range nl {
				switch x := elem.(type) {
				case *StringLiteral:
					if re := p.newRegex(x.Val); re != nil {
						regexArr = append(regexArr, re)
					}

				default:
					p.addParseErrf(p.yyParser.lval.item.PositionRange(),
						"invalid element type in CONTAIN/NOT_CONTAIN list: %s", reflect.TypeOf(elem).String())
				}
			}
			return &BinaryExpr{LHS: l, RHS: regexArr, Op: op.Typ}

		default:
			p.addParseErrf(p.yyParser.lval.item.PositionRange(),
				"invalid type in CONTAIN/NOT_CONTAIN list: %s(%s)", reflect.TypeOf(l).String(), l.String())
		}
	}

	return &BinaryExpr{RHS: r, LHS: l, Op: op.Typ}
}

func (p *parser) newFunc(fname string, args []Node) *FuncExpr {
	agg := &FuncExpr{
		Name:  strings.ToLower(fname),
		Param: args,
	}
	return agg
}

// end of yylex.(*parser).newXXXX

type ParseErrors []ParseError

type ParseError struct {
	Pos        *PositionRange
	Err        error
	Query      string
	LineOffset int
}

func (e *ParseError) Error() string {
	if e.Pos == nil {
		return fmt.Sprintf("%s", e.Err)
	}

	pos := int(e.Pos.Start)
	lastLineBrk := -1
	ln := e.LineOffset + 1
	var posStr string

	if pos < 0 || pos > len(e.Query) {
		posStr = "invalid position:"
	} else {
		for i, c := range e.Query[:pos] {
			if c == '\n' {
				lastLineBrk = i
				ln++
			}
		}

		col := pos - lastLineBrk
		posStr = fmt.Sprintf("%d:%d", ln, col)
	}

	return fmt.Sprintf("%s parse error: %s", posStr, e.Err)
}

// Error impl Error() interface.
func (errs ParseErrors) Error() string {
	var errArray []string
	for _, err := range errs {
		errStr := err.Error()
		if errStr != "" {
			errArray = append(errArray, errStr)
		}
	}

	return strings.Join(errArray, "\n")
}

type PositionRange struct {
	Start, End Pos
}

func (p *parser) newWhereConditions(conditions []Node) *WhereCondition {
	return &WhereCondition{
		conditions: conditions,
	}
}

func Init() {
	log = logger.SLogger("filter-parser")
}
