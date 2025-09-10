# API Client TUI

A powerful terminal-based API client built with [Bubble Tea][bubbletea], providing a fast and efficient alternative to GUI tools like Postman or Insomnia.

![API Client TUI Screenshot](https://example.com/screenshot.png)

## Features

### Core Functionality
- Multi-panel TUI with request builder and response viewer
- HTTP Methods: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS
- Headers Management: Multi-line header input with key:value format
- Request Body: Support for JSON, form data, plain text
- Response Formatting: Auto-formatted JSON with syntax highlighting
- Real-time Status: HTTP status codes, response times, error handling

### Enhanced Features
- Request History: Automatically saves and recalls previous requests
- Environment Variables: Support for {{VARIABLE}} substitution
- Collections: Save and organize related requests
- Response Analysis: Detailed response statistics and content analysis

## Installation

### Binary Installation (mac)

```bash
curl -L https://github.com/nutcas3/api-client-tui/releases/latest/download/api-client-tui --output api-client-tui

sudo mv api-client-tui /usr/local/bin/

sudo chmod +x /usr/local/bin/api-client-tui

api-client-tui

```

### From Source

```bash
git clone https://github.com/nutcas3/api-client-tui.git
cd api-client-tui
go build -o api-client-tui
./api-client-tui
```

### Using Go Install

```bash
go install github.com/nutcas3/api-client-tui@latest
```

## Usage Guide

### Navigation
- **Tab/Shift+Tab**: Navigate between panels
- **↑/↓**: Change HTTP method when method panel is focused
- **Enter**: Send request (when URL panel is focused)
- **q**: Quit application
- **?**: Toggle help
- **Ctrl+h**: Toggle request history
- **Ctrl+e**: Toggle environment variables

### Request Building

#### URL Panel
Enter your API endpoint
```
https://jsonplaceholder.typicode.com/posts/1
```

#### Method Panel
Use ↑/↓ to select HTTP method
- Available: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS

#### Headers Panel
Enter headers (one per line)
```
Content-Type: application/json
Authorization: Bearer your-token-here
X-Custom-Header: custom-value
```

#### Body Panel
Enter request body (for POST/PUT/PATCH)
```json
{
  "title": "Test Post",
  "body": "This is a test",
  "userId": 1
}
```

### Environment Variables

Create `~/.api-client-tui/environments.json`:
```json
{
  "development": {
    "BASE_URL": "http://localhost:3000",
    "API_KEY": "dev-key-123",
    "USER_ID": "1"
  },
  "production": {
    "BASE_URL": "https://api.example.com",
    "API_KEY": "prod-key-789",
    "USER_ID": "42"
  }
}
```

Use variables in requests:
- URL: `{{BASE_URL}}/users/{{USER_ID}}`
- Headers: `Authorization: Bearer {{API_KEY}}`

### Keyboard Shortcuts

#### Navigation
- **Tab**: Next panel
- **Shift+Tab**: Previous panel
- **↑/↓**: Navigate items (methods, history)

#### Actions
- **Enter**: Send request (when URL panel is focused)
- **Ctrl+h**: Toggle request history
- **Ctrl+e**: Toggle environment variables

#### General
- **q**: Quit application
- **?**: Toggle help

### Common Request Examples

#### 1. JSON API Request
- Method: POST
- Headers: `Content-Type: application/json`
- Body: `{"key": "value"}`

#### 2. Form Submission
- Method: POST
- Headers: `Content-Type: application/x-www-form-urlencoded`
- Body: `username=admin&password=secret`

#### 3. File Upload (Multipart)
- Method: POST
- Headers: `Content-Type: multipart/form-data`
- Body: Use appropriate multipart format

## Configuration

The application uses the following configuration files in the `~/.api-client-tui/` directory:

### Main Config (`config.json`)
```json
{
  "timeout": 5,
  "auto_format_json": true,
  "save_history": true,
  "current_env": "development",
  "show_response_time": true,
  "truncate_response": 1000,
  "max_response_size": "10MB",
  "large_response_warning": "1MB"
}
```

### Collections (`collections.json`)
```json
{
  "User Management": [
    {
      "name": "Get All Users",
      "url": "{{BASE_URL}}/users",
      "method": "GET",
      "headers": {
        "Authorization": "Bearer {{API_TOKEN}}"
      }
    },
    {
      "name": "Create User",
      "url": "{{BASE_URL}}/users",
      "method": "POST",
      "headers": {
        "Content-Type": "application/json",
        "Authorization": "Bearer {{API_TOKEN}}"
      },
      "body": "{\"name\": \"{{USER_NAME}}\", \"email\": \"{{USER_EMAIL}}\"}"
    }
  ]
}
```

## Troubleshooting

### Response Formatting

The application automatically handles various response formats:

#### JSON Responses
- Automatic pretty-printing with proper indentation
- Error handling for malformed JSON
- Size-based truncation for large responses (>100KB)

#### Text and HTML Responses
- Automatic truncation for large responses
- Preview mode for HTML content
- Line wrapping for better readability

#### Character Encoding
- Automatic charset detection from Content-Type headers
- UTF-8 validation and conversion
- Fallback to common encodings (windows-1252, iso-8859-1, shift-jis, gbk, big5)
- Replacement of invalid characters with �

#### Large Responses
- Size warnings for responses over 1MB
- Automatic truncation for responses over 10MB
- Display of response size in KB/MB

#### Authentication Issues
- Verify token format in headers
- Check token expiration
- Ensure correct header names

#### Network Timeouts
- Default timeout is 5 seconds
- Connection attempts timeout after 2 seconds
- TLS handshake timeout is 2 seconds
- Response header timeout is 2.5 seconds
- Large responses (>1MB) show size warnings
- Responses over 10MB are automatically truncated

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [Bubble Tea][bubbletea] - A powerful TUI framework
- [Bubbles][bubbles] - TUI components for Bubble Tea
- [Lip Gloss][lipgloss] - Style definitions for terminal applications

[bubbletea]: https://github.com/charmbracelet/bubbletea
[bubbles]: https://github.com/charmbracelet/bubbles
[lipgloss]: https://github.com/charmbracelet/lipgloss
