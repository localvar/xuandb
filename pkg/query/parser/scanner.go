// The content of this file and the corresponding test cases are based on Go's
// text/scanner, with modifications.

package parser

import (
	"bytes"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
)

// Position is a value that represents a source position.
// A position is valid if Line > 0.
type Position struct {
	Filename string // filename, if any
	Offset   int    // byte offset, starting at 0
	Line     int    // line number, starting at 1
	Column   int    // column number, starting at 1 (character count per line)
}

// IsValid reports whether the position is valid.
func (pos *Position) IsValid() bool { return pos.Line > 0 }

func (pos Position) String() string {
	s := pos.Filename
	if s == "" {
		s = "<input>"
	}
	if pos.IsValid() {
		s += fmt.Sprintf(":%d:%d", pos.Line, pos.Column)
	}
	return s
}

// The result of Scan is one of these tokens or a Unicode character.
const (
	ScanResultEOF = -(iota + 1)
	ScanResultComment
	ScanResultIdent
	ScanResultQuotedIdent
	ScanResultInt
	ScanResultFloat
	ScanResultDuration
	ScanResultString
	ScanResultRawString
)

var tokenString = map[rune]string{
	ScanResultEOF:         "EOF",
	ScanResultComment:     "comment",
	ScanResultIdent:       "ident",
	ScanResultQuotedIdent: "quoted ident",
	ScanResultInt:         "integer",
	ScanResultFloat:       "float",
	ScanResultDuration:    "duration",
	ScanResultString:      "string",
	ScanResultRawString:   "raw string",
}

// TokenString returns a printable string for a token or Unicode character.
func TokenString(tok rune) string {
	if s, found := tokenString[tok]; found {
		return s
	}
	return fmt.Sprintf("%q", string(tok))
}

const bufLen = 1024 // at least utf8.UTFMax

// Scanner implements a scanner and tokenizer for UTF-8-encoded text.
// It takes an io.Reader providing the source, which then can be tokenized
// through repeated calls to the Scan function. For compatibility with
// existing tools, the NUL character is not allowed. If the first character
// in the source is a UTF-8 encoded byte order mark (BOM), it is discarded.
type Scanner struct {
	// Input
	src io.Reader

	// Source buffer
	srcBuf [bufLen + 1]byte // +1 for sentinel for common case of s.next()
	srcPos int              // reading position (srcBuf index)
	srcEnd int              // source end (srcBuf index)

	// Source position
	srcBufOffset int // byte offset of srcBuf[0] in source
	line         int // line count
	column       int // character count
	lastLineLen  int // length of last line in characters (for correct column reporting)
	lastCharLen  int // length of last character in bytes

	// Token text buffer
	// Typically, token text is stored completely in srcBuf, but in general
	// the token text's head may be buffered in tokBuf while the token text's
	// tail is stored in srcBuf.
	tokBuf bytes.Buffer // token text head that is not in srcBuf anymore
	tokPos int          // token text tail position (srcBuf index); valid if >= 0
	tokEnd int          // token text tail end (srcBuf index)

	// One character look-ahead
	ch rune // character before current srcPos

	// Error is called for each error encountered. If no Error
	// function is set, the error is reported to os.Stderr.
	Error func(s *Scanner, msg string)

	// ErrorCount is incremented by one for each error encountered.
	ErrorCount int

	// Start position of most recently scanned token; set by Scan.
	// Calling Init invalidates the position (Line == 0).
	// The Filename field is always left untouched by the Scanner.
	// If an error is reported (via Error) and Position is invalid,
	// the scanner is not inside a token. Call Pos to obtain an error
	// position in that case, or to obtain the position immediately
	// after the most recently scanned token.
	Position
}

// Init initializes a [Scanner] with a new source and returns s.
// [Scanner.ErrorCount] is set to 0.
func (s *Scanner) Init(src io.Reader) *Scanner {
	s.src = src

	// initialize source buffer
	// (the first call to next() will fill it by calling src.Read)
	s.srcBuf[0] = utf8.RuneSelf // sentinel
	s.srcPos = 0
	s.srcEnd = 0

	// initialize source position
	s.srcBufOffset = 0
	s.line = 1
	s.column = 0
	s.lastLineLen = 0
	s.lastCharLen = 0

	// initialize token text buffer
	// (required for first call to next()).
	s.tokPos = -1

	// initialize one character look-ahead
	s.ch = -2 // no char read yet, not EOF

	// initialize public fields
	s.ErrorCount = 0
	s.Line = 0 // invalidate token position

	return s
}

