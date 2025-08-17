package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient()
	
	if client == nil {
		t.Fatal("NewHTTPClient returned nil")
	}
	
	if client.maxRetries != maxRetries {
		t.Errorf("Expected maxRetries %d, got %d", maxRetries, client.maxRetries)
	}
	
	if client.retryDelay != retryDelay {
		t.Errorf("Expected retryDelay %v, got %v", retryDelay, client.retryDelay)
	}
	
	if client.client.Timeout != defaultTimeout {
		t.Errorf("Expected timeout %v, got %v", defaultTimeout, client.client.Timeout)
	}
}

func TestNewHTTPClientWithInsecureSkipVerify(t *testing.T) {
	os.Setenv("INSECURE_SKIP_VERIFY", "true")
	defer os.Unsetenv("INSECURE_SKIP_VERIFY")
	
	client := NewHTTPClient()
	
	transport, ok := client.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}
	
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}
}

func TestHTTPClientDoWithRetrySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()
	
	client := NewHTTPClient()
	ctx := context.Background()
	
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	
	resp, err := client.DoWithRetry(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestHTTPClientDoWithRetryFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	
	client := &HTTPClient{
		client: &http.Client{
			Timeout: 1 * time.Second,
		},
		maxRetries: 2,
		retryDelay: 10 * time.Millisecond,
	}
	
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	
	_, err = client.DoWithRetry(ctx, req)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	
	if !strings.Contains(err.Error(), "failed with status 500") {
		t.Errorf("Expected error message about status 500, got: %v", err)
	}
}

func TestHTTPClientDoWithRetryContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	
	client := &HTTPClient{
		client: &http.Client{
			Timeout: 1 * time.Second,
		},
		maxRetries: 3,
		retryDelay: 50 * time.Millisecond,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 80 * time.Millisecond)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	
	_, err = client.DoWithRetry(ctx, req)
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}
}

func TestNewLastFMClient(t *testing.T) {
	httpClient := NewHTTPClient()
	apiKey := "test-api-key"
	
	client := NewLastFMClient(httpClient, apiKey)
	
	if client == nil {
		t.Fatal("NewLastFMClient returned nil")
	}
	
	if client.apiKey != apiKey {
		t.Errorf("Expected apiKey %s, got %s", apiKey, client.apiKey)
	}
	
	if client.baseURL != lastFMAPIURL {
		t.Errorf("Expected baseURL %s, got %s", lastFMAPIURL, client.baseURL)
	}
	
	if client.httpClient != httpClient {
		t.Error("httpClient not set correctly")
	}
}

func TestLastFMClientGetTopAlbums(t *testing.T) {
	mockResponse := LastFMResponse{
		Topalbums: Topalbums{
			Album: []Album{
				{
					Name: "Test Album 1",
					Artist: struct {
						Name string `json:"name"`
					}{Name: "Test Artist 1"},
					URL: "https://www.last.fm/music/Test+Artist+1/Test+Album+1",
				},
				{
					Name: "Test Album 2",
					Artist: struct {
						Name string `json:"name"`
					}{Name: "Test Artist 2"},
					URL: "https://www.last.fm/music/Test+Artist+2/Test+Album+2",
				},
			},
		},
	}
	
	jsonResponse, _ := json.Marshal(mockResponse)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Query().Get("method"), "user.gettopalbums") {
			t.Error("Expected method=user.gettopalbums in query")
		}
		if r.URL.Query().Get("user") != "testuser" {
			t.Error("Expected user=testuser in query")
		}
		if r.URL.Query().Get("api_key") != "test-key" {
			t.Error("Expected api_key=test-key in query")
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}))
	defer server.Close()
	
	httpClient := NewHTTPClient()
	client := &LastFMClient{
		httpClient: httpClient,
		apiKey:     "test-key",
		baseURL:    server.URL + "/",
	}
	
	ctx := context.Background()
	albums, err := client.GetTopAlbums(ctx, "testuser", 10)
	if err != nil {
		t.Fatal(err)
	}
	
	if len(albums) != 2 {
		t.Errorf("Expected 2 albums, got %d", len(albums))
	}
	
	if albums[0].Name != "Test Album 1" {
		t.Errorf("Expected album name 'Test Album 1', got '%s'", albums[0].Name)
	}
	
	if albums[0].Artist.Name != "Test Artist 1" {
		t.Errorf("Expected artist name 'Test Artist 1', got '%s'", albums[0].Artist.Name)
	}
}

func TestLastFMClientGetTopAlbumsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()
	
	httpClient := NewHTTPClient()
	client := &LastFMClient{
		httpClient: httpClient,
		apiKey:     "test-key",
		baseURL:    server.URL + "/",
	}
	
	ctx := context.Background()
	_, err := client.GetTopAlbums(ctx, "testuser", 10)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	
	if !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("Expected unmarshal error, got: %v", err)
	}
}

