package parser

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/localvar/xuandb/pkg/query/ast"
)

func Parse(input string) (ast.Statement, error) {
	slog.Debug("parse query", slog.String("input", input))

	errs := make([]string, 0)
	l := NewLexer(strings.NewReader(input))
	l.ReportError = func(msg string) {
		errs = append(errs, msg)
	}

	if yyParse(l) == 0 {
		return l.Result, nil
	}

	msg := strings.Join(errs, "\n")
	slog.Debug("parse error", slog.String("error", msg))
	return nil, errors.New(msg)
}
