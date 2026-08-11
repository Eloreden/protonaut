package backend

import (
	"fmt"
	"os"
	"path/filepath"
)

const autostartFile = ".config/autostart/protonaut.desktop"

// autostartPath è il percorso del file .desktop XDG di autostart.
func autostartPath() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, autostartFile)
}

// executablePath restituisce il comando da usare in Exec= per riavviare l'app.
// Se l'app gira come AppImage, os.Executable() punterebbe al binario dentro il
// mount temporaneo (/tmp/.mount_xxx/...): usiamo invece la variabile APPIMAGE
// impostata dal runtime AppImage, che contiene il path reale del .AppImage.
func executablePath() (string, error) {
	if ai := os.Getenv("APPIMAGE"); ai != "" {
		return ai, nil
	}
	return os.Executable()
}

// SyncAutostart crea o rimuove il file .desktop di autostart in base a enabled.
func SyncAutostart(enabled bool) error {
	path := autostartPath()

	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	exe, err := executablePath()
	if err != nil {
		return err
	}

	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Protonaut
Comment=Companion dashboard per la libreria Steam/Proton
Exec=%s
Icon=protonaut
Terminal=false
Categories=Game;Utility;
X-GNOME-Autostart-enabled=true
`, exe)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// IsAutostartEnabled indica se il file di autostart è presente.
func IsAutostartEnabled() bool {
	_, err := os.Stat(autostartPath())
	return err == nil
}
