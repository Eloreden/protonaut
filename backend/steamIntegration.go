package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"Protonaut/backend/models"
)

// Scanner incapsula la scoperta delle librerie/giochi Steam e dei processi
// Proton attualmente in esecuzione.
type Scanner struct{}

func NewScanner() *Scanner { return &Scanner{} }

// ---------------------------------------------------------------------------
// Parser Valve KeyValues (.vdf / .acf)
//
// Il formato è una sequenza di coppie "chiave" "valore" oppure "chiave" { ... }
// con stringhe quotate, commenti // e nidificazione tramite graffe.
// ---------------------------------------------------------------------------

// KV è un nodo KeyValues. Le coppie preservano l'ordine e ammettono chiavi
// duplicate (come fa Valve).
type KV struct {
	Pairs []KVPair
}

type KVPair struct {
	Key   string
	Value string // valorizzato per le foglie
	Sub   *KV    // valorizzato per i sotto-oggetti
}

// Get restituisce il valore foglia per la prima chiave corrispondente (case-insensitive).
func (kv *KV) Get(key string) (string, bool) {
	for _, p := range kv.Pairs {
		if p.Sub == nil && strings.EqualFold(p.Key, key) {
			return p.Value, true
		}
	}
	return "", false
}

// GetChild restituisce il primo sotto-oggetto per la chiave indicata.
func (kv *KV) GetChild(key string) *KV {
	for _, p := range kv.Pairs {
		if p.Sub != nil && strings.EqualFold(p.Key, key) {
			return p.Sub
		}
	}
	return nil
}

const (
	tokString = iota
	tokOpen
	tokClose
	tokEOF
)

type tokenizer struct {
	s   string
	pos int
}

func (t *tokenizer) next() (string, int) {
	for t.pos < len(t.s) {
		c := t.s[t.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			t.pos++
			continue
		case c == '/' && t.pos+1 < len(t.s) && t.s[t.pos+1] == '/':
			for t.pos < len(t.s) && t.s[t.pos] != '\n' {
				t.pos++
			}
			continue
		case c == '{':
			t.pos++
			return "{", tokOpen
		case c == '}':
			t.pos++
			return "}", tokClose
		case c == '"':
			return t.readQuoted(), tokString
		default:
			return t.readBare(), tokString
		}
	}
	return "", tokEOF
}

func (t *tokenizer) readQuoted() string {
	t.pos++ // salta l'apice iniziale
	var sb strings.Builder
	for t.pos < len(t.s) {
		ch := t.s[t.pos]
		if ch == '\\' && t.pos+1 < len(t.s) {
			nxt := t.s[t.pos+1]
			switch nxt {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			default:
				sb.WriteByte(nxt) // \\ e \" finiscono qui
			}
			t.pos += 2
			continue
		}
		if ch == '"' {
			t.pos++
			return sb.String()
		}
		sb.WriteByte(ch)
		t.pos++
	}
	return sb.String() // stringa non terminata: tolleriamo
}

