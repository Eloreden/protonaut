package models

type Library struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Game = un gioco installato (da appmanifest_*.acf)
type Game struct {
	AppID      string `json:"appId"`
	Name       string `json:"name"`
	InstallDir string `json:"installDir"`
	LibraryID  string `json:"libraryId"`
	FullPath   string `json:"fullPath"`
	Favorite   bool   `json:"favorite"` // presente nella collezione "favorite" di Steam
}

// SteamStatus = info diagnostica
type SteamStatus struct {
	Found     bool     `json:"found"`
	SteamPath string   `json:"steamPath"`
	VDFPath   string   `json:"vdfPath"`
	Reasons   []string `json:"reasons"`
}

// Companion = un eseguibile Windows da lanciare dentro il prefisso Proton del gioco.
type Companion struct {
	ExePath string `json:"exePath"`
	Name    string `json:"name"` // basename dell'exe
}

// ProtonProcess = un processo in esecuzione lanciato da Steam/Proton,
// mappato al suo AppID tramite /proc/<pid>/environ. È il target su cui
// successivamente si effettua l'iniezione del codice.
type ProtonProcess struct {
	PID       int    `json:"pid"`
	AppID     string `json:"appId"`     // da SteamAppId / SteamGameId
	Name      string `json:"name"`      // nome del gioco se installato/noto
	Exe       string `json:"exe"`       // target di /proc/<pid>/exe
	Cmdline   string `json:"cmdline"`   // riga di comando completa
	IsGameExe bool   `json:"isGameExe"` // euristica: vero .exe del gioco sotto Proton (non helper Wine/Steam)
}