// next reads and returns the next Unicode character. It is designed such
// that only a minimal amount of work needs to be done in the common ASCII
// case (one test to check for both ASCII and end-of-buffer, and one test
// to check for newlines).
func (s *Scanner) next() rune {
	ch, width := rune(s.srcBuf[s.srcPos]), 1

	if ch >= utf8.RuneSelf {
		// uncommon case: not ASCII or not enough bytes
		for s.srcPos+utf8.UTFMax > s.srcEnd && !utf8.FullRune(s.srcBuf[s.srcPos:s.srcEnd]) {
			// not enough bytes: read some more, but first
			// save away token text if any
			if s.tokPos >= 0 {
				s.tokBuf.Write(s.srcBuf[s.tokPos:s.srcPos])
				s.tokPos = 0
				// s.tokEnd is set by Scan()
			}
			// move unread bytes to beginning of buffer
			copy(s.srcBuf[0:], s.srcBuf[s.srcPos:s.srcEnd])
			s.srcBufOffset += s.srcPos
			// read more bytes
			// (an io.Reader must return io.EOF when it reaches
			// the end of what it is reading - simply returning
			// n == 0 will make this loop retry forever; but the
			// error is in the reader implementation in that case)
			i := s.srcEnd - s.srcPos
			n, err := s.src.Read(s.srcBuf[i:bufLen])
			s.srcPos = 0
			s.srcEnd = i + n
			s.srcBuf[s.srcEnd] = utf8.RuneSelf // sentinel
			if err != nil {
				if err != io.EOF {
					s.error(err.Error())
				}
				if s.srcEnd == 0 {
					if s.lastCharLen > 0 {
						// previous character was not EOF
						s.column++
					}
					s.lastCharLen = 0
					return ScanResultEOF
				}
				// If err == EOF, we won't be getting more
				// bytes; break to avoid infinite loop. If
				// err is something else, we don't know if
				// we can get more bytes; thus also break.
				break
			}
		}
		// at least one byte
		ch = rune(s.srcBuf[s.srcPos])
		if ch >= utf8.RuneSelf {
			// uncommon case: not ASCII
			ch, width = utf8.DecodeRune(s.srcBuf[s.srcPos:s.srcEnd])
			if ch == utf8.RuneError && width == 1 {
				// advance for correct error position
				s.srcPos += width
				s.lastCharLen = width
				s.column++
				s.error("invalid UTF-8 encoding")
				return ch
			}
		}
	}

	// advance
	s.srcPos += width
	s.lastCharLen = width
	s.column++

	// special situations
	switch ch {
	case 0:
		// for compatibility with other tools
		s.error("invalid character NUL")
	case '\n':
		s.line++
		s.lastLineLen = s.column
		s.column = 0
	}

	return ch
}

// Next reads and returns the next Unicode character. It returns [ScanResultEOF]
// at the end of the source. It reports a read error by calling s.Error. Next
// does not update the [Scanner.Position] field; use [Scanner.Pos]() to get the
// current position.
func (s *Scanner) Next() rune {
	s.tokPos = -1 // don't collect token text
	s.Line = 0    // invalidate token position
	ch := s.Peek()
	if ch != ScanResultEOF {
		s.ch = s.next()
	}
	return ch
}

// Peek returns the next Unicode character in the source without advancing the
// scanner. It returns [ScanResultEOF] if the scanner's position is at the last
// character of the source.
func (s *Scanner) Peek() rune {
	if s.ch == -2 {
		// this code is only run for the very first character
		s.ch = s.next()
		if s.ch == '\uFEFF' {
			s.ch = s.next() // ignore BOM
		}
	}
	return s.ch
}

func (s *Scanner) error(msg string) {
	s.tokEnd = s.srcPos - s.lastCharLen // make sure token text is terminated
	s.ErrorCount++
	if s.Error == nil {
		return
	}
	s.Error(s, msg)
}

