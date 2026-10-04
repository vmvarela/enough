package project

import (
	"regexp"
	"strings"
)

var markdownLink = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)

func plainMarkdown(s string) string {
	s = markdownLink.ReplaceAllString(s, "$1")
	return strings.TrimSpace(strings.NewReplacer("**", "", "__", "", "`", "", "*", "").Replace(s))
}

// Read a complete opening paragraph, stopping before installation/reference
// sections. Quoted asides, badges and code examples do not declare purpose.
func inferPurpose(readme string) string {
	var paragraph []string
	fence := ""
	finish := func() string {
		text := plainMarkdown(strings.Join(paragraph, " "))
		paragraph = nil
		if len(text) >= 20 && len(strings.Fields(text)) >= 4 {
			return clean(text)
		}
		return ""
	}
	for _, line := range strings.Split(readme, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "```") || strings.HasPrefix(l, "~~~") {
			if p := finish(); p != "" {
				return p
			}
			marker := l[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if strings.HasPrefix(l, "#") {
			if p := finish(); p != "" {
				return p
			}
			title := strings.ToLower(strings.TrimSpace(strings.TrimLeft(l, "#")))
			for _, stop := range []string{"install", "quick start", "usage", "configuration", "development", "license", "features", "commands"} {
				if strings.HasPrefix(title, stop) {
					return ""
				}
			}
			continue
		}
		skip := l == "" || strings.HasPrefix(l, ">") || strings.HasPrefix(l, "![") || strings.HasPrefix(l, "[!") || strings.HasPrefix(l, "<") || strings.HasPrefix(l, "-") || strings.HasPrefix(l, "|")
		if skip {
			if p := finish(); p != "" {
				return p
			}
			continue
		}
		paragraph = append(paragraph, l)
	}
	return finish()
}
