package backend

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"Protonaut/backend/models"
)

const configFile = ".config/Protonaut/companions.json"

// CompanionManager gestisce la config persistente dei companion e il watcher
// che li lancia automaticamente quando il gioco corrispondente è in esecuzione.
type CompanionManager struct {
	scanner *Scanner

	mu         sync.RWMutex
	companions map[string][]models.Companion // appId → lista companion

	// traccia i companion già avviati nella sessione corrente:
	// key = "appId|exePath", true finché il processo companion gira
	launched map[string]bool
	launchMu sync.Mutex
}

func NewCompanionManager(s *Scanner) *CompanionManager {
	cm := &CompanionManager{
		scanner:    s,
		companions: map[string][]models.Companion{},
		launched:   map[string]bool{},
	}
	cm.load()
	return cm
}

// --------------------------------------------------------------------------
// Config persistente
// --------------------------------------------------------------------------

func configPath() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, configFile)
}

func (cm *CompanionManager) load() {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return // primo avvio, file non esiste ancora
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	_ = json.Unmarshal(data, &cm.companions)
}

func (cm *CompanionManager) save() error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	cm.mu.RLock()
	data, err := json.MarshalIndent(cm.companions, "", "  ")
	cm.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// --------------------------------------------------------------------------
// API esposta a Wails
// --------------------------------------------------------------------------

// GetCompanions restituisce tutta la mappa appId → []Companion.
func (cm *CompanionManager) GetCompanions() map[string][]models.Companion {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	// copia difensiva per evitare race con la goroutine watcher
	out := make(map[string][]models.Companion, len(cm.companions))
	for k, v := range cm.companions {
		cp := make([]models.Companion, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// AddCompanion aggiunge un companion per l'appId dato (evita duplicati).
func (cm *CompanionManager) AddCompanion(appId, exePath string) error {
	name := filepath.Base(exePath)
	cm.mu.Lock()
	list := cm.companions[appId]
	for _, c := range list {
		if c.ExePath == exePath {
			cm.mu.Unlock()
			return nil // già presente
		}
	}
	cm.companions[appId] = append(list, models.Companion{ExePath: exePath, Name: name})
	cm.mu.Unlock()
	return cm.save()
}

// RemoveCompanion rimuove il companion con l'exePath indicato per quell'appId.
func (cm *CompanionManager) RemoveCompanion(appId, exePath string) error {
	cm.mu.Lock()
	list := cm.companions[appId]
	newList := list[:0]
	for _, c := range list {
		if c.ExePath != exePath {
			newList = append(newList, c)
		}
	}
	if len(newList) == 0 {
		delete(cm.companions, appId)
	} else {
		cm.companions[appId] = newList
	}
	cm.mu.Unlock()

	// rimuovi anche dal set "già lanciati" così può essere rilanciato
	cm.launchMu.Lock()
	delete(cm.launched, appId+"|"+exePath)
	cm.launchMu.Unlock()

	return cm.save()
}

// --------------------------------------------------------------------------
// Watcher — gira come goroutine per tutta la vita dell'app
// --------------------------------------------------------------------------

func (cm *CompanionManager) StartWatcher() {
	go cm.watchLoop()
}

func (cm *CompanionManager) watchLoop() {
	ticker := time.NewTicker(2500 * time.Millisecond)
	defer ticker.Stop()

	// AppID visti nell'ultimo poll: rileva quando un gioco viene chiuso
	prevRunning := map[string]bool{}

	for range ticker.C {
		procs, err := cm.scanner.GetRunningProtonProcesses()
		if err != nil {
			continue
		}

		currentRunning := map[string]bool{}
		for _, p := range procs {
			currentRunning[p.AppID] = true
		}

		// Giochi che hanno appena smesso di girare → reset "già lanciati"
		for appId := range prevRunning {
			if !currentRunning[appId] {
				cm.clearLaunched(appId)
			}
		}
		prevRunning = currentRunning

		// Per ogni gioco in esecuzione con companion configurati
		cm.mu.RLock()
		toCheck := make(map[string][]models.Companion)
		for appId, list := range cm.companions {
			if currentRunning[appId] {
				cp := make([]models.Companion, len(list))
				copy(cp, list)
				toCheck[appId] = cp
			}
		}
		cm.mu.RUnlock()

		for appId, companions := range toCheck {
			for _, c := range companions {
				key := appId + "|" + c.ExePath
				cm.launchMu.Lock()
				already := cm.launched[key]
				if !already {
					cm.launched[key] = true
				}
				cm.launchMu.Unlock()

				if !already {
					go cm.launch(appId, c.ExePath)
				}
			}
		}
	}
}

// clearLaunched resetta i companion "già avviati" per un appId (gioco chiuso).
func (cm *CompanionManager) clearLaunched(appId string) {
	cm.mu.RLock()
	companions := cm.companions[appId]
	cm.mu.RUnlock()

	cm.launchMu.Lock()
	for _, c := range companions {
		delete(cm.launched, appId+"|"+c.ExePath)
	}
	cm.launchMu.Unlock()
}

// launch esegue protontricks-launch in background e aspetta la fine.
func (cm *CompanionManager) launch(appId, exePath string) {
	cmd := exec.Command("protontricks-launch", "--no-bwrap", "--appid", appId, exePath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()

	// Quando il companion termina, lo rimuoviamo dai "già lanciati"
	// così alla prossima sessione del gioco viene rilancato.
	key := appId + "|" + exePath
	cm.launchMu.Lock()
	delete(cm.launched, key)
	cm.launchMu.Unlock()
}
