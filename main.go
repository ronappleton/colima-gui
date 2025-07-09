package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var iconData []byte

func main() {
	var err error
	iconData, err = os.ReadFile("images/icon.ico")
	if err != nil {
		log.Fatalf("failed to load icon: %v", err)
	}
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("Colima")
	systray.SetTooltip("Colima Tray Manager")

	mStatus := systray.AddMenuItem("Status: Checking...", "")

	systray.AddSeparator()

	mStart := systray.AddMenuItem("Start Colima", "")
	mStop := systray.AddMenuItem("Stop Colima", "")
	mRestart := systray.AddMenuItem("Restart Colima", "")

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "")

	go func() {
		for {
			select {
			case <-mStart.ClickedCh:
				runColimaCmd(mStatus, mStart, mStop, mRestart, "starting...", "start")
			case <-mStop.ClickedCh:
				runColimaCmd(mStatus, mStart, mStop, mRestart, "stopping...", "stop")
			case <-mRestart.ClickedCh:
				runColimaCmd(mStatus, mStart, mStop, mRestart, "restarting...", "restart")
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	go updateStatus(mStatus, mStart, mStop)
}

func onExit() {
	// Cleanup if needed
}

func updateStatus(mStatus, mStart, mStop *systray.MenuItem) {
	for {
		refreshStatus(mStatus, mStart, mStop)

		time.Sleep(5 * time.Second)
	}
}

func refreshStatus(mStatus, mStart, mStop *systray.MenuItem) {
	status, _ := getColimaStatus()
	mStatus.SetTitle(fmt.Sprintf("Status: %s", status))

	switch status {
	case "Running":
		mStart.Hide()
		mStart.Enable()
		mStop.Show()
		mStop.Enable()
	case "Stopped":
		mStop.Hide()
		mStop.Enable()
		mStart.Show()
		mStart.Enable()
	default:
		mStart.Show()
		mStart.Enable()
		mStop.Show()
		mStop.Enable()
	}
}

func runColimaCmd(mStatus, mStart, mStop, mRestart *systray.MenuItem, msg, action string) {
	mStatus.SetTitle(fmt.Sprintf("Status: %s", cases.Title(language.English).String(msg)))
	mStart.Disable()
	mStop.Disable()
	mRestart.Disable()

	go func() {
		exec.Command("colima", action).Run()
		refreshStatus(mStatus, mStart, mStop)
		mRestart.Enable()
	}()
}

func getColimaStatus() (string, error) {
	out, err := exec.Command("colima", "status").CombinedOutput()
	if err != nil {
		return "Unknown", err
	}
	return parseStatus(string(out)), nil
}

func parseStatus(output string) string {
	out := strings.ToLower(strings.TrimSpace(output))
	switch {
	case strings.Contains(out, "running"):
		return "Running"
	case strings.Contains(out, "stopped"):
		return "Stopped"
	}
	if out == "" {
		return "Unknown"
	}
	return strings.TrimSpace(output)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s[0:len(substr)] == substr)
}

func runCommand(name string, arg ...string) {
	cmd := exec.Command(name, arg...)
	cmd.Run()
}
