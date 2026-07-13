package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	receipt "github.com/Quality-Max/qmax-receipt"
	"github.com/Quality-Max/qmax-local-agent/httpx"
	"github.com/Quality-Max/qmax-local-agent/sysmetrics"
)

const (
	Version   = "4.5.0"
	AgentName = "QualityMax Local Agent"
)

// OnRegistered is called after a successful registration with the new agent ID and API key.
type OnRegistered func(agentID, apiKey string)

// Agent represents the local test execution agent.
type Agent struct {
	CloudURL           string
	APIKey             string
	AgentID            string
	RegistrationSecret string
	PollInterval       time.Duration
	HeartbeatInterval  time.Duration
	MachineID          string
	Capabilities       map[string]interface{}
	OnRegistered       OnRegistered

	activeTests sync.Map
	activeCount int
	mu          sync.Mutex
	running     bool
}

// NewAgent creates a new Agent with the given configuration.
func NewAgent(cloudURL, apiKey, agentID, registrationSecret string, pollInterval, heartbeatInterval time.Duration) *Agent {
	a := &Agent{
		CloudURL:           strings.TrimRight(cloudURL, "/"),
		APIKey:             apiKey,
		AgentID:            agentID,
		RegistrationSecret: registrationSecret,
		PollInterval:       pollInterval,
		HeartbeatInterval:  heartbeatInterval,
	}
	a.MachineID = a.getMachineID()
	a.Capabilities = a.detectCapabilities()
	return a
}

func (a *Agent) getMachineID() string {
	hostname, _ := os.Hostname()
	ip := ""
	addrs, err := net.LookupHost(hostname)
	if err == nil && len(addrs) > 0 {
		ip = addrs[0]
	}
	return fmt.Sprintf("%s-%s-%s", runtime.GOOS, hostname, ip)
}

func (a *Agent) detectCapabilities() map[string]interface{} {
	caps := map[string]interface{}{
		"frameworks":       []string{"playwright"},
		"browsers":         []string{},
		"execution_type":   "local_agent",
		"platform":         runtime.GOOS,
		"platform_version": a.getPlatformVersion(),
		"architecture":     runtime.GOARCH,
	}

	if _, err := exec.LookPath("npx"); err == nil {
		caps["playwright_available"] = true
	} else {
		caps["playwright_available"] = false
	}

	var browsers []string

	chromePaths := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
		"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium-browser",
	}
	if pathExists(chromePaths) || commandExists("google-chrome") || commandExists("chromium") {
		browsers = append(browsers, "chromium")
	}

	firefoxPaths := []string{
		"/Applications/Firefox.app/Contents/MacOS/firefox",
		"C:\\Program Files\\Mozilla Firefox\\firefox.exe",
		"/usr/bin/firefox",
	}
	if pathExists(firefoxPaths) || commandExists("firefox") {
		browsers = append(browsers, "firefox")
	}

	if runtime.GOOS == "windows" {
		edgePaths := []string{
			"C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
			"C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe",
		}
		if pathExists(edgePaths) {
			browsers = append(browsers, "msedge")
		}
	}

	if runtime.GOOS == "darwin" {
		if _, err := os.Stat("/Applications/Safari.app"); err == nil {
			browsers = append(browsers, "webkit")
		}
	}

	if len(browsers) == 0 {
		browsers = []string{"chromium"}
		log.Println("WARN: No browsers detected, defaulting to chromium (Playwright will install)")
	}

	caps["browsers"] = browsers
	return caps
}

