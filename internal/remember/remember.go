package remember

import (
	"fmt"
	"github.com/vmvarela/enough/internal/analysis"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Render(r *analysis.Result, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Enough\n\nThis project exists to:\n\n> %s\n", r.Purpose.Statement)
	if len(r.Includes) > 0 {
		b.WriteString("\n## It includes\n\n")
		for _, s := range r.Includes {
			fmt.Fprintf(&b, "- %s\n", s)
		}
	}
	if len(r.Excludes) > 0 {
		b.WriteString("\n## It deliberately does not include\n\n")
		for _, s := range r.Excludes {
			fmt.Fprintf(&b, "- %s\n", s)
		}
	}
	b.WriteString("\n## Why this is enough\n\n")
	if r.State == analysis.Enough || r.State == analysis.Resting {
		b.WriteString("The project's primary promises appear implemented and intentionally tested.\nRemaining limitations are acceptable because they do not prevent the project\nfrom fulfilling its purpose. This declaration is a boundary for future work.\n")
	} else {
		b.WriteString("This declaration records the intended scope. Automatic analysis has not\nverified that the project is complete; review its promises before treating\nthis boundary as a declaration of completion.\n")
	}
	fmt.Fprintf(&b, "\nDeclared enough on %s.\n", now.Format("2006-01-02"))
	return b.String()
}
func Write(r *analysis.Result, now time.Time, force bool) error {
	path := filepath.Join(r.Root, "ENOUGH.md")
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		// Never follow a symlink or write through an existing hard link.
		info, e := os.Lstat(path)
		if e == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("ENOUGH.md must be a regular file")
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		f, e := os.CreateTemp(r.Root, ".enough-*")
		if e != nil {
			return e
		}
		tmp := f.Name()
		defer os.Remove(tmp)
		if _, e = f.WriteString(Render(r, now)); e != nil {
			f.Close()
			return e
		}
		if e = f.Chmod(0644); e != nil {
			f.Close()
			return e
		}
		if e = f.Close(); e != nil {
			return e
		}
		return os.Rename(tmp, path)
	}
	f, e := os.OpenFile(path, flags, 0644)
	if os.IsExist(e) {
		return fmt.Errorf("ENOUGH.md already exists; use --force to replace it")
	}
	if e != nil {
		return e
	}
	if _, e = f.WriteString(Render(r, now)); e != nil {
		f.Close()
		os.Remove(path)
		return e
	}
	return f.Close()
}
