package replace

import (
	"strings"
)

type ReplacerFunc func(string) (string, error)

func Replace(s, startToken, endToken string, replacer ReplacerFunc) (string, error) {
	result := &strings.Builder{}
	for {
		before, tail, ok := strings.Cut(s, startToken)
		if !ok {
			result.WriteString(s)
			break
		}

		result.WriteString(before)

		expr, after, ok := strings.Cut(tail, endToken)
		if !ok || strings.HasSuffix(before, startToken[:1]) {
			result.WriteString(startToken)
			s = tail
			continue
		}

		replaced, err := replacer(expr)
		if err != nil {
			return "", err
		}

		result.WriteString(replaced)
		s = after
	}

	return result.String(), nil
}