func (s *Scanner) errorf(format string, args ...any) {
	s.error(fmt.Sprintf(format, args...))
}

func lower(ch rune) rune        { return ('a' - 'A') | ch } // returns lower-case ch iff ch is ASCII letter
func isIdentRune(ch rune) bool  { return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch) }
func isDecimal(ch rune) bool    { return '0' <= ch && ch <= '9' }
func isHex(ch rune) bool        { return '0' <= ch && ch <= '9' || 'a' <= lower(ch) && lower(ch) <= 'f' }
func isWhitespace(ch rune) bool { return (1<<'\t'|1<<'\n'|1<<'\r'|1<<' ')&(1<<uint(ch)) != 0 }

func (s *Scanner) scanIdentifier() rune {
	// we know the zero'th rune is OK; start scanning at the next one
	ch := s.next()
	for isIdentRune(ch) {
		ch = s.next()
	}
	return ch
}

// digits accepts the sequence { digit } starting with ch0. If base <= 10,
// digits accepts any decimal digit but records the first invalid digit >= base
// in *invalid if *invalid == 0. digits returns the first rune that is not part
// of the sequence anymore, and the number of digit runes scanned.
func (s *Scanner) digits(ch0 rune, base int, invalid *rune) (ch rune, n int) {
	ch = ch0
	if base <= 10 {
		max := rune('0' + base)
		for isDecimal(ch) {
			if ch >= max && *invalid == 0 {
				*invalid = ch
			}
			ch = s.next()
			n++
		}
	} else {
		for isHex(ch) {
			ch = s.next()
			n++
		}
	}
	return
}

// Durations includes at least one duration segment, each segment starts with
// an 10-based integer and follows by a unit, which could be one of 'w', 'd',
// 'h', 'm', 's', 'ms', 'us', 'µs', 'μs' and 'ns'. Note the units could appear
// in any order, and it is also ok for one unit to appear more than once.
//
// Because the duration function is always called with the first integer
// scanned, the result token should be ScanResultInt if the first rune does
// not start a valid unit.
func (s *Scanner) duration(ch rune, notDec bool) (rune, rune) {
	sawUnit := false

	for {
		// scan unit
		switch ch {
		case 'w', 'd', 'h', 's':
			sawUnit = true
			ch = s.next()
		case 'm':
			sawUnit = true
			if ch = s.next(); ch == 's' {
				ch = s.next()
			}
		case 'n', 'u', 'µ', 'μ':
			sawUnit = true
			if ch = s.next(); ch == 's' {
				ch = s.next()
				break
			}
			fallthrough
		default:
			// this is an invalid duration if we already saw a unit, and an
			// integer otherwise
			if sawUnit {
				s.errorf("invalid duration unit")
				return ScanResultDuration, ch
			}
			return ScanResultInt, ch
		}

		// non-decimal ends a duration
		if !isDecimal(ch) {
			break
		}

		// scan digits: a standalone '0' is ok, but leading '0' starts an
		// octal number, which is invalid in duration.
		ch0, n := ch, 0
		ch, n = s.digits(s.next(), 10, nil)
		notDec = notDec || (ch0 == '0' && n > 0)
	}

	if notDec {
		s.error("duration requires decimal integer")
	}

	return ScanResultDuration, ch
}