func TestNewSubsonicClient(t *testing.T) {
	httpClient := NewHTTPClient()
	server := "https://test.example.com"
	user := "testuser"
	password := "testpass"
	
	client := NewSubsonicClient(httpClient, server, user, password)
	
	if client == nil {
		t.Fatal("NewSubsonicClient returned nil")
	}
	
	if client.server != server {
		t.Errorf("Expected server %s, got %s", server, client.server)
	}
	
	if client.user != user {
		t.Errorf("Expected user %s, got %s", user, client.user)
	}
	
	if client.password != password {
		t.Errorf("Expected password %s, got %s", password, client.password)
	}
	
	if client.httpClient != httpClient {
		t.Error("httpClient not set correctly")
	}
}

func TestSubsonicClientSearchAlbum(t *testing.T) {
	mockResponse := SubsonicResponse{
		SubsonicResponse: struct {
			SearchResult3 struct {
				Album []struct {
					Title  string `json:"name"`
					Artist string `json:"artist"`
				} `json:"album"`
			} `json:"searchResult3"`
		}{
			SearchResult3: struct {
				Album []struct {
					Title  string `json:"name"`
					Artist string `json:"artist"`
				} `json:"album"`
			}{
				Album: []struct {
					Title  string `json:"name"`
					Artist string `json:"artist"`
				}{
					{Title: "Test Album", Artist: "Test Artist"},
				},
			},
		},
	}
	
	jsonResponse, _ := json.Marshal(mockResponse)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/search3.view") {
			t.Errorf("Expected path to contain /rest/search3.view, got %s", r.URL.Path)
		}
		
		query := r.URL.Query()
		if query.Get("u") != "testuser" {
			t.Error("Expected user=testuser in query")
		}
		if query.Get("query") == "" {
			t.Error("Expected query parameter")
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}))
	defer server.Close()
	
	httpClient := NewHTTPClient()
	client := &SubsonicClient{
		httpClient: httpClient,
		server:     server.URL,
		user:       "testuser",
		password:   "testpass",
	}
	
	ctx := context.Background()
	albums, err := client.SearchAlbum(ctx, "Test Album")
	if err != nil {
		t.Fatal(err)
	}
	
	if len(albums) != 1 {
		t.Errorf("Expected 1 album, got %d", len(albums))
	}
	
	if albums[0].Title != "Test Album" {
		t.Errorf("Expected album title 'Test Album', got '%s'", albums[0].Title)
	}
	
	if albums[0].Artist != "Test Artist" {
		t.Errorf("Expected artist 'Test Artist', got '%s'", albums[0].Artist)
	}
}

func TestSubsonicClientHasAlbumTrue(t *testing.T) {
	mockResponse := SubsonicResponse{
		SubsonicResponse: struct {
			SearchResult3 struct {
				Album []struct {
					Title  string `json:"name"`
					Artist string `json:"artist"`
				} `json:"album"`
			} `json:"searchResult3"`
		}{
			SearchResult3: struct {
				Album []struct {
					Title  string `json:"name"`
					Artist string `json:"artist"`
				} `json:"album"`
			}{
				Album: []struct {
					Title  string `json:"name"`
					Artist string `json:"artist"`
				}{
					{Title: "Test Album", Artist: "Test Artist"},
				},
			},
		},
	}
	
	jsonResponse, _ := json.Marshal(mockResponse)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}))
	defer server.Close()
	
	httpClient := NewHTTPClient()
	client := &SubsonicClient{
		httpClient: httpClient,
		server:     server.URL,
		user:       "testuser",
		password:   "testpass",
	}
	
	album := Album{
		Name: "Test Album",
		Artist: struct {
			Name string `json:"name"`
		}{Name: "Test Artist"},
	}
	
	ctx := context.Background()
	hasAlbum, err := client.HasAlbum(ctx, album)
	if err != nil {
		t.Fatal(err)
	}
	
	if !hasAlbum {
		t.Error("Expected HasAlbum to return true")
	}
}

