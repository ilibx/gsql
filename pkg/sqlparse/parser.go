package sqlparse

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
	case SEMICOLON:
		return ";"
	case CREATE, TABLE, WITH, AS, INSERT, OVERWRITE, SELECT, FROM, WHERE, ORDER, BY, LIMIT:
		return strings.ToUpper(t.String())
	default:
		return "UNKNOWN"
	}
}

func (t TokenType) String() string {
	switch t {
	case CREATE:
		return "CREATE"
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
		l.readChar()
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
	return &CreateTableStmt{
		Name:        tableName,
		Columns:     columns,
		WithOptions: options,
	}, nil
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
	if err := p.expectPeek(OVERWRITE); err != nil {
		return nil, err
	}
	if err := p.expectPeek(TABLE); err != nil {
		return nil, err
	}
	if err := p.expectPeek(IDENT); err != nil {
		return nil, fmt.Errorf("expected target table name after INSERT OVERWRITE TABLE")
	}
	tableName := p.cur.Literal
	p.nextToken()
	query, err := p.parseSelectQuery()
	if err != nil {
		return nil, err
	}
	return &InsertOverwriteStmt{TableName: tableName, Query: query}, nil
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
	columns, err := p.parseColumnList()
	if err != nil {
		return nil, err
	}
	if err := p.expectPeek(FROM); err != nil {
		return nil, err
	}
	if err := p.expectPeek(IDENT); err != nil {
		return nil, fmt.Errorf("expected table name after FROM")
	}
	tableName := p.cur.Literal
	p.nextToken()

	var where Expression
	var orderBy []string
	var limit int
	if p.cur.Type == WHERE {
		p.nextToken()
		where, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
		p.nextToken()
	}
	if p.cur.Type == ORDER {
		if err := p.expectPeek(BY); err != nil {
			return nil, err
		}
		p.nextToken()
		orderBy, err = p.parseIdentifierList()
		if err != nil {
			return nil, err
		}
		p.nextToken()
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
		p.nextToken()
	}
	if p.cur.Type != SEMICOLON && p.cur.Type != EOF && p.cur.Type != RPAREN {
		return nil, fmt.Errorf("unexpected token %s after SELECT query", tokenName(p.cur.Type))
	}

	return &SelectQuery{
		CTEs:    ctes,
		Columns: columns,
		Table:   tableName,
		Where:   where,
		OrderBy: orderBy,
		Limit:   limit,
	}, nil
}

func (p *Parser) parseCTEList() ([]CTE, error) {
	if p.cur.Type != WITH {
		return nil, nil
	}
	var ctes []CTE
	for {
		if err := p.expectPeek(IDENT); err != nil {
			return nil, err
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
	if p.cur.Type != IDENT {
		return nil, fmt.Errorf("expected expression column name, got %s", tokenName(p.cur.Type))
	}
	left := p.cur.Literal
	if !isComparisonOperator(p.peek.Type) {
		return nil, fmt.Errorf("expected comparison operator after %s, got %s", left, tokenName(p.peek.Type))
	}
	p.nextToken()
	operator := p.cur.Literal
	p.nextToken()
	if p.cur.Type != STRING && p.cur.Type != NUMBER {
		return nil, fmt.Errorf("expected literal on right side of expression, got %s", tokenName(p.cur.Type))
	}
	return &ComparisonExpr{Column: left, Operator: operator, Value: p.cur.Literal}, nil
}

func isComparisonOperator(t TokenType) bool {
	switch t {
	case EQUAL, NOT_EQUAL, LT, GT, LTE, GTE, LIKE:
		return true
	default:
		return false
	}
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
