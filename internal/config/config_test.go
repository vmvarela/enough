package config

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	valid := `version = 1
[purpose]
statement = 'Print certificate expiration.'
[scope]
includes = ["local certificates", # comment
 "HTTPS",]
excludes = ["dashboards"]
[enough]
when = ["inspection works"]
[test]
command = "go test ./..."
`
	c, e := Parse([]byte(valid))
	if e != nil {
		t.Fatal(e)
	}
	if c.Purpose.Statement != "Print certificate expiration." || len(c.Scope.Includes) != 2 || c.Test.Command != "go test ./..." {
		t.Fatalf("%+v", c)
	}
	for _, bad := range []string{"", "version = 2", "version = 1\nunknown = true", "version = 1\n[purpose]\nstatement = 17", "version = 1\n[scope]\nincludes = [17]", "version = 1\n[scope]\nexcludes = [\"\"]", "version = 1\n[purpose]\nstatement = \"\\n\""} {
		if _, e := Parse([]byte(bad)); e == nil {
			t.Errorf("accepted invalid configuration: %q", bad)
		}
	}
	_, e = Parse([]byte("version = 1\n[purpose]\nstatement = secret-token-value"))
	if e == nil || strings.Contains(e.Error(), "secret-token-value") {
		t.Fatal("parser leaked source text")
	}
}