func (s *Scanner) doScanNumberOrDuration(ch rune, sawDot bool) (rune, rune) {
	base := 10         // number base
	prefix := rune(0)  // one of 0 (decimal), '0' (0-octal), 'x', 'o', or 'b'
	hasDigit := false  // whether a digit present
	invalid := rune(0) // invalid digit in literal, or 0

	// integer part
	var tok rune
	if !sawDot {
		tok = ScanResultInt
		if ch == '0' {
			ch = s.next()
			switch lower(ch) {
			case 'x':
				ch = s.next()
				base, prefix = 16, 'x'
			case 'o':
				ch = s.next()
				base, prefix = 8, 'o'
			case 'b':
				ch = s.next()
				base, prefix = 2, 'b'
			default:
				hasDigit = true // leading 0
				// '0s' is a valid duration, but duration requires base to be
				// 10, so only set base to 8 when there's another digit after
				// '0'.
				if isDecimal(ch) {
					base, prefix = 8, '0'
				}
			}
		}
		var n int
		if ch, n = s.digits(ch, base, &invalid); ch == '.' {
			ch = s.next()
			sawDot = true
		}
		hasDigit = hasDigit || (n > 0)
	}

	// fractional part
	if sawDot {
		tok = ScanResultFloat
		if prefix == 'o' || prefix == 'b' {
			s.error("invalid radix point in " + litname(prefix))
		}
		var n int
		ch, n = s.digits(ch, base, &invalid)
		hasDigit = hasDigit || (n > 0)
	}

	if !hasDigit {
		s.error(litname(prefix) + " has no digits")
	}

	// exponent
	if e := lower(ch); e == 'e' || e == 'p' {
		switch {
		case e == 'e' && prefix != 0 && prefix != '0':
			s.errorf("%q exponent requires decimal mantissa", ch)
		case e == 'p' && prefix != 'x':
			s.errorf("%q exponent requires hexadecimal mantissa", ch)
		}
		ch = s.next()
		tok = ScanResultFloat
		if ch == '+' || ch == '-' {
			ch = s.next()
		}
		var n int
		if ch, n = s.digits(ch, 10, nil); n == 0 {
			s.error("exponent has no digits")
		}
	} else if prefix == 'x' && tok == ScanResultFloat {
		s.error("hexadecimal mantissa requires a 'p' exponent")
	}

	if tok != ScanResultInt {
		return tok, ch
	}

	if invalid != 0 {
		s.errorf("invalid digit %q in %s", invalid, litname(prefix))
	}

	// it can also be a duration, so make a try.
	if base == 10 || (base == 8 && prefix == '0') {
		tok, ch = s.duration(ch, base != 10)
	}

	return tok, ch
}

// scanNumberOrDurations accepts an integer, a floating point number or a
// duration.
func (s *Scanner) scanNumberOrDuration(ch rune, sawDot bool) (rune, rune) {
	tok, ch := s.doScanNumberOrDuration(ch, sawDot)

	hasExtra := false
	for isIdentRune(ch) {
		hasExtra = true
		ch = s.next()
	}
	if hasExtra {
		s.errorf("extra character after %s", TokenString(tok))
	}

	return tok, ch
}

func litname(prefix rune) string {
	switch prefix {
	default:
		return "decimal literal"
	case 'x':
		return "hexadecimal literal"
	case 'o', '0':
		return "octal literal"
	case 'b':
		return "binary literal"
	}
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case 'a' <= lower(ch) && lower(ch) <= 'f':
		return int(lower(ch) - 'a' + 10)
	}
	return 16 // larger than any legal digit val
}

func (s *Scanner) scanDigits(ch rune, base, n int) rune {
	for n > 0 && digitVal(ch) < base {
		ch = s.next()
		n--
	}
	if n > 0 {
		s.error("invalid char escape")
	}
	return ch
}

func (s *Scanner) scanEscape(quote byte) rune {
	ch := s.next() // read character after '\'
	switch ch {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', rune(quote):
		// nothing to do
		ch = s.next()
	case '0', '1', '2', '3', '4', '5', '6', '7':
		ch = s.scanDigits(ch, 8, 3)
	case 'x':
		ch = s.scanDigits(s.next(), 16, 2)
	case 'u':
		ch = s.scanDigits(s.next(), 16, 4)
	case 'U':
		ch = s.scanDigits(s.next(), 16, 8)
	default:
		s.error("invalid char escape")
	}
	return ch
}

func (s *Scanner) scanString(quote byte) rune {
	ch := s.next() // read character after quote
	for ch != rune(quote) {
		if ch == '\n' || ch < 0 {
			s.error("literal not terminated")
			break
		}
		if ch == '\\' {
			ch = s.scanEscape(quote)
		} else {
			ch = s.next()
		}
	}
	return s.next()
}

func (s *Scanner) scanRawString() rune {
	ch := s.next() // read character after '`'
	for ch != '`' {
		if ch < 0 {
			s.error("literal not terminated")
			break
		}
		ch = s.next()
	}
	return s.next()
}

func (s *Scanner) scanLineComment() rune {
	ch := s.next() // read character after "--"
	for ch != '\n' && ch >= 0 {
		ch = s.next()
	}
	return ch
}

