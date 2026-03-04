package auth

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// ExtractTokenFromBrowser uses AppleScript (macOS) or PowerShell (Windows)
// to execute JS in the active Slack browser tab. No CDP, no extra HTTP
// requests — reads directly from the already-loaded page.
func ExtractTokenFromBrowser(browser string) (token string, err error) {
	switch runtime.GOOS {
	case "darwin":
		return extractTokenMacOS(browser)
	case "windows":
		return extractTokenWindows(browser)
	default:
		return "", fmt.Errorf("automatic token extraction not supported on %s. Use auth manual instead", runtime.GOOS)
	}
}

func extractTokenMacOS(browser string) (string, error) {
	appName := ""
	switch strings.ToLower(browser) {
	case "edge":
		appName = "Microsoft Edge"
	case "chrome":
		appName = "Google Chrome"
	case "brave":
		appName = "Brave Browser"
	default:
		return "", fmt.Errorf("AppleScript token extraction supports: chrome, edge, brave (got %q)", browser)
	}

	// Priority 1: boot_data.api_token from script tags (workspace-level token)
	// Priority 2: localStorage fallback (may be enterprise-level, has API limitations)
	//
	// Using single-line JS to avoid AppleScript multi-line escaping issues.
	// The JS is passed as a separate -e argument to avoid quote conflicts.
	js := `(function(){` +
		`var s=document.querySelectorAll('script');` +
		`for(var i=0;i<s.length;i++){` +
		`var m=s[i].textContent.match(/\"api_token\"\\s*:\\s*\"(xoxc-[^\"]+)\"/);` +
		`if(m)return m[1]}` +
		`if(typeof boot_data!=='undefined'&&boot_data.api_token)return boot_data.api_token;` +
		`if(typeof TS!=='undefined'&&TS.boot_data&&TS.boot_data.api_token)return TS.boot_data.api_token;` +
		`return 'NOT_FOUND'})()`

	// Build AppleScript that iterates tabs to find Slack
	script := fmt.Sprintf(`tell application "%s"
	set tokenResult to "NOT_FOUND"
	repeat with w in every window
		repeat with t in every tab of w
			if URL of t contains ".slack.com" then
				set tokenResult to (execute t javascript "%s")
				if tokenResult starts with "xoxc-" then
					return tokenResult
				end if
			end if
		end repeat
	end repeat
	return tokenResult
end tell`, appName, strings.ReplaceAll(js, `"`, `\"`))

	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := strings.TrimSpace(string(out))
		return "", fmt.Errorf("AppleScript failed: %s (%w). Make sure %s is running with a Slack tab open", errMsg, err, appName)
	}

	result := strings.TrimSpace(string(out))
	if !strings.HasPrefix(result, "xoxc-") {
		return "", fmt.Errorf("no xoxc- token found in %s Slack tabs. Make sure your Slack workspace is fully loaded", appName)
	}

	return result, nil
}

func extractTokenWindows(browser string) (string, error) {
	var exeName string
	switch strings.ToLower(browser) {
	case "edge":
		exeName = "msedge"
	case "chrome":
		exeName = "chrome"
	default:
		return "", fmt.Errorf("PowerShell token extraction supports: chrome, edge (got %q)", browser)
	}

	// JS to extract token — prioritize boot_data script tags over localStorage
	js := `(function(){` +
		`var s=document.querySelectorAll('script');` +
		`for(var i=0;i<s.length;i++){` +
		`var m=s[i].textContent.match(/\"api_token\"\\s*:\\s*\"(xoxc-[^\"]+)\"/);` +
		`if(m)return m[1]}` +
		`if(typeof boot_data!=='undefined'&&boot_data.api_token)return boot_data.api_token;` +
		`if(typeof TS!=='undefined'&&TS.boot_data&&TS.boot_data.api_token)return TS.boot_data.api_token;` +
		`return 'NOT_FOUND'})()`

	// PowerShell: activate browser, open console, execute JS, read clipboard
	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$proc = Get-Process -Name "%s" -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowTitle -ne "" } | Select-Object -First 1
if (-not $proc) { Write-Output "BROWSER_NOT_FOUND"; exit }
$sig = '[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);'
$type = Add-Type -MemberDefinition $sig -Name Win32 -Namespace Native -PassThru
$type::SetForegroundWindow($proc.MainWindowHandle) | Out-Null
Start-Sleep -Milliseconds 500
[System.Windows.Forms.SendKeys]::SendWait("^+j")
Start-Sleep -Milliseconds 1000
$escaped = '%s' -replace '[+^%%~(){}]', '{$0}'
[System.Windows.Forms.SendKeys]::SendWait("copy($escaped){ENTER}")
Start-Sleep -Milliseconds 500
[System.Windows.Forms.SendKeys]::SendWait("{F12}")
Start-Sleep -Milliseconds 300
$result = [System.Windows.Forms.Clipboard]::GetText()
Write-Output $result
`, exeName, strings.ReplaceAll(js, "'", "''"))

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := strings.TrimSpace(string(out))
		return "", fmt.Errorf("PowerShell failed: %s (%w). Make sure %s is running with a Slack tab open", errMsg, err, exeName)
	}

	result := strings.TrimSpace(string(out))
	if result == "BROWSER_NOT_FOUND" {
		return "", fmt.Errorf("browser %s not found running. Open Slack in %s first", exeName, exeName)
	}
	if !strings.HasPrefix(result, "xoxc-") {
		return "", fmt.Errorf("no xoxc- token found. Make sure the active tab in %s is your Slack workspace", exeName)
	}

	return result, nil
}
