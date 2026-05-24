package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type TokenType int

type Token struct {
	Type    TokenType
	Literal string
}

type Parser struct {
	l    *lexer
	cur  Token
	peek Token
}

type lexer struct {
	input        string
	position     int
	readPosition int
	ch           rune
}

const (
	ILLEGAL TokenType = iota
	EOF
	IDENT
	STRING
	NUMBER
	ASTERISK
	COMMA
	LPAREN
	RPAREN
	EQUAL
	NOT_EQUAL
	LT
	GT
	LTE
	GTE
	LIKE
	SEMICOLON
	CREATE
	TABLE
	WITH
	AS
	INSERT
	OVERWRITE
	SELECT
	FROM
	WHERE
	ORDER
	BY
	LIMIT
	AND
	OR
	GROUP
	HAVING
	DOT
	JOIN
	ON
	IS
	NULL_KEYWORD
	NOT
	IN_KEYWORD
	DISTINCT
	PLUS
	MINUS
	DIV
	EXTERNAL
	OVER
	PARTITION
	INTO
	UNION
	CASE_KW
	WHEN_KW
	THEN_KW
	ELSE_KW
	END_KW
	EXISTS_KW
	VALUES
	EXPLAIN
)

var keywords = map[string]TokenType{
	"create":    CREATE,
	"table":     TABLE,
	"with":      WITH,
	"as":        AS,
	"insert":    INSERT,
	"overwrite": OVERWRITE,
	"select":    SELECT,
	"from":      FROM,
	"where":     WHERE,
	"order":     ORDER,
	"by":        BY,
	"limit":     LIMIT,
	"like":      LIKE,
	"and":       AND,
	"or":        OR,
	"group":     GROUP,
	"having":    HAVING,
	"join":      JOIN,
	"on":        ON,
	"is":        IS,
	"null":      NULL_KEYWORD,
	"not":       NOT,
	"in":        IN_KEYWORD,
	"distinct":  DISTINCT,
	"external":  EXTERNAL,
	"over":      OVER,
	"partition": PARTITION,
	"into":      INTO,
	"union":     UNION,
	"case":      CASE_KW,
	"when":      WHEN_KW,
	"then":      THEN_KW,
	"else":      ELSE_KW,
	"end":       END_KW,
	"exists":    EXISTS_KW,
	"values":    VALUES,
	"explain":   EXPLAIN,
}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(sql string) ([]Statement, error) {
	cleaned := removeComments(sql)
	p.l = newLexer(cleaned)
	p.nextToken()
	p.nextToken()

	var statements []Statement
	for p.cur.Type != EOF {
		if p.cur.Type == SEMICOLON {
			p.nextToken()
			continue
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
		if p.cur.Type == SEMICOLON {
			p.nextToken()
		}
	}
	return statements, nil
}

func (p *Parser) nextToken() {
	p.cur = p.peek
	p.peek = p.l.nextToken()
}

func (p *Parser) curIs(t TokenType) bool {
	return p.cur.Type == t
}

func (p *Parser) peekIs(t TokenType) bool {
	return p.peek.Type == t
}

func (p *Parser) expectPeek(t TokenType) error {
	if p.peekIs(t) {
		p.nextToken()
		return nil
	}
	return fmt.Errorf("expected next token %s, got %s", tokenName(t), tokenName(p.peek.Type))
}

func tokenName(t TokenType) string {
	switch t {
	case ILLEGAL:
		return "ILLEGAL"
	case EOF:
		return "EOF"
	case IDENT:
		return "IDENT"
	case STRING:
		return "STRING"
	case NUMBER:
		return "NUMBER"
	case ASTERISK:
		return "*"
	case COMMA:
		return ","
	case LPAREN:
		return "("
	case RPAREN:
		return ")"
	case EQUAL:
		return "="
	case NOT_EQUAL:
		return "!="
	case LT:
		return "<"
	case GT:
		return ">"
	case LTE:
		return "<="
	case GTE:
		return ">="
	case LIKE:
		return "LIKE"
	case AND:
		return "AND"
	case OR:
		return "OR"
	case DOT:
		return "."
	case JOIN:
		return "JOIN"
	case ON:
		return "ON"
	case GROUP:
		return "GROUP"
	case HAVING:
		return "HAVING"
	case SEMICOLON:
		return ";"
	case CREATE, TABLE, WITH, AS, INSERT, OVERWRITE, SELECT, FROM, WHERE, ORDER, BY, LIMIT, IS, NULL_KEYWORD, NOT, IN_KEYWORD, DISTINCT, EXTERNAL, OVER, PARTITION:
		return strings.ToUpper(t.String())
	default:
		return "UNKNOWN"
	}
}

func (t TokenType) String() string {
	switch t {
	case CREATE:
		return "CREATE"
	case EXTERNAL:
		return "EXTERNAL"
	case OVER:
		return "OVER"
	case PARTITION:
		return "PARTITION"
	case TABLE:
		return "TABLE"
	case WITH:
		return "WITH"
	case AS:
		return "AS"
	case INSERT:
		return "INSERT"
	case OVERWRITE:
		return "OVERWRITE"
	case SELECT:
		return "SELECT"
	case FROM:
		return "FROM"
	case WHERE:
		return "WHERE"
	case ORDER:
		return "ORDER"
	case BY:
		return "BY"
	case LIMIT:
		return "LIMIT"
	case LIKE:
		return "LIKE"
	case DOT:
		return "."
	case JOIN:
		return "JOIN"
	case ON:
		return "ON"
	case AND:
		return "AND"
	case OR:
		return "OR"
	case GROUP:
		return "GROUP"
	case IS:
		return "IS"
	case NULL_KEYWORD:
		return "NULL"
	case NOT:
		return "NOT"
	case IN_KEYWORD:
		return "IN"
	case DISTINCT:
		return "DISTINCT"
	case UNION:
		return "UNION"
	case CASE_KW:
		return "CASE"
	case WHEN_KW:
		return "WHEN"
	case THEN_KW:
		return "THEN"
	case ELSE_KW:
		return "ELSE"
	case END_KW:
		return "END"
	case EXISTS_KW:
		return "EXISTS"
case VALUES:
		return "VALUES"
	default:
		return ""
	}
}

func newLexer(input string) *lexer {
	l := &lexer{input: input}
	l.readChar()
	return l
}

func (l *lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = rune(l.input[l.readPosition])
	}
	l.position = l.readPosition
	l.readPosition++
}

