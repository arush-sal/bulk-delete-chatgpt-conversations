//go:build !windows

package chatgpt

import (
	"os/exec"
	goruntime "runtime"
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
	}
	return candidates
}

func launchDetachedChrome(chromePath string, args []string) error {
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

func profileDirForBrowser(chromePath string, profileDir string) (string, error) {
	return profileDir, nil
}
