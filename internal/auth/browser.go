package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/steipete/sweetcookie"
)

// Supported browsers
var supportedBrowsers = map[string]sweetcookie.Browser{
	"chrome":  sweetcookie.BrowserChrome,
	"edge":    sweetcookie.BrowserEdge,
	"brave":   sweetcookie.BrowserBrave,
	"firefox": sweetcookie.BrowserFirefox,
	"safari":  sweetcookie.BrowserSafari,
}

// ImportFromBrowser extracts the d cookie and xoxc- token from a browser's cookie store.
// Returns (cookie, token, workspaces found, error).
func ImportFromBrowser(browser, browserProfile string) ([]ImportResult, error) {
	browser = strings.ToLower(browser)

	scBrowser, ok := supportedBrowsers[browser]
	if !ok {
		names := make([]string, 0, len(supportedBrowsers))
		for k := range supportedBrowsers {
			names = append(names, k)
		}
		return nil, fmt.Errorf("unsupported browser %q. Supported: %s", browser, strings.Join(names, ", "))
	}

	opts := sweetcookie.Options{
		URL:      "https://slack.com/",
		Names:    []string{"d"},
		Browsers: []sweetcookie.Browser{scBrowser},
		Mode:     sweetcookie.ModeFirst,
	}

	res, err := sweetcookie.Get(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("failed to read cookies from %s: %w", browser, err)
	}

	if len(res.Cookies) == 0 {
		return nil, fmt.Errorf("no 'd' cookie found for .slack.com in %s. Make sure you're logged into Slack in that browser", browser)
	}

	cookie := res.Cookies[0].Value

	// Discover workspaces and extract tokens
	results, err := discoverWorkspaces(cookie)
	if err != nil {
		// Return cookie even if token extraction fails — user can still use manual token entry
		return []ImportResult{{Cookie: cookie, Error: fmt.Sprintf("cookie found but token extraction failed: %v", err)}}, nil
	}

	return results, nil
}

// ImportResult holds the result of importing credentials for one workspace
type ImportResult struct {
	Cookie    string
	Token     string
	Workspace string
	TeamName  string
	Error     string
}

var (
	tokenRegex     = regexp.MustCompile(`"api_token"\s*:\s*"(xoxc-[^"]+)"`)
	teamDomainRegex = regexp.MustCompile(`"team_url"\s*:\s*"https://([^.]+)\.slack\.com/"`)
	teamNameRegex  = regexp.MustCompile(`"name"\s*:\s*"([^"]+)"`)
)

// discoverWorkspaces fetches the Slack page to find all workspaces and their tokens
func discoverWorkspaces(cookie string) ([]ImportResult, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		// Don't follow redirects — we want to read the boot data page
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			// Carry the cookie through redirects
			req.Header.Set("Cookie", "d="+cookie)
			return nil
		},
	}

	// First try the main slack.com page which may list workspaces
	req, err := http.NewRequest("GET", "https://app.slack.com/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "d="+cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	html := string(body)

	// Extract token from boot data
	tokenMatch := tokenRegex.FindStringSubmatch(html)
	if tokenMatch == nil {
		return nil, fmt.Errorf("could not find xoxc- token in page. The cookie may be invalid or expired")
	}

	token := tokenMatch[1]

	// Extract workspace domain
	workspace := ""
	domainMatch := teamDomainRegex.FindStringSubmatch(html)
	if domainMatch != nil {
		workspace = domainMatch[1]
	}

	// Extract team name
	teamName := ""
	nameMatch := teamNameRegex.FindStringSubmatch(html)
	if nameMatch != nil {
		teamName = nameMatch[1]
	}

	return []ImportResult{{
		Cookie:    cookie,
		Token:     token,
		Workspace: workspace,
		TeamName:  teamName,
	}}, nil
}
