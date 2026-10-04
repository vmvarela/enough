package project

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vmvarela/enough/internal/config"
	"github.com/vmvarela/enough/internal/evidence"
)

const MaxFiles = 10000
const MaxFileSize = 1 << 20

// Bound total text as well as individual files to keep large trees small.
const MaxText = 32 << 20

type Purpose struct {
	Statement  string              `json:"statement"`
	Source     string              `json:"source"`
	Confidence evidence.Confidence `json:"confidence"`
}
type Promise struct {
	Text       string              `json:"text"`
	Source     string              `json:"source"`
	Confidence evidence.Confidence `json:"confidence"`
	Status     string              `json:"status"`
	Supporting []string            `json:"supporting,omitempty"`
}
type TextFile struct {
	Path      string
	Text      string
	Test      bool
	Words     map[string]bool
	TodoLines map[int]map[string]bool
}
type Snapshot struct {
	Name              string
	Config            config.Config
	Purpose           Purpose
	Readme            string
	ReadmePath        string
	Files             []TextFile
	Paths             []string
	Promises          []Promise
	Drift             []evidence.Evidence
	Evidence          []evidence.Evidence
	Todos             int
	Tests             int
	InlineTests       int
	Truncated         bool
	HasImplementation bool
}

var sourceExt = map[string]bool{".go": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true, ".py": true, ".rs": true, ".c": true, ".h": true, ".cpp": true, ".java": true, ".sh": true, ".rb": true, ".zig": true, ".swift": true, ".php": true}
var skipped = map[string]bool{".git": true, "vendor": true, "node_modules": true, "dist": true, "build": true, "coverage": true, "testdata": true, "fixtures": true, ".venv": true, "venv": true}

func read(root string, path string) ([]byte, error) {
	boundary, e := os.OpenRoot(root)
	if e != nil {
		return nil, e
	}
	defer boundary.Close()
	info, e := boundary.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", path)
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("%s exceeds 1 MiB", path)
	}
	f, e := boundary.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if e != nil {
		return nil, e
	}
	if len(b) > MaxFileSize {
		return nil, fmt.Errorf("%s exceeds 1 MiB", path)
	}
	if bytes.ContainsRune(b, 0) || !utf8.Valid(b) {
		return nil, fmt.Errorf("%s is not text", path)
	}
	return b, nil
}

