package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/getlantern/systray"
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
	go updateStatus(mStatus)

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
				runCommand("colima", "start")
			case <-mStop.ClickedCh:
				runCommand("colima", "stop")
			case <-mRestart.ClickedCh:
				runCommand("colima", "restart")
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

func onExit() {
	// Cleanup if needed
}

func updateStatus(m *systray.MenuItem) {
	for {
		out, err := exec.Command("colima", "status").Output()
		status := "Status: Unknown"
		if err == nil {
			status = fmt.Sprintf("Status: %s", parseStatus(string(out)))
		}
		m.SetTitle(status)
		time.Sleep(5 * time.Second)
	}
}

func parseStatus(output string) string {
	// Simple example, could be smarter
	if output == "" {
		return "Stopped"
	}
	if contains(output, "Running") {
		return "Running"
	}
	return "Stopped"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s[0:len(substr)] == substr)
}

func runCommand(name string, arg ...string) {
	cmd := exec.Command(name, arg...)
	cmd.Run() // fire and forget, or capture output
}
