package parser

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTokenString(t *testing.T) {
	if ts := TokenString('a'); ts != `"a"` {
		t.Errorf("got %q; want %q", ts, `"a"`)
	}
	for tok := rune(ScanResultEOF); tok >= ScanResultRawString; tok-- {
		if ts := TokenString(tok); ts != tokenString[tok] {
			t.Errorf("got %q; want %q", ts, tokenString[tok])
		}
	}
}

func TestLitName(t *testing.T) {
	want := "decimal literal"
	if n := litname(0); n != want {
		t.Errorf("got %q; want %q", n, want)
	}
	want = "hexadecimal literal"
	if n := litname('x'); n != want {
		t.Errorf("got %q; want %q", n, want)
	}
	want = "octal literal"
	if n := litname('o'); n != want {
		t.Errorf("got %q; want %q", n, want)
	}
	if n := litname('0'); n != want {
		t.Errorf("got %q; want %q", n, want)
	}
	want = "binary literal"
	if n := litname('b'); n != want {
		t.Errorf("got %q; want %q", n, want)
	}
}

// A StringReader delivers its data one string segment at a time via Read.
type StringReader struct {
	data []string
	step int
}

func (r *StringReader) Read(p []byte) (n int, err error) {
	if r.step < len(r.data) {
		s := r.data[r.step]
		n = copy(p, s)
		r.step++
	} else {
		err = io.EOF
	}
	return
}

func readRuneSegments(t *testing.T, segments []string) {
	got := ""
	want := strings.Join(segments, "")
	s := new(Scanner).Init(&StringReader{data: segments})
	for {
		ch := s.Next()
		if ch == ScanResultEOF {
			break
		}
		got += string(ch)
	}
	if got != want {
		t.Errorf("segments=%v got=%s want=%s", segments, got, want)
	}
}

var segmentList = [][]string{
	{},
	{""},
	{"日", "本語"},
	{"\u65e5", "\u672c", "\u8a9e"},
	{"\U000065e5", " ", "\U0000672c", "\U00008a9e"},
	{"\xe6", "\x97\xa5\xe6", "\x9c\xac\xe8\xaa\x9e"},
	{"Hello", ", ", "World", "!"},
	{"Hello", ", ", "", "World", "!"},
}

func TestNext(t *testing.T) {
	for _, s := range segmentList {
		readRuneSegments(t, s)
	}
}

type token struct {
	tok  rune
	text string
}

const f100 = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

