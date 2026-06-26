package jsonrepair

import (
	"strings"
	"unicode/utf8"
)

// normalizeInput preprocesses the input string to handle common variations
// found in LLM output, especially from Chinese/multilingual models:
//
//   - Full-width structural chars (｛｝［］：，) → ASCII equivalents
//   - Code fences (```json ... ```) stripped from start/end
//   - Line comments (// ..., # ...) and block comments (/* ... */)
//
// NOTE: Quote variants (curly/typographic quotes) are NOT normalized here
// because blanket replacement would break string literals that contain
// literal quote characters (e.g. URLs with embedded quotes). Instead,
// the parser itself recognizes curly/typographic quotes as string
// delimiters on the fly.
func normalizeInput(src string) string {
	// Step 1: Normalize full-width structural characters
	src = normalizePunctuation(src)

	// Step 2: Strip code fences
	src = stripCodeFences(src)

	// Step 3: Strip comments
	src = stripComments(src)

	return src
}

// normalizePunctuation replaces full-width punctuation with ASCII equivalents.
// Does NOT touch quote characters — those are handled by the parser.
func normalizePunctuation(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))

	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])

		switch r {
		case '\uff5b': // ｛ → {
			sb.WriteByte('{')
		case '\uff5d': // ｝ → }
			sb.WriteByte('}')
		case '\uff3b': // ［ → [
			sb.WriteByte('[')
		case '\uff3d': // ］ → ]
			sb.WriteByte(']')
		case '\uff1a': // ： → :
			sb.WriteByte(':')
		case '\uff0c': // ， → ,
			sb.WriteByte(',')
		case '\uff1b': // ； → ;
			sb.WriteByte(';')
		default:
			sb.WriteString(s[i : i+size])
		}
		i += size
	}

	return sb.String()
}

// isQuoteByte returns true if the byte is an ASCII quote character.
func isQuoteByte(c byte) bool {
	return c == '"' || c == '\''
}

// isSmartDoubleQuote returns true if the rune is a typographic/curly double quote
// or full-width quotation mark.
func isSmartDoubleQuote(r rune) bool {
	return r == '\u201c' || // " LEFT DOUBLE QUOTATION MARK
		r == '\u201d' || // " RIGHT DOUBLE QUOTATION MARK
		r == '\u201e' || // „ DOUBLE LOW-9 QUOTATION MARK
		r == '\uff02'   // ＂ FULLWIDTH QUOTATION MARK
}

// isSmartSingleQuote returns true if the rune is a typographic/curly single quote
// or full-width apostrophe.
func isSmartSingleQuote(r rune) bool {
	return r == '\u2018' || // ' LEFT SINGLE QUOTATION MARK
		r == '\u2019' || // ' RIGHT SINGLE QUOTATION MARK
		r == '\uff07'   // ＇ FULLWIDTH APOSTROPHE
}

// isSmartQuote returns true if the rune is any kind of smart/typographic quote.
func isSmartQuote(r rune) bool {
	return isSmartDoubleQuote(r) || isSmartSingleQuote(r)
}

// asciiQuoteForSmart maps a smart quote rune to its ASCII equivalent byte.
// Returns 0 if the rune is not a smart quote.
func asciiQuoteForSmart(r rune) byte {
	if isSmartDoubleQuote(r) {
		return '"'
	}
	if isSmartSingleQuote(r) {
		return '\''
	}
	return 0
}

// stripCodeFences removes ```json ... ``` wrappers that LLMs commonly
// wrap their JSON output in. Handles both prefix and suffix fences.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)

	// Strip leading fence: ```json, ```, ```JSON, etc.
	for _, prefix := range []string{"```json", "```JSON", "```"} {
		if strings.HasPrefix(s, prefix) {
			s = s[len(prefix):]
			break
		}
	}

	// Strip trailing fence
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}

	return strings.TrimSpace(s)
}

// stripComments removes C-style and hash comments from JSON-like input.
// Preserves content inside string literals.
// Uses a heuristic: a quote is only treated as a string delimiter if
// followed by a structural character (, } ] :), a space then structural,
// or another matching quote (for empty strings / doubled quotes).
// This avoids breaking strings with unescaped quotes inside (Issue #18).
func stripComments(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))

	i := 0
	inString := false
	var stringDelim byte

	for i < len(s) {
		c := s[i]

		// Inside a string — copy verbatim, handle escapes
		if inString {
			sb.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				sb.WriteByte(s[i])
			} else if c == stringDelim {
				// Lookahead: only end string if followed by structural or matching quote
				j := i + 1
				for j < len(s) && s[j] == ' ' {
					j++
				}
				if j >= len(s) || s[j] == ',' || s[j] == '}' || s[j] == ']' ||
					s[j] == ':' || s[j] == stringDelim {
					inString = false
				}
			}
			i++
			continue
		}

		// Start of string
		if c == '"' || c == '\'' {
			inString = true
			stringDelim = c
			sb.WriteByte(c)
			i++
			continue
		}

		// Line comment: //
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			i += 2
			for i < len(s) && s[i] != '\n' && s[i] != '\r' {
				i++
			}
			continue
		}

		// Block comment: /* ... */
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i < len(s)-1 {
				if s[i] == '*' && s[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		// Hash comment: # ...
		if c == '#' {
			for i < len(s) && s[i] != '\n' && s[i] != '\r' {
				i++
			}
			continue
		}

		sb.WriteByte(c)
		i++
	}

	return sb.String()
}

// getSmartQuoteByteAt checks if the byte at position index+offset in the
// container string is the start of a smart/typographic quote (UTF-8 multi-byte).
// If so, returns the ASCII equivalent byte and true. Otherwise returns 0, false.
func getSmartQuoteByteAt(container string, index int, offset int) (byte, bool) {
	pos := index + offset
	if pos < 0 || pos >= len(container) {
		return 0, false
	}
	r, _ := utf8.DecodeRuneInString(container[pos:])
	ascii := asciiQuoteForSmart(r)
	if ascii != 0 {
		return ascii, true
	}
	return 0, false
}

// isSmartQuoteAt returns true if the byte at position index+offset is
// the start of a smart/typographic quote.
func isSmartQuoteAt(container string, index int, offset int) bool {
	_, ok := getSmartQuoteByteAt(container, index, offset)
	return ok
}
