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

	"github.com/infraedgecloud-tools/tfimpact/internal/config"
	"github.com/infraedgecloud-tools/tfimpact/internal/findings"
	"github.com/infraedgecloud-tools/tfimpact/internal/ghcomment"
	"github.com/infraedgecloud-tools/tfimpact/internal/knowledge"
	"github.com/infraedgecloud-tools/tfimpact/internal/plan"
	"github.com/infraedgecloud-tools/tfimpact/internal/render"
	"github.com/infraedgecloud-tools/tfimpact/internal/rules"
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
	case "rules":
		return listRules(stdout)
	case "knowledge":
		return showKnowledge(args[1:], stdout, stderr)
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
  tfimpact rules                           list the rules and what they mean
  tfimpact knowledge [TYPE]                list the AWS resource types tfimpact knows, or show one
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

func listRules(stdout io.Writer) int {
	for _, r := range rules.Catalogue {
		fmt.Fprintf(stdout, "%-16s %-13s %-14s %s\n", r.ID, r.Domain, r.Severity, r.Summary)
	}
	return exitOK
}

func showKnowledge(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("knowledge", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("knowledge", "", "directory of knowledge TOML files")
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	var kb *knowledge.Base
	if *dir != "" {
		kb, err = knowledge.LoadDir(*dir)
	} else {
		kb, err = knowledge.Load()
	}
	if err != nil {
		fmt.Fprintf(stderr, "tfimpact: %v\n", err)
		return exitUsage
	}
	if len(positional) == 0 {
		for _, t := range kb.Types() {
			r := kb.Get(t)
			fmt.Fprintf(stdout, "%-50s %s\n", t, knowledgeTags(r))
		}
		return exitOK
	}
	r := kb.Get(positional[0])
	if r == nil {
		fmt.Fprintf(stderr, "tfimpact: no knowledge for %s; add internal/knowledge/aws/%s.toml\n", positional[0], positional[0])
		return exitUsage
	}
	fmt.Fprintf(stdout, "%s (%s)\n%s\n", r.Type, r.Display, r.Doc)
	if r.HoldsData {
		fmt.Fprintf(stdout, "\nHolds data: %s\n", r.Data)
	}
	if r.DowntimeOnReplace {
		fmt.Fprintf(stdout, "\nOn replace: %s\n", r.Downtime)
	}
	if r.Delete != nil {
		fmt.Fprintf(stdout, "\nOn delete (%s): %s\n", r.Delete.Severity, r.Delete.Message)
	}
	if len(r.Identity) > 0 {
		fmt.Fprintf(stdout, "\nIdentity for rename detection: %s\n", strings.Join(r.Identity, ", "))
	}
	if len(r.Mitigations) > 0 {
		fmt.Fprintln(stdout, "\nLoss is real when:")
		for _, c := range r.Mitigations {
			fmt.Fprintf(stdout, "  %s: %s\n", conditionText(c), c.Message)
		}
	}
	if len(r.Safeguards) > 0 {
		fmt.Fprintln(stdout, "\nSoftened when:")
		for _, c := range r.Safeguards {
			fmt.Fprintf(stdout, "  %s: %s\n", conditionText(c), c.Message)
		}
	}
	if len(r.Disruptive) > 0 {
		fmt.Fprintln(stdout, "\nDisruptive in-place changes:")
		for _, d := range r.Disruptive {
			sev := d.Severity
			if sev == "" {
				sev = "medium"
			}
			fmt.Fprintf(stdout, "  %s (%s): %s\n", d.Attr, sev, d.Effect)
		}
	}
	if len(r.Flags) > 0 {
		fmt.Fprintln(stdout, "\nExposure flags:")
		for _, f := range r.Flags {
			sev := f.Severity
			if sev == "" {
				sev = "high"
			}
			fmt.Fprintf(stdout, "  %s = %v (%s): %s\n", f.Attr, f.Unsafe, sev, f.Message)
		}
	}
	if len(r.Policies) > 0 {
		fmt.Fprintln(stdout, "\nPolicy documents:")
		for _, p := range r.Policies {
			fmt.Fprintf(stdout, "  %s (%s policy)\n", p.Attr, p.Kind)
		}
	}
	return exitOK
}

func knowledgeTags(r *knowledge.Resource) string {
	var tags []string
	if r.HoldsData {
		tags = append(tags, "data")
	}
	if r.DowntimeOnReplace {
		tags = append(tags, "downtime-on-replace")
	}
	if r.Delete != nil {
		tags = append(tags, "protective")
	}
	if len(r.Disruptive) > 0 {
		tags = append(tags, "disruptive-updates")
	}
	if len(r.Flags) > 0 {
		tags = append(tags, "exposure-flags")
	}
	if len(r.Policies) > 0 {
		tags = append(tags, "policy")
	}
	if len(tags) == 0 {
		return "display only"
	}
	return strings.Join(tags, ", ")
}

func conditionText(c knowledge.Condition) string {
	switch {
	case c.Absent:
		return c.Attr + " not set"
	case c.Present:
		return c.Attr + " set"
	default:
		return fmt.Sprintf("%s = %v", c.Attr, c.Equals)
	}
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