var tokenList = []token{
	{ScanResultComment, "-- line comments"},
	{ScanResultComment, "--"},
	{ScanResultComment, "----"},
	{ScanResultComment, "-- comment"},
	{ScanResultComment, "-- /* comment */"},
	{ScanResultComment, "-- -- comment --"},
	{ScanResultComment, "--" + f100},

	{ScanResultComment, "-- general comments"},
	{ScanResultComment, "/**/"},
	{ScanResultComment, "/***/"},
	{ScanResultComment, "/* comment */"},
	{ScanResultComment, "/* // comment */"},
	{ScanResultComment, "/* /* comment */"},
	{ScanResultComment, "/*\n comment\n*/"},
	{ScanResultComment, "/*" + f100 + "*/"},

	{ScanResultComment, "-- identifiers"},
	{ScanResultIdent, "a"},
	{ScanResultIdent, "a0"},
	{ScanResultIdent, "foobar"},
	{ScanResultIdent, "abc123"},
	{ScanResultIdent, "LGTM"},
	{ScanResultIdent, "_"},
	{ScanResultIdent, "_abc123"},
	{ScanResultIdent, "abc123_"},
	{ScanResultIdent, "_abc_123_"},
	{ScanResultIdent, "_äöü"},
	{ScanResultIdent, "_本"},
	{ScanResultIdent, "äöü"},
	{ScanResultIdent, "本"},
	{ScanResultIdent, "a۰۱۸"},
	{ScanResultIdent, "foo६४"},
	{ScanResultIdent, "bar９８７６"},
	{ScanResultIdent, f100},

	{ScanResultComment, "-- quoted identifiers"},
	{ScanResultQuotedIdent, `" "`},
	{ScanResultQuotedIdent, `"a"`},
	{ScanResultQuotedIdent, `"本"`},
	{ScanResultQuotedIdent, `"\a"`},
	{ScanResultQuotedIdent, `"\b"`},
	{ScanResultQuotedIdent, `"\f"`},
	{ScanResultQuotedIdent, `"\n"`},
	{ScanResultQuotedIdent, `"\r"`},
	{ScanResultQuotedIdent, `"\t"`},
	{ScanResultQuotedIdent, `"\v"`},
	{ScanResultQuotedIdent, `"\""`},
	{ScanResultQuotedIdent, `"\000"`},
	{ScanResultQuotedIdent, `"\777"`},
	{ScanResultQuotedIdent, `"\x00"`},
	{ScanResultQuotedIdent, `"\xff"`},
	{ScanResultQuotedIdent, `"\u0000"`},
	{ScanResultQuotedIdent, `"\ufA16"`},
	{ScanResultQuotedIdent, `"\U00000000"`},
	{ScanResultQuotedIdent, `"\U0000ffAB"`},
	{ScanResultQuotedIdent, `"` + f100 + `"`},

	{ScanResultComment, "-- decimal ints"},
	{ScanResultInt, "0"},
	{ScanResultInt, "1"},
	{ScanResultInt, "9"},
	{ScanResultInt, "42"},
	{ScanResultInt, "1234567890"},

	{ScanResultComment, "-- octal ints"},
	{ScanResultInt, "00"},
	{ScanResultInt, "01"},
	{ScanResultInt, "07"},
	{ScanResultInt, "042"},
	{ScanResultInt, "01234567"},

	{ScanResultComment, "-- hexadecimal ints"},
	{ScanResultInt, "0x0"},
	{ScanResultInt, "0x1"},
	{ScanResultInt, "0xf"},
	{ScanResultInt, "0x42"},
	{ScanResultInt, "0x123456789abcDEF"},
	{ScanResultInt, "0x" + f100},
	{ScanResultInt, "0X0"},
	{ScanResultInt, "0X1"},
	{ScanResultInt, "0XF"},
	{ScanResultInt, "0X42"},
	{ScanResultInt, "0X123456789abcDEF"},
	{ScanResultInt, "0X" + f100},

	{ScanResultComment, "-- floats"},
	{ScanResultFloat, "0."},
	{ScanResultFloat, "1."},
	{ScanResultFloat, "42."},
	{ScanResultFloat, "01234567890."},
	{ScanResultFloat, ".0"},
	{ScanResultFloat, ".1"},
	{ScanResultFloat, ".42"},
	{ScanResultFloat, ".0123456789"},
	{ScanResultFloat, "0.0"},
	{ScanResultFloat, "1.0"},
	{ScanResultFloat, "42.0"},
	{ScanResultFloat, "01234567890.0"},
	{ScanResultFloat, "0e0"},
	{ScanResultFloat, "1e0"},
	{ScanResultFloat, "42e0"},
	{ScanResultFloat, "01234567890e0"},
	{ScanResultFloat, "0E0"},
	{ScanResultFloat, "1E0"},
	{ScanResultFloat, "42E0"},
	{ScanResultFloat, "01234567890E0"},
	{ScanResultFloat, "0e+10"},
	{ScanResultFloat, "1e-10"},
	{ScanResultFloat, "42e+10"},
	{ScanResultFloat, "01234567890e-10"},
	{ScanResultFloat, "0E+10"},
	{ScanResultFloat, "1E-10"},
	{ScanResultFloat, "42E+10"},
	{ScanResultFloat, "01234567890E-10"},

	{ScanResultComment, "-- durations"},
	{ScanResultDuration, `0s`},
	{ScanResultDuration, `1344w2d98m`},
	{ScanResultDuration, `1200ns`},
	{ScanResultDuration, `0d0h0us`},
	{ScanResultDuration, `1w1d1h1m1s123ms456us789us`},
	{ScanResultDuration, `1d1d1d`},
	{ScanResultDuration, `987ns654us321ms0s1m2h3d4w`},

	{ScanResultComment, "-- strings"},
	{ScanResultString, `' '`},
	{ScanResultString, `'a'`},
	{ScanResultString, `'本'`},
	{ScanResultString, `'\a'`},
	{ScanResultString, `'\b'`},
	{ScanResultString, `'\f'`},
	{ScanResultString, `'\n'`},
	{ScanResultString, `'\r'`},
	{ScanResultString, `'\t'`},
	{ScanResultString, `'\v'`},
	{ScanResultString, `'\''`},
	{ScanResultString, `'\000'`},
	{ScanResultString, `'\777'`},
	{ScanResultString, `'\x00'`},
	{ScanResultString, `'\xff'`},
	{ScanResultString, `'\u0000'`},
	{ScanResultString, `'\ufA16'`},
	{ScanResultString, `'\U00000000'`},
	{ScanResultString, `'\U0000ffAB'`},
	{ScanResultString, `'` + f100 + `'`},

	{ScanResultComment, "-- raw strings"},
	{ScanResultRawString, "``"},
	{ScanResultRawString, "`\\`"},
	{ScanResultRawString, "`" + "\n\n/* foobar */\n\n" + "`"},
	{ScanResultRawString, "`" + f100 + "`"},

	{ScanResultComment, "-- individual characters"},
	// NUL character is not allowed
	{'\x01', "\x01"},
	{' ' - 1, string(' ' - 1)},
	{'+', "+"},
	{'/', "/"},
	{'.', "."},
	{'~', "~"},
	{'(', "("},
}

