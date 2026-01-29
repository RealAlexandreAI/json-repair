package jsonrepair

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// RepairJSON
//
//	@Description:
//	@param src
//	@return dst
//	@return err
func RepairJSON(src string) (dst string, err error) {
	defer func() {
		if errR := recover(); errR != nil {
			stack := string(debug.Stack())
			err = fmt.Errorf("repair json panic: %s", stack)
			return
		}
	}()

	src = strings.TrimSpace(src)
	src = strings.TrimPrefix(src, "```json")

	if json.Valid([]byte(src)) {
		buf := &bytes.Buffer{}
		if err = json.Compact(buf, []byte(src)); err != nil {
			return "", err
		}
		dst = buf.String()
		return
	}

	jp := NewJSONParser(src)
	result := jp.parseJSON()

	// Try to marshal the result
	bs, err := JSONMarshal(result)
	if err != nil {
		return "", err
	}
	dst = string(bs)

	// If the result is valid JSON, trim it and only keep the valid part
	dst = strings.TrimSpace(dst)
	return
}

// MustRepairJSON
//
//	@Description:
//	@param src
//	@return dst
func MustRepairJSON(src string) (dst string) {
	defer func() {
		if errR := recover(); errR != nil {
			dst = ""
			return
		}
	}()

	src = strings.TrimSpace(src)
	src = strings.TrimPrefix(src, "```json")

	if json.Valid([]byte(src)) {
		buf := &bytes.Buffer{}
		//nolint
		json.Compact(buf, []byte(src))
		dst = buf.String()
		return
	}

	jp := NewJSONParser(src)
	bs, _ := JSONMarshal(jp.parseJSON())
	dst = string(bs)
	return
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
		marker:    []string{},
	}
}

// JSONParser
// Description:
type JSONParser struct {
	container     string
	index         int
	marker        []string
	recursionDepth int
}

const maxRecursionDepth = 1000

// parseJSON
//
//	Description:
//	receiver p
//	return any
func (p *JSONParser) parseJSON() any {
	// Prevent stack overflow by checking recursion depth
	p.recursionDepth++
	defer func() { p.recursionDepth-- }()

	if p.recursionDepth > maxRecursionDepth {
		return ""
	}

	startIndex := p.index
	consecutiveNoProgress := 0

	for {
		c, b := p.getByte(0)

		if !b {
			return ""
		}

		// Detect infinite loop: if we haven't moved forward in several iterations
		if p.index == startIndex {
			consecutiveNoProgress++
			if consecutiveNoProgress > 10 {
				return ""
			}
		} else {
			startIndex = p.index
			consecutiveNoProgress = 0
		}

		isInMarkers := len(p.marker) > 0

		switch {
		case c == '{':
			p.index++
			return p.parseObject()
		case c == '[':
			p.index++
			return p.parseArray()
		case c == '}':
			return ""
		// TODO Full-width character support
		case isInMarkers && (bytes.IndexByte([]byte{'"', '\''}, c) != -1 || unicode.IsLetter(rune(c))):
			return p.parseString()
		case isInMarkers && (unicode.IsNumber(rune(c)) || bytes.IndexByte([]byte{'-', '.'}, c) != -1):
			return p.parseNumber()
		}

		p.index++
	}

}

// parseObject
//
//	Description:
//	receiver p
//	return map[string]any
func (p *JSONParser) parseObject() map[string]any {

	rst := make(map[string]any)

	var c byte
	var b bool

	c, b = p.getByte(0)

	for b && c != '}' {
		p.skipWhitespaces()

		c, b = p.getByte(0)
		if b && c == ':' {
			p.index++
		}

		p.setMarker("object_key")
		p.skipWhitespaces()

		var key string
		_, b = p.getByte(0)
		for key == "" && b {
			currentIndex := p.index
			key = p.parseString().(string)

			c, b = p.getByte(0)
			if key == "" && b && c == ':' {
				key = "empty_placeholder"
				break
			} else if key == "" && p.index == currentIndex {
				p.index++
			}
		}

		p.skipWhitespaces()

		c, b = p.getByte(0)
		if b && c == '}' {
			continue
		}

		p.skipWhitespaces()

		c, b = p.getByte(0)
		//nolint
		if !b || c != ':' {
		}

		p.index++
		p.resetMarker()
		p.setMarker("object_value")
		value := p.parseJSON()

		p.resetMarker()
		if key == "" && value == "" {
			continue
		}
		rst[key] = value

		c, b = p.getByte(0)
		if b && bytes.IndexByte([]byte{',', '\'', '"'}, c) != -1 {
			p.index++
		}

		p.skipWhitespaces()
		c, b = p.getByte(0)
	}

	c, b = p.getByte(0)
	//nolint
	if b && c != '}' {
	}

	p.index++
	return rst
}

