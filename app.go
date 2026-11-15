package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"Protonaut/backend"
	"Protonaut/backend/models"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version è la versione dell'app, incisa a build-time via
// -ldflags "-X main.Version=v0.1.2". In sviluppo resta "dev".
var Version = "dev"

// App struct
type App struct {
	ctx              context.Context
	scanner          *backend.Scanner
	companionManager *backend.CompanionManager
	coverCache       *backend.CoverCache
	favoritesManager *backend.FavoritesManager
	settingsManager  *backend.SettingsManager
}

// NewApp creates a new App application struct
func NewApp() *App {
	s := backend.NewScanner()
	settings := backend.NewSettingsManager()
	return &App{
		scanner:          s,
		companionManager: backend.NewCompanionManager(s, settings),
		coverCache:       backend.NewCoverCache(),
		favoritesManager: backend.NewFavoritesManager(),
		settingsManager:  settings,
	}
}

// GetSteamStatus restituisce info diagnostiche sul rilevamento di Steam.
func (a *App) GetSteamStatus() models.SteamStatus {
	return a.scanner.GetSteamStatus()
}

// GetLibraries restituisce tutte le librerie Steam configurate.
func (a *App) GetLibraries() ([]models.Library, error) {
	return a.scanner.GetLibraries()
}

// GetInstalledGames restituisce tutti i giochi installati in tutte le
// librerie, con il flag Favorite applicato dal FavoritesManager locale e
// i preferiti ordinati per primi (poi A→Z).
func (a *App) GetInstalledGames() ([]models.Game, error) {
	games, err := a.scanner.GetInstalledGames()
	if err != nil {
		return nil, err
	}
	for i := range games {
		games[i].Favorite = a.favoritesManager.IsFavorite(games[i].AppID)
	}
	sort.SliceStable(games, func(i, j int) bool {
		return games[i].Favorite && !games[j].Favorite
	})
	return games, nil
}

// SetFavorite imposta o rimuove il preferito locale per l'appId dato.
func (a *App) SetFavorite(appId string, favorite bool) error {
	return a.favoritesManager.SetFavorite(appId, favorite)
}

// GetRunningProtonProcesses restituisce i processi Proton attivi con il loro
// PID e AppID (target per l'iniezione del codice).
func (a *App) GetRunningProtonProcesses() ([]models.ProtonProcess, error) {
	return a.scanner.GetRunningProtonProcesses()
}

// GetCoverImage restituisce la copertina del gioco come data URI, servita
// dalla cache locale su disco se già presente, altrimenti risolta tramite
// la Steam Store API e cachata per le richieste successive.
func (a *App) GetCoverImage(appId string) (string, error) {
	return a.coverCache.GetCoverImage(appId)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.companionManager.StartWatcher()
	// Riallinea il file di autostart alla preferenza salvata: se attivo,
	// riscrive Exec= col path corrente (l'AppImage cambia nome a ogni versione).
	_ = backend.SyncAutostart(a.settingsManager.GetSettings().StartOnBoot)
	go a.watchDisplayScale()
}

// GetMonitors restituisce tutti i monitor rilevati con geometria e scala.
func (a *App) GetMonitors() []backend.MonitorInfo {
	return backend.GetMonitors()
}