func makeSource(pattern string) *bytes.Buffer {
	var buf bytes.Buffer
	for _, k := range tokenList {
		fmt.Fprintf(&buf, pattern, k.text)
	}
	return &buf
}

func checkTok(t *testing.T, s *Scanner, line int, got, want rune, text string) {
	if got != want {
		t.Fatalf("tok = %s, want %s for %q", TokenString(got), TokenString(want), text)
	}
	if s.Line != line {
		t.Errorf("line = %d, want %d for %q", s.Line, line, text)
	}
	stext := s.TokenText()
	if stext != text {
		t.Errorf("text = %q, want %q", stext, text)
	} else {
		// check idempotency of TokenText() call
		stext = s.TokenText()
		if stext != text {
			t.Errorf("text = %q, want %q (idempotency check)", stext, text)
		}
	}
}

func checkTokErr(t *testing.T, s *Scanner, line int, want rune, text string) {
	prevCount := s.ErrorCount
	checkTok(t, s, line, s.Scan(), want, text)
	if s.ErrorCount != prevCount+1 {
		t.Fatalf("want error for %q", text)
	}
}

func countNewlines(s string) int {
	n := 0
	for _, ch := range s {
		if ch == '\n' {
			n++
		}
	}
	return n
}

func TestScan(t *testing.T) {
	s := new(Scanner).Init(makeSource(" \t%s\n"))
	tok := s.Scan()
	line := 1
	for _, k := range tokenList {
		checkTok(t, s, line, tok, k.tok, k.text)
		tok = s.Scan()
		line += countNewlines(k.text) + 1 // each token is on a new line
	}
	checkTok(t, s, line, tok, ScanResultEOF, "")
}

func TestInvalidExponent(t *testing.T) {
	const src = "1.5e 1.5E 1e+ 1e- 1.5z"
	s := new(Scanner).Init(strings.NewReader(src))
	s.Error = func(s *Scanner, msg string) {
		const want = "exponent has no digits"
		if msg != want {
			t.Errorf("%s: got error %q; want %q", s.TokenText(), msg, want)
		}
	}
	checkTokErr(t, s, 1, ScanResultFloat, "1.5e")
	checkTokErr(t, s, 1, ScanResultFloat, "1.5E")
	checkTokErr(t, s, 1, ScanResultFloat, "1e+")
	checkTokErr(t, s, 1, ScanResultFloat, "1e-")

	s.Error = func(s *Scanner, msg string) {
		const want = "extra character after float"
		if msg != want {
			t.Errorf("%s: got error %q; want %q", s.TokenText(), msg, want)
		}
	}
	checkTok(t, s, 1, s.Scan(), ScanResultFloat, "1.5z")
	checkTok(t, s, 1, s.Scan(), ScanResultEOF, "")
	if s.ErrorCount != 5 {
		t.Errorf("%d errors, want 4", s.ErrorCount)
	}
}

func TestPosition(t *testing.T) {
	src := makeSource("\t\t\t\t%s\n")
	s := new(Scanner).Init(src)
	s.Scan()
	pos := Position{"", 4, 1, 5}
	for _, k := range tokenList {
		if s.Offset != pos.Offset {
			t.Errorf("offset = %d, want %d for %q", s.Offset, pos.Offset, k.text)
		}
		if s.Line != pos.Line {
			t.Errorf("line = %d, want %d for %q", s.Line, pos.Line, k.text)
		}
		if s.Column != pos.Column {
			t.Errorf("column = %d, want %d for %q", s.Column, pos.Column, k.text)
		}
		pos.Offset += 4 + len(k.text) + 1     // 4 tabs + token bytes + newline
		pos.Line += countNewlines(k.text) + 1 // each token is on a new line
		s.Scan()
	}
	// make sure there were no token-internal errors reported by scanner
	if s.ErrorCount != 0 {
		t.Errorf("%d errors", s.ErrorCount)
	}
}

