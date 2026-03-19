//go:build windows

package chatgpt

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func platformChromeCandidates() []string {
	var candidates []string

	for _, envVar := range []string{"PROGRAMFILES", "PROGRAMFILES(X86)", "LOCALAPPDATA"} {
		root := os.Getenv(envVar)
		if root == "" {
			continue
		}
		candidates = append(candidates,
			filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(root, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		)
	}

	return candidates
}

func launchDetachedChrome(chromePath string, args []string) error {
	cmd := exec.Command(chromePath, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008, // DETACHED_PROCESS
	}
	return cmd.Start()
}

// profileDirForBrowser returns the profile directory as-is on native Windows.
func profileDirForBrowser(_ string, profileDir string) (string, error) {
	return profileDir, nil
}

// isWSLOrWindowsExe reports whether the chrome path looks like a Windows
// executable launched through WSL. Always false on native Windows.
func isWSLOrWindowsExe(_ string) bool {
	return false
}
