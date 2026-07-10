package backend

import "testing"

// Smoke test: gira contro il filesystem reale per validare parser e scoperta.
// Eseguire con:  go test ./backend/ -run Smoke -v
func TestSmokeScan(t *testing.T) {
	s := NewScanner()

	st := s.GetSteamStatus()
	t.Logf("SteamStatus: found=%v root=%s vdf=%s", st.Found, st.SteamPath, st.VDFPath)
	if !st.Found {
		t.Skipf("Steam non trovato, motivi: %v", st.Reasons)
	}

	libs, err := s.GetLibraries()
	if err != nil {
		t.Fatalf("GetLibraries: %v", err)
	}
	t.Logf("Librerie trovate: %d", len(libs))
	for _, l := range libs {
		t.Logf("  [%s] %s -> %s", l.ID, l.Name, l.Path)
	}

	games, err := s.GetInstalledGames()
	if err != nil {
		t.Fatalf("GetInstalledGames: %v", err)
	}
	t.Logf("Giochi installati: %d", len(games))
	for i, g := range games {
		if i >= 10 {
			t.Logf("  ... (+%d altri)", len(games)-10)
			break
		}
		t.Logf("  %-8s %-40s %s", g.AppID, g.Name, g.FullPath)
	}

	procs, err := s.GetRunningProtonProcesses()
	if err != nil {
		t.Fatalf("GetRunningProtonProcesses: %v", err)
	}
	t.Logf("Processi Proton attivi: %d", len(procs))
	for _, p := range procs {
		t.Logf("  pid=%-7d app=%-8s gameExe=%-5v name=%q exe=%s", p.PID, p.AppID, p.IsGameExe, p.Name, p.Exe)
	}
}