func (l *lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return rune(l.input[l.readPosition])
}

func (l *lexer) nextToken() Token {
	l.skipWhitespace()

	var tok Token
	switch l.ch {
	case '*':
		tok = Token{Type: ASTERISK, Literal: "*"}
	case ',':
		tok = Token{Type: COMMA, Literal: ","}
	case '(':
		tok = Token{Type: LPAREN, Literal: "("}
	case ')':
		tok = Token{Type: RPAREN, Literal: ")"}
	case '=':
		tok = Token{Type: EQUAL, Literal: "="}
	case '!':
		if l.peekChar() == '=' {
			tok = Token{Type: NOT_EQUAL, Literal: "!="}
			l.readChar()
		} else {
			tok = Token{Type: ILLEGAL, Literal: string(l.ch)}
		}
	case '<':
		if l.peekChar() == '=' {
			tok = Token{Type: LTE, Literal: "<="}
			l.readChar()
		} else {
			tok = Token{Type: LT, Literal: "<"}
		}
	case '>':
		if l.peekChar() == '=' {
			tok = Token{Type: GTE, Literal: ">="}
			l.readChar()
		} else {
			tok = Token{Type: GT, Literal: ">"}
		}
	case '+':
		tok = Token{Type: PLUS, Literal: "+"}
	case '-':
		tok = Token{Type: MINUS, Literal: "-"}
	case '/':
		tok = Token{Type: DIV, Literal: "/"}
	case '.':
		tok = Token{Type: DOT, Literal: "."}
	case ';':
		tok = Token{Type: SEMICOLON, Literal: ";"}
	case '\'', '"':
		quote := l.ch
		tok.Type = STRING
		tok.Literal = l.readString(quote)
		l.readChar()
		return tok
	case 0:
		tok = Token{Type: EOF, Literal: ""}
	default:
		if isLetter(l.ch) {
			literal := l.readIdentifier()
			if tokType, ok := keywords[strings.ToLower(literal)]; ok {
				tok = Token{Type: tokType, Literal: strings.ToUpper(literal)}
			} else {
				tok = Token{Type: IDENT, Literal: literal}
			}
			return tok
		}
		if isDigit(l.ch) {
			tok = Token{Type: NUMBER, Literal: l.readNumber()}
			return tok
		}
		tok = Token{Type: ILLEGAL, Literal: string(l.ch)}
	}
	l.readChar()
	return tok
}

func (l *lexer) skipWhitespace() {
	for l.ch != 0 && unicode.IsSpace(l.ch) {
		l.readChar()
	}
}