func TestScanNext(t *testing.T) {
	const BOM = '\uFEFF'
	BOMs := string(BOM)
	s := new(Scanner).Init(strings.NewReader(BOMs + "if a == bcd /* com" + BOMs + "ment */ {\n\ta += c\n}" + BOMs + "-- line comment ending in eof"))
	checkTok(t, s, 1, s.Scan(), ScanResultIdent, "if") // the first BOM is ignored
	checkTok(t, s, 1, s.Scan(), ScanResultIdent, "a")
	checkTok(t, s, 1, s.Scan(), '=', "=")
	checkTok(t, s, 0, s.Next(), '=', "")
	checkTok(t, s, 0, s.Next(), ' ', "")
	checkTok(t, s, 0, s.Next(), 'b', "")
	checkTok(t, s, 1, s.Scan(), ScanResultIdent, "cd")
	checkTok(t, s, 1, s.Scan(), ScanResultComment, "/* com\uFEFFment */")
	checkTok(t, s, 1, s.Scan(), '{', "{")
	checkTok(t, s, 2, s.Scan(), ScanResultIdent, "a")
	checkTok(t, s, 2, s.Scan(), '+', "+")
	checkTok(t, s, 0, s.Next(), '=', "")
	checkTok(t, s, 2, s.Scan(), ScanResultIdent, "c")
	checkTok(t, s, 3, s.Scan(), '}', "}")
	checkTok(t, s, 3, s.Scan(), BOM, BOMs)
	checkTok(t, s, 3, s.Scan(), ScanResultComment, "-- line comment ending in eof")
	checkTok(t, s, 3, s.Scan(), -1, "")
	if s.ErrorCount != 0 {
		t.Errorf("%d errors", s.ErrorCount)
	}
}

func testError(t *testing.T, src, pos, msg string, tok rune) {
	s := new(Scanner).Init(strings.NewReader(src))
	errorCalled := false
	s.Error = func(s *Scanner, m string) {
		if !errorCalled {
			// only look at first error
			if p := s.Pos().String(); p != pos {
				t.Errorf("pos = %q, want %q for %q", p, pos, src)
			}
			if m != msg {
				t.Errorf("msg = %q, want %q for %q", m, msg, src)
			}
			errorCalled = true
		}
	}
	tk := s.Scan()
	if tk != tok {
		t.Errorf("tok = %s, want %s for %q", TokenString(tk), TokenString(tok), src)
	}
	if !errorCalled {
		t.Errorf("error handler not called for %q", src)
	}
	if s.ErrorCount == 0 {
		t.Errorf("count = %d, want > 0 for %q", s.ErrorCount, src)
	}
}

func TestError(t *testing.T) {
	s := new(Scanner).Init(strings.NewReader("1d3"))
	checkTokErr(t, s, 1, ScanResultDuration, "1d3")

	testError(t, "\x00", "<input>:1:1", "invalid character NUL", 0)
	testError(t, "\x80", "<input>:1:1", "invalid UTF-8 encoding", utf8.RuneError)
	testError(t, "\xff", "<input>:1:1", "invalid UTF-8 encoding", utf8.RuneError)
	testError(t, "a\x00", "<input>:1:2", "invalid character NUL", ScanResultIdent)
	testError(t, "ab\x80", "<input>:1:3", "invalid UTF-8 encoding", ScanResultIdent)
	testError(t, "abc\xff", "<input>:1:4", "invalid UTF-8 encoding", ScanResultIdent)
	testError(t, `"a`+"\x00", "<input>:1:3", "invalid character NUL", ScanResultQuotedIdent)
	testError(t, `"ab`+"\x80", "<input>:1:4", "invalid UTF-8 encoding", ScanResultQuotedIdent)
	testError(t, `"abc`+"\xff", "<input>:1:5", "invalid UTF-8 encoding", ScanResultQuotedIdent)

	testError(t, "`a"+"\x00", "<input>:1:3", "invalid character NUL", ScanResultRawString)
	testError(t, "`ab"+"\x80", "<input>:1:4", "invalid UTF-8 encoding", ScanResultRawString)
	testError(t, "`abc"+"\xff", "<input>:1:5", "invalid UTF-8 encoding", ScanResultRawString)

	testError(t, `"\'"`, "<input>:1:3", "invalid char escape", ScanResultQuotedIdent)
	testError(t, `'\"'`, "<input>:1:3", "invalid char escape", ScanResultString)

	testError(t, `01238`, "<input>:1:6", "invalid digit '8' in octal literal", ScanResultInt)
	testError(t, `01238123`, "<input>:1:9", "invalid digit '8' in octal literal", ScanResultInt)
	testError(t, `0x`, "<input>:1:3", "hexadecimal literal has no digits", ScanResultInt)
	testError(t, `0xg`, "<input>:1:3", "hexadecimal literal has no digits", ScanResultInt)

	testError(t, `1.5e`, "<input>:1:5", "exponent has no digits", ScanResultFloat)
	testError(t, `1.5E`, "<input>:1:5", "exponent has no digits", ScanResultFloat)
	testError(t, `1.5e+`, "<input>:1:6", "exponent has no digits", ScanResultFloat)
	testError(t, `1.5e-`, "<input>:1:6", "exponent has no digits", ScanResultFloat)

	testError(t, `"abc`, "<input>:1:5", "literal not terminated", ScanResultQuotedIdent)
	testError(t, `"abc`+"\n", "<input>:1:5", "literal not terminated", ScanResultQuotedIdent)
	testError(t, "`abc\n", "<input>:2:1", "literal not terminated", ScanResultRawString)
	testError(t, `/*/`, "<input>:1:4", "comment not terminated", ScanResultComment)
	testError(t, `'`, "<input>:1:2", "literal not terminated", ScanResultString)
	testError(t, `'`+"\n", "<input>:1:2", "literal not terminated", ScanResultString)

	testError(t, "1d2x", "<input>:1:4", "invalid duration unit", ScanResultDuration)
	testError(t, "1d2u", "<input>:1:5", "invalid duration unit", ScanResultDuration)
	testError(t, "1d2n", "<input>:1:5", "invalid duration unit", ScanResultDuration)
	testError(t, "1d2", "<input>:1:4", "invalid duration unit", ScanResultDuration)

	testError(t, "1d03m", "<input>:1:6", "duration requires decimal integer", ScanResultDuration)

	testError(t, `'abc\02m'`, "<input>:1:8", "invalid char escape", ScanResultString)
}

