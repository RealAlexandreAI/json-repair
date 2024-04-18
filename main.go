package jsonrepair

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
)

// RepairJSON
//
//	Description:
//	param in
//	return string
func RepairJSON(in string) string {
	in = strings.TrimSpace(in)
	in = strings.TrimPrefix(in, "```json")

	if json.Valid([]byte(in)) {
		dst := &bytes.Buffer{}
		json.Compact(dst, []byte(in))
		return dst.String()
	}

	jp := NewJSONParser(in)
	marshal, _ := json.Marshal(jp.parseJSON())
	return string(marshal)
}

// NewJSONParser
//
//	Description:
//	param in
//	return *JSONParser
func NewJSONParser(in string) *JSONParser {
	return &JSONParser{
		container: in,
		index:     0,
		marker:    "",
	}
}

// JSONParser
// Description:
type JSONParser struct {
	container string
	index     int
	marker    string
}

// parseJSON
//
//	Description:
//	receiver p
//	return interface{}
func (p *JSONParser) parseJSON() interface{} {
	c, b := p.currentByte()
	if !b {
		return ""
	}

	var lc string
	if unicode.IsLetter(rune(c)) {
		lc = strings.ToLower(string(c))
	}

	if c == '{' {
		p.index++
		return p.parseObject()
	} else if c == '[' {
		p.index++
		return p.parseArray()
	} else if c == '}' && p.marker == "object_value" {
		return ""
	} else if c == '"' {
		return p.parseString()
	} else if c == '\'' {
		return p.parseString('\'')
		// TODO Full-width character support
		//} else if c == 0xE3 && p.index <= len(p.container)-3 && p.container[p.index+1] == 0x80 && p.container[p.index+2] == 0x8C {
		//	return p.parseString('“', '”')
	} else if unicode.IsNumber(rune(c)) || c == '-' {
		return p.parseNumber()
	} else if lc == "t" || lc == "f" || lc == "n" {
		return p.parseBooleanOrNull()
	} else if unicode.IsLetter(rune(c)) {
		return p.parseString()
	} else {
		p.index++
		return p.parseJSON()
	}
}

// parseObject
//
//	Description:
//	receiver p
//	return map[string]interface{}
func (p *JSONParser) parseObject() map[string]interface{} {
	rst := make(map[string]interface{})

	var c byte
	var b bool

	for c, b = p.currentByte(); b && c != '}'; {
		p.skipWhitespaces()

		c, b = p.currentByte()
		if b && c == ':' {
			p.removeByte(0)
			p.insertByte(',')
			p.index++
		}

		p.marker = "object_key"
		p.skipWhitespaces()

		var key string
		_, b = p.currentByte()
		for key = ""; key == "" && b; {
			key = p.parseJSON().(string)

			c, b = p.currentByte()
			if key == "" && c == ':' {
				key = "empty_placeholder"
				break
			}
		}

		c, b = p.currentByte()
		if b && c == '}' {
			continue
		}

		c, b = p.currentByte()
		if b && c != ':' {
			p.insertByte(':')
		}

		p.index++
		p.marker = "object_value"
		value := p.parseJSON()

		p.marker = ""
		if key == "" && value == "" {
			continue
		}
		rst[key] = value

		c, b = p.currentByte()
		if b && contains([]byte{',', '\'', '"'}, c) {
			p.index++
		}

		p.skipWhitespaces()
	}

	c, b = p.currentByte()
	if b && c != '}' {
		p.insertByte('}')
	}
	p.index++
	return rst
}

// parseArray
//
//	Description:
//	receiver p
//	return []interface{}
func (p *JSONParser) parseArray() []interface{} {
	rst := make([]interface{}, 0)
	var c byte
	var b bool

	for c, b = p.currentByte(); b && c != ']'; {
		value := p.parseJSON()

		if value == nil {
			break
		}
		if tc, ok := value.(string); ok && tc == "" {
			break
		}

		rst = append(rst, value)

		c, b = p.currentByte()
		for b && (unicode.IsSpace(rune(c)) || c == ',') {
			p.index++
			c, b = p.currentByte()
		}
	}

	c, b = p.currentByte()
	if b && c != ']' {
		if c == ',' {
			p.removeByte(0)
		}
		p.insertByte(']')
	}

	p.index++
	return rst
}

