package report

import (
	"encoding/json"
	"fmt"
	"github.com/vmvarela/enough/internal/analysis"
	"io"
	"strings"
	"unicode"
)

// Terminal output strips controls even from unusual Git filenames.
func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
func short(s string) string {
	r := []rune(safe(s))
	if len(r) > 160 {
		return string(r[:157]) + "..."
	}
	return string(r)
}
func Text(w io.Writer, r *analysis.Result, command string) error {
	var b strings.Builder
	if command == "next" {
		next(&b, r)
	} else {
		fmt.Fprintf(&b, "%s\npurpose\n  %s\nsource\n  %s\nstability\n  %s\nscope drift\n  %s\n\n%s\n%s\n", short(r.Name), short(r.Purpose.Statement), short(r.Purpose.Source), r.Analysis.Stability, r.Analysis.Scope, strings.ToUpper(string(r.State)), r.Summary)
		if command == "why" {
			b.WriteString("\nWhy:\n")
			limit := min(len(r.Evidence), 12)
			for _, e := range r.Evidence[:limit] {
				fmt.Fprintf(&b, "  [%s] %s\n    %s\n", e.Confidence, short(e.Message), short(e.Source))
			}
			if len(r.Evidence) > limit {
				fmt.Fprintf(&b, "  %d additional signals omitted.\n", len(r.Evidence)-limit)
			}
		}
		b.WriteByte('\n')
		next(&b, r)
	}
	_, e := io.WriteString(w, b.String())
	return e
}
func next(b *strings.Builder, r *analysis.Result) {
	if len(r.Recommendations) > 0 {
		b.WriteString("Worth doing:\n")
		for _, s := range r.Recommendations {
			fmt.Fprintf(b, "  %s\n", short(s))
		}
		return
	}
	switch r.State {
	case analysis.Enough, analysis.Resting:
		b.WriteString("Nothing clearly needs doing.\nThen stop.\n")
	case analysis.Unknown:
		b.WriteString("No next action can be justified from the available evidence.\n")
	default:
		b.WriteString("No specific next action is supported by the evidence.\n")
	}
}
func JSON(w io.Writer, r *analysis.Result) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(r)
}