// An errReader returns (n, err) where err is not io.EOF.
type errReader struct{ n int }

func (er errReader) Read(b []byte) (int, error) {
	n := er.n
	if n > len(b) {
		n = len(b)
	}
	for i := range n {
		b[i] = 0xe4
	}

	return n, io.ErrNoProgress // some error that is not io.EOF
}

func testIOError(t *testing.T, r io.Reader, want rune) {
	s := new(Scanner).Init(r)
	errorCalled := false
	s.Error = func(s *Scanner, msg string) {
		if !errorCalled {
			if want := io.ErrNoProgress.Error(); msg != want {
				t.Errorf("msg = %q, want %q", msg, want)
			}
			errorCalled = true
		}
	}
	tok := s.Scan()
	if tok != want {
		t.Errorf("tok = %s, want %s", TokenString(tok), TokenString(want))
	}
	if !errorCalled {
		t.Errorf("error handler not called")
	}
}
func TestIOError(t *testing.T) {
	testIOError(t, errReader{}, ScanResultEOF)
	testIOError(t, errReader{n: 1}, utf8.RuneError)
}

func checkPos(t *testing.T, got, want Position) {
	if got.Offset != want.Offset || got.Line != want.Line || got.Column != want.Column {
		t.Errorf("got offset, line, column = %d, %d, %d; want %d, %d, %d",
			got.Offset, got.Line, got.Column, want.Offset, want.Line, want.Column)
	}
}

func checkNextPos(t *testing.T, s *Scanner, offset, line, column int, char rune) {
	if ch := s.Next(); ch != char {
		t.Errorf("ch = %s, want %s", TokenString(ch), TokenString(char))
	}
	want := Position{Offset: offset, Line: line, Column: column}
	checkPos(t, s.Pos(), want)
}

func checkScanPos(t *testing.T, s *Scanner, offset, line, column int, char rune) {
	want := Position{Offset: offset, Line: line, Column: column}
	checkPos(t, s.Pos(), want)
	if ch := s.Scan(); ch != char {
		t.Errorf("ch = %s, want %s", TokenString(ch), TokenString(char))
		if string(ch) != s.TokenText() {
			t.Errorf("tok = %q, want %q", s.TokenText(), string(ch))
		}
	}
	checkPos(t, s.Position, want)
}

