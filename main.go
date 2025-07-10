package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var version = "dev"

//go:embed images/icon.ico
var iconData []byte

type Container struct {
	Name    string
	Project string
	Status  string
}

func main() {
	verFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *verFlag {
		fmt.Println(version)
		return
	}

	if os.Getenv("COLIMA_GUI_DETACHED") == "" {
		cmd := exec.Command(os.Args[0], os.Args[1:]...)
		cmd.Env = append(os.Environ(), "COLIMA_GUI_DETACHED=1")
		if runtime.GOOS != "windows" {
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		} else {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		if err := cmd.Start(); err != nil {
			log.Fatalf("failed to start daemon: %v", err)
		}
		return
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

func openTerminal(cmdStr string) error {
	switch runtime.GOOS {
	case "darwin":
		escaped := strings.ReplaceAll(cmdStr, "\"", "\\\"")
		return exec.Command("osascript", "-e", fmt.Sprintf("tell application \"Terminal\" to do script \"%s\"", escaped)).Start()
	case "windows":
		return exec.Command("cmd", "/C", "start", "cmd", "/K", cmdStr).Start()
	default:
		term := os.Getenv("TERMINAL")
		if term == "" {
			term = "x-terminal-emulator"
		}
		return exec.Command(term, "-e", "bash", "-c", cmdStr).Start()
	}
}

func showContainerLogs(name string) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = fmt.Sprintf("docker logs -f %s & pause", name)
	} else {
		cmd = fmt.Sprintf("docker logs -f %s; read -n1 -s -r -p 'Press any key to close...'", name)
	}
	err := openTerminal(cmd)
	if err != nil {
		return
	}
}

func execInContainer(name string) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = fmt.Sprintf("docker exec -it %s cmd", name)
	} else {
		cmd = fmt.Sprintf("docker exec -it %s sh", name)
	}
	err := openTerminal(cmd)
	if err != nil {
		return
	}
}

func parseDockerPSOutput(out string) map[string][]Container {
	projects := make(map[string][]Container)
	lines := strings.Split(strings.TrimSpace(out), "\n")
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

	return projects
}

func getContainersByProject() (map[string][]Container, error) {
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}|{{.Label \"com.docker.compose.project\"}}|{{.Status}}").CombinedOutput()
	if err != nil {
		return nil, err
	}
	return parseDockerPSOutput(string(out)), nil
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
			logsItem := containerItem.AddSubMenuItem("Logs", "")
			execItem := containerItem.AddSubMenuItem("Exec", "")
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

			go func(name string, container *Container) {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-ticker.C:
						out, err := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", name).CombinedOutput()
						if err == nil {
							st := parseContainerStatus(string(out))
							container.Status = st
							updateItems(st)
							updateProjectItems()
						}
					case <-startItem.ClickedCh:
						exec.Command("docker", "start", name).Run()
						container.Status = "Running"
						updateItems("Running")
						updateProjectItems()
					case <-stopItem.ClickedCh:
						exec.Command("docker", "stop", name).Run()
						container.Status = "Stopped"
						updateItems("Stopped")
						updateProjectItems()
					case <-restartItem.ClickedCh:
						exec.Command("docker", "restart", name).Run()
						container.Status = "Running"
						updateItems("Running")
						updateProjectItems()
					case <-logsItem.ClickedCh:
						go showContainerLogs(name)
					case <-execItem.ClickedCh:
						go execInContainer(name)
					case <-delItem.ClickedCh:
						exec.Command("docker", "rm", name).Run()
						return
					}
				}
			}(c.Name, c)
		}

		go func(containers []Container) {
			for {
				select {
				case <-startAll.ClickedCh:
					for i := range containers {
						exec.Command("docker", "start", containers[i].Name).Run()
						containers[i].Status = "Running"
					}
					updateProjectItems()
				case <-stopAll.ClickedCh:
					for i := range containers {
						exec.Command("docker", "stop", containers[i].Name).Run()
						containers[i].Status = "Stopped"
					}
					updateProjectItems()
				case <-restartAll.ClickedCh:
					for i := range containers {
						exec.Command("docker", "restart", containers[i].Name).Run()
						containers[i].Status = "Running"
					}
					updateProjectItems()
				}
			}
		}(containers)

		updateProjectItems()
	}
}