func (t *tokenizer) readBare() string {
	start := t.pos
	for t.pos < len(t.s) {
		ch := t.s[t.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' ||
			ch == '{' || ch == '}' || ch == '"' {
			break
		}
		t.pos++
	}
	return t.s[start:t.pos]
}

func parseKV(data string) *KV {
	t := &tokenizer{s: data}
	root := &KV{}
	parseBody(t, root)
	return root
}

func parseBody(t *tokenizer, node *KV) {
	for {
		key, kind := t.next()
		if kind == tokEOF || kind == tokClose {
			return
		}
		if kind != tokString {
			continue // graffa orfana: ignora
		}
		val, vkind := t.next()
		switch vkind {
		case tokOpen:
			child := &KV{}
			parseBody(t, child)
			node.Pairs = append(node.Pairs, KVPair{Key: key, Sub: child})
		case tokString:
			node.Pairs = append(node.Pairs, KVPair{Key: key, Value: val})
		default:
			return // EOF/close inatteso
		}
	}
}

// ---------------------------------------------------------------------------
// Scoperta librerie e giochi
// ---------------------------------------------------------------------------

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// steamRoots restituisce i percorsi candidati per la root di Steam, in ordine
// di priorità (installazione nativa, link legacy, flatpak).
func (s *Scanner) steamRoots() []string {
	h := homeDir()
	return []string{
		filepath.Join(h, ".local/share/Steam"),
		filepath.Join(h, ".steam/steam"),
		filepath.Join(h, ".steam/root"),
		filepath.Join(h, ".var/app/com.valvesoftware.Steam/.local/share/Steam"),
	}
}

// findLibraryFoldersVDF individua il primo libraryfolders.vdf esistente.
func (s *Scanner) findLibraryFoldersVDF() (steamRoot, vdfPath string, reasons []string) {
	for _, root := range s.steamRoots() {
		vdf := filepath.Join(root, "steamapps", "libraryfolders.vdf")
		if fi, err := os.Stat(vdf); err == nil && !fi.IsDir() {
			return root, vdf, reasons
		}
		reasons = append(reasons, "non trovato: "+vdf)
	}
	return "", "", reasons
}

// GetSteamStatus fornisce informazioni diagnostiche sul rilevamento di Steam.
func (s *Scanner) GetSteamStatus() models.SteamStatus {
	root, vdf, reasons := s.findLibraryFoldersVDF()
	return models.SteamStatus{
		Found:     vdf != "",
		SteamPath: root,
		VDFPath:   vdf,
		Reasons:   reasons,
	}
}

// GetLibraries parsa libraryfolders.vdf e restituisce tutte le librerie.
func (s *Scanner) GetLibraries() ([]models.Library, error) {
	_, vdf, _ := s.findLibraryFoldersVDF()
	if vdf == "" {
		return nil, fmt.Errorf("libraryfolders.vdf non trovato in nessuna root Steam nota")
	}
	data, err := os.ReadFile(vdf)
	if err != nil {
		return nil, fmt.Errorf("lettura %s: %w", vdf, err)
	}

	root := parseKV(string(data))
	lf := root.GetChild("libraryfolders")
	if lf == nil {
		lf = root // tolleranza verso formati senza wrapper
	}

	var libs []models.Library
	for _, p := range lf.Pairs {
		if p.Sub == nil {
			continue
		}
		path, _ := p.Sub.Get("path")
		if path == "" {
			continue
		}
		label, _ := p.Sub.Get("label")
		name := label
		if name == "" {
			name = filepath.Base(path)
		}
		libs = append(libs, models.Library{ID: p.Key, Name: name, Path: path})
	}
	return libs, nil
}

// getFavoriteAppIDs legge la collezione "favorite" da localconfig.vdf e
// restituisce un set di AppID (come stringhe) marcati come preferiti.
func (s *Scanner) getFavoriteAppIDs() map[string]bool {
	favs := map[string]bool{}
	userdataDir := filepath.Join(homeDir(), ".local/share/Steam/userdata")
	entries, err := os.ReadDir(userdataDir)
	if err != nil {
		return favs
	}
	for _, e := range entries {
		cfg := filepath.Join(userdataDir, e.Name(), "config", "localconfig.vdf")
		data, err := os.ReadFile(cfg)
		if err != nil {
			continue
		}
		// Cerca la stringa JSON di user-collections nel file VDF.
		raw := parseKV(string(data))
		collections := kvSearchLeaf(raw, "user-collections")
		if collections == "" {
			continue
		}
		// Struttura JSON: {"favorite":{"id":"favorite","added":[appid,...],"removed":[...]}}
		var col struct {
			Favorite struct {
				Added   []json.Number `json:"added"`
				Removed []json.Number `json:"removed"`
			} `json:"favorite"`
		}
		if err := json.Unmarshal([]byte(collections), &col); err != nil {
			continue
		}
		removedSet := map[string]bool{}
		for _, id := range col.Favorite.Removed {
			removedSet[id.String()] = true
		}
		for _, id := range col.Favorite.Added {
			s := id.String()
			if !removedSet[s] {
				favs[s] = true
			}
		}
	}
	return favs
}

// kvSearchLeaf cerca ricorsivamente la prima foglia con la chiave data.
func kvSearchLeaf(kv *KV, key string) string {
	for _, p := range kv.Pairs {
		if p.Sub == nil && strings.EqualFold(p.Key, key) {
			return p.Value
		}
		if p.Sub != nil {
			if v := kvSearchLeaf(p.Sub, key); v != "" {
				return v
			}
		}
	}
	return ""
}

// isNonGame riconosce i pacchetti di sistema Steam che non sono giochi veri:
// runtime Proton, Steam Linux Runtime, redistributabili, tool di upload, ecc.
func isNonGame(name string) bool {
	low := strings.ToLower(name)
	return strings.HasPrefix(low, "proton ") ||
		strings.HasPrefix(low, "steam linux runtime") ||
		strings.Contains(low, "redistributable") ||
		strings.Contains(low, "steamworks") ||
		strings.HasSuffix(low, " uploader") ||
		strings.HasSuffix(low, " sdk") ||
		strings.HasSuffix(low, " dedicated server")
}

// GetInstalledGames scandisce gli appmanifest_*.acf di ogni libreria.
// Esclude i pacchetti di sistema (Proton, runtime, redistributabili).
// I giochi sono ordinati: preferiti Steam prima (A→Z), poi il resto (A→Z).
func (s *Scanner) GetInstalledGames() ([]models.Game, error) {
	libs, err := s.GetLibraries()
	if err != nil {
		return nil, err
	}
	favs := s.getFavoriteAppIDs()

	var games []models.Game
	for _, lib := range libs {
		steamapps := filepath.Join(lib.Path, "steamapps")
		entries, err := os.ReadDir(steamapps)
		if err != nil {
			continue
		}
		for _, e := range entries {
			n := e.Name()
			if !strings.HasPrefix(n, "appmanifest_") || !strings.HasSuffix(n, ".acf") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(steamapps, n))
			if err != nil {
				continue
			}
			st := parseKV(string(data)).GetChild("AppState")
			if st == nil {
				continue
			}
			appid, _ := st.Get("appid")
			name, _ := st.Get("name")
			if isNonGame(name) {
				continue
			}
			installdir, _ := st.Get("installdir")
			games = append(games, models.Game{
				AppID:      appid,
				Name:       name,
				InstallDir: installdir,
				LibraryID:  lib.ID,
				FullPath:   filepath.Join(steamapps, "common", installdir),
				Favorite:   favs[appid],
			})
		}
	}
	sort.Slice(games, func(i, j int) bool {
		fi, fj := games[i].Favorite, games[j].Favorite
		if fi != fj {
			return fi // i preferiti vengono prima
		}
		return games[i].Name < games[j].Name
	})
	return games, nil
}