func (a *Agent) getPlatformVersion() string {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sw_vers", "-productVersion").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	case "linux":
		out, err := exec.Command("uname", "-r").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	case "windows":
		out, err := exec.Command("cmd", "/c", "ver").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

func pathExists(paths []string) bool {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// --- HTTP helpers ---

// doJSON sends a JSON request through the httpx egress chokepoint (so it is
// recorded in the active Exposure Receipt) and returns the status code and body.
// ctx routes the receipt — daemon callers pass the per-run context from
// receipt.Begin so concurrent assignments record into their own receipts.
func (a *Agent) doJSON(ctx context.Context, method, url string, body interface{}, headers map[string]string) (int, []byte, error) {
	return httpx.DoJSON(ctx, method, url, body, headers, 60*time.Second)
}

func (a *Agent) authHeaders() map[string]string {
	return map[string]string{
		"X-Agent-API-Key": a.APIKey,
	}
}

// --- Registration ---

// Register registers the agent with the cloud server.
func (a *Agent) Register(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/agent/register", a.CloudURL)
	payload := map[string]interface{}{
		"name":                fmt.Sprintf("%s (%s)", AgentName, a.MachineID),
		"machine_id":          a.MachineID,
		"capabilities":        a.Capabilities,
		"version":             Version,
		"api_key":             a.APIKey,
		"registration_secret": a.RegistrationSecret,
	}

	status, body, err := a.doJSON(ctx, "POST", url, payload, nil)
	if err != nil {
		return fmt.Errorf("registration request failed: %w", err)
	}

	if status != http.StatusOK {
		return fmt.Errorf("registration failed: %d - %s", status, string(body))
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("parse registration response: %w", err)
	}

	if id, ok := data["agent_id"].(string); ok {
		a.AgentID = id
	}
	if key, ok := data["api_key"].(string); ok {
		a.APIKey = key
	}

	log.Printf("Agent registered successfully: %s", a.AgentID)

	if a.OnRegistered != nil {
		a.OnRegistered(a.AgentID, a.APIKey)
	}

	return nil
}

// --- Heartbeat ---

// SendHeartbeat sends a heartbeat to the cloud server.
func (a *Agent) SendHeartbeat(ctx context.Context) error {
	if a.AgentID == "" || a.APIKey == "" {
		return fmt.Errorf("agent not registered")
	}

	url := fmt.Sprintf("%s/api/agent/%s/heartbeat", a.CloudURL, a.AgentID)

	var activeIDs []string
	a.activeTests.Range(func(key, _ interface{}) bool {
		activeIDs = append(activeIDs, key.(string))
		return true
	})

	status := "online"
	if len(activeIDs) > 0 {
		status = "busy"
	}

	payload := map[string]interface{}{
		"status":       status,
		"active_tests": activeIDs,
	}

	metrics := sysmetrics.Collect(len(activeIDs))
	if metrics != nil {
		payload["system_metrics"] = metrics
	}

	code, _, err := a.doJSON(ctx, "POST", url, payload, a.authHeaders())
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("heartbeat failed: %d", code)
	}
	return nil
}

// --- Poll assignments ---

// Assignment represents a test assignment from the server.
type Assignment struct {
	ID             json.Number `json:"id"`
	ScriptID       json.Number `json:"script_id"`
	Code           string      `json:"code"`
	Framework      string      `json:"framework"`
	CustomURL      string      `json:"custom_url"`
	ExecutionID    json.Number `json:"execution_id"`
	Headless       bool        `json:"headless"`
	Browser        string      `json:"browser"`
	ViewportWidth  int         `json:"viewport_width"`
	ViewportHeight int         `json:"viewport_height"`
}

// PollAssignments fetches pending assignments from the server.
func (a *Agent) PollAssignments(ctx context.Context) ([]Assignment, error) {
	if a.AgentID == "" || a.APIKey == "" {
		return nil, nil
	}

	url := fmt.Sprintf("%s/api/agent/%s/assignments/pending", a.CloudURL, a.AgentID)
	status, body, err := a.doJSON(ctx, "GET", url, nil, a.authHeaders())
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("poll assignments failed: %d", status)
	}

	var data struct {
		Assignments []Assignment `json:"assignments"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse assignments: %w", err)
	}

	return data.Assignments, nil
}

// --- Test execution ---

// ExecuteTest runs a single test assignment.
func (a *Agent) ExecuteTest(ctx context.Context, assignment Assignment) {
	assignmentID := assignment.ID.String()
	scriptID := assignment.ScriptID.String()
	if assignmentID == "" {
		log.Printf("ERROR: Assignment has empty ID, skipping")
		return
	}
	// Per-assignment Exposure Receipt: concurrent assignments each get their own,
	// routed via ctx so their egress never interleaves in one manifest.
	ctx, rec := receipt.Begin(ctx, "daemon:assignment")
	defer func() {
		if path, err := rec.Finalize(); err != nil {
			log.Printf("WARN: failed to finalize exposure receipt for assignment %s: %v", assignmentID, err)
		} else {
			log.Printf("Exposure receipt written: %s", path)
		}
	}()
	// Clean up active test tracking on all exit paths
	defer func() {
		a.activeTests.Delete(assignmentID)
		a.mu.Lock()
		a.activeCount--
		a.mu.Unlock()
	}()
	testCode := assignment.Code
	browser := assignment.Browser
	if browser == "" {
		browser = "chromium"
	}
	headless := assignment.Headless
	vpWidth := assignment.ViewportWidth
	if vpWidth == 0 {
		vpWidth = 1280
	}
	vpHeight := assignment.ViewportHeight
	if vpHeight == 0 {
		vpHeight = 720
	}

	log.Printf("Assignment %s parameters: browser=%s, headless=%v, viewport=%dx%d",
		assignmentID, browser, headless, vpWidth, vpHeight)

	if testCode == "" && scriptID != "" && scriptID != "<nil>" {
		code, err := a.fetchScriptCode(ctx, scriptID)
		if err != nil {
			log.Printf("WARN: Failed to fetch script code: %v", err)
		} else {
			testCode = code
		}
	}

	if testCode == "" {
		log.Printf("ERROR: Assignment %s has no test code", assignmentID)
		a.reportResult(ctx, assignmentID, false, "No test code provided", nil)
		return
	}

	log.Printf("Executing test assignment %s (script %s)", assignmentID, scriptID)

	testDir, err := os.MkdirTemp("", fmt.Sprintf("qmax-%s-", assignmentID))
	if err != nil {
		a.reportResult(ctx, assignmentID, false, fmt.Sprintf("Failed to create temp dir: %v", err), nil)
		return
	}
	defer os.RemoveAll(testDir)

	a.updateAssignmentStatus(ctx, assignmentID, "started")

	testFile := filepath.Join(testDir, "test.spec.js")
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		a.reportResult(ctx, assignmentID, false, fmt.Sprintf("Failed to write test file: %v", err), nil)
		return
	}

	packageJSON := map[string]interface{}{
		"name":    fmt.Sprintf("qamax-test-%s", assignmentID),
		"version": "1.0.0",
		"scripts": map[string]string{
			"test": "playwright test",
		},
		"dependencies": map[string]string{
			"@playwright/test": "^1.51.0",
		},
	}
	pkgData, _ := json.MarshalIndent(packageJSON, "", "  ")
	if err := os.WriteFile(filepath.Join(testDir, "package.json"), pkgData, 0644); err != nil {
		a.reportResult(ctx, assignmentID, false, fmt.Sprintf("Failed to write package.json: %v", err), nil)
		return
	}

	browserMap := map[string][2]string{
		"chromium": {"chromium", "Desktop Chrome"},
		"firefox":  {"firefox", "Desktop Firefox"},
		"webkit":   {"webkit", "Desktop Safari"},
	}
	mapping, ok := browserMap[browser]
	if !ok {
		log.Printf("WARN: Unknown browser %q, falling back to chromium", browser)
		mapping = browserMap["chromium"]
	}
	playwrightBrowser := mapping[0]
	deviceName := mapping[1]

	log.Printf("Using browser: %s, device: %s, headless: %v", playwrightBrowser, deviceName, headless)

	baseURLJS := "undefined"
	if assignment.CustomURL != "" {
		b, _ := json.Marshal(assignment.CustomURL)
		baseURLJS = string(b)
	}

	config := fmt.Sprintf(`// @ts-check
const { defineConfig, devices } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: 'json',
  projects: [
    {
      name: '%s',
      use: {
        ...devices['%s'],
        baseURL: %s,
        headless: %v,
        viewport: { width: %d, height: %d },
        screenshot: 'on',
        video: 'on',
      },
    },
  ],
});
`, playwrightBrowser, deviceName, baseURLJS, headless, vpWidth, vpHeight)

	configPath := filepath.Join(testDir, "playwright.config.js")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		a.reportResult(ctx, assignmentID, false, fmt.Sprintf("Failed to write config: %v", err), nil)
		return
	}

	log.Printf("Created Playwright config with headless=%v, browser=%s", headless, playwrightBrowser)

	log.Printf("Installing dependencies for assignment %s", assignmentID)
	if err := a.runCommand(ctx, testDir, "npm", "install", 300*time.Second); err != nil {
		a.reportResult(ctx, assignmentID, false, fmt.Sprintf("Dependency installation failed: %v", err), nil)
		return
	}

	log.Printf("Installing Playwright browser '%s' for assignment %s", playwrightBrowser, assignmentID)
	if err := a.runCommand(ctx, testDir, "npx", fmt.Sprintf("playwright install %s", playwrightBrowser), 300*time.Second); err != nil {
		log.Printf("WARN: Browser installation had issues, but continuing: %v", err)
	}

	log.Printf("Running test for assignment %s with browser project: %s", assignmentID, playwrightBrowser)

	var stdout, stderr bytes.Buffer
	testCtx, testCancel := context.WithTimeout(ctx, 600*time.Second)
	defer testCancel()
	cmd := exec.CommandContext(testCtx, "npx", "playwright", "test", "--project", playwrightBrowser)
	cmd.Dir = testDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	if stdout.Len() > 0 {
		preview := stdout.String()
		if len(preview) > 500 {
			preview = preview[:500]
		}
		log.Printf("Test stdout (first 500 chars): %s", preview)
	}
	if stderr.Len() > 0 {
		preview := stderr.String()
		if len(preview) > 500 {
			preview = preview[:500]
		}
		log.Printf("Test stderr (first 500 chars): %s", preview)
	}

	artifacts := a.collectArtifacts(testDir)

	success := runErr == nil
	resultData := map[string]interface{}{
		"success":   success,
		"output":    stdout.String(),
		"errors":    stderr.String(),
		"artifacts": artifacts,
	}

	a.reportResult(ctx, assignmentID, success, stdout.String(), resultData)
}

func (a *Agent) runCommand(ctx context.Context, dir, name, argsStr string, timeout time.Duration) error {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := strings.Fields(argsStr)
	cmd := exec.CommandContext(cmdCtx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(out))
	}
	return nil
}

func (a *Agent) fetchScriptCode(ctx context.Context, scriptID string) (string, error) {
	url := fmt.Sprintf("%s/api/automation/scripts/%s", a.CloudURL, scriptID)
	status, body, err := a.doJSON(ctx, "GET", url, nil, a.authHeaders())
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("fetch script failed: %d", status)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}
	code, _ := data["code"].(string)
	return code, nil
}

// --- Artifact collection ---

func (a *Agent) collectArtifacts(testDir string) map[string]interface{} {
	artifacts := map[string]interface{}{
		"screenshots": []map[string]string{},
		"video":       nil,
	}

	testResults := filepath.Join(testDir, "test-results")
	if _, err := os.Stat(testResults); os.IsNotExist(err) {
		return artifacts
	}

	var screenshots []map[string]string

	_ = filepath.Walk(testResults, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".png") {
			data, err := os.ReadFile(path)
			if err != nil {
				log.Printf("WARN: Failed to read screenshot %s: %v", path, err)
				return nil
			}
			screenshots = append(screenshots, map[string]string{
				"filename": info.Name(),
				"data":     base64.StdEncoding.EncodeToString(data),
			})
		}

		if strings.HasSuffix(info.Name(), ".webm") && artifacts["video"] == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				log.Printf("WARN: Failed to read video %s: %v", path, err)
				return nil
			}
			artifacts["video"] = map[string]string{
				"filename": info.Name(),
				"data":     base64.StdEncoding.EncodeToString(data),
			}
		}

		return nil
	})

	if screenshots != nil {
		artifacts["screenshots"] = screenshots
	}

	return artifacts
}

// --- Status/result reporting ---

func (a *Agent) updateAssignmentStatus(ctx context.Context, assignmentID, status string) {
	if a.AgentID == "" || a.APIKey == "" {
		return
	}

	url := fmt.Sprintf("%s/api/agent/%s/assignments/%s/status", a.CloudURL, a.AgentID, assignmentID)
	payload := map[string]string{"status": status}

	respStatus, _, err := a.doJSON(ctx, "POST", url, payload, a.authHeaders())
	if err != nil {
		log.Printf("ERROR: updating assignment status: %v", err)
		return
	}
	if respStatus != http.StatusOK {
		log.Printf("ERROR: assignment status update failed: %d", respStatus)
	}
}

func (a *Agent) reportResult(ctx context.Context, assignmentID string, success bool, message string, resultData map[string]interface{}) {
	if a.AgentID == "" || a.APIKey == "" {
		return
	}

	url := fmt.Sprintf("%s/api/agent/%s/assignments/%s/result", a.CloudURL, a.AgentID, assignmentID)

	output := ""
	errors := ""
	var artifacts interface{}
	if resultData != nil {
		if v, ok := resultData["output"].(string); ok {
			output = v
		}
		if v, ok := resultData["errors"].(string); ok {
			errors = v
		}
		artifacts = resultData["artifacts"]
	}

	payload := map[string]interface{}{
		"success":   success,
		"message":   message,
		"output":    output,
		"errors":    errors,
		"artifacts": artifacts,
	}

	respStatus, body, err := a.doJSON(ctx, "POST", url, payload, a.authHeaders())
	if err != nil {
		log.Printf("ERROR: reporting result: %v", err)
		return
	}

	if respStatus == http.StatusOK {
		finalStatus := "completed"
		if !success {
			finalStatus = "failed"
		}
		a.updateAssignmentStatus(ctx, assignmentID, finalStatus)
		log.Printf("Result reported for assignment %s: %s", assignmentID, finalStatus)
	} else {
		log.Printf("ERROR: failed to report result: %d - %s", respStatus, string(body))
	}
}

// --- Main loop ---

// Run starts the agent's main loop: register, heartbeat, and poll for assignments.
func (a *Agent) Run(ctx context.Context) error {
	log.Printf("Starting %s v%s", AgentName, Version)
	log.Printf("Machine ID: %s", a.MachineID)
	capsJSON, _ := json.MarshalIndent(a.Capabilities, "", "  ")
	log.Printf("Capabilities: %s", string(capsJSON))

	// Control-plane Exposure Receipt for the daemon process: register, heartbeat
	// and polls record here. Per-assignment/crawl work opens its own receipt via
	// receipt.Begin, which takes precedence through context.
	controlRec := receipt.NewCurrent("daemon:control")
	defer func() {
		if path, err := controlRec.Finalize(); err != nil {
			log.Printf("WARN: failed to finalize daemon control receipt: %v", err)
		} else {
			log.Printf("Daemon control exposure receipt written: %s", path)
		}
	}()

	if err := a.Register(ctx); err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	a.mu.Lock()
	a.running = true
	a.mu.Unlock()

	go a.heartbeatLoop(ctx)

	ticker := time.NewTicker(a.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down agent...")
			a.mu.Lock()
			a.running = false
			a.mu.Unlock()
			a.waitForActiveTests()
			return nil
		case <-ticker.C:
			assignments, err := a.PollAssignments(ctx)
			if err != nil {
				if os.IsTimeout(err) || strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "Timeout") {
					log.Printf("WARN: polling timed out, will retry next cycle")
				} else {
					log.Printf("ERROR: polling assignments: %v", err)
				}
				continue
			}

			for _, assignment := range assignments {
				id := assignment.ID.String()
				if _, exists := a.activeTests.Load(id); exists {
					continue
				}

				a.activeTests.Store(id, true)
				a.mu.Lock()
				a.activeCount++
				a.mu.Unlock()

				go a.ExecuteTest(ctx, assignment)
			}

			// Poll for crawl sessions
			crawlSession, crawlErr := a.PollCrawlSessions(ctx)
			if crawlErr != nil {
				log.Printf("WARN: polling crawl sessions: %v", crawlErr)
			} else if crawlSession != nil {
				go a.ExecuteCrawlSession(ctx, *crawlSession)
			}
		}
	}
}

func (a *Agent) waitForActiveTests() {
	a.mu.Lock()
	count := a.activeCount
	a.mu.Unlock()

	if count > 0 {
		log.Printf("Waiting for %d active tests to complete...", count)
		for {
			a.mu.Lock()
			count = a.activeCount
			a.mu.Unlock()
			if count == 0 {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	consecutiveFailures := 0
	maxBackoff := 300 * time.Second

	for {
		var wait time.Duration
		if consecutiveFailures > 0 {
			backoff := a.HeartbeatInterval * time.Duration(1<<uint(consecutiveFailures))
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			log.Printf("WARN: Heartbeat backoff: waiting %v after %d failures", backoff, consecutiveFailures)
			wait = backoff
		} else {
			wait = a.HeartbeatInterval
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		a.mu.Lock()
		running := a.running
		a.mu.Unlock()
		if !running {
			return
		}

		if err := a.SendHeartbeat(ctx); err != nil {
			consecutiveFailures++
			if consecutiveFailures >= 5 {
				log.Printf("ERROR: Heartbeat failed %d times consecutively", consecutiveFailures)
			}
		} else {
			consecutiveFailures = 0
		}
	}
}
