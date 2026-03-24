package djot

// ParseAttrs parses a djot attribute string like `.class #id key="value"`.
// The input should NOT include the surrounding braces.
// Returns the parsed attributes, or nil if the input is invalid.
func ParseAttrs(input string) map[string]string {
	attrs, _ := ParseAttrsOrdered(input)
	return attrs
}

// ParseAttrsOrdered parses a djot attribute string and returns both the
// attribute map and the key insertion order.
func ParseAttrsOrdered(input string) (map[string]string, []string) {
	p := attrParser{input: input}
	return p.parse()
}

type attrState int

const (
	attrScanning      attrState = iota
	attrScanningClass           // saw "."
	attrScanningID              // saw "#"
	attrScanningKey             // saw start of key
	attrScanningValue           // saw "="
	attrScanningBare            // unquoted value
	attrScanningQuoted          // inside "..." or '...'
	attrScanningEscape          // backslash inside quoted value
	attrScanningComment         // inside %...%
)

type attrParser struct {
	input    string
	pos      int
	attrs    map[string]string
	keyOrder []string
}

func (p *attrParser) parse() (map[string]string, []string) {
	p.attrs = make(map[string]string)
	state := attrScanning
	var key string
	var buf []byte
	var quoteChar byte

	for p.pos < len(p.input) {
		c := p.input[p.pos]

		switch state {
		case attrScanning:
			switch {
			case c == '.':
				state = attrScanningClass
				buf = buf[:0]
			case c == '#':
				state = attrScanningID
				buf = buf[:0]
			case c == '%':
				state = attrScanningComment
			case isAttrKeyStart(c):
				state = attrScanningKey
				buf = append(buf[:0], c)
			case isWhitespace(c):
				// skip
			default:
				return nil, nil
			}

		case attrScanningClass:
			if isClassChar(c) {
				buf = append(buf, c)
			} else {
				if len(buf) == 0 {
					return nil, nil
				}
				p.addClass(string(buf))
				state = attrScanning
				continue // re-examine this char
			}

		case attrScanningID:
			if !isIDExcluded(c) {
				buf = append(buf, c)
			} else {
				if len(buf) == 0 {
					return nil, nil
				}
				p.setAttr("id", string(buf))
				state = attrScanning
				continue
			}

		case attrScanningKey:
			if isAttrKeyChar(c) {
				buf = append(buf, c)
			} else if c == '=' {
				key = string(buf)
				state = attrScanningValue
			} else {
				// Boolean attribute (key with no value).
				p.setAttr(string(buf), "")
				state = attrScanning
				continue
			}

		case attrScanningValue:
			if c == '"' || c == '\'' {
				quoteChar = c
				buf = buf[:0]
				state = attrScanningQuoted
			} else if isBareValueChar(c) {
				buf = append(buf[:0], c)
				state = attrScanningBare
			} else {
				return nil, nil
			}

		case attrScanningBare:
			if isBareValueChar(c) {
				buf = append(buf, c)
			} else if isWhitespace(c) {
				p.setAttr(key, string(buf))
				state = attrScanning
			} else {
				// Bare values can only be terminated by whitespace or end of input.
				// Any other character (like '.' or '/') fails the entire attribute.
				return nil, nil
			}

		case attrScanningQuoted:
			if c == '\\' {
				state = attrScanningEscape
			} else if c == quoteChar {
				p.setAttr(key, string(buf))
				state = attrScanning
			} else {
				buf = append(buf, c)
			}

		case attrScanningEscape:
			buf = append(buf, c)
			state = attrScanningQuoted

		case attrScanningComment:
			if c == '%' {
				state = attrScanning
			}
			// Comment runs to % or end of input — both are valid.
		}

		p.pos++
	}

	// Handle trailing state.
	switch state {
	case attrScanning:
		// ok
	case attrScanningComment:
		// Comment at end of input is valid (comment extends to closing brace).
	case attrScanningClass:
		if len(buf) == 0 {
			return nil, nil
		}
		p.addClass(string(buf))
	case attrScanningID:
		if len(buf) == 0 {
			return nil, nil
		}
		p.setAttr("id", string(buf))
	case attrScanningKey:
		// Boolean attribute at end.
		p.setAttr(string(buf), "")
	case attrScanningBare:
		p.setAttr(key, string(buf))
	default:
		return nil, nil // unclosed quote
	}

	return p.attrs, p.keyOrder
}

func (p *attrParser) setAttr(key, value string) {
	if _, exists := p.attrs[key]; !exists {
		p.keyOrder = append(p.keyOrder, key)
	}
	p.attrs[key] = value
}

func (p *attrParser) addClass(class string) {
	if existing, ok := p.attrs["class"]; ok {
		p.attrs["class"] = existing + " " + class
	} else {
		p.keyOrder = append(p.keyOrder, "class")
		p.attrs["class"] = class
	}
}

// Character classification per the djot spec / JS reference implementation.

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isAttrKeyStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == ':'
}

func isAttrKeyChar(c byte) bool {
	return isAttrKeyStart(c) || (c >= '0' && c <= '9') || c == '-'
}

// isClassChar matches characters valid in .class shorthand.
// JS reference: /^\w/ || '_' || '-' || ':'  (where \w is [a-zA-Z0-9_])
func isClassChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '-' || c == ':'
}

// isIDExcluded returns true for characters that cannot appear in #id values.
// JS reference: /^[^\]\[~!@#$%^&*(){}``,.<>\\|=+/?\s]/
// We check the byte-level exclusion set; non-ASCII bytes are allowed.
func isIDExcluded(c byte) bool {
	switch c {
	case ']', '[', '~', '!', '@', '#', '$', '%', '^', '&', '*',
		'(', ')', '{', '}', '`', ',', '.', '<', '>', '\\', '|',
		'=', '+', '/', '?',
		' ', '\t', '\n', '\r':
		return true
	}
	return false
}

// isBareValueChar matches characters valid in unquoted attribute values.
// Per the djot spec: [a-zA-Z0-9:_-]+
func isBareValueChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == ':' || c == '_' || c == '-'
}
