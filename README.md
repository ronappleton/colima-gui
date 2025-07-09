# Colima GUI

This project provides a small system tray application for managing [Colima](https://github.com/abiosoft/colima) and Docker containers.

The tray menu exposes quick actions to start, stop or restart Colima. It also
lists Docker containers grouped by their Compose project, allowing you to
control individual containers or entire projects.

## Building

```bash
go build
```

The resulting binary can then be launched directly.

## Usage

Run the compiled executable. A tray icon will appear which displays the current
Colima status and offers menu items to manage Colima and containers.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for
more information.
