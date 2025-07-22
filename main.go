package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
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

func (c *containerMenu) setStatus(st string) {
	c.status = st
	c.item.SetTitle(fmt.Sprintf("%s (%s)", c.name, st))
	switch st {
	case "Running":
		c.start.Hide()
		c.stop.Show()
	case "Stopped":
		c.stop.Hide()
		c.start.Show()
	default:
		c.start.Show()
		c.stop.Show()
	}
	c.restart.Show()
}

func newProjectMenu(name string) *projectMenu {
	pm := &projectMenu{
		item:       projectsMenu.AddSubMenuItem(name, ""),
		containers: make(map[string]*containerMenu),
	}
	pm.startAll = pm.item.AddSubMenuItem("Start All", "")
	pm.stopAll = pm.item.AddSubMenuItem("Stop All", "")
	pm.restartAll = pm.item.AddSubMenuItem("Restart All", "")
	pm.item.AddSubMenuItem("", "").Disable()

	pm.updateFunc = func() {
		anyRunning := false
		anyStopped := false
		for _, c := range pm.containers {
			if c.status == "Running" {
				anyRunning = true
			}
			if c.status == "Stopped" {
				anyStopped = true
			}
		}
		if anyRunning {
			pm.stopAll.Show()
		} else {
			pm.stopAll.Hide()
		}
		if anyStopped {
			pm.startAll.Show()
		} else {
			pm.startAll.Hide()
		}
	}

	go func(p *projectMenu) {
		for {
			select {
			case <-p.startAll.ClickedCh:
				mu.Lock()
				for _, c := range p.containers {
					exec.Command("docker", "start", c.name).Run()
					c.setStatus("Running")
				}
				p.updateFunc()
				mu.Unlock()
			case <-p.stopAll.ClickedCh:
				mu.Lock()
				for _, c := range p.containers {
					exec.Command("docker", "stop", c.name).Run()
					c.setStatus("Stopped")
				}
				p.updateFunc()
				mu.Unlock()
			case <-p.restartAll.ClickedCh:
				mu.Lock()
				for _, c := range p.containers {
					exec.Command("docker", "restart", c.name).Run()
					c.setStatus("Running")
				}
				p.updateFunc()
				mu.Unlock()
			}
		}
	}(pm)

	return pm
}

func getOrCreateProject(name string) *projectMenu {
	if pm, ok := projects[name]; ok {
		return pm
	}
	pm := newProjectMenu(name)
	projects[name] = pm
	return pm
}

func (p *projectMenu) getOrCreateContainer(name string) *containerMenu {
	if c, ok := p.containers[name]; ok {
		return c
	}
	c := &containerMenu{name: name}
	c.item = p.item.AddSubMenuItem(name, "")
	c.start = c.item.AddSubMenuItem("Start", "")
	c.stop = c.item.AddSubMenuItem("Stop", "")
	c.restart = c.item.AddSubMenuItem("Restart", "")
	c.del = c.item.AddSubMenuItem("Delete", "")

	go func(cm *containerMenu, pm *projectMenu) {
		for {
			select {
			case <-cm.start.ClickedCh:
				exec.Command("docker", "start", cm.name).Run()
				mu.Lock()
				cm.setStatus("Running")
				pm.updateFunc()
				mu.Unlock()
			case <-cm.stop.ClickedCh:
				exec.Command("docker", "stop", cm.name).Run()
				mu.Lock()
				cm.setStatus("Stopped")
				pm.updateFunc()
				mu.Unlock()
			case <-cm.restart.ClickedCh:
				exec.Command("docker", "restart", cm.name).Run()
				mu.Lock()
				cm.setStatus("Running")
				pm.updateFunc()
				mu.Unlock()
			case <-cm.del.ClickedCh:
				exec.Command("docker", "rm", cm.name).Run()
			}
		}
	}(c, p)

	p.containers[name] = c
	return c
}

type containerMenu struct {
	name    string
	item    *systray.MenuItem
	start   *systray.MenuItem
	stop    *systray.MenuItem
	restart *systray.MenuItem
	del     *systray.MenuItem
	status  string
}

type projectMenu struct {
	item       *systray.MenuItem
	startAll   *systray.MenuItem
	stopAll    *systray.MenuItem
	restartAll *systray.MenuItem
	containers map[string]*containerMenu
	updateFunc func()
}

var (
	projectsMenu *systray.MenuItem
	projects     = make(map[string]*projectMenu)
	mu           sync.Mutex
)

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
	go watchDockerEvents()
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
	mu.Lock()
	projectsMenu = m
	mu.Unlock()

	projectsData, err := getContainersByProject()
	if err != nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()
	for proj, conts := range projectsData {
		pm := getOrCreateProject(proj)
		for i := range conts {
			c := conts[i]
			cm := pm.getOrCreateContainer(c.Name)
			cm.setStatus(parseContainerStatus(c.Status))
		}
		pm.updateFunc()
	}
}

func watchDockerEvents() {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Printf("docker client error: %v", err)
		return
	}

	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("event", "start")
	f.Add("event", "die")

	msgs, errs := cli.Events(context.Background(), types.EventsOptions{Filters: f})
	for {
		select {
		case err := <-errs:
			if err != nil {
				log.Printf("docker event error: %v", err)
				return
			}
		case msg := <-msgs:
			handleDockerEvent(cli, msg)
		}
	}
}

func handleDockerEvent(cli *client.Client, msg events.Message) {
	ctx := context.Background()
	inspect, err := cli.ContainerInspect(ctx, msg.ID)
	if err != nil {
		return
	}
	name := strings.TrimPrefix(inspect.Name, "/")
	project := inspect.Config.Labels["com.docker.compose.project"]
	if project == "" {
		project = "default"
	}
	status := "Stopped"
	if inspect.State != nil && inspect.State.Running {
		status = "Running"
	}

	mu.Lock()
	pm := getOrCreateProject(project)
	cm := pm.getOrCreateContainer(name)
	cm.setStatus(status)
	pm.updateFunc()
	mu.Unlock()
}
