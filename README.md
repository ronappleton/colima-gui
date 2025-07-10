# Colima GUI

This project provides a small system tray application for managing [Colima](https://github.com/abiosoft/colima) and Docker containers.

The tray menu exposes quick actions to start, stop or restart Colima. It also
lists Docker containers grouped by their Compose project, allowing you to
control individual containers or entire projects.

## Building

```bash
go build -ldflags "-X main.version=$(git describe --tags --always)"
```

The resulting binary can then be launched directly.

### Dependencies

The build relies on `pkg-config` and the `libayatana-appindicator3-dev` package
on Linux. Install them with `apt-get install -y libayatana-appindicator3-dev pkg-config`.

## Usage

Run the compiled executable. A tray icon will appear which displays the current
Colima status and offers menu items to manage Colima and containers.
Each listed container now includes **Logs** and **Exec** actions. Selecting
**Logs** opens a read-only terminal window showing the container logs, while
**Exec** starts an interactive shell inside the container.

Use `colima-gui -version` to check the embedded version string.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for
more information.