func TestSubsonicClientHasAlbumFalse(t *testing.T) {
	mockResponse := SubsonicResponse{
		SubsonicResponse: struct {
			SearchResult3 struct {
				Album []struct {
					Title  string `json:"name"`
					Artist string `json:"artist"`
				} `json:"album"`
			} `json:"searchResult3"`
		}{
			SearchResult3: struct {
				Album []struct {
					Title  string `json:"name"`
					Artist string `json:"artist"`
				} `json:"album"`
			}{
				Album: []struct {
					Title  string `json:"name"`
					Artist string `json:"artist"`
				}{
					{Title: "Different Album", Artist: "Different Artist"},
				},
			},
		},
	}
	
	jsonResponse, _ := json.Marshal(mockResponse)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}))
	defer server.Close()
	
	httpClient := NewHTTPClient()
	client := &SubsonicClient{
		httpClient: httpClient,
		server:     server.URL,
		user:       "testuser",
		password:   "testpass",
	}
	
	album := Album{
		Name: "Test Album",
		Artist: struct {
			Name string `json:"name"`
		}{Name: "Test Artist"},
	}
	
	ctx := context.Background()
	hasAlbum, err := client.HasAlbum(ctx, album)
	if err != nil {
		t.Fatal(err)
	}
	
	if hasAlbum {
		t.Error("Expected HasAlbum to return false")
	}
}

