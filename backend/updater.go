package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"Protonaut/backend/models"
)

const releasesAPI = "https://api.github.com/repos/Eloreden/protonaut/releases/latest"

// ghRelease è il sottoinsieme della risposta GitHub Releases che ci serve.
type ghRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
}

// IsAppImage indica se l'app gira come AppImage (variabile APPIMAGE impostata
// dal runtime AppImage). Solo in questo caso l'update in-app ha senso.
func IsAppImage() bool {
	return os.Getenv("APPIMAGE") != ""
}

// appImageUpdateTool cerca lo strumento CLI di AppImageUpdate nel PATH.
func appImageUpdateTool() string {
	for _, name := range []string{"appimageupdatetool", "AppImageUpdate"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// CheckForUpdate interroga l'API GitHub per l'ultima release e la confronta con
// la versione corrente. current è la versione incisa nel binario a build-time.
func CheckForUpdate(current string) (models.UpdateStatus, error) {
	status := models.UpdateStatus{
		CurrentVersion: current,
		IsAppImage:     IsAppImage(),
		CanSelfUpdate:  IsAppImage() && appImageUpdateTool() != "",
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, releasesAPI, nil)
	if err != nil {
		return status, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Protonaut")

	resp, err := client.Do(req)
	if err != nil {
		return status, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return status, fmt.Errorf("GitHub API HTTP %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return status, err
	}

	status.LatestVersion = rel.TagName
	status.ReleaseURL = rel.HTMLURL
	status.ReleaseNotes = rel.Body
	// Se la versione corrente non è semver ("dev" in sviluppo, o un build senza
	// tag) non si può confrontare: meglio non segnalare un aggiornamento fasullo.
	status.UpdateAvailable = isSemver(current) && compareVersions(rel.TagName, current) > 0
	return status, nil
}

// ApplyUpdate lancia appimageupdatetool sull'AppImage corrente, aggiornandola
// in place tramite il delta zsync. Restituisce l'output dello strumento.
func ApplyUpdate() (string, error) {
	if !IsAppImage() {
		return "", errors.New(T("err.updateAppImageOnly"))
	}
	tool := appImageUpdateTool()
	if tool == "" {
		return "", errors.New(T("err.appImageUpdateMissing"))
	}
	appimage := os.Getenv("APPIMAGE")

	cmd := exec.Command(tool, appimage)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("appimageupdatetool: %w", err)
	}
	return string(out), nil
}

// compareVersions confronta due versioni semver-like ("v1.2.3" o "1.2.3").
// Ritorna >0 se a>b, <0 se a<b, 0 se equivalenti. Le componenti mancanti o non
// numeriche valgono 0.
func compareVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

// isSemver dice se v ha la forma "vX.Y.Z" / "X.Y.Z" (eventuale suffisso
// -pre/+build ammesso), cioè se ha senso confrontarla con il tag di una release.
func isSemver(v string) bool {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

func parseVersion(v string) [3]int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	// scarta eventuale suffisso di pre-release/build (es. 1.2.3-rc1, 1.2.3-2-gabc1234)
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		n, _ := strconv.Atoi(strings.TrimSpace(part))
		out[i] = n
	}
	return out
}
