package maat

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// absRoot resolves the repo root argument (default: current directory) to an
// absolute path, matching Python's os.path.abspath(path or ".").
func absRoot(path string) string {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// usageError signals an argument-parsing problem: the CLI prints usage to
// stderr and exits 2, mirroring argparse.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

const usageText = `usage: maat [--version] <command> [options] [PATH]

Documentation-as-code for humans and AI agents.

commands:
  init    scaffold Ma'at into a repository
  sync    regenerate llms.txt, adapters, index
  check   validate docs (CI gate)

Run 'maat <command> --help' for command-specific options.`

// Main is the CLI entry point. It returns a process exit code. stdout/stderr
// are injected so tests can capture output. This is the Go analogue of the
// Python cli.main(argv).
func Main(argv []string, stdout, stderr io.Writer) int {
	if len(argv) == 0 {
		fmt.Fprintln(stderr, usageText)
		return 2
	}

	// Top-level --version / -h before a subcommand.
	switch argv[0] {
	case "--version":
		fmt.Fprintf(stdout, "maat %s\n", Version())
		return 0
	case "-h", "--help":
		fmt.Fprintln(stdout, usageText)
		return 0
	}

	command := argv[0]
	rest := argv[1:]

	var code int
	var err error
	switch command {
	case "init":
		code, err = cmdInit(rest, stdout, stderr)
	case "sync":
		code, err = cmdSync(rest, stdout, stderr)
	case "check":
		code, err = cmdCheck(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "maat: error: unknown command %s\n", pyRepr(command))
		fmt.Fprintln(stderr, usageText)
		return 2
	}

	if err != nil {
		if ue, ok := err.(*usageError); ok {
			fmt.Fprintf(stderr, "maat: error: %s\n", ue.msg)
			return 2
		}
		fmt.Fprintf(stderr, "maat: error: %s\n", err)
		return 2
	}
	return code
}

// parsed holds the parsed flags/positional common to the subcommands.
type parsed struct {
	path      string
	name      string
	summary   string
	force     bool
	format    string
	strict    bool
	agents    []string
	agentsSet bool // distinguishes "--agents=" (explicitly zero agents) from "not given"
}

// parseArgs parses a subcommand's arguments. allowed lists the value-taking
// flags this command accepts. Unknown flags produce a usage error.
func parseArgs(command string, args []string) (*parsed, error) {
	p := &parsed{format: "text"}
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--force" && command == "init":
			p.force = true
		case arg == "--strict" && command == "check":
			p.strict = true
		case arg == "--name" && command == "init":
			val, next, err := flagValue(args, i)
			if err != nil {
				return nil, err
			}
			p.name, i = val, next
		case arg == "--summary" && command == "init":
			val, next, err := flagValue(args, i)
			if err != nil {
				return nil, err
			}
			p.summary, i = val, next
		case arg == "--agents" && command == "init":
			val, next, err := flagValue(args, i)
			if err != nil {
				return nil, err
			}
			agents, err := parseAgentsFlag(val)
			if err != nil {
				return nil, err
			}
			p.agents, p.agentsSet, i = agents, true, next
		case arg == "--format" && command == "check":
			val, next, err := flagValue(args, i)
			if err != nil {
				return nil, err
			}
			if val != "text" && val != "github" {
				return nil, &usageError{fmt.Sprintf("argument --format: invalid choice: %s (choose from 'text', 'github')", pyRepr(val))}
			}
			p.format, i = val, next
		case strings.HasPrefix(arg, "--name=") && command == "init":
			p.name = strings.TrimPrefix(arg, "--name=")
		case strings.HasPrefix(arg, "--summary=") && command == "init":
			p.summary = strings.TrimPrefix(arg, "--summary=")
		case strings.HasPrefix(arg, "--agents=") && command == "init":
			agents, err := parseAgentsFlag(strings.TrimPrefix(arg, "--agents="))
			if err != nil {
				return nil, err
			}
			p.agents, p.agentsSet = agents, true
		case strings.HasPrefix(arg, "--format=") && command == "check":
			val := strings.TrimPrefix(arg, "--format=")
			if val != "text" && val != "github" {
				return nil, &usageError{fmt.Sprintf("argument --format: invalid choice: %s (choose from 'text', 'github')", pyRepr(val))}
			}
			p.format = val
		case arg == "-h" || arg == "--help":
			return nil, &usageError{"help requested"}
		case strings.HasPrefix(arg, "-"):
			return nil, &usageError{fmt.Sprintf("unrecognized arguments: %s", arg)}
		default:
			if p.path != "" {
				return nil, &usageError{fmt.Sprintf("unrecognized arguments: %s", arg)}
			}
			p.path = arg
		}
		i++
	}
	return p, nil
}

