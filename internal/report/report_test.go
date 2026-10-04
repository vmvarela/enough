package report

import (
	"bytes"
	"github.com/vmvarela/enough/internal/analysis"
	"github.com/vmvarela/enough/internal/evidence"
	"github.com/vmvarela/enough/internal/project"
	"strings"
	"testing"
)

func TestTerminalRestraint(t *testing.T) {
	r := &analysis.Result{Name: "tool\x1b[2J\n", State: analysis.Unknown, Purpose: project.Purpose{Statement: strings.Repeat("x", 1000)}, Summary: "Unclear.", Recommendations: []string{}}
	for i := 0; i < 100; i++ {
		r.Evidence = append(r.Evidence, evidence.Evidence{Kind: "purpose", Message: "Unclear.", Source: "README.md", Confidence: evidence.Low})
	}
	var b bytes.Buffer
	if e := Text(&b, r, "why"); e != nil {
		t.Fatal(e)
	}
	if strings.ContainsRune(b.String(), '\x1b') || !strings.Contains(b.String(), "88 additional signals omitted.") {
		t.Fatal(b.String())
	}
	for _, line := range strings.Split(b.String(), "\n") {
		if len([]rune(line)) > 170 {
			t.Fatal("unbounded terminal field")
		}
	}
}
