package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const favoritesFile = ".config/Protonaut/favorites.json"

// FavoritesManager gestisce i preferiti impostati dall'utente in Protonaut,
// persistiti localmente. Non si basa sui preferiti di Steam: il client
// moderno salva le collezioni in localconfig.vdf con ID hashati non
// riconducibili all'AppID reale, quindi quel parsing non è affidabile.
type FavoritesManager struct {
	mu        sync.RWMutex
	favorites map[string]bool
}

func NewFavoritesManager() *FavoritesManager {
	fm := &FavoritesManager{favorites: map[string]bool{}}
	fm.load()
	return fm
}

func favoritesPath() string {
	return filepath.Join(homeDir(), favoritesFile)
}

func (fm *FavoritesManager) load() {
	data, err := os.ReadFile(favoritesPath())
	if err != nil {
		return // primo avvio, file non esiste ancora
	}
	fm.mu.Lock()
	defer fm.mu.Unlock()
	_ = json.Unmarshal(data, &fm.favorites)
}

func (fm *FavoritesManager) save() error {
	path := favoritesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	fm.mu.RLock()
	data, err := json.MarshalIndent(fm.favorites, "", "  ")
	fm.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// IsFavorite indica se l'appId è marcato come preferito.
func (fm *FavoritesManager) IsFavorite(appId string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.favorites[appId]
}

// SetFavorite imposta o rimuove il preferito per l'appId dato.
func (fm *FavoritesManager) SetFavorite(appId string, favorite bool) error {
	fm.mu.Lock()
	if favorite {
		fm.favorites[appId] = true
	} else {
		delete(fm.favorites, appId)
	}
	fm.mu.Unlock()
	return fm.save()
}