func flagValue(args []string, i int) (string, int, error) {
	if i+1 >= len(args) {
		return "", i, &usageError{fmt.Sprintf("argument %s: expected one argument", args[i])}
	}
	return args[i+1], i + 1, nil
}

// parseAgentsFlag parses --agents' value: a comma-separated list of adapter
// names, "all" as shorthand for adapterOrder, or "" for explicitly zero
// agents (AGENTS.md only, no per-agent adapter files).
func parseAgentsFlag(val string) ([]string, error) {
	val = strings.TrimSpace(val)
	if val == "all" {
		return append([]string{}, adapterOrder...), nil
	}
	var names []string
	for _, part := range strings.Split(val, ",") {
		if part = strings.TrimSpace(part); part != "" {
			names = append(names, part)
		}
	}
	if err := validateAgentNames(names); err != nil {
		return nil, &usageError{fmt.Sprintf("argument --agents: %s", err)}
	}
	return names, nil
}

// cmdInit's (int, error) signature matches cmdSync/cmdCheck so Main can
// dispatch to all three uniformly.
func cmdInit(args []string, stdout, _ io.Writer) (int, error) {
	p, err := parseArgs("init", args)
	if err != nil {
		return 0, err
	}
	root := absRoot(p.path)
	name, summary := p.name, p.summary
	agents, agentsChosen := p.agents, p.agentsSet
	if name == "" && summary == "" && !agentsChosen && isInteractiveTerminal(stdout) {
		// Seed the wizard's agent multi-select from whatever's already
		// configured (LoadConfig defaults to adapterOrder when .maat.yml is
		// absent), so a re-init shows the repo's actual current state rather
		// than always resetting to "everything checked."
		seedCfg, _ := LoadConfig(root)
		w, err := runInitWizard(filepath.Base(root), toStringList(seedCfg["adapters"]))
		if err != nil {
			return 0, err
		}
		if !w.ok {
			// User aborted the wizard (Ctrl-C/Esc) — SIGINT exit-code convention.
			return 130, nil
		}
		name, summary, agents = w.name, w.summary, w.agents
		agentsChosen = true
	}
	if name == "" {
		name = filepath.Base(root)
	}
	if !agentsChosen {
		agents = adapterOrder
	}
	result, err := RunInit(root, name, summary, agents, p.force)
	if err != nil {
		return 0, err
	}
	color := isColorEnabled(stdout)
	for _, rel := range result.Created {
		if color {
			fmt.Fprintln(stdout, styledCreateLine(rel))
		} else {
			fmt.Fprintf(stdout, "  create  %s\n", rel)
		}
	}
	for _, rel := range result.Skipped {
		if color {
			fmt.Fprintln(stdout, styledSkipLine(rel))
		} else {
			fmt.Fprintf(stdout, "  skip    %s (exists; use --force to overwrite)\n", rel)
		}
	}
	for _, rel := range result.Generated {
		if color {
			fmt.Fprintln(stdout, styledGenLine(rel))
		} else {
			fmt.Fprintf(stdout, "  gen     %s\n", rel)
		}
	}
	fmt.Fprintf(stdout, "\nMa'at initialized in %s\n", root)
	if agentsChosen && contains(result.Skipped, ".maat.yml") {
		// .maat.yml already existed and was preserved untouched (like any
		// other scaffold file), so a newly-chosen agent not already in its
		// adapters: list needs a hand-edit + sync — Ma'at has no YAML
		// emitter to splice it in automatically (see ADR 0002/0011).
		if cfg, cErr := LoadConfig(root); cErr == nil {
			existing := toStringList(cfg["adapters"])
			var missing []string
			for _, a := range agents {
				if !contains(existing, a) {
					missing = append(missing, "`"+a+"`")
				}
			}
			if len(missing) > 0 {
				fmt.Fprintf(stdout, "\nTo also generate adapter files for %s, add %s to `adapters:` in .maat.yml, then run `maat sync`.\n",
					strings.Join(missing, ", "), strings.Join(missing, ", "))
			}
		}
	}
	if len(result.Skipped) > 0 {
		// Brownfield adoption (ADR 0008): pre-existing files were preserved,
		// so the scaffold is incomplete by design. Tell the user what that
		// means and where the procedure for closing the gap lives. The
		// maintenance contract (update protocol + skills) was still spliced
		// into the instruction file as a managed block (ADR 0009), so call
		// that out — it explains why AGENTS.md can appear as both skip and gen.
		fmt.Fprintf(stdout, "\n%d file(s) already existed and were left untouched (listed as `skip` above).\n"+
			"They are yours — Ma'at never overwrites hand-written files (use --force to override).\n"+
			"Ma'at did splice its maintenance contract (the documentation update protocol\n"+
			"and the skills index) into a managed block in your existing instruction file —\n"+
			"that is why it may appear under both `skip` and `gen`. Your hand-written text\n"+
			"is preserved; only the marked block is Ma'at's.\n"+
			"To finish adopting Ma'at in this existing repository:\n"+
			"  1. Run `maat check` to see what the preserved files are still missing.\n"+
			"  2. Reconcile each skipped file with its scaffolded counterpart —\n"+
			"     or let your AI agent do it: point it at %s/retrospect/SKILL.md,\n"+
			"     which walks through deriving docs and ADRs from an existing codebase.\n"+
			"  3. Re-run `maat check` until green, then commit.\n", len(result.Skipped), skillsRoot)
	} else {
		fmt.Fprint(stdout, "Next: edit AGENTS.md's project overview, then run `maat check`.\n")
	}
	return 0, nil
}