func (s *Scanner) scanBlockComment() rune {
	// general comment
	ch := s.next() // read character after "/*"
	for {
		if ch < 0 {
			s.error("comment not terminated")
			break
		}
		ch0 := ch
		ch = s.next()
		if ch0 == '*' && ch == '/' {
			ch = s.next()
			break
		}
	}
	return ch
}

// Scan reads the next token or Unicode character from source and returns it.
// It returns [ScanResultEOF] at the end of the source. It reports scanner
// errors (read and token errors) by calling s.Error.
func (s *Scanner) Scan() rune {
	ch := s.Peek()

	// reset token text position
	s.tokPos = -1
	s.Line = 0

	// skip white space
	for isWhitespace(ch) {
		ch = s.next()
	}

	// start collecting token text
	s.tokBuf.Reset()
	s.tokPos = s.srcPos - s.lastCharLen

	// set token position
	// (this is a slightly optimized version of the code in Pos())
	s.Offset = s.srcBufOffset + s.tokPos
	if s.column > 0 {
		// common case: last character was not a '\n'
		s.Line = s.line
		s.Column = s.column
	} else {
		// last character was a '\n'
		// (we cannot be at the beginning of the source
		// since we have called next() at least once)
		s.Line = s.line - 1
		s.Column = s.lastLineLen
	}

	// determine token value
	tok := ch
	switch {
	case ch == '_' || unicode.IsLetter(ch):
		ch = s.scanIdentifier()
		tok = ScanResultIdent

	case isDecimal(ch):
		tok, ch = s.scanNumberOrDuration(ch, false)

	default:
		switch ch {
		case ScanResultEOF:
			break

		case '-':
			if ch = s.next(); ch == '-' {
				ch = s.scanLineComment()
				tok = ScanResultComment
			}

		case '/':
			if ch = s.next(); ch == '*' {
				ch = s.scanBlockComment()
				tok = ScanResultComment
			}

		case '.':
			if ch = s.next(); isDecimal(ch) {
				tok, ch = s.scanNumberOrDuration(ch, true)
			}

		case '"':
			ch = s.scanString('"')
			tok = ScanResultQuotedIdent

		case '\'':
			ch = s.scanString('\'')
			tok = ScanResultString

		case '`':
			ch = s.scanRawString()
			tok = ScanResultRawString

		default:
			ch = s.next()
		}
	}

	// end of token text
	s.tokEnd = s.srcPos - s.lastCharLen

	s.ch = ch
	return tok
}

// Pos returns the position of the character immediately after
// the character or token returned by the last call to [Scanner.Scan].
// Use the [Scanner.Position] field for the start position of the most
// recently scanned token.
func (s *Scanner) Pos() (pos Position) {
	pos.Filename = s.Filename
	pos.Offset = s.srcBufOffset + s.srcPos - s.lastCharLen
	switch {
	case s.column > 0:
		// common case: last character was not a '\n'
		pos.Line = s.line
		pos.Column = s.column
	case s.lastLineLen > 0:
		// last character was a '\n'
		pos.Line = s.line - 1
		pos.Column = s.lastLineLen
	default:
		// at the beginning of the source
		pos.Line = 1
		pos.Column = 1
	}
	return
}

// TokenText returns the string corresponding to the most recently scanned token.
// Valid after calling [Scanner.Scan] and in calls of [Scanner.Error].
func (s *Scanner) TokenText() string {
	if s.tokPos < 0 {
		// no token text
		return ""
	}

	if s.tokEnd < s.tokPos {
		// if EOF was reached, s.tokEnd is set to -1 (s.srcPos == 0)
		s.tokEnd = s.tokPos
	}
	// s.tokEnd >= s.tokPos

	if s.tokBuf.Len() == 0 {
		// common case: the entire token text is still in srcBuf
		return string(s.srcBuf[s.tokPos:s.tokEnd])
	}

	// part of the token text was saved in tokBuf: save the rest in
	// tokBuf as well and return its content
	s.tokBuf.Write(s.srcBuf[s.tokPos:s.tokEnd])
	s.tokPos = s.tokEnd // ensure idempotency of TokenText() call
	return s.tokBuf.String()
}
