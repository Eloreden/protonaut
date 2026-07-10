package main

import (
	"context"
	"fmt"
	"time"

	"Protonaut/backend"
	"Protonaut/backend/models"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx              context.Context
	scanner          *backend.Scanner
	companionManager *backend.CompanionManager
}

// NewApp creates a new App application struct
func NewApp() *App {
	s := backend.NewScanner()
	return &App{
		scanner:          s,
		companionManager: backend.NewCompanionManager(s),
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

// GetInstalledGames restituisce tutti i giochi installati in tutte le librerie.
func (a *App) GetInstalledGames() ([]models.Game, error) {
	return a.scanner.GetInstalledGames()
}

// GetRunningProtonProcesses restituisce i processi Proton attivi con il loro
// PID e AppID (target per l'iniezione del codice).
func (a *App) GetRunningProtonProcesses() ([]models.ProtonProcess, error) {
	return a.scanner.GetRunningProtonProcesses()
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.companionManager.StartWatcher()
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
		Title: "Seleziona companion exe",
		Filters: []runtime.FileFilter{
			{DisplayName: "Eseguibili Windows (*.exe)", Pattern: "*.exe"},
			{DisplayName: "Tutti i file (*)", Pattern: "*"},
		},
	})
	return path, err
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
