package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Quality-Max/qmax-local-agent/policy"
	receipt "github.com/Quality-Max/qmax-receipt"
)

func main() {
	// Unify state under ~/.qmax (migrating any legacy ~/.qamax) before anything
	// reads config or writes a receipt, so existing logins + the agent signing
	// identity survive the rename.
	migrateLegacyStateDir()
	if dir, err := ConfigDir(); err == nil {
		receipt.BaseDir = dir // ~/.qmax — receipts + signing key live alongside config
	}

	// Stamp the agent version into every Exposure Receipt and apply egress config.
	receipt.AgentVersion = Version
	configureEgressFromEnv()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	// `run` (daemon) and `receipt` manage their own receipts; help/version do no
	// egress. Every other command is wrapped in a CLI Exposure Receipt below.
	switch cmd {
	case "run", "receipt", "help", "--help", "-h", "version", "--version", "-v":
		dispatch(cmd)
		return
	}

	rec := receipt.NewCurrent("cli:" + cmd)
	defer finalizeCLIReceipt(rec)
	dispatch(cmd)
}

// configureEgressFromEnv applies the egress guard mode from the environment.
// Env (not flags) so it composes with each command's own flag parser and works
// cleanly in CI: QMAX_STRICT_EGRESS=1, QMAX_ALLOW_AUTH_CAPTURE=1.
func configureEgressFromEnv() {
	if isTruthy(os.Getenv("QMAX_STRICT_EGRESS")) {
		policy.SetMode(policy.ModeStrict)
	}
	if isTruthy(os.Getenv("QMAX_ALLOW_AUTH_CAPTURE")) {
		policy.SetAllowAuthCapture(true)
	}
}

func isTruthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// finalizeCLIReceipt writes the receipt only if the command actually egressed,
// keeping the receipts directory free of empty manifests.
func finalizeCLIReceipt(r *receipt.Receipt) {
	if r.EntryCount() == 0 {
		return
	}
	if path, err := r.Finalize(); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: failed to write exposure receipt: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Exposure receipt: %s\n", path)
	}
}

func dispatch(cmd string) {
	switch cmd {
	case "run":
		cmdRun(os.Args[2:])
	case "login":
		cmdLogin(os.Args[2:])
	case "capture":
		cmdCapture(os.Args[2:])
	case "projects":
		cmdProjects(os.Args[2:])
	case "status":
		cmdStatus(os.Args[2:])
	case "token":
		cmdToken(os.Args[2:])
	case "logout":
		cmdLogout(os.Args[2:])
	case "test":
		cmdTest(os.Args[2:])
	case "crawl":
		cmdCrawl(os.Args[2:])
	case "repo":
		cmdRepo(os.Args[2:])
	case "import":
		cmdImport(os.Args[2:])
	case "pr":
		cmdPR(os.Args[2:])
	case "sast":
		cmdSast(os.Args[2:])
	case "ci":
		cmdCI(os.Args[2:])
	case "skill-feedback":
		cmdSkillFeedback(os.Args[2:])
	case "receipt":
		cmdReceipt(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	case "version", "--version", "-v":
		fmt.Printf("qmax v%s\n", Version)
	default:
		// Backward compat: if first arg starts with "-", treat as `run` with all args
		// e.g. qmax --cloud-url URL → qmax run --cloud-url URL
		if strings.HasPrefix(cmd, "-") {
			cmdRun(os.Args[1:])
		} else {
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Printf(`qmax v%s — QualityMax Local Agent CLI

Usage:
  qmax <command> [flags]

Commands:
  run        Start the agent daemon (poll for test assignments)
  login      Authenticate with QualityMax via browser OAuth
  capture    Launch Chrome, capture cookies, upload as auth data
  projects   List available projects
  test       Test operations (cases, scripts, run, generate, status)
  crawl      AI-powered crawl (start, status, results, jobs)
  repo       Repository operations (list, review, coverage, quality)
  import     Import repositories or documents for test generation
  pr         Create pull requests with generated tests
  status     Show current auth and agent status
  token      Print the saved OAuth token to stdout
  logout     Remove saved credentials
  sast       SAST security scanning (verify, install, scan, setup)
  ci         Headless CI runner (auth + run + report for GitHub Actions)
  skill-feedback  Report on a QualityMax battle skill (.claude/skills/*-qm/)
  receipt    Inspect Exposure Receipts (list, show, verify) — what left your network

Egress guard (env):
  QMAX_STRICT_EGRESS=1        Block any request not on the egress allow-list
  QMAX_ALLOW_AUTH_CAPTURE=1   Permit cookie/localStorage upload (off by default)

Flags:
  --help     Show this help message
  --version  Show version

Backward compatibility:
  qmax --cloud-url URL   (equivalent to: qmax run --cloud-url URL)
`, Version)
}