func Inspect(ctx context.Context, root string) (Snapshot, error) {
	s := Snapshot{Name: filepath.Base(root), Files: []TextFile{}, Promises: []Promise{}, Evidence: []evidence.Evidence{}, Drift: []evidence.Evidence{}}
	if b, e := read(root, "enough.toml"); e == nil {
		s.Config, e = config.Parse(b)
		if e != nil {
			return s, e
		}
	} else if !os.IsNotExist(e) {
		return s, e
	}
	for _, p := range []string{"README.md", "README", "README.rst", "README.txt"} {
		b, e := read(root, p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			s.Truncated = true
			continue
		}
		s.Readme = string(b)
		s.ReadmePath = p
		break
	}
	if b, e := read(root, "package.json"); e == nil {
		var p struct {
			Name        string
			Description string
		}
		if json.Unmarshal(b, &p) == nil {
			if p.Name != "" {
				s.Name = clean(p.Name)
			}
			if p.Description != "" {
				s.Purpose = Purpose{clean(p.Description), "package.json", evidence.Medium}
			}
		}
	}
	if b, e := read(root, "go.mod"); e == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "module ") {
				s.Name = filepath.Base(strings.TrimSpace(strings.TrimPrefix(line, "module ")))
				break
			}
		}
	}
	if p := inferPurpose(s.Readme); p != "" {
		s.Purpose = Purpose{p, "inferred from " + s.ReadmePath, evidence.Medium}
	}
	if s.Config.Purpose.Statement != "" {
		s.Purpose = Purpose{s.Config.Purpose.Statement, "enough.toml", evidence.High}
	}
	if s.Purpose.Statement == "" {
		s.Purpose = Purpose{"unclear", "repository name", evidence.Low}
	}
	s.Evidence = append(s.Evidence, evidence.Evidence{Kind: "purpose", Message: "Purpose source: " + s.Purpose.Source + ".", Source: s.Purpose.Source, Confidence: s.Purpose.Confidence})
	count, total := 0, 0
	limit := fmt.Errorf("inspection limit")
	boundary, err := os.OpenRoot(root)
	if err != nil {
		return s, err
	}
	defer boundary.Close()
	err = fs.WalkDir(boundary.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if path == "." {
			return nil
		}
		rel := path
		if d.IsDir() {
			if skipped[d.Name()] || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		count++
		if count > MaxFiles {
			s.Truncated = true
			return limit
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !sourceExt[strings.ToLower(filepath.Ext(rel))] {
			return nil
		}
		s.Paths = append(s.Paths, rel)
		b, e := read(root, rel)
		if e != nil {
			if strings.Contains(e.Error(), "not text") {
				return nil
			}
			s.Truncated = true
			return nil
		}
		if total+len(b) > MaxText {
			s.Truncated = true
			return limit
		}
		total += len(b)
		test := isTest(rel)
		text := string(b)
		original := text
		inline := []testRange{}
		if filepath.Ext(rel) == ".zig" {
			inline = zigTests(text)
			s.InlineTests += len(inline)
		}
		if test || len(inline) > 0 {
			s.Tests++
		}
		if !test && len(inline) > 0 {
			masked := []byte(text)
			for _, r := range inline {
				for i := r.start; i < r.end; i++ {
					if masked[i] != '\n' {
						masked[i] = ' '
					}
				}
			}
			text = string(masked)
		}
		if !test && len(strings.TrimSpace(text)) > 0 {
			s.HasImplementation = true
		}
		file := indexFile(rel, text, test)
		s.Files = append(s.Files, file)
		if !test {
			s.Todos += len(file.TodoLines)
		}
		for _, r := range inline {
			if !test {
				s.Files = append(s.Files, indexFile(fmt.Sprintf("%s:%d", rel, r.line), original[r.start:r.end], true))
			}
		}

		return nil
	})
	if err != nil && err != limit {
		return s, fmt.Errorf("cannot inspect repository files: %w", err)
	}
	s.Promises = Promises(s)
	s.Drift = Scope(s)
	if s.Tests > 0 {
		s.Evidence = append(s.Evidence, evidence.Evidence{Kind: "tests", Message: "Tests are present; they were not executed.", Source: "source tree", Confidence: evidence.Low})
	}
	if s.Todos > 0 {
		s.Evidence = append(s.Evidence, evidence.Evidence{Kind: "todos", Message: "Unfinished markers were found; only promise-related markers affect the decision.", Source: "source tree", Confidence: evidence.Low})
	}
	if s.Truncated {
		s.Evidence = append(s.Evidence, evidence.Evidence{Kind: "limit", Message: "Some repository content could not be inspected; completeness remains uncertain.", Source: "source tree", Confidence: evidence.High})
	}
	return s, nil
}
func isTest(p string) bool {
	p = strings.ToLower(p)
	base := filepath.Base(p)
	return strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.zig") || strings.HasSuffix(base, "_tests.zig") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasPrefix(base, "test_") || strings.Contains("/"+p, "/tests/") || strings.Contains("/"+p, "/test/")
}
func clean(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsControl(r) {
			b.WriteRune(r)
		}
	}
	s = strings.TrimSpace(b.String())
	if len(s) > 500 {
		s = string([]rune(s)[:min(200, len([]rune(s)))])
	}
	return s
}

