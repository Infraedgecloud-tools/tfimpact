// Package config reads .tfimpact.toml: suppressions with a required reason.
//
// Suppressed findings stay in the output, marked as suppressed, and do not
// count towards --fail-on. An expired suppression stops applying.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/infraedgecloud-tools/tfimpact/internal/findings"
)

// DefaultPath is looked for in the working directory when --config is not given.
const DefaultPath = ".tfimpact.toml"

// Config is the parsed file.
type Config struct {
	Version  int64         `toml:"version"`
	Suppress []Suppression `toml:"suppress"`
	Path     string        `toml:"-"`
}

// Suppression silences one rule on matching addresses.
type Suppression struct {
	Rule    string `toml:"rule"`
	Address string `toml:"address"` // glob on the resource address; "*" matches all
	Reason  string `toml:"reason"`
	Expires string `toml:"expires"` // YYYY-MM-DD
}

// Load reads and validates the file. A missing default file is not an error.
func Load(p string, explicit bool) (*Config, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			return nil, nil
		}
		return nil, fmt.Errorf("read config %s: %w", p, err)
	}
	var c Config
	md, err := toml.Decode(string(data), &c)
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", p, err)
	}
	if und := md.Undecoded(); len(und) > 0 {
		return nil, fmt.Errorf("config %s: unknown keys %v", p, und)
	}
	if c.Version != 1 {
		return nil, fmt.Errorf("config %s: version must be 1", p)
	}
	for i, s := range c.Suppress {
		if s.Rule == "" {
			return nil, fmt.Errorf("config %s: suppress[%d] needs a rule", p, i)
		}
		if strings.TrimSpace(s.Reason) == "" {
			return nil, fmt.Errorf("config %s: suppress[%d] (%s) needs a reason", p, i, s.Rule)
		}
		if s.Address == "" {
			c.Suppress[i].Address = "*"
		}
		if s.Expires != "" {
			if _, err := time.Parse("2006-01-02", s.Expires); err != nil {
				return nil, fmt.Errorf("config %s: suppress[%d] expires must be YYYY-MM-DD", p, i)
			}
		}
	}
	c.Path = p
	return &c, nil
}

// matchAddress reports whether a suppression's address pattern covers a
// finding's address. "*" matches any run of characters and is the only
// wildcard; everything else is literal, including the brackets, quotes and
// slashes that for_each and count put into an address.
//
// This used to be path.Match, which reads "[" as a character class and stops
// "*" at a "/": an exact pattern of aws_s3_bucket.b["a"] never matched the
// address aws_s3_bucket.b["a"], and no pattern could reach a key containing
// a slash. Every resource with for_each or count was unsuppressible.
func matchAddress(pattern, address string) bool {
	if pattern == "*" || pattern == address {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return false
	}
	if !strings.HasPrefix(address, parts[0]) {
		return false
	}
	rest := address[len(parts[0]):]
	last := len(parts) - 1
	for _, part := range parts[1:last] {
		i := strings.Index(rest, part)
		if i < 0 {
			return false
		}
		rest = rest[i+len(part):]
	}
	return strings.HasSuffix(rest, parts[last])
}

// Apply marks findings suppressed according to the config, as of now.
func (c *Config) Apply(fs []findings.Finding, now time.Time) {
	if c == nil {
		return
	}
	today := now.UTC().Format("2006-01-02")
	for i := range fs {
		for _, s := range c.Suppress {
			if s.Rule != fs[i].RuleID && s.Rule != "*" {
				continue
			}
			if s.Expires != "" && s.Expires < today {
				continue
			}
			if !matchAddress(s.Address, fs[i].Address()) {
				continue
			}
			fs[i].Suppressed = true
			reason := s.Reason
			fs[i].SuppressionReason = &reason
			if s.Expires != "" {
				exp := s.Expires
				fs[i].SuppressionExpires = &exp
			}
			break
		}
	}
}