func (l *lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *lexer) readString(quote rune) string {
	l.readChar()
	position := l.position
	for l.ch != 0 && l.ch != quote {
		if l.ch == '\\' {
			l.readChar()
			if l.ch != 0 {
				l.readChar()
			}
		} else {
			l.readChar()
		}
	}
	return l.input[position:l.position]
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

func (p *Parser) parseStatement() (Statement, error) {
	switch p.cur.Type {
	case CREATE:
		return p.parseCreateTable()
	case INSERT:
		return p.parseInsertOverwrite()
	case EXPLAIN:
		p.nextToken()
		query, err := p.parseSelectQuery()
		if err != nil {
			return nil, err
		}
		return &ExplainStmt{Query: query}, nil
	case WITH, SELECT:
		query, err := p.parseSelectQuery()
		if err != nil {
			return nil, err
		}
		return &SelectStmt{Query: query}, nil
	default:
		return nil, fmt.Errorf("unsupported statement: %s", p.cur.Literal)
	}
}

func (p *Parser) parseCreateTable() (*CreateTableStmt, error) {
	external := false
	if p.peekIs(EXTERNAL) {
		external = true
		p.nextToken()
	}
	if err := p.expectPeek(TABLE); err != nil {
		return nil, err
	}
	if err := p.expectPeek(IDENT); err != nil {
		return nil, fmt.Errorf("expected table name after CREATE TABLE")
	}
	tableName := p.cur.Literal
	if err := p.expectPeek(LPAREN); err != nil {
		return nil, err
	}
	p.nextToken()
	columns, err := p.parseColumnDefinitions()
	if err != nil {
		return nil, err
	}
	if err := p.expectPeek(RPAREN); err != nil {
		return nil, err
	}
	if err := p.expectPeek(WITH); err != nil {
		return nil, err
	}
	if err := p.expectPeek(LPAREN); err != nil {
		return nil, err
	}
	p.nextToken()
	options, err := p.parseWithOptions()
	if err != nil {
		return nil, err
	}
	if err := p.expectPeek(RPAREN); err != nil {
		return nil, err
	}
	p.nextToken()

	partitionBy := p.parsePartitionedBy()

	return &CreateTableStmt{
		Name:        tableName,
		Columns:     columns,
		WithOptions: options,
		External:    external,
		PartitionBy: partitionBy,
	}, nil
}

func (p *Parser) parsePartitionedBy() []string {
	if p.cur.Type != IDENT || strings.ToLower(p.cur.Literal) != "partitioned" {
		return nil
	}
	p.nextToken()
	if p.cur.Type != BY {
		return nil
	}
	p.nextToken()
	if p.cur.Type != LPAREN {
		return nil
	}
	p.nextToken()
	cols, err := p.parseDottedList()
	if err != nil {
		return nil
	}
	if p.cur.Type != RPAREN {
		return nil
	}
	p.nextToken()
	return cols
}

func (p *Parser) parseColumnDefinitions() ([]ColumnDef, error) {
	var columns []ColumnDef
	for {
		if p.cur.Type != IDENT {
			return nil, fmt.Errorf("expected column name, got %s", tokenName(p.cur.Type))
		}
		name := p.cur.Literal
		if err := p.expectPeek(IDENT); err != nil {
			return nil, fmt.Errorf("expected column type after %s", name)
		}
		colType := p.cur.Literal
		columns = append(columns, ColumnDef{Name: name, Type: colType})
		if p.peekIs(COMMA) {
			p.nextToken()
			p.nextToken()
			continue
		}
		break
	}
	return columns, nil
}

func (p *Parser) parseWithOptions() (map[string]string, error) {
	options := make(map[string]string)
	for {
		if p.cur.Type != IDENT {
			return nil, fmt.Errorf("expected WITH option key, got %s", tokenName(p.cur.Type))
		}
		key := p.cur.Literal
		if err := p.expectPeek(EQUAL); err != nil {
			return nil, err
		}
		p.nextToken()
		if p.cur.Type != STRING && p.cur.Type != IDENT && p.cur.Type != NUMBER {
			return nil, fmt.Errorf("expected WITH option value for %s", key)
		}
		options[key] = p.cur.Literal
		if p.peekIs(COMMA) {
			p.nextToken()
			p.nextToken()
			continue
		}
		break
	}
	return options, nil
}

func (p *Parser) parseInsertOverwrite() (*InsertOverwriteStmt, error) {
	appendMode := false
	if p.peekIs(INTO) {
		appendMode = true
		p.nextToken()
	} else if p.peekIs(OVERWRITE) {
		p.nextToken()
	} else {
		return nil, fmt.Errorf("expected INTO or OVERWRITE after INSERT")
	}

	// 支持 INSERT INTO TABLE users 和 INSERT INTO users 两种语法
	if p.peekIs(TABLE) {
		p.nextToken()
	}
	if err := p.expectPeek(IDENT); err != nil {
		return nil, fmt.Errorf("expected target table name after INSERT %s", map[bool]string{true: "INTO", false: "OVERWRITE"}[appendMode])
	}
	tableName := p.cur.Literal
	p.nextToken()

	// 支持 INSERT INTO ... VALUES
	if p.curIs(VALUES) {
		p.nextToken()
		values, err := p.parseValuesClause()
		if err != nil {
			return nil, err
		}
		return &InsertOverwriteStmt{TableName: tableName, Values: values, Append: appendMode}, nil
	}

	// 原有的 INSERT INTO ... SELECT 逻辑
	query, err := p.parseSelectQuery()
	if err != nil {
		return nil, err
	}
	return &InsertOverwriteStmt{TableName: tableName, Query: query, Append: appendMode}, nil
}

func (p *Parser) parseSelectQuery() (*SelectQuery, error) {
	var ctes []CTE
	if p.cur.Type == WITH {
		var err error
		ctes, err = p.parseCTEList()
		if err != nil {
			return nil, err
		}
	}
	if p.cur.Type != SELECT {
		return nil, fmt.Errorf("expected SELECT, got %s", tokenName(p.cur.Type))
	}
	p.nextToken()
	distinct := false
	if p.cur.Type == DISTINCT {
		distinct = true
		p.nextToken()
	}
	columns, columnExprs, aggregates, windowExprs, colAliases, err := p.parseSelectItems()
	if err != nil {
		return nil, err
	}
	if p.curIs(FROM) {
		p.nextToken()
	} else if err := p.expectPeek(FROM); err != nil {
		return nil, err
	}

	var tableName string
	var tableAlias string
	var fromSubquery *SelectQuery
	var fromAlias string
	if p.peekIs(LPAREN) {
		p.nextToken()
		p.nextToken()
		fromSubquery, err = p.parseSelectQuery()
		if err != nil {
			return nil, err
		}
		if p.cur.Type != RPAREN {
			return nil, fmt.Errorf("expected ) after subquery in FROM clause")
		}
		p.nextToken()
		if p.curIs(AS) {
			p.nextToken()
		}
		if p.cur.Type == IDENT {
			fromAlias = p.cur.Literal
			tableAlias = fromAlias
			p.nextToken()
		} else {
			return nil, fmt.Errorf("expected alias for subquery in FROM clause")
		}
	} else {
		if p.cur.Type != IDENT {
			if err := p.expectPeek(IDENT); err != nil {
				return nil, fmt.Errorf("expected table name after FROM")
			}
		}
		tableName = p.cur.Literal
		p.nextToken()
		if p.curIs(AS) {
			p.nextToken()
		}
		if p.cur.Type == IDENT {
			tableAlias = p.cur.Literal
			p.nextToken()
		}
	}

	var joins []JoinClause
	for p.curIs(JOIN) {
		p.nextToken()
		if p.cur.Type != IDENT {
			return nil, fmt.Errorf("expected table name after JOIN")
		}
		rightTable := p.cur.Literal
		var rightAlias string
		p.nextToken()
		if p.curIs(AS) {
			p.nextToken()
		}
		if p.cur.Type == IDENT {
			rightAlias = p.cur.Literal
			p.nextToken()
		}
		if p.cur.Type != ON {
			return nil, fmt.Errorf("expected ON after JOIN table %s", rightTable)
		}
		p.nextToken()
		leftCol, err := p.parseDottedIdentifier()
		if err != nil {
			return nil, fmt.Errorf("JOIN ON left side: %w", err)
		}
		if p.cur.Type != EQUAL {
			return nil, fmt.Errorf("expected = in JOIN ON clause")
		}
		p.nextToken()
		rightCol, err := p.parseDottedIdentifier()
		if err != nil {
			return nil, fmt.Errorf("JOIN ON right side: %w", err)
		}
		joins = append(joins, JoinClause{RightTable: rightTable, RightAlias: rightAlias, LeftColumn: leftCol, RightColumn: rightCol})
	}

	var where Expression
	var groupBy []string
	var having Expression
	var orderBy []SortOrder
	var limit int
	var hasLimit bool
	if p.cur.Type == WHERE {
		p.nextToken()
		where, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	if p.cur.Type == GROUP {
		if err := p.expectPeek(BY); err != nil {
			return nil, err
		}
		p.nextToken()
		groupBy, err = p.parseDottedList()
		if err != nil {
			return nil, err
		}
		if p.cur.Type == HAVING {
			p.nextToken()
			having, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
		}
	} else if p.cur.Type == HAVING {
		p.nextToken()
		having, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	if p.cur.Type == ORDER {
		if err := p.expectPeek(BY); err != nil {
			return nil, err
		}
		p.nextToken()
		orderBy, err = p.parseOrderByList()
		if err != nil {
			return nil, err
		}
	}
	if p.cur.Type == LIMIT {
		p.nextToken()
		if p.cur.Type != NUMBER {
			return nil, fmt.Errorf("expected integer after LIMIT")
		}
		limit, err = strconv.Atoi(p.cur.Literal)
		if err != nil {
			return nil, fmt.Errorf("invalid LIMIT value: %w", err)
		}
		hasLimit = true
		p.nextToken()
	}
	query := &SelectQuery{
		CTEs:          ctes,
		Columns:       columns,
		Table:         tableName,
		TableAlias:    tableAlias,
		FromSubquery:  fromSubquery,
		FromAlias:     fromAlias,
		ColumnExprs:   columnExprs,
		Aggregates:    aggregates,
		WindowExprs:   windowExprs,
		Joins:         joins,
		Distinct:      distinct,
		Where:         where,
		GroupBy:       groupBy,
		Having:        having,
		OrderBy:       orderBy,
		Limit:         limit,
		HasLimit:      hasLimit,
		ColumnAliases: colAliases,
	}
	if len(aggregates) > 0 && len(groupBy) == 0 && len(columns) == 0 {
		query.Columns = columns
	}

	// UNION [ALL]
	if p.cur.Type == UNION {
		unionAll := false
		p.nextToken()
		if p.cur.Type == IDENT && strings.ToLower(p.cur.Literal) == "all" {
			unionAll = true
			p.nextToken()
		}
		unionQuery, err := p.parseSelectQuery()
		if err != nil {
			return nil, err
		}
		query.UnionQuery = unionQuery
		query.UnionAll = unionAll
	}

	// Remaining token check: only enforce for end-of-statement markers
	// (UNION is already handled above)
	if p.cur.Type != SEMICOLON && p.cur.Type != EOF && p.cur.Type != RPAREN {
		return nil, fmt.Errorf("unexpected token %s after SELECT query", tokenName(p.cur.Type))
	}

	return query, nil
}

func (p *Parser) parseCTEList() ([]CTE, error) {
	if p.cur.Type != WITH {
		return nil, nil
	}
	p.nextToken() // consume WITH
	var ctes []CTE
	for {
		if p.cur.Type != IDENT {
			return nil, fmt.Errorf("expected CTE name, got %s", tokenName(p.cur.Type))
		}
		cteName := p.cur.Literal
		if err := p.expectPeek(AS); err != nil {
			return nil, err
		}
		if err := p.expectPeek(LPAREN); err != nil {
			return nil, err
		}
		p.nextToken()
		query, err := p.parseSelectQuery()
		if err != nil {
			return nil, err
		}
		if p.cur.Type != RPAREN {
			return nil, fmt.Errorf("expected closing parenthesis for CTE %s", cteName)
		}
		ctes = append(ctes, CTE{Name: cteName, Query: query})
		if p.peekIs(COMMA) {
			p.nextToken()
			p.nextToken()
			continue
		}
		p.nextToken()
		break
	}
	return ctes, nil
}

func (p *Parser) parseColumnList() ([]string, error) {
	if p.cur.Type == ASTERISK {
		p.nextToken()
		return []string{"*"}, nil
	}
	var columns []string
	for {
		if p.cur.Type != IDENT {
			return nil, fmt.Errorf("expected column name, got %s", tokenName(p.cur.Type))
		}
		columns = append(columns, p.cur.Literal)
		if p.peekIs(COMMA) {
			p.nextToken()
			p.nextToken()
			continue
		}
		break
	}
	return columns, nil
}

func (p *Parser) parseIdentifierList() ([]string, error) {
	var items []string
	for {
		if p.cur.Type != IDENT {
			return nil, fmt.Errorf("expected identifier, got %s", tokenName(p.cur.Type))
		}
		items = append(items, p.cur.Literal)
		if p.peekIs(COMMA) {
			p.nextToken()
			p.nextToken()
			continue
		}
		break
	}
	return items, nil
}

func (p *Parser) parseExpression() (Expression, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.curIs(AND) || p.curIs(OR) {
		op := strings.ToUpper(p.cur.Literal)
		p.nextToken()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &LogicalExpr{Left: left, Operator: op, Right: right}
	}
	return left, nil
}

func (p *Parser) parseComparison() (Expression, error) {
	// Handle parenthesized expressions
	if p.cur.Type == LPAREN {
		p.nextToken()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.cur.Type != RPAREN {
			return nil, fmt.Errorf("expected ) to close parenthesized expression")
		}
		p.nextToken()
		return expr, nil
	}

	if p.cur.Type == IDENT && p.peekIs(LPAREN) {
		funcName := strings.ToUpper(p.cur.Literal)
		p.nextToken()
		p.nextToken()
		var col string
		var parseErr error
		if p.cur.Type == ASTERISK {
			col = "*"
			p.nextToken()
		} else if p.cur.Type == IDENT {
			col, parseErr = p.parseDottedIdentifier()
			if parseErr != nil {
				return nil, fmt.Errorf("inside %s(): %w", funcName, parseErr)
			}
		} else {
			return nil, fmt.Errorf("expected column or * inside %s()", funcName)
		}
		if p.cur.Type != RPAREN {
			return nil, fmt.Errorf("expected ) after argument in %s()", funcName)
		}
		p.nextToken()
		left := funcName + "(" + col + ")"
		// After aggregate: check for IS NULL, IS NOT NULL, IN, NOT IN, or comparison
		return p.parsePostAggregateOp(left)
	}

	// Check for NOT EXISTS
	if p.cur.Type == NOT && p.peek.Type == EXISTS_KW {
		p.nextToken()
		p.nextToken()
		if p.cur.Type != LPAREN {
			return nil, fmt.Errorf("expected ( after NOT EXISTS")
		}
		p.nextToken()
		subquery, err := p.parseSelectQuery()
		if err != nil {
			return nil, fmt.Errorf("NOT EXISTS subquery: %w", err)
		}
		if p.cur.Type != RPAREN {
			return nil, fmt.Errorf("expected ) after NOT EXISTS subquery")
		}
		p.nextToken()
		return &ExistsExpr{Not: true, Subquery: subquery}, nil
	}

	// Check for EXISTS / NOT EXISTS
	if p.cur.Type == EXISTS_KW {
		p.nextToken()
		if p.cur.Type != LPAREN {
			return nil, fmt.Errorf("expected ( after EXISTS")
		}
		p.nextToken()
		subquery, err := p.parseSelectQuery()
		if err != nil {
			return nil, fmt.Errorf("EXISTS subquery: %w", err)
		}
		if p.cur.Type != RPAREN {
			return nil, fmt.Errorf("expected ) after EXISTS subquery")
		}
		p.nextToken()
		return &ExistsExpr{Subquery: subquery}, nil
	}

	if p.cur.Type != IDENT {
		return nil, fmt.Errorf("expected column name in expression, got %s", tokenName(p.cur.Type))
	}
	col, err := p.parseDottedIdentifier()
	if err != nil {
		return nil, err
	}

	// Check for arithmetic expressions (e.g., "age + 5")
	var arithExpr Expression
	if isArithmeticOp(p.cur) {
		arithExpr, err = p.parseArithmeticExpr(&ColumnRef{Name: col})
		if err != nil {
			return nil, err
		}
		col = exprToString(arithExpr)
	}

	// Check for IS NULL, IS NOT NULL
	if p.cur.Type == IS {
		p.nextToken()
		if p.cur.Type == NOT {
			p.nextToken()
			if p.cur.Type != NULL_KEYWORD {
				return nil, fmt.Errorf("expected NULL after IS NOT")
			}
			p.nextToken()
			return &NullTestExpr{Column: col, IsNull: false}, nil
		}
		if p.cur.Type != NULL_KEYWORD {
			return nil, fmt.Errorf("expected NULL after IS")
		}
		p.nextToken()
		return &NullTestExpr{Column: col, IsNull: true}, nil
	}
	// Check for NOT IN / IN
	not := false
	if p.cur.Type == NOT && p.peekIs(IN_KEYWORD) {
		not = true
		p.nextToken()
		p.nextToken()
	} else if p.cur.Type == IN_KEYWORD {
		p.nextToken()
	} else {
		// Regular comparison operator
		if !isComparisonOperator(p.cur.Type) {
			return nil, fmt.Errorf("expected comparison operator after %s, got %s", col, tokenName(p.cur.Type))
		}
		operator := p.cur.Literal
		p.nextToken()
		if p.cur.Type == IDENT {
			rightCol, parseErr := p.parseDottedIdentifier()
			if parseErr != nil {
				return nil, parseErr
			}
			return &ComparisonExpr{Column: col, Operator: operator, RightColumn: rightCol, Expr: arithExpr}, nil
		}
		if p.cur.Type != STRING && p.cur.Type != NUMBER {
			return nil, fmt.Errorf("expected literal on right side of expression, got %s", tokenName(p.cur.Type))
		}
		val := p.cur.Literal
		p.nextToken()
		return &ComparisonExpr{Column: col, Operator: operator, Value: val, Expr: arithExpr}, nil
	}
	return p.parseInExpr(col, not)
}

func (p *Parser) parsePostAggregateOp(left string) (Expression, error) {
	if p.cur.Type == IS {
		p.nextToken()
		if p.cur.Type == NOT {
			p.nextToken()
			if p.cur.Type != NULL_KEYWORD {
				return nil, fmt.Errorf("expected NULL after IS NOT")
			}
			p.nextToken()
			return &NullTestExpr{Column: left, IsNull: false}, nil
		}
		if p.cur.Type != NULL_KEYWORD {
			return nil, fmt.Errorf("expected NULL after IS")
		}
		p.nextToken()
		return &NullTestExpr{Column: left, IsNull: true}, nil
	}
	if p.cur.Type == NOT && p.peekIs(IN_KEYWORD) {
		p.nextToken()
		p.nextToken()
		return p.parseInExpr(left, true)
	}
	if p.cur.Type == IN_KEYWORD {
		p.nextToken()
		return p.parseInExpr(left, false)
	}
	if !isComparisonOperator(p.cur.Type) {
		return nil, fmt.Errorf("expected comparison operator after %s, got %s", left, tokenName(p.cur.Type))
	}
	operator := p.cur.Literal
	p.nextToken()
	if p.cur.Type == IDENT {
		rightCol, parseErr := p.parseDottedIdentifier()
		if parseErr != nil {
			return nil, parseErr
		}
		return &ComparisonExpr{Column: left, Operator: operator, RightColumn: rightCol}, nil
	}
	if p.cur.Type != STRING && p.cur.Type != NUMBER {
		return nil, fmt.Errorf("expected literal on right side of expression, got %s", tokenName(p.cur.Type))
	}
	val := p.cur.Literal
	p.nextToken()
	return &ComparisonExpr{Column: left, Operator: operator, Value: val}, nil
}

func (p *Parser) parseInExpr(col string, not bool) (Expression, error) {
	if p.cur.Type != LPAREN {
		return nil, fmt.Errorf("expected ( after IN")
	}
	p.nextToken()

	// Check for subquery: IN (SELECT ...)
	if p.cur.Type == SELECT {
		subquery, err := p.parseSelectQuery()
		if err != nil {
			return nil, fmt.Errorf("IN subquery: %w", err)
		}
		if p.cur.Type != RPAREN {
			return nil, fmt.Errorf("expected ) after IN subquery")
		}
		p.nextToken()
		return &InExpr{Column: col, Not: not, Subquery: subquery}, nil
	}

	var values []string
	for {
		if p.cur.Type != STRING && p.cur.Type != NUMBER {
			return nil, fmt.Errorf("expected value in IN list, got %s", tokenName(p.cur.Type))
		}
		values = append(values, p.cur.Literal)
		if p.peekIs(COMMA) {
			p.nextToken()
			p.nextToken()
			continue
		}
		p.nextToken()
		break
	}
	if p.cur.Type != RPAREN {
		return nil, fmt.Errorf("expected ) after IN list")
	}
	p.nextToken()
	return &InExpr{Column: col, Not: not, Values: values}, nil
}


func isComparisonOperator(t TokenType) bool {
	switch t {
	case EQUAL, NOT_EQUAL, LT, GT, LTE, GTE, LIKE:
		return true
	default:
		return false
	}
}

func (p *Parser) parseSelectItems() ([]string, []Expression, []AggregateExpr, []WindowExpr, map[string]string, error) {
	colAliases := make(map[string]string)
	if p.cur.Type == ASTERISK {
		return []string{"*"}, nil, nil, nil, colAliases, nil
	}
	var columns []string
	var columnExprs []Expression
	var aggregates []AggregateExpr
	var windowExprs []WindowExpr
	for {
		if p.cur.Type == CASE_KW {
			caseExpr, err := p.parseCaseExpr()
			if err != nil {
				return nil, nil, nil, nil, nil, err
			}
			colKey := fmt.Sprintf("CASE_%d", len(columns))
			columns = append(columns, colKey)
			columnExprs = append(columnExprs, caseExpr)
			aggregates = append(aggregates, AggregateExpr{})
			windowExprs = append(windowExprs, WindowExpr{})
			if p.curIs(AS) || (p.cur.Type == IDENT && len(columns) > 0) {
				if p.curIs(AS) {
					p.nextToken()
				}
				if p.cur.Type == IDENT {
					colAliases[p.cur.Literal] = colKey
				}
			}
			if p.cur.Type == COMMA {
				p.nextToken()
				continue
			}
			break
		}
		if p.cur.Type != IDENT {
			// Literal expressions: STRING or NUMBER in SELECT (e.g., '2026' AS year)
			if p.cur.Type == STRING || p.cur.Type == NUMBER {
				litKey := fmt.Sprintf("LITERAL_%d", len(columns))
				columnExprs = append(columnExprs, &LiteralExpr{Value: p.cur.Literal})
				columns = append(columns, litKey)
				aggregates = append(aggregates, AggregateExpr{})
				windowExprs = append(windowExprs, WindowExpr{})
				if p.peekIs(AS) {
					p.nextToken()
					p.nextToken()
					if p.cur.Type == IDENT {
						colAliases[p.cur.Literal] = litKey
					}
				}
				if p.cur.Type == COMMA {
					p.nextToken()
					continue
				}
				break
			}
			return nil, nil, nil, nil, nil, fmt.Errorf("expected column name or aggregate function, got %s", tokenName(p.cur.Type))
		}
		ident := p.cur.Literal
		if p.peekIs(LPAREN) {
			funcName := strings.ToUpper(ident)
			distinct := false
			p.nextToken()
			p.nextToken()
			if p.cur.Type == DISTINCT {
				distinct = true
				p.nextToken()
			}
			var args []string
			if p.cur.Type == ASTERISK {
				args = []string{"*"}
				p.nextToken()
			} else if p.cur.Type == IDENT {
				col, parseErr := p.parseDottedIdentifier()
				if parseErr != nil {
					return nil, nil, nil, nil, nil, fmt.Errorf("inside %s(): %w", funcName, parseErr)
				}
				args = []string{col}
			} else if p.cur.Type == NUMBER || p.cur.Type == STRING {
				args = []string{p.cur.Literal}
				p.nextToken()
			} else if p.cur.Type == RPAREN {
				// zero-arg function like ROW_NUMBER()
			} else {
				return nil, nil, nil, nil, nil, fmt.Errorf("expected column name or * inside %s()", funcName)
			}
			if p.cur.Type != RPAREN {
				return nil, nil, nil, nil, nil, fmt.Errorf("expected ) after argument in %s()", funcName)
			}
			argStr := ""
			if len(args) > 0 {
				argStr = args[0]
			}
			distinctPrefix := ""
			if distinct {
				distinctPrefix = "DISTINCT "
			}
			colKey := funcName + "(" + distinctPrefix + argStr + ")"

			if p.peekIs(OVER) {
				// Window function: OVER (PARTITION BY ... ORDER BY ...)
				p.nextToken() // consume )
				p.nextToken() // consume OVER
				windowExpr, err := p.parseWindowSpec(funcName, args)
				if err != nil {
					return nil, nil, nil, nil, nil, err
				}
				windowExprs = append(windowExprs, *windowExpr)
				columns = append(columns, colKey)
				columnExprs = append(columnExprs, nil)
				if p.curIs(AS) {
					p.nextToken()
					if p.curIs(IDENT) {
						colAliases[p.cur.Literal] = colKey
					}
				}
			} else {
				aggregates = append(aggregates, AggregateExpr{FuncName: funcName, Column: argStr, Distinct: distinct})
				columns = append(columns, colKey)
				columnExprs = append(columnExprs, nil)
				if p.peekIs(AS) {
					p.nextToken()
					if p.peekIs(IDENT) {
						p.nextToken()
						colAliases[p.cur.Literal] = colKey
					}
				}
			}
		} else {
			colName := p.cur.Literal
			hasExpr := false
			if p.peekIs(DOT) {
				p.nextToken()
				p.nextToken()
				if p.cur.Type != IDENT {
					return nil, nil, nil, nil, nil, fmt.Errorf("expected column name after .")
				}
				colName = colName + "." + p.cur.Literal
			}
			if isArithmeticOp(p.peek) {
				p.nextToken()
				expr, err := p.parseArithmeticExpr(&ColumnRef{Name: colName})
				if err != nil {
					return nil, nil, nil, nil, nil, err
				}
				colName = exprToString(expr)
				columns = append(columns, colName)
				columnExprs = append(columnExprs, expr)
				hasExpr = true
				// After parseArithmeticExpr, p.cur is the first non-arithmetic token (possibly AS)
				if p.curIs(AS) {
					p.nextToken()
					if p.cur.Type == IDENT {
						colAliases[p.cur.Literal] = colName
						p.nextToken()
					}
				}
			} else {
				if !hasExpr {
					columns = append(columns, colName)
					columnExprs = append(columnExprs, nil)
				}
				if p.peekIs(AS) {
					p.nextToken()
					if p.peekIs(IDENT) {
						p.nextToken()
						colAliases[p.cur.Literal] = colName
					}
				}
			}
		}
		if p.curIs(COMMA) {
			p.nextToken()
			continue
		}
		if p.peekIs(COMMA) {
			p.nextToken()
			p.nextToken()
			continue
		}
		break
	}
	return columns, columnExprs, aggregates, windowExprs, colAliases, nil
}

func (p *Parser) parseWindowSpec(funcName string, args []string) (*WindowExpr, error) {
	if p.cur.Type != LPAREN {
		return nil, fmt.Errorf("expected ( after OVER")
	}
	p.nextToken()

	var partitionBy []string
	var orderBy []SortOrder

	if p.cur.Type == PARTITION {
		if err := p.expectPeek(BY); err != nil {
			return nil, err
		}
		p.nextToken()
		var err error
		partitionBy, err = p.parseDottedList()
		if err != nil {
			return nil, err
		}
	}

	if p.cur.Type == ORDER {
		if err := p.expectPeek(BY); err != nil {
			return nil, err
		}
		p.nextToken()
		var err error
		orderBy, err = p.parseOrderByList()
		if err != nil {
			return nil, err
		}
	}

	if p.cur.Type != RPAREN {
		return nil, fmt.Errorf("expected ) after window specification, got %s", tokenName(p.cur.Type))
	}
	p.nextToken()

	return &WindowExpr{
		FuncName:    funcName,
		Args:        args,
		PartitionBy: partitionBy,
		OrderBy:     orderBy,
	}, nil
}

const (
	precLowest = iota
	precTerm
	precFactor
)

func arithmeticPrecedence(tokType TokenType) int {
	switch tokType {
	case PLUS, MINUS:
		return precTerm
	case ASTERISK, DIV:
		return precFactor
	default:
		return precLowest
	}
}

func isArithmeticOp(tok Token) bool {
	return tok.Type == PLUS || tok.Type == MINUS || tok.Type == ASTERISK || tok.Type == DIV
}

func (p *Parser) parseArithmeticExpr(left Expression) (Expression, error) {
	if left == nil {
		var err error
		left, err = p.parseArithmeticPrimary()
		if err != nil {
			return nil, err
		}
	}
	return p.parseArithmeticBinary(left, precLowest)
}

func (p *Parser) parseArithmeticBinary(left Expression, minPrec int) (Expression, error) {
	for isArithmeticOp(p.cur) {
		opPrec := arithmeticPrecedence(p.cur.Type)
		if opPrec < minPrec {
			return left, nil
		}
		op := p.cur.Literal
		p.nextToken()
		right, err := p.parseArithmeticPrimary()
		if err != nil {
			return nil, err
		}
		for isArithmeticOp(p.cur) && arithmeticPrecedence(p.cur.Type) > opPrec {
			right, err = p.parseArithmeticBinary(right, arithmeticPrecedence(p.cur.Type))
			if err != nil {
				return nil, err
			}
		}
		left = &BinaryExpr{Left: left, Operator: op, Right: right}
	}
	return left, nil
}

func (p *Parser) parseArithmeticPrimary() (Expression, error) {
	if p.cur.Type == NUMBER || p.cur.Type == STRING {
		val := p.cur.Literal
		p.nextToken()
		return &LiteralExpr{Value: val}, nil
	}
	if p.cur.Type == CASE_KW {
		return p.parseCaseExpr()
	}
	if p.cur.Type == IDENT {
		col, err := p.parseDottedIdentifier()
		if err != nil {
			return nil, err
		}
		return &ColumnRef{Name: col}, nil
	}
	if p.cur.Type == LPAREN {
		p.nextToken()
		expr, err := p.parseArithmeticExpr(nil)
		if err != nil {
			return nil, err
		}
		if p.cur.Type != RPAREN {
			return nil, fmt.Errorf("expected ) after subexpression")
		}
		p.nextToken()
		return expr, nil
	}
	return nil, fmt.Errorf("expected arithmetic operand, got %s", tokenName(p.cur.Type))
}

func (p *Parser) parseCaseExpr() (*CaseExpr, error) {
	p.nextToken() // skip CASE
	expr := &CaseExpr{}

	for p.cur.Type == WHEN_KW {
		p.nextToken()
		cond, err := p.parseExpression()
		if err != nil {
			return nil, fmt.Errorf("CASE WHEN condition: %w", err)
		}
		if p.cur.Type != THEN_KW {
			return nil, fmt.Errorf("expected THEN after CASE WHEN condition, got %s", tokenName(p.cur.Type))
		}
		p.nextToken()
		res, err := p.parseArithmeticExpr(nil)
		if err != nil {
			return nil, fmt.Errorf("CASE THEN result: %w", err)
		}
		expr.Branches = append(expr.Branches, CaseBranch{Condition: cond, Result: res})
	}

	if p.cur.Type == ELSE_KW {
		p.nextToken()
		elseExpr, err := p.parseArithmeticExpr(nil)
		if err != nil {
			return nil, fmt.Errorf("CASE ELSE: %w", err)
		}
		expr.Else = elseExpr
	}

	if p.cur.Type != END_KW {
		return nil, fmt.Errorf("expected END at end of CASE expression, got %s", tokenName(p.cur.Type))
	}
	p.nextToken()
	return expr, nil
}

func exprToString(expr Expression) string {
	switch v := expr.(type) {
	case *ColumnRef:
		return v.Name
	case *BinaryExpr:
		return "(" + exprToString(v.Left) + " " + v.Operator + " " + exprToString(v.Right) + ")"
	case *ComparisonExpr:
		if v.RightColumn != "" {
			return v.Column + " " + v.Operator + " " + v.RightColumn
		}
		return v.Column + " " + v.Operator + " " + v.Value
	case *LiteralExpr:
		return v.Value
	default:
		return ""
	}
}

func (p *Parser) parseDottedList() ([]string, error) {
	var items []string
	for {
		item, err := p.parseDottedIdentifier()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		if p.cur.Type == COMMA {
			p.nextToken()
			continue
		}
		break
	}
	return items, nil
}

func (p *Parser) parseOrderByList() ([]SortOrder, error) {
	var items []SortOrder
	for {
		col, err := p.parseDottedIdentifier()
		if err != nil {
			return nil, err
		}
		desc := false
		if strings.EqualFold(p.cur.Literal, "ASC") {
			p.nextToken()
		} else if strings.EqualFold(p.cur.Literal, "DESC") {
			desc = true
			p.nextToken()
		}
		items = append(items, SortOrder{Column: col, Desc: desc})
		if p.cur.Type == COMMA {
			p.nextToken()
			continue
		}
		break
	}
	return items, nil
}

func (p *Parser) parseDottedIdentifier() (string, error) {
	if p.cur.Type != IDENT {
		return "", fmt.Errorf("expected identifier, got %s", tokenName(p.cur.Type))
	}
	result := p.cur.Literal
	p.nextToken()
	for p.curIs(DOT) {
		p.nextToken()
		if p.cur.Type != IDENT {
			return "", fmt.Errorf("expected identifier after .")
		}
		result += "." + p.cur.Literal
		p.nextToken()
	}
	return result, nil
}

func (p *Parser) parseValuesClause() ([][]string, error) {
	if p.cur.Type != LPAREN {
		return nil, fmt.Errorf("expected '(' after VALUES, got %s", tokenName(p.cur.Type))
	}
	p.nextToken() // consume '('

	var rows [][]string
	for {
		var row []string
		for {
			if p.cur.Type != STRING && p.cur.Type != NUMBER {
				return nil, fmt.Errorf("expected value in VALUES clause, got %s", tokenName(p.cur.Type))
			}
			row = append(row, p.cur.Literal)
			p.nextToken()
			if p.cur.Type != COMMA {
				break
			}
			p.nextToken() // consume ','
		}
		rows = append(rows, row)

		if p.cur.Type != RPAREN {
			return nil, fmt.Errorf("expected ')' after VALUES row, got %s", tokenName(p.cur.Type))
		}
		p.nextToken() // consume ')'

		if p.cur.Type != COMMA {
			break
		}
		p.nextToken() // consume ','
		if p.cur.Type != LPAREN {
			return nil, fmt.Errorf("expected '(' after ',' in VALUES clause, got %s", tokenName(p.cur.Type))
		}
		p.nextToken() // consume '('
	}
	return rows, nil
}

func removeComments(sql string) string {
	lines := strings.Split(sql, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}