// watchDisplayScale controlla ogni 2 secondi il monitor attivo tramite
// ScreenGetAll (usa gdk_display_get_monitor_at_window, funziona su Wayland)
// e incrocia le dimensioni logiche con i dati di scala di kscreen-doctor.
func (a *App) watchDisplayScale() {
	// Carica i monitor con scala reale da kscreen-doctor una sola volta.
	// Se kscreen-doctor non è disponibile, non c'è nulla da fare.
	kdMonitors := backend.GetMonitors()
	if len(kdMonitors) == 0 {
		return
	}

	// Indice per lookup rapido: "WxH" → MonitorInfo
	bySize := make(map[string]backend.MonitorInfo, len(kdMonitors))
	for _, m := range kdMonitors {
		key := fmt.Sprintf("%dx%d", m.Width, m.Height)
		bySize[key] = m
	}

	var lastMonitorName string
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		screens, err := runtime.ScreenGetAll(a.ctx)
		if err != nil {
			continue
		}
		var current *runtime.Screen
		for i := range screens {
			if screens[i].IsCurrent {
				current = &screens[i]
			}
		}
		if current == nil {
			continue
		}

		// Cerca il monitor kscreen corrispondente per dimensione logica.
		key := fmt.Sprintf("%dx%d", current.Size.Width, current.Size.Height)
		m, ok := bySize[key]
		if !ok {
			// Fallback: usa i dati GDK senza scala reale.
			m = backend.MonitorInfo{
				Name:  "monitor",
				Width: current.Size.Width, Height: current.Size.Height,
				Scale: float64(current.PhysicalSize.Width) / float64(current.Size.Width),
			}
		}

		if m.Name != lastMonitorName {
			lastMonitorName = m.Name
			runtime.EventsEmit(a.ctx, "display:scale", m)

			var js string
			if m.Scale > 1 {
				js = fmt.Sprintf(`
(function() {
    var s = document.getElementById('__proton_scale');
    if (!s) { s = document.createElement('style'); s.id = '__proton_scale'; document.head.appendChild(s); }
    s.textContent = ':root { zoom: %[1]g !important; }';
    document.documentElement.style.setProperty('zoom', '%[1]g', 'important');
})();
`, m.Scale)
			} else {
				js = `
(function() {
    var s = document.getElementById('__proton_scale');
    if (s) s.textContent = '';
    document.documentElement.style.removeProperty('zoom');
})();
`
			}
			runtime.WindowExecJS(a.ctx, js)
		}
	}
}

// GetCompanions restituisce la mappa appId → companion configurati.
func (a *App) GetCompanions() map[string][]models.Companion {
	return a.companionManager.GetCompanions()
}

// AddCompanion aggiunge un companion per il gioco con l'appId dato.
func (a *App) AddCompanion(appId string, exePath string) error {
	return a.companionManager.AddCompanion(appId, exePath)
}

// RemoveCompanion rimuove il companion con l'exePath indicato.
func (a *App) RemoveCompanion(appId string, exePath string) error {
	return a.companionManager.RemoveCompanion(appId, exePath)
}

// SelectCompanionExe apre il file picker nativo e restituisce il path scelto.
func (a *App) SelectCompanionExe() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: backend.T("dialog.selectCompanion"),
		Filters: []runtime.FileFilter{
			{DisplayName: backend.T("dialog.windowsExe"), Pattern: "*.exe"},
			{DisplayName: backend.T("dialog.allFiles"), Pattern: "*"},
		},
	})
	return path, err
}

// GetSettings restituisce le preferenze globali correnti.
func (a *App) GetSettings() models.Settings {
	return a.settingsManager.GetSettings()
}

// SetSettings aggiorna e persiste le preferenze globali e sincronizza il file
// di autostart in base al flag StartOnBoot.
func (a *App) SetSettings(s models.Settings) error {
	if err := a.settingsManager.SetSettings(s); err != nil {
		return err
	}
	return backend.SyncAutostart(s.StartOnBoot)
}

// LaunchCompanions lancia manualmente i companion configurati per l'appId dato
// (clic sulla cella del gioco). Restituisce quanti ne ha avviati.
func (a *App) LaunchCompanions(appId string) int {
	return a.companionManager.LaunchCompanionsFor(appId)
}

// GetVersion restituisce la versione corrente dell'app.
func (a *App) GetVersion() string {
	return Version
}

// CheckForUpdate confronta la versione corrente con l'ultima release GitHub.
func (a *App) CheckForUpdate() (models.UpdateStatus, error) {
	return backend.CheckForUpdate(Version)
}

// ApplyUpdate aggiorna l'AppImage in place tramite appimageupdatetool.
// Disponibile solo quando l'app gira come AppImage.
func (a *App) ApplyUpdate() (string, error) {
	return backend.ApplyUpdate()
}

// RestartApp riavvia l'app (usato dopo un aggiornamento). Lancia una nuova
// istanza dell'AppImage aggiornato e chiude quella corrente.
func (a *App) RestartApp() error {
	appimage := os.Getenv("APPIMAGE")
	if appimage == "" {
		runtime.Quit(a.ctx)
		return nil
	}
	cmd := exec.Command(appimage)
	cmd.Dir = filepath.Dir(appimage) // non ereditare la cwd dentro il mount dell'AppImage
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	runtime.Quit(a.ctx)
	return nil
}

// OpenReleaseURL apre la pagina della release nel browser di sistema.
func (a *App) OpenReleaseURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
