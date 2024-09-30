package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/localvar/xuandb/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func tokenName(id int) string {
	if id < IDENT {
		return string(rune(id))
	}
	if id > ERR_TOKEN {
		return fmt.Sprintf("tok%d", id)
	}
	return yyToknames[id-IDENT+3]
}

func TestUnescape(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		input  string
		quote  byte
		expect string
	}{
		{`"abcdefg"`, '"', "abcdefg"},
		{`"\oabcdefg"`, '"', "\uFFFDoabcdefg"},
		{`"\a\b\f\n\r\t\v\\\"bcdefg"`, '"', "\a\b\f\n\r\t\v\\\"bcdefg"},
		{`"abcdefg\`, '"', "abcdefg\uFFFD"},
		{`"abcd\007efg"`, '"', "abcd\aefg"},
		{`"abcd\007efg"`, '"', "abcd\aefg"},
		{`"abcd\x07efg"`, '"', "abcd\aefg"},
		{`"abcd\u0007efg"`, '"', "abcd\aefg"},
		{`"abcd\U00000007efg"`, '"', "abcd\aefg"},
		{`"abcd\U00000007e\nfg"`, '"', "abcd\ae\nfg"},
		{`"abcd\U0000007z\nfg"`, '"', "abcd\az\nfg"},
		{`"abcd\Uzfg"`, '"', "abcd\uFFFDzfg"},
		{`"abcd\u007`, '"', "abcd\a"},
	}

	for i, c := range cases {
		result := unescape(c.input, c.quote)
		assert.Equal(c.expect, result, fmt.Sprintf("case %d", i+1))
	}

	assert.Panics(func() {
		unescape("'abcdefg'defghi'", '\'')
	})
}

func checkBool(t *testing.T, l *Lexer, val bool) {
	var lval yySymType
	if id := l.Lex(&lval); id != VAL_BOOL {
		t.Errorf(`got token %q, want "VAL_BOOL"`, tokenName(id))
	} else if lval.bool != val {
		t.Errorf("get boolean value %v, want %v", lval.bool, val)
	}
}

func checkInt(t *testing.T, l *Lexer, val uint64) {
	var lval yySymType
	if id := l.Lex(&lval); id != VAL_INT {
		t.Errorf(`got token %q, want "VAL_INT"`, tokenName(id))
	} else if lval.int != val {
		t.Errorf("get boolean value %v, want %v", lval.int, val)
	}
}

func checkFloat(t *testing.T, l *Lexer, val float64) {
	var lval yySymType
	if id := l.Lex(&lval); id != VAL_FLT {
		t.Errorf(`got token %q, want "VAL_FLT"`, tokenName(id))
	} else if lval.float != val {
		t.Errorf("get float value %v, want %v", lval.float, val)
	}
}

func checkDuration(t *testing.T, l *Lexer, val time.Duration) {
	var lval yySymType
	if id := l.Lex(&lval); id != VAL_DURATION {
		t.Errorf(`got token %q, want "VAL_DURATION"`, tokenName(id))
	} else if lval.int != uint64(val) {
		t.Errorf("get duration value %v, want %v", utils.FormatDuration(time.Duration(lval.int)), utils.FormatDuration(val))
	}
}

func checkToken(t *testing.T, l *Lexer, want int) {
	var lval yySymType
	if id := l.Lex(&lval); id != want {
		t.Errorf(`got token %q, want %q`, tokenName(id), tokenName(want))
	}
}

func checkText(t *testing.T, l *Lexer, tokWant int, textWant string) {
	var lval yySymType
	if tok := l.Lex(&lval); tok != tokWant {
		t.Errorf(`got token %q, want %q`, tokenName(tok), tokenName(tokWant))
	}
	if lval.str != textWant {
		t.Errorf(`got token text %q, want %q`, lval.str, textWant)
	}
}

func TestLexer(t *testing.T) {
	src := `
-- this is line comment
/* this is
 block comment
*/
TRUE FALSE CREATE IDENT
"CREATE"
0b123 123 99999999999999999999999999999999
1e+d 123.5 1.2e9999999
1d2k 1d2h 99999999999999999999999w20d
'this is a string'` +
		"`this is a raw string`" +
		`+-*/%|||&&&^~!=!~! ==~>=>>><=<<<><@
	`
	l := NewLexer(strings.NewReader(src))

	checkBool(t, l, true)
	checkBool(t, l, false)
	checkToken(t, l, CREATE)
	checkText(t, l, IDENT, "IDENT")
	checkText(t, l, IDENT, "CREATE")
	checkText(t, l, ERR_TOKEN, "0b123")
	checkInt(t, l, 123)
	checkText(t, l, ERR_TOKEN, "99999999999999999999999999999999")
	checkText(t, l, ERR_TOKEN, "1e+d")
	checkFloat(t, l, 123.5)
	checkText(t, l, ERR_TOKEN, "1.2e9999999")
	checkText(t, l, ERR_TOKEN, "1d2k")
	checkDuration(t, l, 26*time.Hour)
	checkText(t, l, ERR_TOKEN, "99999999999999999999999w20d")
	checkText(t, l, VAL_STR, "this is a string")
	checkText(t, l, VAL_STR, "this is a raw string")
	checkToken(t, l, OP_ADD)
	checkToken(t, l, OP_SUB)
	checkToken(t, l, OP_MUL)
	checkToken(t, l, OP_DIV)
	checkToken(t, l, OP_MOD)
	checkToken(t, l, OP_OR)
	checkToken(t, l, OP_BITWISE_OR)
	checkToken(t, l, OP_AND)
	checkToken(t, l, OP_BITWISE_AND)
	checkToken(t, l, OP_BITWISE_XOR)
	checkToken(t, l, OP_BITWISE_NOT)
	checkToken(t, l, OP_NOT_EQU)
	checkToken(t, l, OP_NOT_MATCH)
	checkToken(t, l, OP_NOT)
	checkToken(t, l, OP_EQU)
	checkToken(t, l, OP_MATCH)
	checkToken(t, l, OP_GTE)
	checkToken(t, l, OP_RSHIFT)
	checkToken(t, l, OP_GT)
	checkToken(t, l, OP_LTE)
	checkToken(t, l, OP_LSHIFT)
	checkToken(t, l, OP_NOT_EQU)
	checkToken(t, l, OP_LT)
	checkToken(t, l, '@')
	checkToken(t, l, 0)

	l = NewLexer(strings.NewReader("9dw"))
	l.ReportError = func(msg string) {
		const want = "<input>:1:1: : abc"
		if msg != want {
			t.Errorf("error msg = %q, want = %q", msg, want)
		}
	}
	l.Error("abc")

	l.ReportError = func(msg string) {
		const want = "<input>:1:1: 9dw: extra character after duration"
		if msg != want {
			t.Errorf("error msg = %q, want = %q", msg, want)
		}
	}
	l.Lex(&yySymType{})
}