// ---------------------------------------------------------------------------
// Scoperta processi Proton in esecuzione (target di iniezione)
// ---------------------------------------------------------------------------

// wineSystemExes sono gli eseguibili di servizio di Wine/Steam che non sono il
// vero gioco e vanno quindi esclusi dal target di iniezione.
var wineSystemExes = map[string]bool{
	"services.exe":            true,
	"explorer.exe":            true,
	"winedevice.exe":          true,
	"plugplay.exe":            true,
	"rpcss.exe":               true,
	"svchost.exe":             true,
	"conhost.exe":             true,
	"tabtip.exe":              true,
	"wineboot.exe":            true,
	"start.exe":               true,
	"cmd.exe":                 true,
	"steam.exe":               true,
	"steamwebhelper.exe":      true,
	"gameoverlayui.exe":       true,
	"steamerrorreporter.exe":  true,
	"crashhandler.exe":        true,
	"iscriptevaluator.exe":    true,
}

// GetRunningProtonProcesses scandisce /proc e mappa ogni processo lanciato da
// Steam al suo AppID leggendo /proc/<pid>/environ. Marca come IsGameExe il
// processo che esegue il vero .exe del gioco sotto Proton.
func (s *Scanner) GetRunningProtonProcesses() ([]models.ProtonProcess, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("lettura /proc: %w", err)
	}

	// Mappa AppID -> nome gioco per arricchire l'output (best effort).
	nameByApp := map[string]string{}
	if games, err := s.GetInstalledGames(); err == nil {
		for _, g := range games {
			nameByApp[g.AppID] = g.Name
		}
	}

	var procs []models.ProtonProcess
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue // non è una directory di processo
		}
		base := filepath.Join("/proc", e.Name())

		environ, err := os.ReadFile(filepath.Join(base, "environ"))
		if err != nil {
			continue // processo terminato o non di proprietà dell'utente
		}
		appid := envValue(environ, "SteamAppId")
		if appid == "" {
			appid = envValue(environ, "SteamGameId")
		}
		if appid == "" || appid == "0" {
			continue
		}

		cmdRaw, _ := os.ReadFile(filepath.Join(base, "cmdline"))
		cmd := strings.TrimSpace(strings.ReplaceAll(string(cmdRaw), "\x00", " "))
		exe, _ := os.Readlink(filepath.Join(base, "exe"))

		procs = append(procs, models.ProtonProcess{
			PID:       pid,
			AppID:     appid,
			Name:      nameByApp[appid],
			Exe:       exe,
			Cmdline:   cmd,
			IsGameExe: isGameExe(cmd),
		})
	}

	sort.Slice(procs, func(i, j int) bool {
		if procs[i].AppID != procs[j].AppID {
			return procs[i].AppID < procs[j].AppID
		}
		return procs[i].PID < procs[j].PID
	})
	return procs, nil
}

// envValue cerca "key=" nel blocco environ (variabili separate da NUL).
func envValue(environ []byte, key string) string {
	prefix := key + "="
	for _, kv := range strings.Split(string(environ), "\x00") {
		if v, ok := strings.CutPrefix(kv, prefix); ok {
			return v
		}
	}
	return ""
}

// isGameExe stabilisce, in modo euristico, se la cmdline corrisponde al vero
// eseguibile del gioco (un .exe Windows che non sia un servizio Wine/Steam).
func isGameExe(cmdline string) bool {
	lower := strings.ToLower(cmdline)
	idx := strings.LastIndex(lower, ".exe")
	if idx < 0 {
		return false
	}
	// Estrai il basename dell'.exe immediatamente prima dell'estensione.
	seg := lower[:idx+4]
	if cut := strings.LastIndexAny(seg, "/\\ "); cut >= 0 {
		seg = seg[cut+1:]
	}
	return !wineSystemExes[seg]
}