// See cmdInit for why this always returns 0 on success.
//
//nolint:unparam
func cmdSync(args []string, stdout, _ io.Writer) (int, error) {
	p, err := parseArgs("sync", args)
	if err != nil {
		return 0, err
	}
	root := absRoot(p.path)
	cfg, err := LoadConfig(root)
	if err != nil {
		return 0, err
	}
	if err := enforceVersion(cfg); err != nil {
		return 0, err
	}
	changed, err := RunSync(root, cfg)
	if err != nil {
		return 0, err
	}
	if len(changed) == 0 {
		fmt.Fprintln(stdout, "Already in sync — no files changed.")
	} else {
		color := isColorEnabled(stdout)
		for _, rel := range changed {
			if color {
				fmt.Fprintln(stdout, styledUpdateLine(rel))
			} else {
				fmt.Fprintf(stdout, "  update  %s\n", rel)
			}
		}
		fmt.Fprintf(stdout, "\nSynced %d file(s).\n", len(changed))
	}
	return 0, nil
}

func cmdCheck(args []string, stdout, stderr io.Writer) (int, error) {
	p, err := parseArgs("check", args)
	if err != nil {
		return 0, err
	}
	root := absRoot(p.path)
	cfg, err := LoadConfig(root)
	if err != nil {
		return 0, err
	}
	if err := enforceVersion(cfg); err != nil {
		return 0, err
	}
	if p.strict {
		check, _ := cfg["check"].(map[string]any)
		if check == nil {
			check = map[string]any{}
			cfg["check"] = check
		}
		check["staleness"] = "error"
	}

	docsDir := AnyToStr(cfg["docs_dir"])
	docsPath := filepath.Join(root, docsDir)
	if info, err := os.Stat(docsPath); err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "No %s/ directory found. Run `maat init` first.\n", docsDir)
		return 2, nil
	}

	model, err := ScanModel(root, docsDir)
	if err != nil {
		return 0, err
	}
	findings := RunAll(model, cfg, root)

	var errors, warnings int
	for _, f := range findings {
		if f.Severity == "error" {
			errors++
		} else {
			warnings++
		}
	}

	// --format=github output is machine-consumed (GitHub Actions annotation
	// syntax) and must never carry ANSI codes, regardless of TTY/color state.
	color := isColorEnabled(stdout) && p.format != "github"
	if p.format == "github" {
		emitGitHub(findings, stdout)
	} else if color {
		for _, f := range findings {
			fmt.Fprintln(stdout, styledFindingLine(f))
		}
	} else {
		for _, f := range findings {
			fmt.Fprintln(stdout, f.String())
		}
	}

	if color {
		fmt.Fprint(stdout, styledCheckSummary(len(model.Documents), errors, warnings))
	} else {
		fmt.Fprintf(stdout, "\nChecked %d document(s): %d error(s), %d warning(s).\n",
			len(model.Documents), errors, warnings)
	}

	if errors > 0 {
		return 1, nil
	}
	return 0, nil
}

// emitGitHub prints GitHub Actions workflow annotations.
func emitGitHub(findings []Finding, stdout io.Writer) {
	for _, f := range findings {
		level := "warning"
		if f.Severity == "error" {
			level = "error"
		}
		loc := ""
		if strings.ContainsRune(f.Where, os.PathSeparator) || strings.Contains(f.Where, "/") {
			loc = "file=" + f.Where + ","
		}
		fmt.Fprintf(stdout, "::%s %stitle=maat %s::%s\n", level, loc, f.Code, f.Message)
	}
}
