package parser

type Parser struct {
	s *Scanner
	y yyParser
}

func New() *Parser {
	return &Parser{}
}

func Parse(input string) (Statement, error) {
	//p := New()
	return nil, nil
	//return p.Parse(input)
}
