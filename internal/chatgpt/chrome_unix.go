//go:build !windows

package chatgpt

import (
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"syscall"
)

func platformChromeCandidates() []string {
	candidates := []string{
		// Linux
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/snap/bin/chromium",
		// macOS
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		// Windows via WSL
		"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
		"/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
		"/mnt/c/Program Files/Microsoft/Edge/Application/msedge.exe",
	}
	return candidates
}

func launchDetachedChrome(chromePath string, args []string) error {
	if isWSLOrWindowsExe(chromePath) {
		windowsChromePath, err := wslToWindowsPath(chromePath)
		if err != nil {
			return err
		}

		cmdArgs := []string{"/C", "start", "", windowsChromePath}
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command("cmd.exe", cmdArgs...)
		return cmd.Start()
	}

	if goruntime.GOOS == "darwin" {
		if appBundle, ok := macAppBundlePath(chromePath); ok {
			cmdArgs := []string{"-na", appBundle, "--args"}
			cmdArgs = append(cmdArgs, args...)
			cmd := exec.Command("open", cmdArgs...)
			cmd.Stdin = nil
			cmd.Stdout = nil
			cmd.Stderr = nil
			return cmd.Start()
		}
	}

	cmd := exec.Command(chromePath, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd.Start()
}

// profileDirForBrowser converts the profile directory to a Windows path when
// running under WSL and launching a Windows browser executable.
func profileDirForBrowser(chromePath string, profileDir string) (string, error) {
	if isWSLOrWindowsExe(chromePath) {
		return wslToWindowsPath(profileDir)
	}
	return profileDir, nil
}

// isWSLOrWindowsExe reports whether the chrome path looks like a Windows
// executable launched through WSL.
func isWSLOrWindowsExe(chromePath string) bool {
	return strings.HasSuffix(strings.ToLower(chromePath), ".exe") || strings.HasPrefix(chromePath, "/mnt/")
}

func wslToWindowsPath(path string) (string, error) {
	out, err := exec.Command("wslpath", "-w", filepath.Clean(path)).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

