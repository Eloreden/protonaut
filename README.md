# Protonaut

A desktop companion dashboard for your Steam/Proton library on Linux, built with [Wails](https://wails.io) (Go + React).

Protonaut doesn't launch your games — Steam already does that. Instead, it sits alongside Steam and:

1. Reads your local Steam libraries directly from disk to build a game list, without needing the Steam API or an internet connection.
2. Watches running processes to detect which Proton game is currently active.
3. Automatically launches a companion tool (e.g. [Protontricks](https://github.com/Matoking/protontricks)) that you've paired with that game, so tools like winetricks GUIs, trainers, or mod managers start alongside it without manual steps.

## How it works

- **Library scanning** (`backend/steamIntegration.go`): parses Steam's `libraryfolders.vdf` to find every library folder, then reads each `appmanifest_*.acf` to list installed apps. Non-game entries (Proton runtimes, Steam Linux Runtime, redistributables, SDKs, dedicated servers, uploaders) are filtered out heuristically.
- **Cover art** (`backend/coverCache.go`): on first view, resolves the real header image URL via the Steam Store API and caches it to `~/.cache/Protonaut/covers/{appid}.jpg`. This avoids a known issue with the naive CDN URL pattern, which can return a stale/placeholder community-hub image instead of a proper 404.
- **Favorites** (`backend/favoritesManager.go`): starred games are stored locally in `~/.config/Protonaut/favorites.json`. This is intentional — Steam's own collections are stored in `localconfig.vdf` under hashed IDs that can't be reliably mapped back to an AppID, so favorites are managed entirely by Protonaut instead.
- **Process detection**: polls `/proc/<pid>/environ` for running processes to find `SteamAppId`, which identifies which game (if any) is currently running under Proton, and displays a "running" badge with its PID.
- **Companion launch** (`backend/companionManager.go`): companions are configured per-game in `~/.config/Protonaut/companions.json`. When a paired game is detected as running, its companion is launched automatically (e.g. `protontricks-launch --no-bwrap --appid <id> <exe>`); companions can also be added/removed manually from the UI via a native file picker. Clicking a game's cell launches its companions on demand — handy when automatic launch is disabled.
- **Settings** (`backend/settingsManager.go`): global preferences are stored in `~/.config/Protonaut/settings.json` and edited from the gear icon in the header. Toggles cover automatic game detection, automatic companion launch, and "start on boot".
- **Start on boot** (`backend/autostart.go`): when enabled, writes an XDG autostart entry to `~/.config/autostart/protonaut.desktop`. For AppImage installs it records the real `$APPIMAGE` path and re-syncs it on every startup, so autostart keeps working even though the AppImage's filename changes with each version.
- **Updates** (`backend/updater.go`): checks the GitHub Releases API and compares the running version against the latest tag. AppImage installs can update in place from the app (via `appimageupdatetool`, delta download over zsync); package-manager installs (AUR) are pointed to the system updater instead. The install type is detected at runtime through `$APPIMAGE`, so a single build behaves correctly in both cases.
- **Display scaling** (`backend/displayInfo.go`): detects the current monitor and its fractional scale (via `kscreen-doctor --outputs` on KDE) and applies matching CSS zoom, keeping the UI crisp across multi-monitor / HiDPI setups.

## Features

- Real Steam library parsing with non-game filtering
- Locally managed favorites, shown first, independent of Steam's own collections
- Disk-cached cover art with automatic stale-image recovery
- Live detection of running Proton games via `/proc`
- Automatic companion tool launch tied to the detected running game
- Manual companion management (add/remove) per game, plus click-to-launch on demand
- Settings page: toggle auto-detection, auto-launch, and start-on-boot
- In-app update check with one-click update for AppImage installs
- Multi-monitor DPI/fractional-scaling awareness

## Requirements

- Linux with Steam and Proton
- Go 1.23+
- Node.js (for the frontend)
- [Wails v2](https://wails.io) CLI
- [Protontricks](https://github.com/Matoking/protontricks) (optional, only needed to use companion auto-launch)

## Installation

### Arch Linux (AUR)

```bash
yay -S protonaut-git
```

### AppImage (any distro)

Download the AppImage from the [releases page](https://github.com/Eloreden/protonaut/releases) — the exact filename varies by version/architecture, referred to below as `<file>.AppImage`.

1. Move it wherever you want to keep it (e.g. `~/Applications/`) and open a terminal in that folder.
2. Make it executable — downloaded files never have the executable bit set, so a double-click in most file managers will just open/preview it instead of running it:
   ```bash
   chmod +x <file>.AppImage
   ```
3. Run it from the terminal (or double-click it afterwards, now that it's executable):
   ```bash
   ./<file>.AppImage
   ```

**Known permission/runtime issues and how to fix them:**

- **"Permission denied" when running `./<file>.AppImage`** — step 2 above was skipped, or the filesystem it's stored on was mounted `noexec` (some `/tmp` or removable-media mounts are). Re-run `chmod +x`, and if it still fails, move the file to your home directory and try again.
- **`dlopen(): error loading libfuse.so.2` / "cannot mount AppImage, please check your FUSE setup"** — AppImages mount themselves at runtime through FUSE2, but several current distros (Ubuntu 24.04+, Fedora, and Arch, which now ships FUSE3 by default) no longer install `fuse2`/`libfuse2` out of the box. Fix it either by installing the compatibility package (`sudo pacman -S fuse2` on Arch, `sudo apt install libfuse2t64` on Ubuntu, etc.), or by skipping FUSE entirely and running the AppImage in extract-and-run mode, which unpacks it to a temp folder and executes it directly instead of mounting it:
  ```bash
  ./<file>.AppImage --appimage-extract-and-run
  ```
- **Why AppImage instead of Flatpak** — Flatpak was evaluated and dropped: its sandbox blocks read access to `/proc/<pid>/environ` for processes outside the sandbox and prevents exec'ing `protontricks-launch` on the host, both of which are core to how Protonaut detects running games and launches companions. AppImage runs as a normal, unsandboxed user process, so it keeps full access to `/proc` and can invoke host binaries like `protontricks-launch` with no extra configuration needed.

### From source

```bash
git clone https://github.com/Eloreden/protonaut.git
cd protonaut
wails build -tags webkit2_41
```

The built binary is placed in `build/bin/`.

> **Note on the `webkit2_41` tag:** on Arch-based distros (and anywhere WebKitGTK 4.0 has been dropped) Wails must be told to link against `webkit2gtk-4.1`. Without `-tags webkit2_41` the link step fails with unresolved `Jxl*` symbols (WebKitGTK 4.0 pulls in an old `libjxl`), and with `wails dev` the process stays alive but the window never opens. The release pipeline and the AUR package already build with this tag.

## Updating

- **AppImage** — open Settings (gear icon) and use **Check** / **Update now**. Updating requires [AppImageUpdate](https://github.com/AppImage/AppImageUpdate) (bundled with AppImageLauncher); it downloads only the delta and replaces the file in place, so it works even though the filename changes between versions. Without it, the app shows the available version and links to the release.
- **AUR** — update through your package manager as usual (e.g. `yay -Syu` / Octopi). As a `-git` VCS package it always builds from the latest commit.

## Development

```bash
wails dev -tags webkit2_41
```

This runs the app with hot reload for the React frontend.

## Building

```bash
wails build -tags webkit2_41
```

## License

MIT — see [LICENSE](LICENSE).