var marker = regexp.MustCompile(`(?i)\b(TODO|FIXME|XXX)\b`)
var flagRE = regexp.MustCompile(`--[a-z][a-z0-9-]*`)
var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields("a an the this that project tool command cli supports support provides provide can with from for and or to in of is are be works work implemented implement implementation documented documentation local remote output add feature functionality installation enough software repository analysis when how use using show print check its it does do has have not no TODO FIXME XXX") {
		stop[strings.ToLower(w)] = true
	}
}
func tokens(s string) []string {
	set := map[string]bool{}
	for t := range wordSet(s) {
		if !stop[t] && len(t) > 1 {
			set[t] = true
		}
	}
	out := []string{}
	for t := range set {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
func addWords(out map[string]bool, s string) {
	s = strings.ToLower(s)
	for i := 0; i < len(s); {
		start := i
		for i < len(s) && ((s[i] >= 'a' && s[i] <= 'z') || (s[i] >= '0' && s[i] <= '9')) {
			i++
		}
		if i > start {
			out[s[start:i]] = true
		} else {
			i++
		}
	}
}
func wordSet(s string) map[string]bool {
	out := map[string]bool{}
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		if i > 0 && r >= 'A' && r <= 'Z' && ((rs[i-1] >= 'a' && rs[i-1] <= 'z') || (i+1 < len(rs) && rs[i+1] >= 'a' && rs[i+1] <= 'z')) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	addWords(out, b.String())
	for w := range out {
		if len(w) > 4 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") && !strings.HasSuffix(w, "us") && !strings.HasSuffix(w, "is") {
			out[strings.TrimSuffix(w, "s")] = true
			delete(out, w)
		}
	}
	return out
}

func hasWords(hay map[string]bool, ts []string) bool {
	if len(ts) == 0 {
		return false
	}
	for _, t := range ts {
		if !hay[t] {
			return false
		}
	}
	return true
}
func indexFile(path, text string, test bool) TextFile {
	f := TextFile{Path: path, Text: text, Test: test, Words: map[string]bool{}, TodoLines: map[int]map[string]bool{}}
	for n, line := range strings.Split(text, "\n") {
		l := strings.TrimSpace(line)
		if marker.MatchString(l) {
			f.TodoLines[n+1] = wordSet(l)
			continue
		}
		if strings.HasPrefix(l, "//") || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "/*") || strings.HasPrefix(l, "*") {
			continue
		}
		for w := range wordSet(l) {
			f.Words[w] = true
		}
	}
	return f
}
func scopeWord(text, term string) bool {
	words := wordSet(text)
	return words[term] || words[term+"s"] || (term == "monitor" && words["monitoring"])
}

func Promises(s Snapshot) []Promise {
	out := []Promise{}
	seen := map[string]bool{}
	add := func(text, src string, c evidence.Confidence) {
		if c != evidence.High {
			text = plainMarkdown(text)
		}
		text = clean(text)
		if text == "" || seen[strings.ToLower(text)] || len(out) >= 100 {
			return
		}
		seen[strings.ToLower(text)] = true
		out = append(out, Promise{Text: text, Source: src, Confidence: c, Status: "unclear"})
	}
	for _, p := range s.Config.Enough.When {
		add(p, "enough.toml:enough.when", evidence.High)
	}
	for _, p := range s.Config.Scope.Includes {
		add(p, "enough.toml:scope.includes", evidence.High)
	}
	section, code := false, false
	sectionLevel := 0
	for n, line := range strings.Split(s.Readme, "\n") {
		l := strings.TrimSpace(line)
		low := strings.ToLower(l)
		if strings.HasPrefix(l, "```") {
			code = !code
			continue
		}
		if code {
			if section && !strings.HasPrefix(l, "#") {
				for _, f := range flagRE.FindAllString(l, -1) {
					add(f, fmt.Sprintf("%s:%d", s.ReadmePath, n+1), evidence.Medium)
				}
			}
			continue
		}
		if strings.HasPrefix(l, "#") {
			level := len(l) - len(strings.TrimLeft(l, "#"))
			title := strings.ToLower(strings.TrimSpace(strings.TrimLeft(l, "#")))
			recognized := title == "features" || title == "key features" || title == "usage" || title == "commands" || title == "supports"
			if recognized {
				section = true
				sectionLevel = level
			} else if level <= sectionLevel {
				section = false
			}
			continue
		}
		src := fmt.Sprintf("%s:%d", s.ReadmePath, n+1)
		if section && !code && (strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ")) {
			add(l[2:], src, evidence.Medium)
		}
		if !code && (strings.HasPrefix(low, "supports ") || strings.HasPrefix(low, "provides ") || strings.HasPrefix(low, "can ")) {
			add(l, src, evidence.Medium)
		}
	}
	if len(out) == 0 && s.Purpose.Confidence != evidence.Low {
		add(s.Purpose.Statement, s.Purpose.Source, s.Purpose.Confidence)
	}
	for i := range out {
		p := &out[i]
		ts := tokens(p.Text)
		for _, f := range s.Files {
			if !f.Test {
				// Stable line ordering keeps supporting locations deterministic.
				lines := []int{}
				for n := range f.TodoLines {
					lines = append(lines, n)
				}
				sort.Ints(lines)
				for _, n := range lines {
					if hasWords(f.TodoLines[n], ts) {
						p.Status = "missing"
						p.Supporting = []string{fmt.Sprintf("%s:%d", f.Path, n)}
						break
					}
				}
			}
		}
		source, test := supportingPair(s.Files, ts)

		if p.Status != "missing" && source != "" && test != "" {
			p.Status = "satisfied"
			p.Supporting = []string{source, test}
		}
	}
	return out
}