// parseArray
//
//	Description:
//	receiver p
//	return []any
func (p *JSONParser) parseArray() []any {

	rst := make([]any, 0)

	var c byte
	var b bool

	p.setMarker("array")

	c, b = p.getByte(0)

	for b && c != ']' {

		p.skipWhitespaces()
		value := p.parseJSON()

		if value == nil || value == "" {
			break
		}

		if tc, ok := value.(string); ok && tc == "" {
			break
		}

		c, b = p.getByte(-1)
		if value == "..." && b && c == '.' {
		} else {
			rst = append(rst, value)
		}

		c, b = p.getByte(0)
		for b && (unicode.IsSpace(rune(c)) || c == ',') {
			p.index++
			c, b = p.getByte(0)
		}

		if p.getMarker() == "object_value" && c == '}' {
			break
		}
	}

	c, b = p.getByte(0)
	if b && c != ']' {
		//nolint
		if c == ',' {
		}
		p.index--
	}

	p.index++
	p.resetMarker()
	return rst
}

// parseString
//
//	Description:
//	receiver p
//	param quotes
//	return any
func (p *JSONParser) parseString() any {

	var missingQuotes, doubledQuotes = false, false
	var lStringDelimiter, rStringDelimiter byte = '"', '"'

	var c byte
	var b bool

	c, b = p.getByte(0)
	for b && bytes.IndexByte([]byte{'"', '\''}, c) == -1 && !unicode.IsLetter(rune(c)) {
		p.index++
		c, b = p.getByte(0)
	}

	if !b {
		return ""
	}

	switch {
	case c == '\'':

		lStringDelimiter = '\''
		rStringDelimiter = '\''
	case unicode.IsLetter(rune(c)):

		if bytes.IndexByte([]byte{'t', 'f', 'n'}, byte(unicode.ToLower(rune(c)))) != -1 &&
			p.getMarker() != "object_key" {
			value := p.parseBooleanOrNull()
			if vs, ok := value.(string); !ok {
				return value
			} else {
				if vs != "" {
					return vs
				}
			}
		}

		missingQuotes = true
	}

	if !missingQuotes {
		p.index++
	}

	c, b = p.getByte(0)

	if b && c == lStringDelimiter {
		i := 1
		nextC, nextB := p.getByte(i)
		for nextB && nextC != rStringDelimiter {
			i++
			nextC, nextB = p.getByte(i)
		}

		c, b = p.getByte(i + 1)
		if nextB && b && c == rStringDelimiter {
			doubledQuotes = true
			p.index++
		} else {
			i = 1
			nextC, nextB = p.getByte(i)
			// Skip all whitespace characters (space, newline, tab, etc.)
			for nextB && unicode.IsSpace(rune(nextC)) {
				i++
				nextC, nextB = p.getByte(i)
			}

			// Fix for Issue #19: properly handle empty strings followed by structural characters
			if nextB && bytes.IndexByte([]byte{',', ']', '}'}, nextC) != -1 {
				// This is an empty string, skip the closing quote and return
				p.index++
				return ""
			} else if nextB && bytes.IndexByte([]byte{',', ']', '}'}, nextC) == -1 {
				p.index++
			}
		}
	}

	var rst []byte

	c, b = p.getByte(0)

	for b && c != rStringDelimiter {
		if missingQuotes {
			if p.getMarker() == "object_key" && (c == ':' || unicode.IsSpace(rune(c))) {
				break
			} else if p.getMarker() == "object_value" && bytes.IndexByte([]byte{',', '}'}, c) != -1 {

				rStringDelimiterMissing := true
				i := 1
				nextC, nextB := p.getByte(i)
				for nextB && nextC != rStringDelimiter {
					i++
					nextC, nextB = p.getByte(i)
				}

				if nextB {
					i++
					nextC, nextB = p.getByte(i)
				}

				for nextB && nextC == ' ' {
					i++
					nextC, nextB = p.getByte(i)
				}

				if nextB && bytes.IndexByte([]byte{',', '}'}, nextC) != -1 {
					rStringDelimiterMissing = false
				}

				if rStringDelimiterMissing {
					break
				}

			}
		}

		rst = append(rst, c)
		p.index++

		c, b = p.getByte(0)

		if len(rst) > 1 && rst[len(rst)-1] == '\\' {

			rst = rst[:len(rst)-1]

			if bytes.IndexByte([]byte{rStringDelimiter, 't', 'n', 'r', 'b', '\\'}, c) != -1 {

				escapeSeqs := map[byte]byte{
					't': '\t',
					'n': '\n',
					'r': '\r',
					'b': '\b',
				}

				if ce, ok := escapeSeqs[c]; ok {
					rst = append(rst, ce)
				} else {
					rst = append(rst, c)
				}

				p.index++
				c, b = p.getByte(0)
			}
		}

		if c == rStringDelimiter {
			// Special handling for unescaped quotes inside string values (Issue #18)
			// Check if this quote is actually inside the string content
			if !missingQuotes && p.getMarker() == "object_value" {
				i := 1
				nextC, nextB := p.getByte(i)

				// Skip whitespace after the quote
				for nextB && unicode.IsSpace(rune(nextC)) {
					i++
					nextC, nextB = p.getByte(i)
				}

				// Check if this is truly the end of the string
				// A real string end should be followed by: , or } or ]
				isFollowedByStructural := nextB && bytes.IndexByte([]byte{',', '}', ']'}, nextC) != -1

				if !isFollowedByStructural {
					// Not followed by structural character
					// This is likely an unescaped quote inside the string
					// Look ahead to find the next quote that IS followed by structural char
					j := i
					foundBetterEnd := false
					for j < 200 {
						nc, nb := p.getByte(j)
						if !nb {
							break
						}
						if nc == rStringDelimiter {
							// Check what follows this quote
							afterQuote, afterQuoteB := p.getByte(j + 1)
							k := j + 1
							for afterQuoteB && unicode.IsSpace(rune(afterQuote)) {
								k++
								afterQuote, afterQuoteB = p.getByte(k)
							}
							if afterQuoteB && bytes.IndexByte([]byte{',', '}', ']'}, afterQuote) != -1 {
								// Found a quote followed by structural char
								// But need to verify: if it's comma, what comes after?
								if afterQuote == ',' {
									// Check if after comma there's a proper key-value pair
									// or if it's more string content
									kk := k + 1
									afterComma, afterCommaB := p.getByte(kk)
									for afterCommaB && unicode.IsSpace(rune(afterComma)) {
										kk++
										afterComma, afterCommaB = p.getByte(kk)
									}
									// If after comma is NOT a quote (start of key), comma is part of content
									if afterCommaB && afterComma != rStringDelimiter {
										// Continue searching
										j++
										continue
									}
								}
								// Found a better end
								foundBetterEnd = true
								break
							}
						}
						j++
					}

					if foundBetterEnd {
						// Continue reading, treating current quote as content
						rst = append(rst, c)
						p.index++
						c, b = p.getByte(0)
						continue
					}
				}
				// If followed by structural char, or no better end found, treat as string end
			}

			if doubledQuotes && p.container[p.index+1] == rStringDelimiter {

			} else if missingQuotes && p.getMarker() == "object_value" {

				i := 1
				nextC, nextB := p.getByte(i)
				for nextB && bytes.IndexByte([]byte{rStringDelimiter, lStringDelimiter}, nextC) == -1 {
					i++
					nextC, nextB = p.getByte(i)
				}

				if nextB {
					i++
					nextC, nextB = p.getByte(i)
					for nextB && nextC == ' ' {
						i++
						nextC, nextB = p.getByte(i)
					}

					if nextB && nextC == ':' {
						p.index--
						c, b = p.getByte(0)
						break
					}
				}

			} else {

				i := 1
				nextC, nextB := p.getByte(i)
				checkCommaInObjectValue := true
				for nextB && bytes.IndexByte([]byte{rStringDelimiter, lStringDelimiter}, nextC) == -1 {

					if unicode.IsLetter(rune(c)) {
						checkCommaInObjectValue = false
					}

					if (slices.Contains(p.marker, "object_key") && bytes.IndexByte([]byte{':', '}'}, nextC) != -1) ||
						(slices.Contains(p.marker, "object_value") && nextC == '}') ||
						(slices.Contains(p.marker, "array") && bytes.IndexByte([]byte{']', ','}, nextC) != -1) ||
						(checkCommaInObjectValue && p.getMarker() == "object_value" && nextC == ',') {
						break
					}

					i++
					nextC, nextB = p.getByte(i)
				}

				if nextC == ',' && p.getMarker() == "object_value" {
					i++
					nextC, nextB = p.getByte(i)
					for nextB && nextC != rStringDelimiter {
						i++
						nextC, nextB = p.getByte(i)
					}
					i++
					nextC, nextB = p.getByte(i)

					for nextB && nextC == ' ' {
						i++
						nextC, nextB = p.getByte(i)
					}

					if nextB && nextC == '}' {
						rst = append(rst, c)
						p.index++
						c, b = p.getByte(0)
					}
				} else if nextB && nextC == rStringDelimiter {

					if p.getMarker() == "object_value" {
						i++
						nextC, nextB = p.getByte(i)
						for nextB && nextC != rStringDelimiter {
							i++
							nextC, nextB = p.getByte(i)
						}
						i++
						nextC, nextB = p.getByte(i)
						for nextB && nextC != ':' {
							if bytes.IndexByte([]byte{',', lStringDelimiter, rStringDelimiter}, nextC) != -1 {
								break
							}
							i++
							nextC, nextB = p.getByte(i)
						}

						if nextC != ':' {
							rst = append(rst, c)
							p.index++
							c, b = p.getByte(0)
						}

					}

				}
			}
		}
	}

	if b && missingQuotes &&
		p.getMarker() == "object_key" &&
		unicode.IsSpace(rune(c)) {
		p.skipWhitespaces()
		ci, bi := p.getByte(0)
		if !bi || bytes.IndexByte([]byte{':', ','}, ci) == -1 {
			return ""
		}
	}

	if !b || c != rStringDelimiter {
	} else {
		p.index++
	}

	return strings.TrimRightFunc(string(rst), unicode.IsSpace)
}

