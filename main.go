package main

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

const (
	lastFMAPIURL    = "http://ws.audioscrobbler.com/2.0/"
	subsonicAPIPath = "/rest/search3.view"
	defaultTimeout  = 10 * time.Second
	maxRetries      = 3
	retryDelay      = 1 * time.Second
	maxRecommendations = 5
	lastFMAlbumLimit = 200
)

type Album struct {
	Name   string `json:"name"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
	URL string `json:"url"`
}

type Topalbums struct {
	Album []Album `json:"album"`
}

type LastFMResponse struct {
	Topalbums Topalbums `json:"topalbums"`
}

type SubsonicResponse struct {
	SubsonicResponse struct {
		SearchResult3 struct {
			Album []struct {
				Title  string `json:"name"`
				Artist string `json:"artist"`
			} `json:"album"`
		} `json:"searchResult3"`
	} `json:"subsonic-response"`
}

type Config struct {
	LastFMAPIKey   string
	LastFMUser     string
	SubsonicServer string
	SubsonicUser   string
	SubsonicPass   string
}

var httpClient = createHTTPClient()

type ProgressIndicator struct {
	mu       sync.Mutex
	active   bool
	message  string
	current  int
	total    int
	showBar  bool
	stopChan chan bool
}

func NewSpinner(message string) *ProgressIndicator {
	return &ProgressIndicator{
		message:  message,
		showBar:  false,
		stopChan: make(chan bool),
	}
}

func NewProgressBar(message string, total int) *ProgressIndicator {
	return &ProgressIndicator{
		message:  message,
		total:    total,
		showBar:  true,
		stopChan: make(chan bool),
	}
}

func (p *ProgressIndicator) Start() {
	p.mu.Lock()
	p.active = true
	p.mu.Unlock()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		spinChars := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
		i := 0

		for {
			select {
			case <-p.stopChan:
				return
			case <-ticker.C:
				p.mu.Lock()
				if !p.active {
					p.mu.Unlock()
					return
				}
				
				if p.showBar {
					percent := float64(p.current) / float64(p.total) * 100
					barWidth := 30
					filled := int(float64(barWidth) * percent / 100)
					bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
					fmt.Printf("\r%s [%s] %d/%d (%.1f%%)", p.message, bar, p.current, p.total, percent)
				} else {
					fmt.Printf("\r%s %c", p.message, spinChars[i%len(spinChars)])
					i++
				}
				p.mu.Unlock()
			}
		}
	}()
}

func (p *ProgressIndicator) Update(current int) {
	p.mu.Lock()
	p.current = current
	p.mu.Unlock()
}

func (p *ProgressIndicator) Stop() {
	p.mu.Lock()
	p.active = false
	p.mu.Unlock()
	
	close(p.stopChan)
	fmt.Print("\r" + strings.Repeat(" ", 80) + "\r")
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cfg := loadConfig()
	
	spinner := NewSpinner("Fetching Last.fm top albums...")
	spinner.Start()
	albums, err := fetchLastFMTopAlbums(ctx, cfg)
	spinner.Stop()
	
	if err != nil {
		fmt.Printf("Error fetching Last.fm albums: %v\n", err)
		os.Exit(1)
	}
	
	recommendation := findMissingAlbums(ctx, cfg, albums)
	printRecommendation(recommendation)
}

func loadConfig() *Config {
	cfg := &Config{
		LastFMAPIKey:   os.Getenv("LASTFM_API_KEY"),
		LastFMUser:     os.Getenv("LASTFM_USER"),
		SubsonicServer: os.Getenv("SUBSONIC_SERVER"),
		SubsonicUser:   os.Getenv("SUBSONIC_USER"),
		SubsonicPass:   os.Getenv("SUBSONIC_PASSWORD"),
	}

	missing := []string{}
	if cfg.LastFMAPIKey == "" {
		missing = append(missing, "LASTFM_API_KEY")
	}
	if cfg.LastFMUser == "" {
		missing = append(missing, "LASTFM_USER")
	}
	if cfg.SubsonicServer == "" {
		missing = append(missing, "SUBSONIC_SERVER")
	}
	if cfg.SubsonicUser == "" {
		missing = append(missing, "SUBSONIC_USER")
	}
	if cfg.SubsonicPass == "" {
		missing = append(missing, "SUBSONIC_PASSWORD")
	}
	if len(missing) > 0 {
		fmt.Printf("Missing: %v\n", missing)
		os.Exit(1)
	}

	return cfg
}

func fetchLastFMTopAlbums(ctx context.Context, cfg *Config) ([]Album, error) {
	url := fmt.Sprintf("%s?method=user.gettopalbums&user=%s&api_key=%s&format=json&period=12month&limit=%d",
		lastFMAPIURL, cfg.LastFMUser, cfg.LastFMAPIKey, lastFMAlbumLimit)

	var resp *http.Response
	var err error

	for i := range maxRetries {
		req, reqErr := http.NewRequestWithContext(ctx, "GET", url, nil)
		if reqErr != nil {
			err = reqErr
			continue
		}

		resp, err = httpClient.Do(req)

		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay):
			}
		}
	}

	if resp == nil {
		return nil, fmt.Errorf("failed to get response from Last.fm after %d retries: %w", maxRetries, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Last.fm API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var lastFMResp LastFMResponse
	err = json.Unmarshal(body, &lastFMResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal Last.fm response: %w", err)
	}

	return lastFMResp.Topalbums.Album, nil
}

func findMissingAlbums(ctx context.Context, cfg *Config, albums []Album) []*Album {
	missing := make([]*Album, 0, maxRecommendations)
	ignoredURLs := loadIgnoredURLs()

	progress := NewProgressBar("Checking albums in library...", len(albums))
	progress.Start()
	defer progress.Stop()

	for i, album := range albums {
		progress.Update(i + 1)
		
		if isURLIgnored(album.URL, ignoredURLs) {
			continue
		}

		exists, err := checkSubsonic(ctx, cfg, album)
		if err != nil {
			continue
		}
		if !exists {
			missing = append(missing, &album)
			if len(missing) >= maxRecommendations {
				break
			}
		}
	}
	return missing
}

func checkSubsonic(ctx context.Context, cfg *Config, album Album) (bool, error) {
	salt := time.Now().Format("20060102150405")
	token := md5.Sum([]byte(cfg.SubsonicPass + salt))
	tokenStr := hex.EncodeToString(token[:])

	query := url.QueryEscape(cleanString(album.Name))
	url := fmt.Sprintf("%s%s?u=%s&t=%s&s=%s&v=1.16.1&c=albumcheck&f=json&query=%s",
		cfg.SubsonicServer, subsonicAPIPath,
		url.QueryEscape(cfg.SubsonicUser), // Encode username
		tokenStr,
		salt,
		query)

	var resp *http.Response
	var err error

	for i := range maxRetries {
		req, reqErr := http.NewRequestWithContext(ctx, "GET", url, nil)
		if reqErr != nil {
			err = reqErr
			continue
		}

		resp, err = httpClient.Do(req)

		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-time.After(retryDelay):
			}
		}
	}

	if resp == nil {
		return false, fmt.Errorf("failed to get response from Subsonic after %d retries: %w", maxRetries, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Subsonic API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read Subsonic response body: %w", err)
	}

	var subsonicResp SubsonicResponse
	err = json.Unmarshal(body, &subsonicResp)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal Subsonic response: %w", err)
	}

	for _, a := range subsonicResp.SubsonicResponse.SearchResult3.Album {
		if strings.EqualFold(cleanString(a.Title), cleanString(album.Name)) && strings.EqualFold(cleanString(a.Artist), cleanString(album.Artist.Name)) {
			return true, nil
		}
	}
	return false, nil
}

func cleanString(s string) string {
	// Trim leading/trailing spaces
	cleaned := strings.TrimSpace(s)

	// Step 1: Remove everything in brackets at the end
	re := regexp.MustCompile(`\([^)]*\)$`)
	cleaned = re.ReplaceAllString(cleaned, "")

	// Step 2: Remove non-letters, non-periods, and non-spaces
	re = regexp.MustCompile(`[^\d\p{L} ]`)
	cleaned = re.ReplaceAllString(cleaned, "")

	// Step 3: Collapse multiple spaces to one
	re = regexp.MustCompile(`\s+`)
	cleaned = re.ReplaceAllString(cleaned, " ")

	// Trim leading/trailing spaces
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}

func printRecommendation(albums []*Album) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	if len(albums) == 0 {
		fmt.Println("All top albums exist in your Subsonic library!")
		return
	}

	fmt.Fprintln(w, "RECOMMENDED ALBUMS\t")
	fmt.Fprintln(w, strings.Repeat("=", 80))
	for i, album := range albums {
		fmt.Fprintf(w, "%d. %s - %s\n", i+1, album.Artist.Name, album.Name)
		fmt.Fprintf(w, "   Last.fm URL:\t%s\n", album.URL)
		fmt.Fprintln(w, strings.Repeat("-", 80))
	}
}

func createHTTPClient() *http.Client {
	skipVerify := os.Getenv("INSECURE_SKIP_VERIFY") == "true"

	return &http.Client{
		Timeout: defaultTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipVerify,
			},
		},
	}
}

func loadIgnoredURLs() []string {
	filePath := os.Getenv("IGNORE_FILE")
	if filePath == "" {
		return []string{} // No ignore file specified
	}

	file, err := os.Open(filePath)
	if err != nil {
		// Handle the error, e.g., log it or print a warning
		fmt.Printf("Warning: Could not open ignore file: %v\n", err)
		return []string{} // Return an empty slice, effectively ignoring the error
	}
	defer file.Close()

	var ignoredURLs []string
	content, _ := io.ReadAll(file)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line) // Remove leading/trailing whitespace
		if line != "" {                // Ignore empty lines
			ignoredURLs = append(ignoredURLs, line)
		}
	}

	return ignoredURLs
}

func isURLIgnored(url string, ignoredURLs []string) bool {
	return slices.Contains(ignoredURLs, url)
}
