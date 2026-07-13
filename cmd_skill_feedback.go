package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// cmdSkillFeedback — Channel 2 of the QualityMax skill feedback system.
// Customers using .claude/skills/*-qm/ run this to report when a skill
// missed a pattern, hallucinated, or just worked well.
//
// Spec: docs/SKILL_FEEDBACK_CHANNELS.md
func cmdSkillFeedback(args []string) {
	if len(args) < 1 {
		printSkillFeedbackUsage()
		os.Exit(1)
	}

	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printSkillFeedbackUsage()
		return
	}

	skillName := args[0]
	if !strings.HasSuffix(skillName, "-qm") {
		fmt.Fprintf(os.Stderr, "Error: skill name must end with -qm (got %q)\n", skillName)
		os.Exit(1)
	}

	fs := flag.NewFlagSet("skill-feedback", flag.ExitOnError)
	good := fs.Bool("good", false, "Report this skill as having helped")
	bad := fs.Bool("bad", false, "Report this skill as having missed/hallucinated (default)")
	prURL := fs.String("pr", "", "PR URL (auto-detected from git context if omitted)")
	version := fs.String("version", "", "Skill version (omit for latest)")
	source := fs.String("source", "cli", "Source: cli, reaction, reply, oss_pr")
	_ = fs.Parse(args[1:])

	note := strings.Join(fs.Args(), " ")
	if note == "" {
		fmt.Fprintln(os.Stderr, "Error: please include a note describing what worked or what was missed.")
		fmt.Fprintln(os.Stderr, "Usage: qmax skill-feedback <skill-name> [--good|--bad] \"<note>\"")
		os.Exit(1)
	}

	sentiment := "negative"
	if *good && *bad {
		fmt.Fprintln(os.Stderr, "Error: pass --good OR --bad, not both.")
		os.Exit(1)
	}
	if *good {
		sentiment = "positive"
	}

	resolvedPR := *prURL
	if resolvedPR == "" {
		resolvedPR = detectPRFromGit()
	}

	cfg := mustLoadConfig()
	apiURL := cfg.GetAPIBaseURL()

	payload := map[string]interface{}{
		"skill_name": skillName,
		"sentiment":  sentiment,
		"note":       note,
		"source":     *source,
	}
	if *version != "" {
		payload["skill_version"] = *version
	}
	if resolvedPR != "" {
		payload["pr_url"] = resolvedPR
	}

	body := authPost(cfg, apiURL+"/api/skill-feedback", payload)

	var resp struct {
		Success bool `json:"success"`
	}
	mustUnmarshal(body, &resp)

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Server returned no success flag. Raw: %s\n", string(body))
		os.Exit(1)
	}

	icon := "👍"
	if sentiment == "negative" {
		icon = "👎"
	}
	fmt.Printf("%s Recorded %s feedback on %s. Thanks — this feeds the skill-postmortem-qm review queue.\n",
		icon, sentiment, skillName)
	if resolvedPR != "" {
		fmt.Printf("   Linked to PR: %s\n", resolvedPR)
	}
}

// detectPRFromGit best-effort extracts the current branch's PR URL via `gh pr view --json url`.
// Returns empty string if gh is not installed or no PR exists.
func detectPRFromGit() string {
	out, err := exec.Command("gh", "pr", "view", "--json", "url", "-q", ".url").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func printSkillFeedbackUsage() {
	fmt.Println(`Usage: qmax skill-feedback <skill-name> [flags] "<note>"

Reports actionable feedback on a QualityMax battle skill (.claude/skills/*-qm/).
Feeds the weekly skill-postmortem review queue so the skills evolve from
real-world usage, not just our internal dogfooding.

Skill name must end with -qm.

Flags:
  --good           Skill helped (default: --bad, since useful negatives are rarer)
  --bad            Skill missed a pattern, hallucinated, or wasn't invoked
  --pr URL         PR URL (auto-detected via "gh pr view" if omitted)
  --version V      Skill version (default: latest at time of submission)
  --source SRC     Channel: cli (default), reaction, reply, oss_pr

Examples:
  qmax skill-feedback sast-presurgery-qm --bad \
    "Missed PII-in-error-response — round 4 of #621 was about detail=str(e) leaking emails."

  qmax skill-feedback mobile-auth-qm --good \
    "Followed the exchange-code pattern, single round on PR #45."`)
}