func TestPos(t *testing.T) {
	// corner case: empty source
	s := new(Scanner).Init(strings.NewReader(""))
	checkPos(t, s.Pos(), Position{Offset: 0, Line: 1, Column: 1})
	s.Peek() // peek doesn't affect the position
	checkPos(t, s.Pos(), Position{Offset: 0, Line: 1, Column: 1})

	// corner case: source with only a newline
	s = new(Scanner).Init(strings.NewReader("\n"))
	checkPos(t, s.Pos(), Position{Offset: 0, Line: 1, Column: 1})
	checkNextPos(t, s, 1, 2, 1, '\n')
	// after EOF position doesn't change
	for i := 10; i > 0; i-- {
		checkScanPos(t, s, 1, 2, 1, ScanResultEOF)
	}
	if s.ErrorCount != 0 {
		t.Errorf("%d errors", s.ErrorCount)
	}

	// corner case: source with only a single character
	s = new(Scanner).Init(strings.NewReader("本"))
	checkPos(t, s.Pos(), Position{Offset: 0, Line: 1, Column: 1})
	checkNextPos(t, s, 3, 1, 2, '本')
	// after EOF position doesn't change
	for i := 10; i > 0; i-- {
		checkScanPos(t, s, 3, 1, 2, ScanResultEOF)
	}
	if s.ErrorCount != 0 {
		t.Errorf("%d errors", s.ErrorCount)
	}

	// positions after calling Next
	s = new(Scanner).Init(strings.NewReader("  foo६४  \n\n本語\n"))
	checkNextPos(t, s, 1, 1, 2, ' ')
	s.Peek() // peek doesn't affect the position
	checkNextPos(t, s, 2, 1, 3, ' ')
	checkNextPos(t, s, 3, 1, 4, 'f')
	checkNextPos(t, s, 4, 1, 5, 'o')
	checkNextPos(t, s, 5, 1, 6, 'o')
	checkNextPos(t, s, 8, 1, 7, '६')
	checkNextPos(t, s, 11, 1, 8, '४')
	checkNextPos(t, s, 12, 1, 9, ' ')
	checkNextPos(t, s, 13, 1, 10, ' ')
	checkNextPos(t, s, 14, 2, 1, '\n')
	checkNextPos(t, s, 15, 3, 1, '\n')
	checkNextPos(t, s, 18, 3, 2, '本')
	checkNextPos(t, s, 21, 3, 3, '語')
	checkNextPos(t, s, 22, 4, 1, '\n')
	// after EOF position doesn't change
	for i := 10; i > 0; i-- {
		checkScanPos(t, s, 22, 4, 1, ScanResultEOF)
	}
	if s.ErrorCount != 0 {
		t.Errorf("%d errors", s.ErrorCount)
	}

	// positions after calling Scan
	s = new(Scanner).Init(strings.NewReader("abc\n本語\n\nx"))
	checkScanPos(t, s, 0, 1, 1, ScanResultIdent)
	checkPos(t, s.Pos(), Position{Offset: 3, Line: 1, Column: 4})
	s.Peek() // peek doesn't affect the position
	checkPos(t, s.Pos(), Position{Offset: 3, Line: 1, Column: 4})
	s.Scan()
	checkPos(t, s.Position, Position{Offset: 4, Line: 2, Column: 1})
	checkPos(t, s.Pos(), Position{Offset: 10, Line: 2, Column: 3})
	s.Scan()
	checkPos(t, s.Position, Position{Offset: 12, Line: 4, Column: 1})
	checkPos(t, s.Pos(), Position{Offset: 13, Line: 4, Column: 2})

	// after EOF position doesn't change
	for i := 10; i > 0; i-- {
		checkScanPos(t, s, 13, 4, 2, ScanResultEOF)
	}

	if s.ErrorCount != 0 {
		t.Errorf("%d errors", s.ErrorCount)
	}
}

type countReader int

func (r *countReader) Read([]byte) (int, error) {
	*r++
	return 0, io.EOF
}

func TestNextEOFHandling(t *testing.T) {
	var r countReader

	// corner case: empty source
	s := new(Scanner).Init(&r)

	tok := s.Next()
	if tok != ScanResultEOF {
		t.Error("1) EOF not reported")
	}

	tok = s.Peek()
	if tok != ScanResultEOF {
		t.Error("2) EOF not reported")
	}

	if r != 1 {
		t.Errorf("scanner called Read %d times, not once", r)
	}
}

func TestScanEOFHandling(t *testing.T) {
	var r countReader

	// corner case: empty source
	s := new(Scanner).Init(&r)

	tok := s.Scan()
	if tok != ScanResultEOF {
		t.Error("1) EOF not reported")
	}

	tok = s.Peek()
	if tok != ScanResultEOF {
		t.Error("2) EOF not reported")
	}

	if r != 1 {
		t.Errorf("scanner called Read %d times, not once", r)
	}
}

func TestIssue29723(t *testing.T) {
	s := new(Scanner).Init(strings.NewReader(`x "`))
	s.Error = func(s *Scanner, _ string) {
		got := s.TokenText() // this call shouldn't panic
		const want = `"`
		if got != want {
			t.Errorf("got %q; want %q", got, want)
		}
	}
	for r := s.Scan(); r != ScanResultEOF; r = s.Scan() {
	}
}

