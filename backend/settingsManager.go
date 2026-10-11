package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"Protonaut/backend/models"
)

const settingsFile = ".config/Protonaut/settings.json"

// SettingsManager gestisce le preferenze globali persistenti dell'app.
type SettingsManager struct {
	mu       sync.RWMutex
	settings models.Settings
}

func NewSettingsManager() *SettingsManager {
	sm := &SettingsManager{
		// Default: comportamento automatico completo (retrocompatibile).
		settings: models.Settings{
			AutoLaunchCompanions: true,
			AutoDetectGames:      true,
		},
	}
	sm.load()
	SetLanguage(sm.settings.Language)
	return sm
}

func settingsPath() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, settingsFile)
}

func (sm *SettingsManager) load() {
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return // primo avvio, usa i default
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	_ = json.Unmarshal(data, &sm.settings)
}

func (sm *SettingsManager) save() error {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	sm.mu.RLock()
	data, err := json.MarshalIndent(sm.settings, "", "  ")
	sm.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetSettings restituisce una copia delle preferenze correnti.
func (sm *SettingsManager) GetSettings() models.Settings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings
}

// SetSettings sostituisce le preferenze e le persiste su disco.
func (sm *SettingsManager) SetSettings(s models.Settings) error {
	sm.mu.Lock()
	sm.settings = s
	sm.mu.Unlock()
	SetLanguage(s.Language)
	return sm.save()
}
