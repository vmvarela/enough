package cli

import (
	"context"
	"flag"
	"fmt"
	"github.com/vmvarela/enough/internal/analysis"
	"github.com/vmvarela/enough/internal/remember"
	"github.com/vmvarela/enough/internal/report"
	"io"
	"strings"
	"time"
)

const Usage = `enough — Know when to stop.

Usage: enough [check|why|next|remember|version] [--json] [--force] [repository]

Analysis is local and read-only. Only remember writes ENOUGH.md.
--json prints schema-versioned evidence; exit codes remain the same.
--force replaces an existing ENOUGH.md (remember only).
Exit codes: 0 enough/resting, 1 unfinished, 2 growing, 3 overgrown,
            4 unknown, 10 execution/configuration error.
`

func Run(ctx context.Context, args []string, out, errOut io.Writer, version string, a analysis.Analyzer, now func() time.Time) int {
	command := "check"
	flags := []string{}
	pos := []string{}
	// Boolean options work before and after the subcommand or repository.
	literal := false
	for _, s := range args {
		if s == "--" {
			literal = true
			continue
		}
		if !literal && s != "-" && strings.HasPrefix(s, "-") {
			flags = append(flags, s)
		} else {
			pos = append(pos, s)
		}
	}
	if len(pos) > 0 {
		switch pos[0] {
		case "check", "why", "next", "remember", "version":
			command = pos[0]
			pos = pos[1:]
		}
	}
	fs := flag.NewFlagSet("enough", flag.ContinueOnError)
	fs.SetOutput(errOut)
	jsonFlag := fs.Bool("json", false, "")
	force := fs.Bool("force", false, "")
	fs.Usage = func() { fmt.Fprint(errOut, Usage) }
	if e := fs.Parse(flags); e != nil {
		if e == flag.ErrHelp {
			return 0
		}
		return 10
	}
	if len(pos) > 1 || (*force && command != "remember") || (*jsonFlag && (command == "remember" || command == "version")) {
		fmt.Fprint(errOut, Usage)
		return 10
	}
	if command == "version" {
		if len(pos) > 0 {
			return 10
		}
		if _, e := fmt.Fprintln(out, "enough "+version); e != nil {
			return 10
		}
		return 0
	}
	repo := "."
	if len(pos) == 1 {
		repo = pos[0]
	}
	r, e := a.Analyze(ctx, repo)
	if e != nil {
		fmt.Fprintf(errOut, "enough: %s\n", e)
		return 10
	}
	if command == "remember" {
		if e := remember.Write(r, now(), *force); e != nil {
			fmt.Fprintf(errOut, "enough: %s\n", e)
			return 10
		}
		if _, e := fmt.Fprintln(out, "Created ENOUGH.md. Keep this project's frame."); e != nil {
			return 10
		}
		return 0
	}
	if *jsonFlag {
		e = report.JSON(out, r)
	} else {
		e = report.Text(out, r, command)
	}
	if e != nil {
		fmt.Fprintln(errOut, "enough: cannot write output")
		return 10
	}
	return analysis.ExitCode(r.State)
}