func TestNumberAndDurations(t *testing.T) {
	for _, test := range []struct {
		tok              rune
		src, tokens, err string
	}{
		// binaries
		{ScanResultInt, "0b0", "0b0", ""},
		{ScanResultInt, "0b1010", "0b1010", ""},
		{ScanResultInt, "0B1110", "0B1110", ""},

		{ScanResultInt, "0b", "0b", "binary literal has no digits"},
		{ScanResultInt, "0b0190", "0b0190", "invalid digit '9' in binary literal"},
		{ScanResultInt, "0b01a0", "0b01a0", "extra character after integer"},

		// binary floats (invalid)
		{ScanResultFloat, "0b.", "0b.", "invalid radix point in binary literal"},
		{ScanResultFloat, "0b.1", "0b.1", "invalid radix point in binary literal"},
		{ScanResultFloat, "0b1.0", "0b1.0", "invalid radix point in binary literal"},
		{ScanResultFloat, "0b1e10", "0b1e10", "'e' exponent requires decimal mantissa"},
		{ScanResultFloat, "0b1P-1", "0b1P-1", "'P' exponent requires hexadecimal mantissa"},

		// octals
		{ScanResultInt, "0o0", "0o0", ""},
		{ScanResultInt, "0o1234", "0o1234", ""},
		{ScanResultInt, "0O1234", "0O1234", ""},

		{ScanResultInt, "0o", "0o", "octal literal has no digits"},
		{ScanResultInt, "0o8123", "0o8123", "invalid digit '8' in octal literal"},
		{ScanResultInt, "0o1293", "0o1293", "invalid digit '9' in octal literal"},
		{ScanResultInt, "0o12a3", "0o12a3", "extra character after integer"},

		// octal floats (invalid)
		{ScanResultFloat, "0o.", "0o.", "invalid radix point in octal literal"},
		{ScanResultFloat, "0o.2", "0o.2", "invalid radix point in octal literal"},
		{ScanResultFloat, "0o1.2", "0o1.2", "invalid radix point in octal literal"},
		{ScanResultFloat, "0o1E+2", "0o1E+2", "'E' exponent requires decimal mantissa"},
		{ScanResultFloat, "0o1p10", "0o1p10", "'p' exponent requires hexadecimal mantissa"},

		// 0-octals
		{ScanResultInt, "0", "0", ""},
		{ScanResultInt, "0123", "0123", ""},

		{ScanResultInt, "08123", "08123", "invalid digit '8' in octal literal"},
		{ScanResultInt, "01293", "01293", "invalid digit '9' in octal literal"},
		{ScanResultInt, "0F.", "0F .", "extra character after integer"},
		{ScanResultInt, "0123F.", "0123F .", "extra character after integer"},
		{ScanResultInt, "0123456x", "0123456x", "extra character after integer"},

		// decimals
		{ScanResultInt, "1", "1", ""},
		{ScanResultInt, "1234", "1234", ""},

		{ScanResultInt, "1f", "1f", "extra character after integer"},

		// decimal floats
		{ScanResultFloat, "0.", "0.", ""},
		{ScanResultFloat, "123.", "123.", ""},
		{ScanResultFloat, "0123.", "0123.", ""},

		{ScanResultFloat, ".0", ".0", ""},
		{ScanResultFloat, ".123", ".123", ""},
		{ScanResultFloat, ".0123", ".0123", ""},

		{ScanResultFloat, "0.0", "0.0", ""},
		{ScanResultFloat, "123.123", "123.123", ""},
		{ScanResultFloat, "0123.0123", "0123.0123", ""},

		{ScanResultFloat, "0e0", "0e0", ""},
		{ScanResultFloat, "123e+0", "123e+0", ""},
		{ScanResultFloat, "0123E-1", "0123E-1", ""},

		{ScanResultFloat, "0.e+1", "0.e+1", ""},
		{ScanResultFloat, "123.E-10", "123.E-10", ""},
		{ScanResultFloat, "0123.e123", "0123.e123", ""},

		{ScanResultFloat, ".0e-1", ".0e-1", ""},
		{ScanResultFloat, ".123E+10", ".123E+10", ""},
		{ScanResultFloat, ".0123E123", ".0123E123", ""},

		{ScanResultFloat, "0.0e1", "0.0e1", ""},
		{ScanResultFloat, "123.123E-10", "123.123E-10", ""},
		{ScanResultFloat, "0123.0123e+456", "0123.0123e+456", ""},

		{ScanResultFloat, "0e", "0e", "exponent has no digits"},
		{ScanResultFloat, "0E+", "0E+", "exponent has no digits"},
		{ScanResultFloat, "1e+f", "1e+f", "exponent has no digits"},
		{ScanResultFloat, "0p0", "0p0", "'p' exponent requires hexadecimal mantissa"},
		{ScanResultFloat, "1.0P-1", "1.0P-1", "'P' exponent requires hexadecimal mantissa"},

		// hexadecimals
		{ScanResultInt, "0x0", "0x0", ""},
		{ScanResultInt, "0x1234", "0x1234", ""},
		{ScanResultInt, "0xcafef00d", "0xcafef00d", ""},
		{ScanResultInt, "0XCAFEF00D", "0XCAFEF00D", ""},

		{ScanResultInt, "0x", "0x", "hexadecimal literal has no digits"},
		{ScanResultInt, "0x1g", "0x1g", "extra character after integer"},

		// hexadecimal floats
		{ScanResultFloat, "0x0p0", "0x0p0", ""},
		{ScanResultFloat, "0x12efp-123", "0x12efp-123", ""},
		{ScanResultFloat, "0xABCD.p+0", "0xABCD.p+0", ""},
		{ScanResultFloat, "0x.0189P-0", "0x.0189P-0", ""},
		{ScanResultFloat, "0x1.ffffp+1023", "0x1.ffffp+1023", ""},

		{ScanResultFloat, "0x.", "0x.", "hexadecimal literal has no digits"},
		{ScanResultFloat, "0x0.", "0x0.", "hexadecimal mantissa requires a 'p' exponent"},
		{ScanResultFloat, "0x.0", "0x.0", "hexadecimal mantissa requires a 'p' exponent"},
		{ScanResultFloat, "0x1.1", "0x1.1", "hexadecimal mantissa requires a 'p' exponent"},
		{ScanResultFloat, "0x1.1e0", "0x1.1e0", "hexadecimal mantissa requires a 'p' exponent"},
		{ScanResultFloat, "0x1.2gp1a", "0x1.2gp1a", "hexadecimal mantissa requires a 'p' exponent"},
		{ScanResultFloat, "0x0p", "0x0p", "exponent has no digits"},
		{ScanResultFloat, "0xeP-", "0xeP-", "exponent has no digits"},
		{ScanResultFloat, "0x1234PAB", "0x1234PAB", "exponent has no digits"},
		{ScanResultFloat, "0x1.2p1a", "0x1.2p1a", "extra character after float"},

		// durations
		{ScanResultDuration, "0u", "0u", "invalid duration unit"},
		{ScanResultDuration, "0μ", "0μ", "invalid duration unit"},
		{ScanResultDuration, "0µ", "0µ", "invalid duration unit"},
		{ScanResultDuration, "1d0x", "1d0x", "invalid duration unit"},
		{ScanResultDuration, "1d0", "1d0", "invalid duration unit"},
		{ScanResultDuration, "01d0", "01d0", "invalid duration unit"},
		{ScanResultDuration, "01d02s", "01d02s", "duration requires decimal integer"},
		{ScanResultDuration, "1d02s", "1d02s", "duration requires decimal integer"},
		{ScanResultDuration, "1d2sv", "1d2sv", "extra character after duration"},
		{ScanResultDuration, "1d0s", "1d0s", ""},
	} {
		s := new(Scanner).Init(strings.NewReader(test.src))
		var err string
		s.Error = func(s *Scanner, msg string) {
			if err == "" {
				err = msg
			}
		}

		for i, want := range strings.Split(test.tokens, " ") {
			err = ""
			tok := s.Scan()
			lit := s.TokenText()
			if i == 0 {
				if tok != test.tok {
					t.Errorf("%q: got token %s; want %s", test.src, TokenString(tok), TokenString(test.tok))
				}
				if err != test.err {
					t.Errorf("%q: got error %q; want %q", test.src, err, test.err)
				}
			}
			if lit != want {
				t.Errorf("%q: got literal %q (%s); want %s", test.src, lit, TokenString(tok), want)
			}
		}

		// make sure we read all
		if tok := s.Scan(); tok != ScanResultEOF {
			t.Errorf("%q: got %s; want EOF", test.src, TokenString(tok))
		}
	}
}

