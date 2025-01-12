/*
This is free and unencumbered software released into the public domain. For more
information, see <http://unlicense.org/> or the accompanying UNLICENSE file.
*/

package gedcom

import (
	"fmt"
	"io"
	"strconv"
)

// A scanner is a GEDCOM scanning state machine.
type scanner struct {
	r      io.RuneScanner
	err    error
	state  int
	line   int
	offset int
	level  int
	buf    []rune
	tag    string
	value  string
	xref   string
}

func newScanner(r io.RuneScanner) *scanner {
	return &scanner{
		r:     r,
		state: stateBegin,
		buf:   make([]rune, 0, 4),
	}
}

const (
	stateBegin = iota
	stateLevel
	stateSeekTagOrXref
	stateSeekTag
	stateTag
	stateXref
	stateSeekValue
	stateValue
	stateEnd
	stateError
)

func (s *scanner) next() bool {
	s.state = stateBegin
	s.level = 0
	s.buf = s.buf[:0]
	s.xref = ""
	s.tag = ""
	s.value = ""
	s.offset = 0
	s.line++

	for {
		c, n, err := s.r.ReadRune()
		if err != nil {
			if err != io.EOF {
				s.state = stateError
				s.err = fmt.Errorf("read: %w", err)
			}

			if s.state != stateEnd && s.state != stateBegin {
				s.state = stateError
				s.err = io.ErrUnexpectedEOF
			}

			return false
		}
		s.offset += n

		switch s.state {
		case stateBegin:
			switch {
			case c >= '0' && c <= '9':
				s.buf = append(s.buf, c)
				s.state = stateLevel
			case isSpace(c):
				continue
			default:
				s.state = stateError
				s.err = fmt.Errorf("found non-whitespace %q (%#[1]x) before level", c)
				return false
			}
		case stateLevel:
			switch {
			case c >= '0' && c <= '9':
				s.buf = append(s.buf, c)
				continue
			case c == ' ':
				parsedLevel, perr := strconv.ParseInt(string(s.buf), 10, 64)
				if perr != nil {
					s.err = fmt.Errorf("parse level: %w", perr)
					return false
				}
				s.level = int(parsedLevel)
				s.buf = s.buf[:0]
				s.state = stateSeekTagOrXref
			default:
				s.state = stateError
				s.err = fmt.Errorf("level contained non-numerics")
				return false
			}

		case stateSeekTag:
			switch {
			case isAlphaNumeric(c):
				s.buf = append(s.buf, c)
				s.state = stateTag
			case c == ' ':
				continue
			default:
				s.state = stateError
				s.err = fmt.Errorf("tag contained non-alphanumeric (%#x)", c)
				return false
			}
		case stateSeekTagOrXref:
			switch {
			case isAlphaNumeric(c):
				s.buf = append(s.buf, c)
				s.state = stateTag
			case c == '@':
				s.state = stateXref
			case c == ' ':
				continue
			default:
				s.state = stateError
				s.err = fmt.Errorf("tag or xref contained non-alphanumeric (%#x)", c)
				return false
			}

		case stateTag:
			switch {
			case isAlphaNumeric(c):
				s.buf = append(s.buf, c)
				continue
			case c == '\n' || c == '\r':
				s.tag = string(s.buf)
				s.buf = s.buf[:0]
				s.state = stateEnd
				return true
			case c == ' ':
				s.tag = string(s.buf)
				s.buf = s.buf[:0]
				s.state = stateSeekValue
			default:
				s.state = stateError
				s.err = fmt.Errorf("tag contained non-alphanumeric (%#x)", c)
				return false
			}

		case stateXref:
			switch {
			case isAlphaNumeric(c):
				s.buf = append(s.buf, c)
				continue
			case c == '@':
				continue
			case c == ' ':
				s.xref = string(s.buf)
				s.buf = s.buf[:0]
				s.state = stateSeekTag
			default:
				s.state = stateError
				s.err = fmt.Errorf("xref contained non-alphanumeric (%#x)", c)
				return false
			}
		case stateSeekValue:
			switch {
			case c == '\n' || c == '\r':
				s.state = stateEnd
				return true
			case c == ' ':
				continue
			default:
				s.buf = append(s.buf, c)
				s.state = stateValue
			}

		case stateValue:
			switch {
			case c == '\n' || c == '\r':
				// Check to see if there is a malformed NOTE that contains an embedded newline
				// For example, Ancestry GEDCOM exports that include source "London, England, Church of England Births and Baptisms, 1813-1917"
				// have the following NOTE tag split over two lines (yet the CONC tag is correctly formatted!)
				//
				//   1 NOTE Board of Guardian Records and Church of England Parish Registers. London Metropolitan Archives, London.
				//   <p>Images produced by permission of the City of London Corporation. The City of London gives n

				if s.tag == "NOTE" {
					next, _, err := s.r.ReadRune()
					s.r.UnreadRune()
					if err == nil {
						if !isAlphaNumeric(next) {
							// Looks like it might be a malformed note, so continue parsing
							s.buf = append(s.buf, '\n')
							continue
						}
					}
				}

				s.value = string(s.buf)
				s.buf = s.buf[:0]
				s.state = stateEnd
				return true
			default:
				s.buf = append(s.buf, c)
				continue
			}
		}
	}
}

func isSpace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func isAlphaNumeric(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}
