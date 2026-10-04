package project

import "strings"

type testRange struct{ start, end, line int }

// This is lexical test-block detection, not a Zig parser. Comments, strings
// and multiline string lines are skipped so their braces cannot end a test.
func zigToken(text string, at int) (string, int, int) {
	for at < len(text) {
		if strings.ContainsRune(" \t\r\n", rune(text[at])) {
			at++
			continue
		}
		if strings.HasPrefix(text[at:], "//") || strings.HasPrefix(text[at:], `\\`) {
			if n := strings.IndexByte(text[at:], '\n'); n >= 0 {
				at += n + 1
				continue
			}
			return "", len(text), len(text)
		}
		if strings.HasPrefix(text[at:], "/*") {
			if n := strings.Index(text[at+2:], "*/"); n >= 0 {
				at += n + 4
				continue
			}
			return "", len(text), len(text)
		}
		break
	}
	start := at
	if at == len(text) {
		return "", at, at
	}
	if text[at] == '"' || text[at] == '\'' {
		quote := text[at]
		at++
		for at < len(text) {
			if text[at] == '\\' {
				at += min(2, len(text)-at)
				continue
			}
			if text[at] == quote {
				return "string", start, at + 1
			}
			at++
		}
		return "", start, at
	}
	if (text[at] >= 'a' && text[at] <= 'z') || (text[at] >= 'A' && text[at] <= 'Z') || text[at] == '_' {
		at++
		for at < len(text) && ((text[at] >= 'a' && text[at] <= 'z') || (text[at] >= 'A' && text[at] <= 'Z') || (text[at] >= '0' && text[at] <= '9') || text[at] == '_') {
			at++
		}
		return text[start:at], start, at
	}
	return text[at : at+1], start, at + 1
}
func zigTests(text string) []testRange {
	out := []testRange{}
	for at := 0; at < len(text); {
		token, start, next := zigToken(text, at)
		at = next
		if token != "test" {
			continue
		}
		token, _, next = zigToken(text, at)
		if token == "string" {
			token, _, next = zigToken(text, next)
		}
		if token != "{" {
			continue
		}
		depth := 1
		at = next
		for depth > 0 && at < len(text) {
			token, _, next = zigToken(text, at)
			at = next
			switch token {
			case "{":
				depth++
			case "}":
				depth--
			}
		}
		if depth == 0 {
			out = append(out, testRange{start, at, 1 + strings.Count(text[:start], "\n")})
		}
	}
	return out
}
