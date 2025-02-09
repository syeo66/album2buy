# Album Gap Analyzer

A Go utility that identifies top Last.fm albums missing from your Subsonic library.

My reasoning: I’ve been shifting away from Spotify because the platform feels increasingly cluttered with AI-generated content, pays artists poorly, and aligns with business practices I no longer wish to support. Instead, I’ve returned to purchasing downloadable music. Thanks to my past scrobbling history, I can now identify gaps in my offline collection—essentially pinpointing which albums I streamed on Spotify but haven’t yet acquired. Put simply: it’s a way to systematically decide, “What should I buy next based on my listening habits?”

## Features
- **Last.fm Integration**: Fetches your top 50 albums from the last year
- **Subsonic Compatibility**: Checks against your Subsonic music library
- **Smart Recommendations**: Identifies up to 5 missing albums
- **Retry Logic**: Robust error handling with 3 retry attempts

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

## Requirements
- Go 1.21+
- dotenvx (`go install github.com/dotenvx/dotenvx@latest`)
- Valid Last.fm API credentials
- Subsonic server (1.16.1+ recommended)

## Security Note
The client currently skips SSL certificate verification (`InsecureSkipVerify: true`). Since the data is not really what I'd consider sensitive I guess that's okay. But you should be aware of it. 

## License
MIT © Red Ochsenbein 2025

