package value

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func Escape(s string) string {
	s = strconv.Quote(s)
	return s[1 : len(s)-1]
}

func unquoteRaw(s string) (string, error) {
	if strings.HasPrefix(s, "```") {
		if !strings.HasSuffix(s, "```") {
			return "", fmt.Errorf("raw string does not end with ```")
		}
		s = s[3 : len(s)-3]
	} else if strings.HasSuffix(s, "`") {
		s = s[1 : len(s)-1]
	} else {
		return "", fmt.Errorf("raw string does not end with `")
	}

	buf := strings.Builder{}
	buf.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if i < len(s)-1 && s[i] == '\\' && s[i+1] == '`' {
			buf.WriteRune('`')
			i++
		} else {
			buf.WriteByte(s[i])
		}
	}
	return buf.String(), nil
}

const (
	tripleQuote = `"""`
	singleQuote = `"`
)

func Unquote(s string) (string, error) {
	if strings.HasPrefix(s, "`") {
		return unquoteRaw(s)
	}
	if strings.HasPrefix(s, tripleQuote) {
		// Chop off two quotes, leaving one quote on each side. Also quote unquoted quotes
		s = strings.TrimPrefix(s, tripleQuote)
		s = strings.TrimSuffix(s, tripleQuote)
		s = singleQuote + quoteUnquotedComma(s) + singleQuote

		lines := strings.Split(s, "\n")

		// First and last line must only be quotes for indent parsing
		if lines[0] == "\"" && strings.TrimSpace(lines[len(lines)-1]) == "\"" {
			// The expected prefix is taken from the last line prefix
			prefix := strings.TrimSuffix(lines[len(lines)-1], "\"")

			foundPrefix := true
			// Make sure all line excluding the first line have the prefix
			for _, line := range lines[1:] {
				if !strings.HasPrefix(line, prefix) {
					foundPrefix = false
					break
				}
			}

			if foundPrefix {
				// Now trim the prefix on all lines except the first and the last.
				// The first and last line can just be dropped.  We drop the last line so that we don't end up with
				// a spurious "\n" at the end
				lines = lines[1 : len(lines)-1]
				for i := range lines {
					lines[i] = strings.TrimPrefix(lines[i], prefix)
				}
				if len(lines) == 0 {
					// If we have no lines, then this is an empty string and just set it to ""
					lines = []string{""}
				} else {
					// Add back the starting quote we dropped above
					lines[0] = "\"" + lines[0]
					// Add back the ending quote we dropped above
					lines[len(lines)-1] = lines[len(lines)-1] + "\""
				}
			}
		}
		s = strings.Join(lines, "\\n")
	}
	if strings.HasPrefix(s, "\"") {
		ret, err := strconv.Unquote(s)
		if errors.Is(err, strconv.ErrSyntax) {
			err = fmt.Errorf("%w: invalid or missing escape (\\) sequence %s", err, s)
		} else if err != nil {
			err = fmt.Errorf("%w: %s", err, s)
		}
		return ret, err
	}
	return s, nil
}

func quoteUnquotedComma(s string) string {
	result := strings.Builder{}
	result.Grow(len(s))

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			result.WriteByte('\\')
		}
		result.WriteByte(ch)
		if ch == '\\' && i < len(s) {
			result.WriteByte(s[i+1])
			i++
		}
	}

	return result.String()
}
