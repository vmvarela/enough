package config

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"strings"
)

type Config struct {
	Version int `toml:"version"`
	Purpose struct {
		Statement string `toml:"statement"`
	} `toml:"purpose"`
	Scope struct {
		Includes []string `toml:"includes"`
		Excludes []string `toml:"excludes"`
	} `toml:"scope"`
	Enough struct {
		When []string `toml:"when"`
	} `toml:"enough"`
	Test struct {
		Command string `toml:"command"`
	} `toml:"test"`
}

func Parse(data []byte) (Config, error) {
	var c Config
	meta, err := toml.Decode(string(data), &c)
	if err != nil {
		return c, fmt.Errorf("invalid enough.toml (check TOML syntax and field types)")
	}
	if len(meta.Undecoded()) > 0 {
		return c, fmt.Errorf("enough.toml contains unsupported keys")
	}
	if c.Version != 1 {
		return c, fmt.Errorf("enough.toml requires version = 1")
	}
	if len(c.Purpose.Statement) > 500 || strings.ContainsAny(c.Purpose.Statement, "\r\n\x1b") {
		return c, fmt.Errorf("purpose.statement must be a single line of at most 500 bytes")
	}
	c.Purpose.Statement = strings.TrimSpace(c.Purpose.Statement)
	for _, list := range [][]string{c.Scope.Includes, c.Scope.Excludes, c.Enough.When} {
		if len(list) > 100 {
			return c, fmt.Errorf("configuration lists are limited to 100 entries")
		}
		for _, s := range list {
			if strings.TrimSpace(s) == "" || len(s) > 500 || strings.ContainsAny(s, "\r\n\x1b") {
				return c, fmt.Errorf("configuration entries must be nonempty single lines of at most 500 bytes")
			}
		}
	}
	return c, nil
}