// parseNumber
//
//	Description:
//	receiver p
//	return any
func (p *JSONParser) parseNumber() any {
	var rst []byte

	numberChars := []byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-', '.', 'e', 'E', '/', ','}

	var c byte
	var b bool

	c, b = p.getByte(0)

	isArray := p.getMarker() == "array"

	for b && bytes.IndexByte(numberChars, c) != -1 &&
		(c != ',' || !isArray) {
		rst = append(rst, c)
		p.index++
		c, b = p.getByte(0)
	}

	if len(rst) > 1 && bytes.IndexByte([]byte{'-', 'e', 'E', '/', ','}, rst[len(rst)-1]) != -1 {
		rst = rst[:len(rst)-1]
		p.index--
	}

	switch {
	case len(rst) == 0:
		// Avoid infinite recursion by returning empty string instead
		return ""
	case bytes.IndexByte(rst, ',') != -1:
		return string(rst)
	case bytes.IndexByte(rst, '.') != -1,
		bytes.IndexByte(rst, 'e') != -1,
		bytes.IndexByte(rst, 'E') != -1:
		r, _ := strconv.ParseFloat(string(rst), 32)
		return r
	case string(rst) == "-":
		// Avoid infinite recursion by returning 0 instead
		return 0
	}

	r, _ := strconv.Atoi(string(rst))
	return r
}