type corruptReader struct{ count int }

func (cr *corruptReader) Read(b []byte) (int, error) {
	if cr.count == 0 {
		cr.count++
		b[0] = 'a'
		b[1] = ' '
		b[2] = 'b'
		return 3, nil
	}
	if cr.count == 1 {
		cr.count++
		data := []byte("国")
		data = data[:len(data)-1]
		copy(b, data)
		return len(data), io.ErrNoProgress
	}
	return 0, io.ErrNoProgress
}

// test a cornor case of [Scanner.TokenText]
func TestTokenText(t *testing.T) {
	want := rune(ScanResultIdent)

	s := new(Scanner).Init(&corruptReader{})
	errorCalled := false
	s.Error = func(s *Scanner, msg string) {
		if tt := s.TokenText(); tt != "b" {
			t.Errorf("token text = %q, want %q", tt, "b")
		}

		if !errorCalled {
			if want := io.ErrNoProgress.Error(); msg != want {
				t.Errorf("msg = %q, want %q", msg, want)
			}
			errorCalled = true
		}
	}

	tok := s.Scan()
	if tok != want {
		t.Errorf("tok = %s, want %s", TokenString(tok), TokenString(want))
	}
	if errorCalled {
		t.Errorf("error handler was called")
	}

	tok = s.Scan()
	if tok != want {
		t.Errorf("tok = %s, want %s", TokenString(tok), TokenString(want))
	}
	if !errorCalled {
		t.Errorf("error handler not called")
	}
}
