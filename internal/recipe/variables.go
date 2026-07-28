package recipe

import (
	"fmt"
	"strings"
)

// Variables returns every ${identifier} reference in encounter order.
func Variables(value string) ([]string, error) {
	var variables []string
	remaining := value
	offset := 0
	for {
		start := strings.Index(remaining, "${")
		if start < 0 {
			return variables, nil
		}
		start += offset
		closeRelative := strings.IndexByte(value[start+2:], '}')
		if closeRelative < 0 {
			return nil, fmt.Errorf("unterminated variable reference at byte %d", start)
		}
		end := start + 2 + closeRelative
		identifier := value[start+2 : end]
		if !validIdentifier(identifier) {
			return nil, fmt.Errorf("invalid variable reference %q at byte %d", identifier, start)
		}
		variables = append(variables, identifier)
		offset = end + 1
		remaining = value[offset:]
	}
}

func validIdentifier(value string) bool {
	if value == "" || !isIdentifierStart(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		if !isIdentifierPart(value[index]) {
			return false
		}
	}
	return true
}