// parseString
//
//	Description:
//	receiver p
//	param quotes
//	return interface{}
func (p *JSONParser) parseString(quotes ...byte) interface{} {
	fixedQuotes, doubleDelimiter := false, false
	var lStringDelimiter, rStringDelimiter byte = '"', '"'

	switch len(quotes) {
	case 2:
		lStringDelimiter = quotes[0]
		rStringDelimiter = quotes[1]
	case 1:
		lStringDelimiter = quotes[0]
		rStringDelimiter = quotes[0]
	}

	if p.index+1 < len(p.container) && p.container[p.index+1] == lStringDelimiter {
		doubleDelimiter = true
		p.index++
	}

	var c byte
	var b bool
	c, b = p.currentByte()

	if b && c != lStringDelimiter {
		p.insertByte(lStringDelimiter)
		fixedQuotes = true
	} else {
		p.index++
	}

	start := p.index
	c, b = p.currentByte()

	fixBrokenMarkdownLink := false

	for b && c != rStringDelimiter {
		if fixedQuotes {
			if p.marker == "object_key" && (c == ':' || unicode.IsSpace(rune(c))) {
				break
			} else if p.marker == "object_value" && contains([]byte{',', '}'}, c) {
				break
			}
		}

		p.index++
		c, b = p.currentByte()

		if p.index-1 >= 0 && p.container[p.index-1] == '\\' {
			if contains([]byte{rStringDelimiter, 't', 'n', 'r', 'b', '\\'}, c) {
				p.index++
				c, b = p.currentByte()
			} else {
				p.removeByte(-1)
				p.index--
			}
		}

		if c == rStringDelimiter && p.index+1 < len(p.container) && p.container[p.index+1] != ',' &&
			(fixBrokenMarkdownLink || (p.index-2 >= 0 && p.container[p.index-2] == ']') && (p.index-1 >= 0 && p.container[p.index-1] == '(')) {
			fixBrokenMarkdownLink = !fixBrokenMarkdownLink
			p.index++
			c, b = p.currentByte()
		}

	}

	if b && fixedQuotes && p.marker == "object_key" && unicode.IsSpace(rune(c)) {
		p.skipWhitespaces()
		c, b = p.currentByte()
		if !b || !contains([]byte{':', ','}, c) {
			return ""
		}
	}

	end := p.index

	if c != rStringDelimiter {
		p.insertByte(rStringDelimiter)
	} else {
		p.index++
		if doubleDelimiter && p.container[p.index] == rStringDelimiter {
			p.index++
		}
	}

	return p.container[start:end]
}

// parseNumber
//
//	Description:
//	receiver p
//	return interface{}
func (p *JSONParser) parseNumber() interface{} {
	var rst []byte
	numberChars := []byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-', '.', 'e', 'E'}

	var c byte
	var b bool

	for c, b = p.currentByte(); b && contains(numberChars, c); {
		rst = append(rst, c)
		p.index++
		c, b = p.currentByte()
	}

	if len(rst) > 0 {
		if contains(rst, '.') || contains(rst, 'e') || contains(rst, 'E') {
			r, _ := strconv.ParseFloat(string(rst), 32)
			return r
		} else if string(rst) == "-" {
			return p.parseJSON()
		} else {
			r, _ := strconv.Atoi(string(rst))
			return r
		}
	} else {
		return p.parseString()
	}
}

// parseBooleanOrNull
//
//	Description:
//	receiver p
//	return interface{}
func (p *JSONParser) parseBooleanOrNull() interface{} {
	ls := strings.ToLower(p.container[p.index:])

	if strings.HasPrefix(ls, "true") {
		p.index += 4
		return true
	} else if strings.HasPrefix(ls, "false") {
		p.index += 5
		return false
	} else if strings.HasPrefix(ls, "null") {
		p.index += 4
		return nil
	}

	return p.parseString()
}

// skipWhitespaces
//
//	Description:
//	receiver p
func (p *JSONParser) skipWhitespaces() {
	var c byte
	var b bool
	c, b = p.currentByte()

	for b && unicode.IsSpace(rune(c)) {
		p.index++
		c, b = p.currentByte()
	}
}

// currentChar
//
//	Description:
//	receiver p
//	return byte
//	return bool
func (p *JSONParser) currentByte() (byte, bool) {
	if p.index >= len(p.container) {
		return ' ', false
	}

	return p.container[p.index], true
}

// removeByte
//
//	Description:
//	receiver p
//	param count
func (p *JSONParser) removeByte(count int) {
	p.container = p.container[:p.index+count] + p.container[p.index+1+count:]
}

// insertByte
//
//	Description:
//	receiver p
//	param in
func (p *JSONParser) insertByte(in byte) {
	p.container = p.container[:p.index] + string(in) + p.container[p.index:]
	p.index++
}

// contains
//
//	Description:
//	param slice
//	param element
//	return bool
func contains(slice []byte, element byte) bool {
	for _, el := range slice {
		if el == element {
			return true
		}
	}
	return false
}
