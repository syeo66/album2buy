package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestFindMissingAlbumsIntegration(t *testing.T) {
	subsonicMockResponse := SubsonicResponse{
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
					{Title: "Existing Album", Artist: "Existing Artist"},
				},
			},
		},
	}
	
	jsonResponse, _ := json.Marshal(subsonicMockResponse)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}))
	defer server.Close()
	
	httpClient := NewHTTPClient()
	subsonicClient := &SubsonicClient{
		httpClient: httpClient,
		server:     server.URL,
		user:       "testuser",
		password:   "testpass",
	}
	
	albums := []Album{
		{
			Name: "Existing Album",
			Artist: struct {
				Name string `json:"name"`
			}{Name: "Existing Artist"},
			URL: "https://www.last.fm/music/Existing+Artist/Existing+Album",
		},
		{
			Name: "Missing Album 1",
			Artist: struct {
				Name string `json:"name"`
			}{Name: "Missing Artist 1"},
			URL: "https://www.last.fm/music/Missing+Artist+1/Missing+Album+1",
		},
		{
			Name: "Missing Album 2",
			Artist: struct {
				Name string `json:"name"`
			}{Name: "Missing Artist 2"},
			URL: "https://www.last.fm/music/Missing+Artist+2/Missing+Album+2",
		},
	}
	
	ctx := context.Background()
	missing := findMissingAlbums(ctx, subsonicClient, albums)
	
	if len(missing) != 2 {
		t.Errorf("Expected 2 missing albums, got %d", len(missing))
	}
	
	if missing[0].Name != "Missing Album 1" {
		t.Errorf("Expected first missing album 'Missing Album 1', got '%s'", missing[0].Name)
	}
	
	if missing[1].Name != "Missing Album 2" {
		t.Errorf("Expected second missing album 'Missing Album 2', got '%s'", missing[1].Name)
	}
}

func TestFindMissingAlbumsWithIgnoredURLs(t *testing.T) {
	subsonicMockResponse := SubsonicResponse{
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
				}{},
			},
		},
	}
	
	jsonResponse, _ := json.Marshal(subsonicMockResponse)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}))
	defer server.Close()
	
	httpClient := NewHTTPClient()
	subsonicClient := &SubsonicClient{
		httpClient: httpClient,
		server:     server.URL,
		user:       "testuser",
		password:   "testpass",
	}
	
	albums := []Album{
		{
			Name: "Missing Album 1",
			Artist: struct {
				Name string `json:"name"`
			}{Name: "Missing Artist 1"},
			URL: "https://www.last.fm/music/Missing+Artist+1/Missing+Album+1",
		},
		{
			Name: "Missing Album 2",
			Artist: struct {
				Name string `json:"name"`
			}{Name: "Missing Artist 2"},
			URL: "https://www.last.fm/music/Missing+Artist+2/Missing+Album+2",
		},
	}
	
	// Mock the ignored URLs by creating a temporary ignore file
	tmpFile, err := os.CreateTemp("", "ignore_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	
	ignoreContent := "https://www.last.fm/music/Missing+Artist+1/Missing+Album+1"
	if _, err := tmpFile.WriteString(ignoreContent); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	
	originalIgnoreFile := os.Getenv("IGNORE_FILE")
	os.Setenv("IGNORE_FILE", tmpFile.Name())
	defer func() {
		if originalIgnoreFile == "" {
			os.Unsetenv("IGNORE_FILE")
		} else {
			os.Setenv("IGNORE_FILE", originalIgnoreFile)
		}
	}()
	
	ctx := context.Background()
	missing := findMissingAlbums(ctx, subsonicClient, albums)
	
	if len(missing) != 1 {
		t.Errorf("Expected 1 missing album (after ignoring), got %d", len(missing))
	}
	
	if missing[0].Name != "Missing Album 2" {
		t.Errorf("Expected missing album 'Missing Album 2', got '%s'", missing[0].Name)
	}
}

