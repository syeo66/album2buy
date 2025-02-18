package main

import (
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
	"strings"
	"text/tabwriter"
	"time"
)

const (
	lastFMAPIURL    = "http://ws.audioscrobbler.com/2.0/"
	subsonicAPIPath = "/rest/search3.view"
	defaultTimeout  = 10 * time.Second
	maxRetries      = 3
	retryDelay      = 1 * time.Second
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

func main() {
	cfg := loadConfig()
	albums := fetchLastFMTopAlbums(cfg)
	recommendation := findMissingAlbums(cfg, albums)
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

func fetchLastFMTopAlbums(cfg *Config) []Album {
	url := fmt.Sprintf("%s?method=user.gettopalbums&user=%s&api_key=%s&format=json&period=12month&limit=200",
		lastFMAPIURL, cfg.LastFMUser, cfg.LastFMAPIKey)

	var resp *http.Response
	var err error

	for i := 0; i < maxRetries; i++ {
		resp, err = httpClient.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(retryDelay)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var lastFMResp LastFMResponse
	err = json.Unmarshal(body, &lastFMResp)

	if err != nil {
		fmt.Printf("could not unmarshal last fm data: %v+", err)
	}

	return lastFMResp.Topalbums.Album
}

func findMissingAlbums(cfg *Config, albums []Album) []*Album {
	missing := make([]*Album, 0, 5)

	ignoredURLs := loadIgnoredURLs()

	for _, album := range albums {
		if isURLIgnored(album.URL, ignoredURLs) {
			continue
		}

		exists, err := checkSubsonic(cfg, album)
		if err != nil {
			continue
		}
		if !exists {
			missing = append(missing, &album)
			if len(missing) >= 5 {
				break
			}
		}
	}
	return missing
}

func checkSubsonic(cfg *Config, album Album) (bool, error) {
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

	for i := 0; i < maxRetries; i++ {
		resp, err = httpClient.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(retryDelay)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var subsonicResp SubsonicResponse
	err = json.Unmarshal(body, &subsonicResp)

	if err != nil {
		fmt.Printf("could not unmarshal subsonic data: %v+", err)
	}

	for _, a := range subsonicResp.SubsonicResponse.SearchResult3.Album {
		if strings.EqualFold(cleanString(a.Title), cleanString(album.Name)) && strings.EqualFold(cleanString(a.Artist), cleanString(album.Artist.Name)) {
			return true, nil
		}
	}
	return false, nil
}

func cleanString(s string) string {
	// Step 1: Remove non-letters, non-periods, and non-spaces
	re := regexp.MustCompile(`[^\p{L} ]`)
	cleaned := re.ReplaceAllString(s, "")

	// Step 2: Collapse multiple spaces to one
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
	for _, ignoredURL := range ignoredURLs {
		if url == ignoredURL {
			return true
		}
	}
	return false
}
