package parser

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/localvar/xuandb/pkg/utils"
)

// Lexer implements a lexer for the parser.
type Lexer struct {
	Scanner
	LogError func(msg string)
}

// NewLexer creates and returns a new lexer with source 'src'.
func NewLexer(src io.Reader) *Lexer {
	l := &Lexer{}
	l.Scanner.Error = func(s *Scanner, msg string) {
		l.Error(msg)
	}
	l.Scanner.Init(src)
	return l
}

func (l *Lexer) parseIdent(lval *yySymType) int {
	tt := l.TokenText()
	if utt := strings.ToUpper(tt); utt == "TRUE" {
		lval.bool = true
		return VAL_BOOL
	} else if utt == "FALSE" {
		lval.bool = false
		return VAL_BOOL
	} else if id, ok := keywords[utt]; ok {
		return id
	} else {
		lval.str = tt
		return IDENT
	}
}

func (l *Lexer) parseInt(lval *yySymType, hasScanErr bool) int {
	tt := l.TokenText()
	if hasScanErr {
		lval.str = tt
		return ERR_TOKEN
	}

	v, err := strconv.ParseUint(tt, 0, 64)
	if err == nil {
		lval.int = v
		return VAL_INT
	}

	l.Error(errors.Unwrap(err).Error())
	lval.str = tt
	return ERR_TOKEN
}

func (l *Lexer) parseFloat(lval *yySymType, hasScanErr bool) int {
	tt := l.TokenText()
	if hasScanErr {
		lval.str = tt
		return ERR_TOKEN
	}

	v, err := strconv.ParseFloat(tt, 64)
	if err == nil {
		lval.float = v
		return VAL_FLT
	}

	l.Error(errors.Unwrap(err).Error())
	lval.str = tt
	return ERR_TOKEN
}

func (l *Lexer) parseDuration(lval *yySymType, hasScanErr bool) int {
	tt := l.TokenText()
	if hasScanErr {
		lval.str = tt
		return ERR_TOKEN
	}

	v, err := utils.ParseDuration(tt)
	if err == nil {
		lval.int = uint64(v)
		return VAL_DURATION
	}

	l.Error(err.Error())
	lval.str = tt
	return ERR_TOKEN
}

// Lex implements method Lex of interface yyLexer.
func (l *Lexer) Lex(lval *yySymType) int {
	for {
		errCount := l.ErrorCount
		sr := l.Scan()
		errCount = l.ErrorCount - errCount

		switch sr {
		case ScanResultEOF:
			return 0

		case ScanResultComment:
			continue

		case ScanResultIdent:
			return l.parseIdent(lval)

		case ScanResultQuotedIdent:
			lval.str = unescape(l.TokenText(), '"')
			return IDENT

		case ScanResultInt:
			return l.parseInt(lval, errCount > 0)

		case ScanResultFloat:
			return l.parseFloat(lval, errCount > 0)

		case ScanResultDuration:
			return l.parseDuration(lval, errCount > 0)

		case ScanResultString:
			lval.str = unescape(l.TokenText(), '\'')
			return VAL_STR

		case ScanResultRawString:
			tt := l.TokenText()[1:]
			if tt[len(tt)-1] == '`' {
				tt = tt[:len(tt)-1]
			}
			lval.str = tt
			return VAL_STR

		case '+':
			return OP_ADD

		case '-':
			return OP_SUB

		case '*':
			return OP_MUL

		case '/':
			return OP_DIV

		case '%':
			return OP_MOD

		case '|':
			if l.Peek() == '|' {
				l.Next()
				return OP_OR
			}
			return OP_BITWISE_OR

		case '&':
			if l.Peek() == '&' {
				l.Next()
				return OP_AND
			}
			return OP_BITWISE_AND

		case '^':
			return OP_BITWISE_XOR

		case '~':
			return OP_BITWISE_NOT

		case '!':
			if ch := l.Peek(); ch == '=' {
				l.Next()
				return OP_NOT_EQU
			} else if ch == '~' {
				l.Next()
				return OP_NOT_MATCH
			}
			return OP_NOT

		case '=':
			if l.Peek() == '~' {
				l.Next()
				return OP_MATCH
			}
			return OP_EQU

		case '>':
			if ch := l.Peek(); ch == '=' {
				l.Next()
				return OP_GTE
			} else if ch == '>' {
				l.Next()
				return OP_RSHIFT
			}
			return OP_GT

		case '<':
			if ch := l.Peek(); ch == '=' {
				l.Next()
				return OP_LTE
			} else if ch == '<' {
				l.Next()
				return OP_LSHIFT
			} else if ch == '>' {
				l.Next()
				return OP_NOT_EQU
			}
			return OP_LT

		default:
			return int(sr)
		}
	}
}

