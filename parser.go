package sjson

import (
	"fmt"
	"strings"
)

type parserState int

var retryError = fmt.Errorf("retry")

const debug = false

const (
	pFalse parserState = iota
	pTrue
	pNull
	pString
	pObject
	pArray
	pNumber
	pObjectKey
	pObjectValue
)

func (p parserState) String() string {
	switch p {
	case pFalse:
		return "pFalse"
	case pTrue:
		return "pTrue"
	case pNull:
		return "pNull"
	case pString:
		return "pString"
	case pObject:
		return "pObject"
	case pArray:
		return "pArray"
	case pNumber:
		return "pNumber"
	case pObjectKey:
		return "pObjectKey"
	case pObjectValue:
		return "pObjectValue"
	default:
		panic("invalid state")
	}
}

const (
	quote        = '"'
	leftCurly    = '{'
	rightCurly   = '}'
	leftSquared  = '['
	rightSquared = ']'
)

func isWsp(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}

type state struct {
	name     parserState
	position uint
}

type Parser struct {
	data  []byte
	stack []state
}

func (p *Parser) Reset() {
	p.data = p.data[:0]
	p.stack = p.stack[:0]
}

func (p *Parser) state() state {
	return p.stack[len(p.stack)-1]
}

func (p *Parser) prevByte() byte {
	if len(p.data) == 0 {
		return 0x00
	}

	return p.data[len(p.data)-1]
}

func (p *Parser) prevRelByte() byte {
	if len(p.data) == 0 {
		return 0x00
	}
	if len(p.data)-1 < int(p.state().position) {
		return 0x00
	}

	return p.data[len(p.data)-1]
}

func (p *Parser) pushState(s parserState) {
	if debug {
		fmt.Printf("pushState %s\n", s)
	}
	p.stack = append(p.stack, state{
		name:     s,
		position: uint(len(p.data) - 1),
	})
}

func (p *Parser) fail(why string, args ...any) error {
	return fmt.Errorf("failed parsing stream: %s at position %d", fmt.Sprintf(why, args...), len(p.data)-1)
}

func (p *Parser) popState() {
	if len(p.stack) == 0 {
		return
	}
	if debug {
		next := "none"
		if len(p.stack)-2 >= 0 {
			next = p.stack[len(p.stack)-2].name.String()
		}
		fmt.Printf("popState (current was %s, will be %s)\n", p.state().name, next)
	}
	p.stack = p.stack[:len(p.stack)-1]
}

func (p *Parser) replaceState(new parserState) {
	if debug {
		fmt.Printf("replaceState %s -> %s\n", p.state().name, new)
	}
	p.popState()
	p.pushState(new)
}

func (p *Parser) retry() error {
	p.popState()
	return retryError
}

func (p *Parser) append(b byte) {
	p.data = append(p.data, b)
}

func (p *Parser) handleWordParsing(word string, b byte) error {
	idx := len(p.data) - int(p.state().position)

	if b != word[idx] {
		return p.fail("expected %c (reading '%s'), found `%c' instead", word[idx], word, b)
	}
	p.append(b)
	if idx == len(word)-1 {
		p.popState()
	}

	return nil
}

func (p *Parser) Feed(b byte) ([]byte, error) {
	if len(p.stack) == 0 {
		return nil, p.parseValue(b)
	}

	err := func(b byte) error {
		var e error
		for {
			if len(p.stack) == 0 {
				e = p.fail("unexpected character '%c', as the parser state is not ready to read it", b)
				break
			}

			switch p.state().name {
			case pFalse:
				e = p.parseFalse(b)
			case pTrue:
				e = p.parseTrue(b)
			case pNull:
				e = p.parseNull(b)
			case pNumber:
				e = p.parseNumber(b)
			case pString:
				e = p.parseString(b)
			case pArray:
				e = p.parseArray(b)
			case pObject:
				e = p.parseObject(b)
			case pObjectKey:
				e = p.parseObjectKey(b)
			case pObjectValue:
				e = p.parseObjectValue(b)
			default:
				e = fmt.Errorf("bug: Unexpected parser state %#v", p.state())
			}
			if e != retryError {
				break
			}
		}
		return e
	}(b)

	if err != nil {
		return nil, err
	}

	if len(p.stack) == 0 {
		// last state was popped, we got a successful parse.
		data := p.data
		p.data = p.data[:0]
		return data, nil
	}

	return nil, nil
}

