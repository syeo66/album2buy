# Album2Buy 

A Go utility that identifies top Last.fm albums missing from your Subsonic library.

My reasoning: I’ve been shifting away from Spotify because the platform feels increasingly cluttered with AI-generated content, pays artists poorly, and aligns with business practices I no longer wish to support. Instead, I’ve returned to purchasing downloadable music. Thanks to my past scrobbling history, I can now identify gaps in my offline collection—essentially pinpointing which albums I streamed on Spotify but haven’t yet acquired. Put simply: it’s a way to systematically decide, “What should I buy next based on my listening habits?”

## Features
- **Last.fm Integration**: Fetches your top 200 albums from the last year
- **Subsonic Compatibility**: Checks against your Subsonic music library
- **Smart Recommendations**: Identifies up to 5 missing albums
- **Retry Logic**: Robust error handling with 3 retry attempts
- **Modular Architecture**: Clean separation between HTTP clients and API logic
- **Progress Indicators**: Visual feedback with spinners and progress bars
- **Comprehensive Testing**: 73.9% test coverage with unit and integration tests

## Installation

Requires Go 1.21+

```bash
git clone https://github.com/syeo66/album2buy.git
cd album2buy
go build -o album2buy *.go
```

Install dotenvx for environment management

```bash
curl -fsS https://dotenvx.sh | sh
```

## Configuration
Create `.env` file:

```env
LASTFM_API_KEY=your_api_key_here
LASTFM_USER=your_lastfm_username
SUBSONIC_SERVER=https://your.subsonic.server
SUBSONIC_USER=your_subsonic_user
SUBSONIC_PASSWORD=your_subsonic_password
```

## Usage

```bash
./run.sh # Uses dotenvx to load environment variables
```

Sample output:
```
RECOMMENDED ALBUMS
================================================================================
1. Dream Theater - Parasomnia (24-bit HD audio)
   Last.fm URL:  https://www.last.fm/music/Dream+Theater/Parasomnia+(24-bit+HD+audio)
--------------------------------------------------------------------------------
2. Chris Haigh - Massive Rocktronica - Gothic Storm
   Last.fm URL:  https://www.last.fm/music/Chris+Haigh/Massive+Rocktronica+-+Gothic+Storm
--------------------------------------------------------------------------------
3. Jeremy Soule - The Elder Scrolls V: Skyrim (Original Game Soundtrack)
   Last.fm URL:  https://www.last.fm/music/Jeremy+Soule/The+Elder+Scrolls+V:+Skyrim+(Original+Game+Soundtrack)
--------------------------------------------------------------------------------
4. Poppy - New Way Out
   Last.fm URL:  https://www.last.fm/music/Poppy/New+Way+Out
--------------------------------------------------------------------------------
5. Blue Stahli - Obsidian
   Last.fm URL:  https://www.last.fm/music/Blue+Stahli/Obsidian
--------------------------------------------------------------------------------
```

## Environment Variables
| Variable | Description |
|----------|-------------|
| `LASTFM_API_KEY` | [Last.fm API key](https://www.last.fm/api/account/create) |
| `LASTFM_USER` | Last.fm username |
| `SUBSONIC_SERVER` | Subsonic server URL (include protocol) |
| `SUBSONIC_USER` | Subsonic account username |
| `SUBSONIC_PASSWORD` | Subsonic account password |
| `IGNORE_FILE` | Path to a list of ignored Last.fm URL's |

## Architecture

The application is built with a modular architecture for maintainability and testability:

### Core Components
- **`HTTPClient`**: Centralized HTTP client with configurable retry logic and TLS settings
- **`LastFMClient`**: Dedicated client for Last.fm API operations
- **`SubsonicClient`**: Dedicated client for Subsonic API operations with authentication
- **`ProgressIndicator`**: Visual feedback system with spinners and progress bars

### Key Benefits
- **Separation of Concerns**: Each client handles its specific API responsibilities
- **Error Handling**: Consistent retry logic and detailed error messages
- **Testability**: Modular design enables easy unit testing and mocking
- **Security**: Secure credential handling and optional TLS verification

## Requirements
- Go 1.21+
- dotenvx (`go install github.com/dotenvx/dotenvx@latest`)
- Valid Last.fm API credentials
- Subsonic server (1.16.1+ recommended)

## Development

### Code Structure
```
main.go                 # Main application logic (501 lines)
main_test.go           # Unit tests for all components
integration_test.go    # End-to-end integration tests
├── HTTPClient          # Core HTTP client with retry logic
├── LastFMClient        # Last.fm API operations
├── SubsonicClient      # Subsonic API operations
├── ProgressIndicator   # Visual progress feedback
└── Utility functions   # String cleaning, configuration, etc.
```

### Testing

The project includes comprehensive test coverage with both unit and integration tests:

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage report
go test -cover ./...

# Generate detailed coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test files
go test -run TestHTTPClient
go test -run TestLastFMClient
go test -run TestSubsonicClient
```

#### Test Coverage
- **73.9% overall coverage** across all components
- **23 test cases** covering critical functionality
- **Unit tests** for individual components (HTTPClient, LastFMClient, SubsonicClient)
- **Integration tests** for end-to-end workflows
- **Mock servers** for external API testing
- **Environment variable testing** for configuration

#### Test Structure
- `main_test.go`: Unit tests for all major components
- `integration_test.go`: End-to-end workflow tests

### Build and Development
```bash
go build -o album2buy *.go  # Build application
gofmt -w .                  # Format code
go doc -all .               # View code documentation
go vet ./...                # Static analysis
```

### Code Documentation
All major types and functions include comprehensive Go documentation comments following standard conventions. Use `go doc` to explore the API documentation locally.

For detailed testing information, see [TESTING.md](TESTING.md).

## License
MIT © Red Ochsenbein 2025