func Scope(s Snapshot) []evidence.Evidence {
	out := []evidence.Evidence{}
	// Only affirmative feature statements plus source paths support drift.
	features := []string{}
	section := false
	code := false
	for _, line := range strings.Split(s.Readme, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "```") {
			code = !code
			continue
		}
		if code {
			continue
		}
		if strings.HasPrefix(l, "#") {
			title := strings.ToLower(strings.TrimSpace(strings.TrimLeft(l, "#")))
			section = title == "features" || title == "supports" || title == "commands"
			continue
		}
		low := strings.ToLower(l)
		if strings.Contains(low, "not ") || strings.Contains(low, "no ") || strings.Contains(low, "exclude") || strings.Contains(low, "without ") || strings.Contains(low, "out of scope") || strings.Contains(low, "outside") || strings.Contains(low, "doesn't") || strings.Contains(low, "never ") {
			continue
		}
		if section && (strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ")) {
			features = append(features, low)
		} else if strings.HasPrefix(low, "supports ") || strings.HasPrefix(low, "provides ") {
			features = append(features, low)
		}
	}
	aliases := map[string][]string{"web ui": {"web", "dashboard"}, "dashboards": {"dashboard"}, "plugins": {"plugin"}, "monitoring": {"monitor", "prometheus"}, "alerts": {"alert", "notification"}, "certificate renewal": {"renewal"}, "ai apis": {"openai", "anthropic"}, "cloud service": {"server"}}
	check := func(cap string) bool {
		terms, known := aliases[strings.ToLower(cap)]
		if len(terms) == 0 {
			terms = tokens(cap)
		}
		for _, term := range terms {
			documented := false
			for _, f := range features {
				if known && scopeWord(f, term) || !known && hasWords(wordSet(f), terms) {
					documented = true
				}
				if term == "web" && !(strings.Contains(f, "web ui") || strings.Contains(f, "web interface")) {
					documented = false
				}
			}
			if !documented {
				continue
			}
			for _, f := range s.Files {
				if !f.Test && ((known && scopeWord(f.Path, term)) || (!known && hasWords(wordSet(f.Path), terms))) && len(f.Words) > 0 {
					out = append(out, evidence.Evidence{Kind: "scope", Message: "An excluded capability is advertised and has source evidence: " + cap + ".", Source: s.ReadmePath + " + " + f.Path, Confidence: evidence.High})
					return true
				}
			}
		}
		return false
	}
	for _, cap := range s.Config.Scope.Excludes {
		check(cap)
	}
	if len(s.Config.Scope.Excludes) == 0 && s.Purpose.Confidence != evidence.Low {
		// A narrow CLI purpose plus three documented new functional areas is stronger
		// than directory count. No historical-purpose claim is made in the MVP.
		p := strings.ToLower(s.Purpose.Statement)
		if strings.Contains(p, "cli") || strings.Contains(p, "command line") || strings.Contains(p, "small tool") {
			candidates := []string{"dashboards", "monitoring", "alerts", "certificate renewal", "plugins"}
			found := []evidence.Evidence{}
			for _, cap := range candidates {
				if strings.Contains(p, strings.TrimSuffix(cap, "s")) {
					continue
				}
				start := len(out)
				if check(cap) {
					found = append(found, out[start])
					out = out[:start]
				}
			}
			if len(found) >= 3 {
				for i := range found {
					found[i].Confidence = evidence.Medium
					found[i].Message = strings.Replace(found[i].Message, "An excluded capability", "A capability beyond the narrow declared purpose", 1)
				}
				out = append(out, found...)
			}
		}
	}
	return out
}

func supportingPair(files []TextFile, ts []string) (string, string) {
	if len(ts) == 0 {
		return "", ""
	}
	required := (len(ts)*3 + 4) / 5
	if len(ts) > 1 && required < 2 {
		required = 2
	}
	count := func(a, b map[string]bool) int {
		n := 0
		for _, t := range ts {
			if a[t] && (b == nil || b[t]) {
				n++
			}
		}
		return n
	}
	sources, tests := []TextFile{}, []TextFile{}
	for _, f := range files {
		if count(f.Words, nil) < required {
			continue
		}
		if f.Test {
			tests = append(tests, f)
		} else {
			sources = append(sources, f)
		}
	}
	for _, source := range sources {
		for _, test := range tests {
			if count(source.Words, test.Words) >= required {
				return source.Path, test.Path
			}
		}
	}
	return "", ""
}
