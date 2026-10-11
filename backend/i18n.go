package backend

import (
	"os"
	"strings"
	"sync/atomic"
)

var supportedLanguages = map[string]bool{"en": true, "it": true}

var currentLanguage atomic.Value

var translations = map[string]map[string]string{
	"en": {
		"dialog.selectCompanion":    "Select companion exe",
		"dialog.windowsExe":         "Windows executables (*.exe)",
		"dialog.allFiles":           "All files (*)",
		"err.vdfNotFound":           "libraryfolders.vdf not found in any known Steam root",
		"err.updateAppImageOnly":    "in-app update is only available for the AppImage version",
		"err.appImageUpdateMissing": "AppImageUpdate not found: install 'appimageupdate' or AppImageLauncher",
	},
	"it": {
		"dialog.selectCompanion":    "Seleziona companion exe",
		"dialog.windowsExe":         "Eseguibili Windows (*.exe)",
		"dialog.allFiles":           "Tutti i file (*)",
		"err.vdfNotFound":           "libraryfolders.vdf non trovato in nessuna root Steam nota",
		"err.updateAppImageOnly":    "l'aggiornamento in-app è disponibile solo per la versione AppImage",
		"err.appImageUpdateMissing": "AppImageUpdate non trovato: installa 'appimageupdate' o AppImageLauncher",
	},
}

// ResolveLanguage converte la preferenza utente ("" = automatica) in un
// codice lingua supportato, ricavando quella di sistema dalle variabili locale.
func ResolveLanguage(pref string) string {
	if supportedLanguages[pref] {
		return pref
	}
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		v := os.Getenv(env)
		if v == "" {
			continue
		}
		code := strings.ToLower(v)
		if i := strings.IndexAny(code, "_.@:-"); i >= 0 {
			code = code[:i]
		}
		if supportedLanguages[code] {
			return code
		}
	}
	return "en"
}

// SetLanguage imposta la lingua usata dalle stringhe del backend.
func SetLanguage(pref string) {
	currentLanguage.Store(ResolveLanguage(pref))
}

// T restituisce la stringa tradotta per la chiave nella lingua corrente.
func T(key string) string {
	lang, _ := currentLanguage.Load().(string)
	if lang == "" {
		lang = ResolveLanguage("")
	}
	if s, ok := translations[lang][key]; ok {
		return s
	}
	if s, ok := translations["en"][key]; ok {
		return s
	}
	return key
}
