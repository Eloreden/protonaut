# Protonaut

A desktop companion dashboard for your Steam/Proton library on Linux, built with Wails (Go + React) — detects running Proton games and auto-launches paired companion tools (e.g. Protontricks).

## Features

- Parses Steam libraries (`libraryfolders.vdf` + `appmanifest_*.acf`) and filters out non-game entries (Proton, runtimes, redistributables, SDKs, dedicated servers)
- Reads favorites from Steam's `localconfig.vdf`
- Detects running Proton game processes via `/proc/<pid>/environ`
- Auto-launches companion tools (e.g. `protontricks-launch`) when a paired game starts running
- Multi-monitor DPI-aware UI scaling (KDE/Wayland via `kscreen-doctor`)

## Requirements

- Linux with Steam and Proton
- Go 1.23+
- Node.js (for the frontend)
- [Wails v2](https://wails.io) CLI
- [Protontricks](https://github.com/Matoking/protontricks) (optional, for companion tools)

## Development

```bash
wails dev
```

## Building

```bash
wails build
```