// Error implements method Error of interface yyLexer.
func (l *Lexer) Error(msg string) {
	if l.LogError == nil {
		return
	}

	s := &l.Scanner
	pos := s.Position
	if !pos.IsValid() {
		pos = s.Pos()
	}

	l.LogError(fmt.Sprintf("%s: %s: %s", pos, s.TokenText(), msg))
}

// decodeDigits decodes the first 'n' digits of 'str' to a rune, it returns
// the result rune and the number of digits that were actually decoded.
func decodeDigits(str string, base, n int) (rune, int) {
	var val, l int
	for l < n {
		if l >= len(str) {
			break
		}
		v := digitVal(rune(str[l]))
		if v >= base {
			break
		}
		val = val*base + v
		l++
	}

	if l == 0 {
		return utf8.RuneError, 0
	}

	// return what we have unescaped
	return rune(val), l
}

// unescapeRune unescapes the first rune of the input string.
func unescapeRune(str string, quote byte) (rune, int) {
	if len(str) == 0 {
		return utf8.RuneError, 0
	}

	switch str[0] {
	case 'a':
		return '\a', 1
	case 'b':
		return '\b', 1
	case 'f':
		return '\f', 1
	case 'n':
		return '\n', 1
	case 'r':
		return '\r', 1
	case 't':
		return '\t', 1
	case 'v':
		return '\v', 1
	case '\\':
		return '\\', 1
	case quote:
		return rune(quote), 1
	case '0', '1', '2', '3', '4', '5', '6', '7':
		return decodeDigits(str, 8, 3)
	case 'x':
		r, l := decodeDigits(str[1:], 16, 2)
		return r, l + 1
	case 'u':
		r, l := decodeDigits(str[1:], 16, 4)
		return r, l + 1
	case 'U':
		r, l := decodeDigits(str[1:], 16, 8)
		return r, l + 1
	}

	return utf8.RuneError, 0
}

// unescape unescapes the input string. The first byte of the input is 'quote',
// while it may and may not have a trailing 'quote'.
func unescape(str string, quote byte) string {
	// remove the leading quote, we are sure that len(str) > 0
	str = str[1:]

	var sb strings.Builder
	// the unescaped string is shorter than the input in most of the cases
	sb.Grow(len(str))

	// s is the start position of the content which need to be copied to the
	// result, e is the end position.
	s, e := 0, 0
	for e < len(str) {
		c := str[e]

		// quote marks the end of the input string
		if c == quote {
			if e < len(str)-1 {
				panic(string(quote) + " should be the last rune.")
			}
			break
		}

		// skip non-escape sequence
		if c != '\\' {
			e++
			continue
		}

		// copy content before '\' to the result buffer
		if e > s {
			sb.WriteString(str[s:e])
		}

		// escape one rune and write it to the result buffer
		e++
		r, l := unescapeRune(str[e:], quote)
		sb.WriteRune(r)
		e += l

		// update the start position
		s = e
	}

	if e > s {
		sb.WriteString(str[s:e])
	}

	return sb.String()
}