func (p *Parser) parseValue(b byte) error {
	if isWsp(b) {
		return nil
	}

	p.data = append(p.data, b)
	if b == 't' {
		p.pushState(pTrue)
	} else if b == 'f' {
		p.pushState(pFalse)
	} else if b == 'n' {
		p.pushState(pNull)
	} else if b == quote {
		p.pushState(pString)
	} else if b == leftCurly {
		p.pushState(pObject)
	} else if b == leftSquared {
		p.pushState(pArray)
	} else if b == '-' || (b >= '0' && b <= '9') {
		p.pushState(pNumber)
	} else {
		return p.fail("expected t, f, n, \", {, [, -, or a number from 0-9, got `%c'", b)
	}

	return nil
}

func (p *Parser) parseFalse(b byte) error { return p.handleWordParsing("false", b) }
func (p *Parser) parseTrue(b byte) error  { return p.handleWordParsing("true", b) }
func (p *Parser) parseNull(b byte) error  { return p.handleWordParsing("null", b) }

func (p *Parser) parseNumber(b byte) error {
	prevRel := p.prevRelByte()
	prevParse := p.data[p.state().position:]
	switch b {
	case '-':
		if prevRel != 0x00 && prevRel != 'e' && prevRel != 'E' {
			return p.fail("unexpected '-'")
		}
		p.append(b)
		return nil
	case '+':
		if prevRel != 'e' && prevRel != 'E' {
			return p.fail("unexpected '+'")
		}
		p.append(b)
		return nil
	case '.':
		if strings.ContainsAny(string(prevParse), ".eE") ||
			prevRel == '-' || string(prevParse) == "0" {
			return p.fail("unexpected '.'")
		}
		p.append(b)
		return nil
	case 'e', 'E':
		if prevRel < '0' || prevRel > '9' {
			return p.fail("unexpected '%c', expected a number", b)
		}
		p.append(b)
		return nil
	case ']', '}', ',', '\r', '\n', ' ', '\t':
		if prevRel == 'e' || prevRel == 'E' || prevRel == '+' || prevRel == '-' || prevRel == '.' {
			return p.fail("unexpected '%c', expected a number", b)
		}
		return p.retry()
	}

	// Otherwise we need a number
	if b < '0' || b > '9' {
		return p.fail("unexpected '%c'", b)
	}

	if string(prevParse) == "-0" || string(prevParse) == "0" {
		return p.fail("invalid number format")
	}

	p.append(b)
	return nil
}

func (p *Parser) parseString(b byte) error {
	prevRel := p.prevRelByte()
	p.append(b)
	if b == quote && prevRel != '\\' {
		p.popState()
		return nil
	}
	return nil
}

func (p *Parser) parseArray(b byte) error {
	if isWsp(b) {
		return nil
	}
	prevRel := p.prevRelByte()

	if b == rightSquared && prevRel != ',' {
		p.append(b)
		p.popState()
		return nil
	} else if b == ',' && prevRel != '[' && prevRel != ',' {
		p.append(b)
		return nil
	}
	if prevRel == '[' || prevRel == ',' {
		return p.parseValue(b)
	}

	return p.fail("expected ',', found `%c' instead", b)
}

func (p *Parser) parseObject(b byte) error {
	if b == rightCurly {
		p.append(b)
		p.popState()
		return nil
	}

	p.pushState(pObjectKey)
	return retryError
}

func (p *Parser) parseObjectKey(b byte) error {
	if isWsp(b) {
		return nil
	}

	prev := p.prevByte()
	if b != '"' && prev != '"' {
		// must be opening a string
		return p.fail("expected '\"', found `%c'", b)
	} else if b == '"' && prev != '"' {
		p.append(b)
		p.pushState(pString)
		return nil
	}

	if b != ':' {
		return p.fail("expected ';', found `%c'", b)
	}

	p.append(b)
	p.replaceState(pObjectValue)
	return nil
}

func (p *Parser) parseObjectValue(b byte) error {
	if isWsp(b) {
		return nil
	}

	prevRel := p.prevRelByte()
	if prevRel != ':' && b == '}' {
		return p.retry()
	}

	if prevRel != ':' && b == ',' {
		p.append(b)
		p.replaceState(pObjectKey)
		return nil
	}

	if prevRel == ':' {
		return p.parseValue(b)
	}

	return p.fail("unexpected `%c'", b)
}
