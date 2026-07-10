package backend

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const coverCacheDir = ".cache/Protonaut/covers"

// CoverCache mette in cache locale le copertine dei giochi. Il vecchio path
// CDN (cdn.akamai.steamstatic.com/steam/apps/{appid}/header.jpg) può
// restituire 200 con un'immagine placeholder obsoleta della community hub
// invece di un 404, per i titoli pubblicati dopo la migrazione al nuovo
// asset pipeline di Valve — quindi l'URL reale va risolto via Store API.
type CoverCache struct {
	client *http.Client
}

func NewCoverCache() *CoverCache {
	return &CoverCache{client: &http.Client{Timeout: 10 * time.Second}}
}

func coverCacheDirPath() string {
	return filepath.Join(homeDir(), coverCacheDir)
}

func coverFilePath(appId string) string {
	return filepath.Join(coverCacheDirPath(), appId+".jpg")
}

// GetCoverImage restituisce la copertina come data URI base64. Se già
// presente su disco la legge direttamente, senza chiamate di rete.
func (c *CoverCache) GetCoverImage(appId string) (string, error) {
	path := coverFilePath(appId)
	if data, err := os.ReadFile(path); err == nil {
		return toDataURI(data), nil
	}

	data, err := c.fetchAndCache(appId)
	if err != nil {
		return "", err
	}
	return toDataURI(data), nil
}

func toDataURI(data []byte) string {
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
}

// fetchAndCache interroga la Steam Store API per l'URL reale della
// copertina (header_image), la scarica e la salva su disco.
func (c *CoverCache) fetchAndCache(appId string) ([]byte, error) {
	imgURL, err := c.resolveHeaderImageURL(appId)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Get(imgURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover fetch: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	dir := coverCacheDirPath()
	if err := os.MkdirAll(dir, 0755); err == nil {
		_ = os.WriteFile(coverFilePath(appId), data, 0644)
	}

	return data, nil
}

func (c *CoverCache) resolveHeaderImageURL(appId string) (string, error) {
	apiURL := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%s", appId)
	resp, err := c.client.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("steam api: status %d", resp.StatusCode)
	}

	var result map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			HeaderImage string `json:"header_image"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	entry, ok := result[appId]
	if !ok || !entry.Success || entry.Data.HeaderImage == "" {
		return "", fmt.Errorf("nessuna immagine trovata per appId %s", appId)
	}
	return entry.Data.HeaderImage, nil
}
