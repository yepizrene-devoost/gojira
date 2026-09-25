# GoJira TUI

A terminal user interface (TUI) application built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea) to interact with Jira Cloud.

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-blue.svg)

## Features

- 🔐 Secure authentication via API tokens
- 📋 View all boards (Scrum/Kanban)
- 🏃 Switch between sprints
- 📊 Kanban board view with columns
- 🎫 Browse issues for each board
- 🔍 Filter issues
- ⌨️ Keyboard navigation (vim-like)
- 🎨 Beautiful terminal UI with colors
- 🌐 Open issues in browser

## Prerequisites

- Go 1.21 or higher
- A Jira Cloud account
- Jira API token

## Getting Your Jira API Token

1. Go to [Atlassian API Tokens](https://id.atlassian.com/manage-profile/security/api-tokens)
2. Click **Create API token**
3. Give it a name (e.g., "GoJira TUI")
4. Copy the token

## Installation

```bash
# Clone the repository
git clone https://github.com/rene/gojira.git
cd gojira

# Build
go build -o gojira .

# Or run directly
go run .
```

## Configuration

Create a `.env` file in the project root:

```env
JIRA_EMAIL=your-email@example.com
JIRA_API_TOKEN=your-api-token-here
JIRA_DOMAIN=your-domain.atlassian.net
```

Or export environment variables:

```bash
export JIRA_EMAIL="your-email@example.com"
export JIRA_API_TOKEN="your-api-token-here"
export JIRA_DOMAIN="your-domain.atlassian.net"
```

## Usage

```bash
# Run the application
./gojira

# Or
go run .
```

### Keyboard Shortcuts

#### Board List
| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate boards |
| `Enter` | Select board |
| `q` | Quit |

#### Board View (Kanban)
| Key | Action |
|-----|--------|
| `←/→` or `h/l` | Navigate columns |
| `↑/↓` or `j/k` | Navigate issues |
| `Enter` | View issue details |
| `s` | Change sprint |
| `o` | Open in browser |
| `Esc` | Back to boards |

## Project Structure

```
gojira/
├── config/
│   └── env.go              # Environment configuration
├── jira/
│   ├── client.go           # Jira API client
│   └── types.go            # Data types
├── tui/
│   ├── common/
│   │   └── styles.go       # Shared styles and utilities
│   ├── boards/
│   │   └── model.go        # Board/Kanban view
│   ├── sprints/
│   │   └── model.go        # Sprint selector
│   ├── issues/
│   │   ├── model.go        # Issue list view
│   │   └── browser.go      # Open in browser
│   └── model.go            # Main TUI model
├── main.go                 # Entry point
├── .env.example            # Example configuration
└── README.md
```

## API Endpoints Used

- `GET /rest/api/3/serverInfo` - Test connection
- `GET /rest/api/3/project` - List projects
- `GET /rest/agile/1.0/board` - List boards
- `GET /rest/agile/1.0/board/{id}/configuration` - Board columns
- `GET /rest/agile/1.0/board/{id}/sprint` - List sprints
- `GET /rest/agile/1.0/board/{id}/sprint/{id}/issue` - Sprint issues
- `GET /rest/api/3/issue/{key}` - Issue details

## Development

```bash
# Run tests
go test ./...

# Build for production
go build -ldflags="-s -w" -o gojira .
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
