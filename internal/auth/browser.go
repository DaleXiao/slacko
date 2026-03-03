package auth

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Supported browsers
var supportedBrowsers = []string{"chrome", "edge", "brave", "firefox", "safari"}

// ImportFromBrowser extracts the d cookie from a browser's cookie store.
// Supported browsers: chrome, edge, brave, firefox, safari.
// Uses sweetcookie CLI if available, otherwise provides manual instructions.
func ImportFromBrowser(browser, browserProfile string) (string, error) {
	browser = strings.ToLower(browser)

	valid := false
	for _, b := range supportedBrowsers {
		if browser == b {
			valid = true
			break
		}
	}
	if !valid {
		return "", fmt.Errorf("unsupported browser %q. Supported: %s", browser, strings.Join(supportedBrowsers, ", "))
	}

	// Try sweetcookie first (supports all browsers on macOS/Linux/Windows)
	if cookie, err := trySweetcookie(browser, browserProfile); err == nil {
		return cookie, nil
	}

	// Fallback: platform-specific manual instructions
	switch runtime.GOOS {
	case "darwin":
		return "", manualInstructions(browser)
	case "linux":
		return "", manualInstructions(browser)
	case "windows":
		return "", manualInstructions(browser)
	default:
		return "", fmt.Errorf("browser cookie import not supported on %s. Use 'slacko auth manual' instead", runtime.GOOS)
	}
}

func trySweetcookie(browser, profile string) (string, error) {
	path, err := exec.LookPath("sweetcookie")
	if err != nil {
		return "", err
	}

	args := []string{"get", "--browser", browser, "--domain", ".slack.com", "--name", "d"}
	if profile != "" {
		args = append(args, "--browser-profile", profile)
	}

	out, err := exec.Command(path, args...).Output()
	if err != nil {
		return "", fmt.Errorf("sweetcookie failed: %w", err)
	}

	cookie := strings.TrimSpace(string(out))
	if cookie == "" {
		return "", fmt.Errorf("no d cookie found for .slack.com in %s", browser)
	}
	return cookie, nil
}

func manualInstructions(browser string) error {
	browserName := map[string]string{
		"chrome":  "Chrome",
		"edge":    "Edge",
		"brave":   "Brave",
		"firefox": "Firefox",
		"safari":  "Safari",
	}[browser]

	return fmt.Errorf(`cookie import requires sweetcookie CLI.

Install:  go install github.com/steipete/sweetcookie/cmd/sweetcookie@latest

Or extract manually:
1. Open %s → app.slack.com → F12 → Application → Cookies
2. Find the 'd' cookie for .slack.com domain
3. Copy its value
4. Run: slacko auth manual --token xoxc-YOUR-TOKEN --cookie "YOUR-D-COOKIE"

For the xoxc- token:
  F12 → Network → filter 'api/' → check any request form data for 'token'`, browserName)
}
