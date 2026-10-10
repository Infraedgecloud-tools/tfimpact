// Command tfimpact explains what a Terraform or OpenTofu plan will do to your
// AWS resources: data loss, replacements, restarts, renames that destroy, and
// IAM or network widening. It reads plan JSON and runs entirely locally.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/infraedge-tools/tfimpact/internal/config"
	"github.com/infraedge-tools/tfimpact/internal/findings"
	"github.com/infraedge-tools/tfimpact/internal/ghcomment"
	"github.com/infraedge-tools/tfimpact/internal/knowledge"
	"github.com/infraedge-tools/tfimpact/internal/plan"
	"github.com/infraedge-tools/tfimpact/internal/render"
	"github.com/infraedge-tools/tfimpact/internal/rules"
)

// Set by the release build (goreleaser ldflags).
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// now is replaced in tests so golden JSON files stay stable.
var now = time.Now

const (
	exitOK        = 0
	exitThreshold = 1
	exitUsage     = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "explain":
		return explain(args[1:], stdin, stdout, stderr)
	case "comment":
		return comment(args[1:], stderr)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, versionString())
		return exitOK
	case "help", "--help", "-h":
		usage(stdout)
		return exitOK
	}
	fmt.Fprintf(stderr, "tfimpact: unknown command %q\n\n", args[0])
	usage(stderr)
	return exitUsage
}

func usage(w io.Writer) {
	fmt.Fprint(w, `tfimpact explains what a Terraform or OpenTofu plan will do to your AWS resources.

Usage:
  tfimpact explain <plan.json|-> [flags]   explain a plan produced by 'terraform show -json tfplan'
  tfimpact comment --body FILE             create or update the pull-request comment (GitHub Actions)
  tfimpact version

Explain flags:
  --format markdown|text|json   what to print to stdout (default: text on a terminal, markdown otherwise)
  --output FILE                 also write the Markdown report to FILE
  --json FILE                   also write the findings JSON (schema 1.1) to FILE
  --fail-on LEVEL               exit 1 if an unsuppressed finding reaches LEVEL: none, low, medium, high, critical (default none)
  --no-values                   never print attribute values, only attribute names
  --config FILE                 suppressions file (default .tfimpact.toml if present)
  --knowledge DIR               load resource knowledge from DIR instead of the built-in copy
  --quiet                       print nothing to stdout

Exit codes: 0 ok · 1 threshold reached · 2 usage or input error
`)
}

func versionString() string {
	s := "tfimpact " + version
	if commit != "" {
		s += " (" + commit
		if date != "" {
			s += ", " + date
		}
		s += ")"
	}
	return s + " " + runtime.GOOS + "/" + runtime.GOARCH
}

func explain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "", "markdown, text or json")
	output := fs.String("output", "", "write the Markdown report to this file")
	jsonOut := fs.String("json", "", "write the findings JSON to this file")
	failOn := fs.String("fail-on", "none", "severity threshold for exit code 1")
	noValues := fs.Bool("no-values", false, "never print attribute values")
	configPath := fs.String("config", "", "suppressions file")
	knowledgeDir := fs.String("knowledge", "", "directory of knowledge TOML files")
	quiet := fs.Bool("quiet", false, "print nothing to stdout")
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(positional) != 1 {
		fmt.Fprintln(stderr, "tfimpact explain: exactly one plan file (or - for stdin) is required")
		return exitUsage
	}
	threshold, ok := parseThreshold(*failOn)
	if !ok {
		fmt.Fprintf(stderr, "tfimpact explain: --fail-on must be none, low, medium, high or critical, not %q\n", *failOn)
		return exitUsage
	}
	if *format != "" && *format != "markdown" && *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "tfimpact explain: --format must be markdown, text or json, not %q\n", *format)
		return exitUsage
	}

	source := positional[0]
	var in io.Reader
	if source == "-" {
		in = stdin
	} else {
		f, err := os.Open(source)
		if err != nil {
			fmt.Fprintf(stderr, "tfimpact: %v\n", err)
			return exitUsage
		}
		defer f.Close()
		in = f
	}
	p, err := plan.Load(in)
	if err != nil {
		fmt.Fprintf(stderr, "tfimpact: %v\n", err)
		return exitUsage
	}

	var kb *knowledge.Base
	if *knowledgeDir != "" {
		kb, err = knowledge.LoadDir(*knowledgeDir)
	} else {
		kb, err = knowledge.Load()
	}
	if err != nil {
		fmt.Fprintf(stderr, "tfimpact: %v\n", err)
		return exitUsage
	}

	cfgPath, explicit := config.DefaultPath, false
	if *configPath != "" {
		cfgPath, explicit = *configPath, true
	}
	cfg, err := config.Load(cfgPath, explicit)
	if err != nil {
		fmt.Fprintf(stderr, "tfimpact: %v\n", err)
		return exitUsage
	}

	now := now()
	result := rules.Analyze(p, kb, rules.Options{NoValues: *noValues})
	result.Tool = "tfimpact"
	result.ToolVersion = version
	result.Source = source
	result.GeneratedAt = now
	if cfg != nil {
		result.ConfigPath = cfg.Path
		cfg.Apply(result.Findings, now)
	}

	if *output != "" {
		if err := writeFile(*output, func(w io.Writer) error { return render.Markdown(w, result) }); err != nil {
			fmt.Fprintf(stderr, "tfimpact: %v\n", err)
			return exitUsage
		}
	}
	if *jsonOut != "" {
		if err := writeFile(*jsonOut, func(w io.Writer) error { return render.JSON(w, result) }); err != nil {
			fmt.Fprintf(stderr, "tfimpact: %v\n", err)
			return exitUsage
		}
	}
	if !*quiet {
		f := *format
		if f == "" {
			f = "markdown"
			if isTerminal(stdout) {
				f = "text"
			}
		}
		var rerr error
		switch f {
		case "markdown":
			rerr = render.Markdown(stdout, result)
		case "text":
			rerr = render.Text(stdout, result)
		case "json":
			rerr = render.JSON(stdout, result)
		}
		if rerr != nil {
			fmt.Fprintf(stderr, "tfimpact: %v\n", rerr)
			return exitUsage
		}
	}
	if threshold != "" && result.Reaches(threshold) {
		return exitThreshold
	}
	return exitOK
}

// parseInterspersed lets flags appear before or after positional arguments,
// as in `tfimpact explain plan.json --fail-on high`.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func parseThreshold(s string) (findings.Severity, bool) {
	if strings.EqualFold(s, "none") || s == "" {
		return "", true
	}
	return findings.Parse(s)
}

func writeFile(path string, write func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func comment(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("comment", flag.ContinueOnError)
	fs.SetOutput(stderr)
	body := fs.String("body", "", "file holding the Markdown comment body")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if *body == "" {
		fmt.Fprintln(stderr, "tfimpact comment: --body FILE is required")
		return exitUsage
	}
	data, err := os.ReadFile(*body)
	if err != nil {
		fmt.Fprintf(stderr, "tfimpact: %v\n", err)
		return exitUsage
	}
	env, err := ghcomment.FromEnvironment()
	if err != nil {
		fmt.Fprintf(stderr, "tfimpact: %v\n", err)
		return exitUsage
	}
	if err := ghcomment.Upsert(env, string(data)); err != nil {
		if errors.Is(err, ghcomment.ErrNotPullRequest) {
			fmt.Fprintln(stderr, "tfimpact comment: not a pull request event, nothing posted")
			return exitOK
		}
		fmt.Fprintf(stderr, "tfimpact: %v\n", err)
		return exitUsage
	}
	return exitOK
}
