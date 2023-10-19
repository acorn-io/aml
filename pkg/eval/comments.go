package eval

import "strings"

type Comments struct {
	Comments [][]string
}

func (c Comments) Last() string {
	if len(c.Comments) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(c.Comments[0], "\n"))
}
