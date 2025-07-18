package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var iconData []byte

type Container struct {
	Name    string
	Project string
	Status  string
}

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
	systray.SetTooltip("Colima Tray Manager")

	mStatus := systray.AddMenuItem("Status: Checking...", "")

	systray.AddSeparator()

	mStart := systray.AddMenuItem("Start Colima", "")
	mStop := systray.AddMenuItem("Stop Colima", "")
	mRestart := systray.AddMenuItem("Restart Colima", "")

	systray.AddSeparator()
	projectsMenu := systray.AddMenuItem("Projects", "")
	populateProjectsMenu(projectsMenu)

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

func parseContainerStatus(output string) string {
	out := strings.ToLower(strings.TrimSpace(output))
	switch {
	case strings.HasPrefix(out, "up") || strings.Contains(out, "running"):
		return "Running"
	case strings.HasPrefix(out, "exited") || strings.Contains(out, "created") || strings.Contains(out, "stopped"):
		return "Stopped"
	}
	if out == "" {
		return "Unknown"
	}
	return strings.TrimSpace(output)
}

func getContainersByProject() (map[string][]Container, error) {
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}|{{.Label \"com.docker.compose.project\"}}|{{.Status}}").CombinedOutput()
	if err != nil {
		return nil, err
	}
	projects := make(map[string][]Container)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 1 {
			continue
		}
		name := parts[0]
		project := "default"
		if len(parts) > 1 && parts[1] != "" {
			project = parts[1]
		}
		status := ""
		if len(parts) > 2 {
			status = parts[2]
		}
		projects[project] = append(projects[project], Container{Name: name, Project: project, Status: status})
	}

	return projects, nil
}

func populateProjectsMenu(m *systray.MenuItem) {
	projects, err := getContainersByProject()
	if err != nil {
		return
	}

	projectNames := make([]string, 0, len(projects))
	for name := range projects {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	for _, proj := range projectNames {
		containers := projects[proj]
		projItem := m.AddSubMenuItem(proj, "")

		startAll := projItem.AddSubMenuItem("Start All", "")
		stopAll := projItem.AddSubMenuItem("Stop All", "")
		restartAll := projItem.AddSubMenuItem("Restart All", "")
		projItem.AddSubMenuItem("", "").Disable()

		updateProjectItems := func() {
			anyRunning := false
			anyStopped := false
			for _, c := range containers {
				status := parseContainerStatus(c.Status)
				if status == "Running" {
					anyRunning = true
				}
				if status == "Stopped" {
					anyStopped = true
				}
			}
			if anyRunning {
				stopAll.Show()
			} else {
				stopAll.Hide()
			}
			if anyStopped {
				startAll.Show()
			} else {
				startAll.Hide()
			}
		}

		for i := range containers {
			c := &containers[i]
			status := parseContainerStatus(c.Status)
			containerItem := projItem.AddSubMenuItem(c.Name, status)
			startItem := containerItem.AddSubMenuItem("Start", "")
			stopItem := containerItem.AddSubMenuItem("Stop", "")
			restartItem := containerItem.AddSubMenuItem("Restart", "")
			delItem := containerItem.AddSubMenuItem("Delete", "")

			updateItems := func(st string) {
				containerItem.SetTitle(fmt.Sprintf("%s (%s)", c.Name, st))
				switch st {
				case "Running":
					startItem.Hide()
					stopItem.Show()
				case "Stopped":
					stopItem.Hide()
					startItem.Show()
				default:
					startItem.Show()
					stopItem.Show()
				}
				restartItem.Show()
			}

			updateItems(status)

			go func(name string, cont *Container) {
				for {
					select {
					case <-startItem.ClickedCh:
						exec.Command("docker", "start", name).Run()
						cont.Status = "Running"
						updateItems("Running")
						updateProjectItems()
					case <-stopItem.ClickedCh:
						exec.Command("docker", "stop", name).Run()
						cont.Status = "Stopped"
						updateItems("Stopped")
						updateProjectItems()
					case <-restartItem.ClickedCh:
						exec.Command("docker", "restart", name).Run()
						cont.Status = "Running"
						updateItems("Running")
						updateProjectItems()
					case <-delItem.ClickedCh:
						exec.Command("docker", "rm", name).Run()
					}
				}
			}(c.Name, c)
		}

		go func(conts []Container) {
			for {
				select {
				case <-startAll.ClickedCh:
					for i := range conts {
						exec.Command("docker", "start", conts[i].Name).Run()
						conts[i].Status = "Running"
					}
					updateProjectItems()
				case <-stopAll.ClickedCh:
					for i := range conts {
						exec.Command("docker", "stop", conts[i].Name).Run()
						conts[i].Status = "Stopped"
					}
					updateProjectItems()
				case <-restartAll.ClickedCh:
					for i := range conts {
						exec.Command("docker", "restart", conts[i].Name).Run()
						conts[i].Status = "Running"
					}
					updateProjectItems()
				}
			}
		}(containers)

		updateProjectItems()
	}
}