func TestCleanString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test Album", "Test Album"},
		{"Test Album (Deluxe Edition)", "Test Album"},
		{"Test Album (2021 Remaster)", "Test Album"},
		{"Test Album [Bonus Tracks]", "Test Album Bonus Tracks"},
		{"Test  Album   ", "Test Album"},
		{"Test-Album&Special", "TestAlbumSpecial"},
		{"Test@Album#123", "TestAlbum123"},
		{"   Test Album   ", "Test Album"},
		{"", ""},
		{"Test Album (Live) (Bonus)", "Test Album Live"},
	}
	
	for _, test := range tests {
		result := cleanString(test.input)
		if result != test.expected {
			t.Errorf("cleanString(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestLoadIgnoredURLsNoFile(t *testing.T) {
	os.Unsetenv("IGNORE_FILE")
	
	urls := loadIgnoredURLs()
	if len(urls) != 0 {
		t.Errorf("Expected empty slice, got %v", urls)
	}
}

func TestLoadIgnoredURLsWithFile(t *testing.T) {
	content := "https://www.last.fm/music/Artist1/Album1\nhttps://www.last.fm/music/Artist2/Album2\n\n\nhttps://www.last.fm/music/Artist3/Album3"
	
	tmpFile, err := os.CreateTemp("", "ignore_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	
	os.Setenv("IGNORE_FILE", tmpFile.Name())
	defer os.Unsetenv("IGNORE_FILE")
	
	urls := loadIgnoredURLs()
	expected := []string{
		"https://www.last.fm/music/Artist1/Album1",
		"https://www.last.fm/music/Artist2/Album2",
		"https://www.last.fm/music/Artist3/Album3",
	}
	
	if len(urls) != len(expected) {
		t.Errorf("Expected %d URLs, got %d", len(expected), len(urls))
	}
	
	for i, url := range urls {
		if url != expected[i] {
			t.Errorf("Expected URL %s, got %s", expected[i], url)
		}
	}
}

func TestIsURLIgnored(t *testing.T) {
	ignoredURLs := []string{
		"https://www.last.fm/music/Artist1/Album1",
		"https://www.last.fm/music/Artist2/Album2",
	}
	
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://www.last.fm/music/Artist1/Album1", true},
		{"https://www.last.fm/music/Artist2/Album2", true},
		{"https://www.last.fm/music/Artist3/Album3", false},
		{"", false},
	}
	
	for _, test := range tests {
		result := isURLIgnored(test.url, ignoredURLs)
		if result != test.expected {
			t.Errorf("isURLIgnored(%q) = %v, expected %v", test.url, result, test.expected)
		}
	}
}

func TestNewSpinner(t *testing.T) {
	message := "Testing..."
	spinner := NewSpinner(message)
	
	if spinner == nil {
		t.Fatal("NewSpinner returned nil")
	}
	
	if spinner.message != message {
		t.Errorf("Expected message %s, got %s", message, spinner.message)
	}
	
	if spinner.showBar {
		t.Error("Expected showBar to be false for spinner")
	}
	
	if spinner.stopChan == nil {
		t.Error("Expected stopChan to be initialized")
	}
}

func TestNewProgressBar(t *testing.T) {
	message := "Progress..."
	total := 100
	progressBar := NewProgressBar(message, total)
	
	if progressBar == nil {
		t.Fatal("NewProgressBar returned nil")
	}
	
	if progressBar.message != message {
		t.Errorf("Expected message %s, got %s", message, progressBar.message)
	}
	
	if progressBar.total != total {
		t.Errorf("Expected total %d, got %d", total, progressBar.total)
	}
	
	if !progressBar.showBar {
		t.Error("Expected showBar to be true for progress bar")
	}
	
	if progressBar.stopChan == nil {
		t.Error("Expected stopChan to be initialized")
	}
}

func TestProgressIndicatorUpdate(t *testing.T) {
	progressBar := NewProgressBar("Test", 100)
	
	progressBar.Update(50)
	
	if progressBar.current != 50 {
		t.Errorf("Expected current to be 50, got %d", progressBar.current)
	}
}

func TestLoadConfigMissingValues(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	
	originalEnv := map[string]string{
		"LASTFM_API_KEY":     os.Getenv("LASTFM_API_KEY"),
		"LASTFM_USER":        os.Getenv("LASTFM_USER"),
		"SUBSONIC_SERVER":    os.Getenv("SUBSONIC_SERVER"),
		"SUBSONIC_USER":      os.Getenv("SUBSONIC_USER"),
		"SUBSONIC_PASSWORD":  os.Getenv("SUBSONIC_PASSWORD"),
	}
	
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()
	
	for key := range originalEnv {
		os.Unsetenv(key)
	}
	
	if os.Getenv("SKIP_EXIT_TEST") == "" {
		_, err := os.Open("/dev/null")
		if err == nil {
			t.Skip("Skipping exit test to avoid process termination")
		}
	}
}

func TestLoadConfigValidValues(t *testing.T) {
	originalEnv := map[string]string{
		"LASTFM_API_KEY":     os.Getenv("LASTFM_API_KEY"),
		"LASTFM_USER":        os.Getenv("LASTFM_USER"),
		"SUBSONIC_SERVER":    os.Getenv("SUBSONIC_SERVER"),
		"SUBSONIC_USER":      os.Getenv("SUBSONIC_USER"),
		"SUBSONIC_PASSWORD":  os.Getenv("SUBSONIC_PASSWORD"),
	}
	
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()
	
	os.Setenv("LASTFM_API_KEY", "test-api-key")
	os.Setenv("LASTFM_USER", "test-user")
	os.Setenv("SUBSONIC_SERVER", "https://test.example.com")
	os.Setenv("SUBSONIC_USER", "test-subsonic-user")
	os.Setenv("SUBSONIC_PASSWORD", "test-password")
	
	cfg := loadConfig()
	
	if cfg.LastFMAPIKey != "test-api-key" {
		t.Errorf("Expected LastFMAPIKey 'test-api-key', got '%s'", cfg.LastFMAPIKey)
	}
	
	if cfg.LastFMUser != "test-user" {
		t.Errorf("Expected LastFMUser 'test-user', got '%s'", cfg.LastFMUser)
	}
	
	if cfg.SubsonicServer != "https://test.example.com" {
		t.Errorf("Expected SubsonicServer 'https://test.example.com', got '%s'", cfg.SubsonicServer)
	}
	
	if cfg.SubsonicUser != "test-subsonic-user" {
		t.Errorf("Expected SubsonicUser 'test-subsonic-user', got '%s'", cfg.SubsonicUser)
	}
	
	if cfg.SubsonicPass != "test-password" {
		t.Errorf("Expected SubsonicPass 'test-password', got '%s'", cfg.SubsonicPass)
	}
}

func TestPrintRecommendationNoAlbums(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	
	r, w, _ := os.Pipe()
	os.Stdout = w
	
	go func() {
		defer w.Close()
		printRecommendation([]*Album{})
	}()
	
	io.Copy(&buf, r)
	os.Stdout = oldStdout
	
	output := buf.String()
	if !strings.Contains(output, "All top albums exist in your Subsonic library!") {
		t.Errorf("Expected message about all albums existing, got: %s", output)
	}
}

func TestPrintRecommendationWithAlbums(t *testing.T) {
	albums := []*Album{
		{
			Name: "Test Album 1",
			Artist: struct {
				Name string `json:"name"`
			}{Name: "Test Artist 1"},
			URL: "https://www.last.fm/music/Test+Artist+1/Test+Album+1",
		},
		{
			Name: "Test Album 2",
			Artist: struct {
				Name string `json:"name"`
			}{Name: "Test Artist 2"},
			URL: "https://www.last.fm/music/Test+Artist+2/Test+Album+2",
		},
	}
	
	var buf bytes.Buffer
	oldStdout := os.Stdout
	
	r, w, _ := os.Pipe()
	os.Stdout = w
	
	go func() {
		defer w.Close()
		printRecommendation(albums)
	}()
	
	io.Copy(&buf, r)
	os.Stdout = oldStdout
	
	output := buf.String()
	
	if !strings.Contains(output, "RECOMMENDED ALBUMS") {
		t.Error("Expected 'RECOMMENDED ALBUMS' in output")
	}
	
	if !strings.Contains(output, "Test Artist 1 - Test Album 1") {
		t.Error("Expected first album in output")
	}
	
	if !strings.Contains(output, "Test Artist 2 - Test Album 2") {
		t.Error("Expected second album in output")
	}
	
	if !strings.Contains(output, "https://www.last.fm/music/Test+Artist+1/Test+Album+1") {
		t.Error("Expected first album URL in output")
	}
}