// parseBooleanOrNull
//
//	Description:
//	receiver p
//	return any
func (p *JSONParser) parseBooleanOrNull() any {

	startingIndex := p.index

	type genericStruct struct {
		va string
		vt any
	}

	var gs *genericStruct

	var c byte
	var b bool
	c, b = p.getByte(0)
	c = byte(unicode.ToLower(rune(c)))

	if b {
		switch {
		case c == 't':
			gs = &genericStruct{
				va: "true",
				vt: true,
			}
		case c == 'f':
			gs = &genericStruct{
				va: "false",
				vt: false,
			}
		case c == 'n':
			gs = &genericStruct{
				va: "null",
				vt: nil,
			}
		}
	}

	if gs != nil {
		i := 0
		for b && i < len(gs.va) && c == gs.va[i] {
			i++
			p.index++
			c, b = p.getByte(0)
			c = byte(unicode.ToLower(rune(c)))
		}

		if i == len(gs.va) {
			return gs.vt
		}
	}

	p.index = startingIndex
	return ""
}

// currentChar
//
//	Description:
//	receiver p
//	return byte
//	return bool
func (p *JSONParser) getByte(count int) (byte, bool) {
	if p.index+count < 0 || p.index+count >= len(p.container) {
		return ' ', false
	}

	return p.container[p.index+count], true
}

// skipWhitespaces
//
//	Description:
//	receiver p
func (p *JSONParser) skipWhitespaces() {
	var c byte
	var b bool
	c, b = p.getByte(0)

	for b && unicode.IsSpace(rune(c)) {
		p.index++
		c, b = p.getByte(0)
	}
}

// setMarker
//
//	@Description:
//	@receiver p
//	@param in
func (p *JSONParser) setMarker(in string) {
	if in != "" {
		p.marker = append(p.marker, in)
	}
}

// resetMarker
//
//	@Description:
//	@receiver p
func (p *JSONParser) resetMarker() {
	if len(p.marker) > 0 {
		p.marker = p.marker[:len(p.marker)-1]
	}
}

// getMarker
//
//	@Description:
//	@receiver p
//	@return string
func (p *JSONParser) getMarker() string {
	if len(p.marker) > 0 {
		return p.marker[len(p.marker)-1]
	}

	return ""
}

// JSONMarshal
//
//	Description: ref https://stackoverflow.com/questions/28595664/how-to-stop-json-marshal-from-escaping-and
//	param t
//	return []byte
//	return error
func JSONMarshal(t any) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}