func TestFindMissingAlbumsMaxRecommendations(t *testing.T) {
	subsonicMockResponse := SubsonicResponse{
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
				}{},
			},
		},
	}
	
	jsonResponse, _ := json.Marshal(subsonicMockResponse)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}))
	defer server.Close()
	
	httpClient := NewHTTPClient()
	subsonicClient := &SubsonicClient{
		httpClient: httpClient,
		server:     server.URL,
		user:       "testuser",
		password:   "testpass",
	}
	
	albums := []Album{}
	for i := 0; i < 10; i++ {
		albums = append(albums, Album{
			Name: fmt.Sprintf("Missing Album %d", i+1),
			Artist: struct {
				Name string `json:"name"`
			}{Name: fmt.Sprintf("Missing Artist %d", i+1)},
			URL: fmt.Sprintf("https://www.last.fm/music/Missing+Artist+%d/Missing+Album+%d", i+1, i+1),
		})
	}
	
	ctx := context.Background()
	missing := findMissingAlbums(ctx, subsonicClient, albums)
	
	if len(missing) != maxRecommendations {
		t.Errorf("Expected %d missing albums (max recommendations), got %d", maxRecommendations, len(missing))
	}
}

func TestEndToEndWorkflow(t *testing.T) {
	lastFMResponse := LastFMResponse{
		Topalbums: Topalbums{
			Album: []Album{
				{
					Name: "Album in Library",
					Artist: struct {
						Name string `json:"name"`
					}{Name: "Artist in Library"},
					URL: "https://www.last.fm/music/Artist+in+Library/Album+in+Library",
				},
				{
					Name: "Missing Album",
					Artist: struct {
						Name string `json:"name"`
					}{Name: "Missing Artist"},
					URL: "https://www.last.fm/music/Missing+Artist/Missing+Album",
				},
			},
		},
	}
	
	subsonicResponseWithAlbum := SubsonicResponse{
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
					{Title: "Album in Library", Artist: "Artist in Library"},
				},
			},
		},
	}
	
	subsonicResponseEmpty := SubsonicResponse{
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
				}{},
			},
		},
	}
	
	lastFMServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse, _ := json.Marshal(lastFMResponse)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}))
	defer lastFMServer.Close()
	
	subsonicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var jsonResponse []byte
		query := r.URL.Query().Get("query")
		
		// The cleanString function will be applied to the query, so we need to match the cleaned version
		if strings.Contains(query, "Album") && strings.Contains(query, "Library") {
			jsonResponse, _ = json.Marshal(subsonicResponseWithAlbum)
		} else {
			jsonResponse, _ = json.Marshal(subsonicResponseEmpty)
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}))
	defer subsonicServer.Close()
	
	httpClient := NewHTTPClient()
	lastFMClient := &LastFMClient{
		httpClient: httpClient,
		apiKey:     "test-key",
		baseURL:    lastFMServer.URL + "/",
	}
	
	subsonicClient := &SubsonicClient{
		httpClient: httpClient,
		server:     subsonicServer.URL,
		user:       "testuser",
		password:   "testpass",
	}
	
	ctx := context.Background()
	
	albums, err := lastFMClient.GetTopAlbums(ctx, "testuser", 10)
	if err != nil {
		t.Fatal(err)
	}
	
	if len(albums) != 2 {
		t.Errorf("Expected 2 albums from Last.fm, got %d", len(albums))
	}
	
	missing := findMissingAlbums(ctx, subsonicClient, albums)
	
	if len(missing) != 1 {
		t.Errorf("Expected 1 missing album, got %d", len(missing))
	}
	
	if missing[0].Name != "Missing Album" {
		t.Errorf("Expected missing album 'Missing Album', got '%s'", missing[0].Name)
	}
	
	if missing[0].Artist.Name != "Missing Artist" {
		t.Errorf("Expected missing artist 'Missing Artist', got '%s'", missing[0].Artist.Name)
	}
}