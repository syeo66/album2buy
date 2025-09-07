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
	lastFMAPIURL       = "http://ws.audioscrobbler.com/2.0/"
	subsonicAPIPath    = "/rest/search3.view"
	defaultTimeout     = 10 * time.Second
	maxRetries         = 3
	retryDelay         = 1 * time.Second
	maxRecommendations = 5
	lastFMAlbumLimit   = 500
)

// Album represents a music album from Last.fm API response
type Album struct {
	Name   string `json:"name"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
	URL string `json:"url"`
}

// Topalbums represents the top albums section of Last.fm API response
type Topalbums struct {
	Album []Album `json:"album"`
}

// LastFMResponse represents the complete Last.fm API response structure
type LastFMResponse struct {
	Topalbums Topalbums `json:"topalbums"`
}

// SubsonicResponse represents the Subsonic API search response structure
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

// Config holds all configuration values loaded from environment variables
type Config struct {
	LastFMAPIKey   string
	LastFMUser     string
	SubsonicServer string
	SubsonicUser   string
	SubsonicPass   string
}

// HTTPClient wraps http.Client with retry logic and configuration
type HTTPClient struct {
	client     *http.Client
	maxRetries int
	retryDelay time.Duration
}

// NewHTTPClient creates a new HTTPClient with default configuration and optional TLS verification skip
func NewHTTPClient() *HTTPClient {
	skipVerify := os.Getenv("INSECURE_SKIP_VERIFY") == "true"

	return &HTTPClient{
		client: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: skipVerify,
				},
			},
		},
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

// DoWithRetry executes an HTTP request with automatic retry logic on failures
func (h *HTTPClient) DoWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := range h.maxRetries {
		resp, err = h.client.Do(req)

		if err == nil && resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		if i < h.maxRetries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(h.retryDelay):
			}
		}
	}

	if resp == nil {
		return nil, fmt.Errorf("failed to get response after %d retries: %w", h.maxRetries, err)
	}

	return resp, fmt.Errorf("request failed with status %d after %d retries", resp.StatusCode, h.maxRetries)
}

// LastFMClient handles all Last.fm API operations
type LastFMClient struct {
	httpClient *HTTPClient
	apiKey     string
	baseURL    string
}

// NewLastFMClient creates a new Last.fm API client
func NewLastFMClient(httpClient *HTTPClient, apiKey string) *LastFMClient {
	return &LastFMClient{
		httpClient: httpClient,
		apiKey:     apiKey,
		baseURL:    lastFMAPIURL,
	}
}

// GetTopAlbums fetches the user's top albums from Last.fm for the past 12 months
func (l *LastFMClient) GetTopAlbums(ctx context.Context, user string, limit int) ([]Album, error) {
	url := fmt.Sprintf("%s?method=user.gettopalbums&user=%s&api_key=%s&format=json&period=12month&limit=%d",
		l.baseURL, user, l.apiKey, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := l.httpClient.DoWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Last.fm API request failed: %w", err)
	}
	defer resp.Body.Close()

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

// SubsonicClient handles all Subsonic API operations with authentication
type SubsonicClient struct {
	httpClient *HTTPClient
	server     string
	user       string
	password   string
}

// NewSubsonicClient creates a new Subsonic API client
func NewSubsonicClient(httpClient *HTTPClient, server, user, password string) *SubsonicClient {
	return &SubsonicClient{
		httpClient: httpClient,
		server:     server,
		user:       user,
		password:   password,
	}
}

// SearchAlbum searches for albums in the Subsonic library by name
func (s *SubsonicClient) SearchAlbum(ctx context.Context, albumName string) ([]struct {
	Title  string `json:"name"`
	Artist string `json:"artist"`
}, error) {
	salt := time.Now().Format("20060102150405")
	token := md5.Sum([]byte(s.password + salt))
	tokenStr := hex.EncodeToString(token[:])

	query := url.QueryEscape(cleanString(albumName))
	requestURL := fmt.Sprintf("%s%s?u=%s&t=%s&s=%s&v=1.16.1&c=albumcheck&f=json&query=%s",
		s.server, subsonicAPIPath,
		url.QueryEscape(s.user),
		tokenStr,
		salt,
		query)

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.DoWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Subsonic API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Subsonic response body: %w", err)
	}

	var subsonicResp SubsonicResponse
	err = json.Unmarshal(body, &subsonicResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal Subsonic response: %w", err)
	}

	return subsonicResp.SubsonicResponse.SearchResult3.Album, nil
}

// HasAlbum checks if a specific album exists in the Subsonic library
func (s *SubsonicClient) HasAlbum(ctx context.Context, album Album) (bool, error) {
	albums, err := s.SearchAlbum(ctx, album.Name)
	if err != nil {
		return false, err
	}

	for _, a := range albums {
		if strings.EqualFold(cleanString(a.Title), cleanString(album.Name)) &&
			strings.EqualFold(cleanString(a.Artist), cleanString(album.Artist.Name)) {
			return true, nil
		}
	}
	return false, nil
}

// ProgressIndicator provides visual feedback for long-running operations
type ProgressIndicator struct {
	mu       sync.Mutex
	active   bool
	message  string
	current  int
	total    int
	showBar  bool
	stopChan chan bool
}

// NewSpinner creates a new spinner progress indicator for indeterminate operations
func NewSpinner(message string) *ProgressIndicator {
	return &ProgressIndicator{
		message:  message,
		showBar:  false,
		stopChan: make(chan bool),
	}
}

// NewProgressBar creates a new progress bar for operations with known total
func NewProgressBar(message string, total int) *ProgressIndicator {
	return &ProgressIndicator{
		message:  message,
		total:    total,
		showBar:  true,
		stopChan: make(chan bool),
	}
}

// Start begins the progress indicator animation
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

// Update sets the current progress value for progress bars
func (p *ProgressIndicator) Update(current int) {
	p.mu.Lock()
	p.current = current
	p.mu.Unlock()
}

// Stop terminates the progress indicator and clears the display
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

	httpClient := NewHTTPClient()
	lastFMClient := NewLastFMClient(httpClient, cfg.LastFMAPIKey)
	subsonicClient := NewSubsonicClient(httpClient, cfg.SubsonicServer, cfg.SubsonicUser, cfg.SubsonicPass)

	spinner := NewSpinner("Fetching Last.fm top albums...")
	spinner.Start()
	albums, err := lastFMClient.GetTopAlbums(ctx, cfg.LastFMUser, lastFMAlbumLimit)
	spinner.Stop()

	if err != nil {
		fmt.Printf("Error fetching Last.fm albums: %v\n", err)
		os.Exit(1)
	}

	recommendation := findMissingAlbums(ctx, subsonicClient, albums)
	printRecommendation(recommendation)
}

// loadConfig loads configuration from environment variables and validates required fields
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

// ErrorStats tracks statistics about API errors during album checking
type ErrorStats struct {
	Total       int
	Successful  int
	Failed      int
	RateLimit   int
	ServerError int
	Network     int
	Other       int
}

// findMissingAlbums identifies albums from Last.fm that are not present in the Subsonic library
func findMissingAlbums(ctx context.Context, subsonicClient *SubsonicClient, albums []Album) []*Album {
	missing := make([]*Album, 0, maxRecommendations)
	ignoredURLs := loadIgnoredURLs()
	errorStats := &ErrorStats{}

	progress := NewProgressBar("Checking albums in library...", len(albums))
	progress.Start()
	defer progress.Stop()

	for i, album := range albums {
		progress.Update(i + 1)

		if isURLIgnored(album.URL, ignoredURLs) {
			continue
		}

		errorStats.Total++
		exists, err := subsonicClient.HasAlbum(ctx, album)
		if err != nil {
			errorStats.Failed++
			categorizeError(err, errorStats)
			
			// Show error details if verbose mode is enabled
			if os.Getenv("VERBOSE") == "true" {
				fmt.Printf("\nError checking album '%s - %s': %v\n", album.Artist.Name, album.Name, err)
			}
			continue
		}
		
		errorStats.Successful++
		if !exists {
			missing = append(missing, &album)
			if len(missing) >= maxRecommendations {
				break
			}
		}
	}
	
	// Report error statistics if there were any failures
	if errorStats.Failed > 0 {
		fmt.Printf("\nAPI Statistics: %d/%d requests successful", errorStats.Successful, errorStats.Total)
		if errorStats.Failed > 0 {
			fmt.Printf(" (%d failed)", errorStats.Failed)
		}
		fmt.Println()
		
		if errorStats.RateLimit > 0 {
			fmt.Printf("⚠️  Rate limiting detected (%d requests) - server may be limiting API calls\n", errorStats.RateLimit)
		}
		if errorStats.ServerError > 0 {
			fmt.Printf("⚠️  Server errors detected (%d requests) - Subsonic server may be overloaded\n", errorStats.ServerError)
		}
		if errorStats.Network > 0 {
			fmt.Printf("⚠️  Network issues detected (%d requests) - connection problems to server\n", errorStats.Network)
		}
		if errorStats.Other > 0 {
			fmt.Printf("⚠️  Other errors detected (%d requests) - run with VERBOSE=true for details\n", errorStats.Other)
		}
	}
	
	return missing
}

// categorizeError analyzes the error to determine its likely cause
func categorizeError(err error, stats *ErrorStats) {
	errStr := strings.ToLower(err.Error())
	
	// Check for rate limiting indicators
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") || 
	   strings.Contains(errStr, "too many requests") {
		stats.RateLimit++
		return
	}
	
	// Check for server errors
	if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || 
	   strings.Contains(errStr, "503") || strings.Contains(errStr, "504") ||
	   strings.Contains(errStr, "internal server error") || 
	   strings.Contains(errStr, "bad gateway") || 
	   strings.Contains(errStr, "service unavailable") || 
	   strings.Contains(errStr, "gateway timeout") {
		stats.ServerError++
		return
	}
	
	// Check for network issues
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "timeout") ||
	   strings.Contains(errStr, "network") || strings.Contains(errStr, "dial") ||
	   strings.Contains(errStr, "no such host") {
		stats.Network++
		return
	}
	
	// Everything else
	stats.Other++
}

// cleanString normalizes album and artist names for comparison by removing brackets,
// special characters, and standardizing whitespace
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

// printRecommendation displays the list of recommended albums in a formatted table
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

// loadIgnoredURLs reads a list of Last.fm URLs to ignore from the file specified
// in the IGNORE_FILE environment variable
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

// isURLIgnored checks if a URL is in the ignore list
func isURLIgnored(url string, ignoredURLs []string) bool {
	return slices.Contains(ignoredURLs, url)
}
