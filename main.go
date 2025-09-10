package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	urlPanel = iota
	methodPanel
	headersPanel
	bodyPanel
	responsePanel
)

var httpMethods = []string{
	"GET",
	"POST",
	"PUT",
	"DELETE",
	"PATCH",
	"HEAD",
	"OPTIONS",
}

var (
	primaryColor = lipgloss.Color("#4ECDC4")   // Teal
	accentColor  = lipgloss.Color("#FF6B6B")    // Red
	mutedColor   = lipgloss.Color("#999999")     // Gray
	whiteColor   = lipgloss.Color("#FFFFFF")

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder())

	focusedStyle = baseStyle.
			BorderForeground(accentColor)

	blurredStyle = baseStyle.
			BorderForeground(mutedColor)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	methodPanelStyle = baseStyle.
				BorderForeground(primaryColor).
				Padding(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	statusSuccessStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	statusErrorStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	statusStyle = lipgloss.NewStyle() 
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(whiteColor).
			Background(primaryColor).
			Padding(0, 1)
)

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	Left          key.Binding
	Right         key.Binding
	Tab           key.Binding
	ShiftTab      key.Binding
	Enter         key.Binding
	Quit          key.Binding
	ToggleHelp    key.Binding
	ToggleHistory key.Binding
	ToggleEnvs    key.Binding
	SaveRequest   key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "right"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next panel"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "previous panel"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "send request"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	ToggleHelp: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	ToggleHistory: key.NewBinding(
		key.WithKeys("ctrl+h"),
		key.WithHelp("ctrl+h", "toggle history"),
	),
	ToggleEnvs: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "toggle environments"),
	),
	SaveRequest: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "save request"),
	),
}

type Response struct {
	StatusCode    int
	Status        string
	Headers       http.Header
	Body          string
	FormattedBody string
	ResponseTime  time.Duration
	Error         error
	ContentLength int64
}

type Model struct {
	urlInput      textinput.Model
	methodList    list.Model
	headersInput  textinput.Model
	bodyInput     textinput.Model
	responseView  viewport.Model
	spinner       spinner.Model
	activePanel   int
	response      Response
	loading       bool
	width         int
	height        int
	showHelp      bool
	showHistory   bool
	showEnvs      bool
	lastBody      string
	configManager *ConfigManager
	requestError  error
}

func initialModel() Model {
	urlInput := textinput.New()
	urlInput.Placeholder = "https://api.example.com/endpoint"
	urlInput.Width = 50
	urlInput.Blur()

	methodItems := make([]list.Item, len(httpMethods))
	for i, method := range httpMethods {
		methodItems[i] = item{title: method}
	}
	methodDelegate := list.NewDefaultDelegate()
	methodDelegate.ShowDescription = false
	methodDelegate.SetSpacing(1)
	methodDelegate.Styles.SelectedTitle = methodDelegate.Styles.SelectedTitle.
		Foreground(primaryColor).
		Bold(true)

	methodList := list.New(methodItems, methodDelegate, 35, 8)
	methodList.Title = "HTTP Methods"
	methodList.Styles.Title = methodList.Styles.Title.
		Foreground(primaryColor).
		Bold(true).
		MarginLeft(1)
	methodList.SetShowTitle(true)
	methodList.SetFilteringEnabled(false)
	methodList.Styles.NoItems = methodList.Styles.NoItems.
		Foreground(accentColor)
	methodList.Select(0) // Select GET by default

	headersInput := textinput.New()
	headersInput.Placeholder = "Content-Type: application/json\nAuthorization: Bearer token"
	headersInput.Width = 50
	headersInput.CharLimit = 0
	headersInput.SetValue("")

	bodyInput := textinput.New()
	bodyInput.Placeholder = "{\n  \"key\": \"value\"\n}"
	bodyInput.Width = 50
	bodyInput.CharLimit = 0
	bodyInput.SetValue("")

	responseView := viewport.New(0, 0)
	responseView.Style = blurredStyle

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(accentColor)

	configManager, err := NewConfigManager()
	if err != nil {
		fmt.Printf("Error initializing config: %v\n", err)
	}

	return Model{
		urlInput:      urlInput,
		methodList:    methodList,
		headersInput:  headersInput,
		bodyInput:     bodyInput,
		responseView:  responseView,
		spinner:       s,
		activePanel:   methodPanel, // Start with method panel active
		showHelp:      false,
		showHistory:   false,
		showEnvs:      false,
		lastBody:      bodyInput.Value(),
		configManager: configManager,
	}
}

type item struct {
	title string
}

func (i item) Title() string {
	switch i.title {
	case "GET":
		return "GET     - Retrieve data"
	case "POST":
		return "POST    - Create new data"
	case "PUT":
		return "PUT     - Update existing data"
	case "DELETE":
		return "DELETE  - Remove data"
	case "PATCH":
		return "PATCH   - Partial update"
	case "HEAD":
		return "HEAD    - Headers only"
	case "OPTIONS":
		return "OPTIONS - Get allowed methods"
	default:
		return i.title
	}
}

func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.title }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...\n(Response truncated, too long to display fully)"
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	m.requestError = nil

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Tab):
			if m.activePanel == methodPanel {
				m.activePanel = urlPanel
				return m.updateFocus()
			}

			switch m.activePanel {
			case urlPanel:
				m.activePanel = headersPanel
			case headersPanel:
				m.activePanel = bodyPanel
			case bodyPanel:
				m.activePanel = responsePanel
			default:
				m.activePanel = methodPanel
			}
			return m.updateFocus()

		case key.Matches(msg, keys.ShiftTab):
			switch m.activePanel {
			case methodPanel:
				m.activePanel = responsePanel
			case urlPanel:
				m.activePanel = methodPanel
			case headersPanel:
				m.activePanel = urlPanel
			case bodyPanel:
				m.activePanel = headersPanel
			case responsePanel:
				m.activePanel = bodyPanel
			}
			return m.updateFocus()

		case key.Matches(msg, keys.Enter):
			if m.activePanel == urlPanel && m.urlInput.Value() != "" {
				m.loading = true
				m.lastBody = m.bodyInput.Value()
				return m, m.sendRequest()
			}

		case key.Matches(msg, keys.ToggleHelp):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, keys.ToggleHistory):
			m.showHistory = !m.showHistory
			m.showEnvs = false // Close other panels
			return m, nil

		case key.Matches(msg, keys.ToggleEnvs):
			m.showEnvs = !m.showEnvs
			m.showHistory = false // Close other panels
			return m, nil

		case key.Matches(msg, keys.SaveRequest):
			if m.configManager != nil && m.urlInput.Value() != "" {
				headers := make(map[string]string)
				headerLines := strings.SplitSeq(m.headersInput.Value(), "\n")
				for line := range headerLines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}

				method := httpMethods[0] // Default to GET
				if i := m.methodList.Index(); i >= 0 && i < len(httpMethods) {
					method = httpMethods[i]
				}

				reqItem := RequestItem{
					ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
					Name:    fmt.Sprintf("%s %s", method, m.urlInput.Value()),
					URL:     m.urlInput.Value(),
					Method:  method,
					Headers: headers,
					Body:    m.bodyInput.Value(),
				}

				_ = m.configManager.addToCollection("Default", reqItem)
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updatePanelSizes()

	case Response:
		m.response = msg
		m.loading = false
		if msg.Error != nil {
			m.requestError = msg.Error
		}
	
		m.bodyInput.SetValue(m.lastBody)
		m.responseView.SetContent(m.formatResponse())
		return m, nil
	}

	switch m.activePanel {
	case urlPanel:
		m.urlInput, cmd = m.urlInput.Update(msg)
		cmds = append(cmds, cmd)

	case methodPanel:
		m.methodList, cmd = m.methodList.Update(msg)
		cmds = append(cmds, cmd)

	case headersPanel:
		m.headersInput, cmd = m.headersInput.Update(msg)
		cmds = append(cmds, cmd)

	case bodyPanel:
		m.bodyInput, cmd = m.bodyInput.Update(msg)
		cmds = append(cmds, cmd)

	case responsePanel:
		m.responseView, cmd = m.responseView.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.loading {
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateFocus() (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	m.urlInput.Blur()
	m.headersInput.Blur()
	m.bodyInput.Blur()

	switch m.activePanel {
	case methodPanel:
		return m, nil

	case urlPanel:
		m.urlInput.Focus()
		cmds = append(cmds, textinput.Blink)

	case headersPanel:
		m.headersInput.Focus()
		cmds = append(cmds, textinput.Blink)

	case bodyPanel:
		m.bodyInput.Focus()
		cmds = append(cmds, textinput.Blink)
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m *Model) updatePanelSizes() {
	headerHeight := 4
	footerHeight := 2
	availableHeight := m.height - headerHeight - footerHeight

	methodWidth := max(m.width/3, 35)
	m.methodList.SetSize(methodWidth, 8)

	m.urlInput.Width = m.width - methodWidth - 8

	m.headersInput.Width = (m.width - 4) / 2
	m.bodyInput.Width = (m.width - 4) / 2

	m.responseView.Width = m.width - 4
	m.responseView.Height = availableHeight / 2
}

func (m Model) sendRequest() tea.Cmd {
	return func() tea.Msg {
		// Don't modify model state here - it won't propagate
		timeout := 5 * time.Second // Set to 5s for reliability
		if m.configManager != nil && m.configManager.Config.Timeout > 0 {
			timeout = time.Duration(m.configManager.Config.Timeout) * time.Second
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		url := m.urlInput.Value()
		if m.configManager != nil {
			url = m.configManager.replaceEnvVars(url)
		}

		method := httpMethods[0] // Default to GET
		if i := m.methodList.Index(); i >= 0 && i < len(httpMethods) {
			method = httpMethods[i]
		}

		var reqBody io.Reader
		if method != "GET" && method != "HEAD" {
			reqBody = strings.NewReader(m.bodyInput.Value())
		}

		req, err := http.NewRequest(method, url, reqBody)
		if err != nil {
			return Response{Error: err}
		}

		headers := parseHeaders(m.headersInput.Value())
		for k, v := range headers {
			req.Header.Add(k, v)
		}
		
		// Add default User-Agent if not set
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "api-client-tui/1.0")
		}

		// Use a simpler HTTP client configuration
		client := &http.Client{
			Timeout: timeout,
		}

		req = req.WithContext(ctx)

		resultChan := make(chan Response, 1)
		startTime := time.Now()

		go func() {
			resp, err := client.Do(req)
			responseTime := time.Since(startTime)

			if err != nil {
				var errMsg string
				switch {
				case ctx.Err() == context.DeadlineExceeded:
					errMsg = fmt.Sprintf("Request timed out after %v. The server took too long to respond.", timeout)
				case strings.Contains(err.Error(), "no such host"):
					errMsg = "Could not resolve host. Please check the URL and your internet connection."
				case strings.Contains(err.Error(), "connection refused"):
					errMsg = "Connection refused. The server is not accepting connections."
				case strings.Contains(err.Error(), "certificate"):
					errMsg = "SSL/TLS certificate error. The server's security certificate could not be verified."
				case strings.Contains(err.Error(), "EOF"):
					errMsg = "Connection closed unexpectedly. The server terminated the connection."
				case strings.Contains(err.Error(), "i/o timeout"):
					errMsg = "Connection timed out. The server is not responding."
				case strings.Contains(err.Error(), "connection reset"):
					errMsg = "Connection was reset. The server closed the connection abruptly."
				default:
					errMsg = "Request failed: " + err.Error()
				}
				resultChan <- Response{
					Error:        errors.New(errMsg),
					ResponseTime: responseTime,
				}
				return
			}
			defer resp.Body.Close()

			contentLength := resp.ContentLength
			if contentLength > 10*1024*1024 { // 10MB limit
				resultChan <- Response{
					StatusCode:     resp.StatusCode,
					Status:         resp.Status,
					Headers:        resp.Header,
					Error:          fmt.Errorf("response too large (%.1f MB) - size limit is 10MB", float64(contentLength)/(1024*1024)),
					ResponseTime:   responseTime,
					ContentLength: contentLength,
				}
				return
			} else if contentLength > 1*1024*1024 { // Show warning for responses over 1MB
				fmt.Printf("Large response detected (%.1f MB). Reading...", float64(contentLength)/(1024*1024))
			}

			var bodyBuf bytes.Buffer
			limitReader := io.LimitReader(resp.Body, 10*1024*1024)
			_, err = io.Copy(&bodyBuf, limitReader)
			if err != nil {
				resultChan <- Response{
					StatusCode:     resp.StatusCode,
					Status:         resp.Status,
					Headers:        resp.Header,
					Error:          fmt.Errorf("failed to read response: %v", err),
					ResponseTime:   responseTime,
					ContentLength: contentLength,
				}
				return
			}
			respBody := bodyBuf.Bytes()

			contentType := resp.Header.Get("Content-Type")
			encoding := "utf-8" // default
			if idx := strings.LastIndex(contentType, "charset="); idx != -1 {
				encoding = strings.TrimSpace(contentType[idx+8:])
				if semicolon := strings.Index(encoding, ";"); semicolon != -1 {
					encoding = encoding[:semicolon]
				}
			}

			var decodedBody []byte
			if encoding != "utf-8" && encoding != "UTF-8" {
				if enc, err := htmlindex.Get(encoding); err == nil {
					if decoded, _, err := transform.Bytes(enc.NewDecoder(), respBody); err == nil && utf8.Valid(decoded) {
						decodedBody = decoded
					}
				}
			}

			if decodedBody == nil {
				decodedBody = []byte(strings.Map(func(r rune) rune {
					if r == utf8.RuneError {
						return '�'
					}
					return r
				}, string(respBody)))
			}

			formattedBody := string(decodedBody)
			if len(decodedBody) > 100*1024 { // 100KB
				formattedBody = fmt.Sprintf("Large response (%d KB) - showing first 1000 chars:\n%s", len(decodedBody)/1024, truncateString(string(decodedBody), 1000))
			} else if m.configManager == nil || m.configManager.Config.AutoFormatJSON {
				if strings.Contains(contentType, "application/json") {
					var prettyJSON bytes.Buffer
					if err := json.Indent(&prettyJSON, decodedBody, "", "  "); err != nil {
						formattedBody = "Error formatting JSON: " + err.Error() + "\nRaw response:\n" + string(decodedBody)
					} else {
						formattedBody = prettyJSON.String()
					}
				} else if strings.Contains(contentType, "text/html") {
					formattedBody = "HTML Response:\n" + truncateString(string(decodedBody), 1000)
				}
			}

			if m.configManager != nil && m.configManager.Config.SaveHistory {
				go func() {
					reqItem := RequestItem{
						URL:     url,
						Method:  method,
						Headers: headers,
						Body:    m.bodyInput.Value(),
					}
					_ = m.configManager.addToHistory(reqItem)
				}()
			}

			response := Response{
				StatusCode:    resp.StatusCode,
				Status:        resp.Status,
				Headers:       resp.Header,
				Body:          string(respBody),
				FormattedBody: formattedBody,
				ResponseTime:  responseTime,
				ContentLength: contentLength,
			}
			resultChan <- response
		}()

		select {
		case res := <-resultChan:
			return res
		case <-time.After(timeout + 1*time.Second):
			return Response{
				Error:        fmt.Errorf("forced timeout: request took longer than %v", timeout),
				ResponseTime: time.Since(startTime),
			}
		}
	}
}

func (m Model) formatResponse() string {
	if m.response.Error != nil {
		var sb strings.Builder
		sb.WriteString(errorStyle.Render("Error: " + m.response.Error.Error()))
		
		if m.response.StatusCode > 0 {
			sb.WriteString(fmt.Sprintf("\nStatus: %d - %s", m.response.StatusCode, http.StatusText(m.response.StatusCode)))
		}
		if m.response.ContentLength > 0 {
			sb.WriteString(fmt.Sprintf("\nReceived: %.1f KB", float64(m.response.ContentLength)/1024))
		}
		if m.response.ResponseTime > 0 {
			sb.WriteString(fmt.Sprintf("\nTime: %v", m.response.ResponseTime))
		}
		return sb.String()
	}

	var sb strings.Builder

	statusStyle := statusSuccessStyle
	if m.response.StatusCode >= 400 {
		statusStyle = statusErrorStyle
	}

	statusLine := fmt.Sprintf("Status: %d - %s", m.response.StatusCode, m.response.Status)
	if m.response.ContentLength > 0 {
		statusLine += fmt.Sprintf(" (%.1f KB)", float64(m.response.ContentLength)/1024)
	}
	sb.WriteString(statusStyle.Render(statusLine + "\n"))
	sb.WriteString(fmt.Sprintf("Time: %v\n\n", m.response.ResponseTime))

	sb.WriteString("Headers:\n")
	for k, v := range m.response.Headers {
		sb.WriteString(fmt.Sprintf("%s: %s\n", k, strings.Join(v, ", ")))
	}
	sb.WriteString("\n")

	sb.WriteString("Body:\n")
	sb.WriteString(m.response.FormattedBody)

	return sb.String()
}

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	header := headerStyle.Render("API Client TUI")

	methodStyle := methodPanelStyle.Copy().
		MarginRight(2).
		BorderForeground(primaryColor)
	if m.activePanel == methodPanel {
		methodStyle = methodStyle.BorderForeground(accentColor)
	}
	methodView := methodStyle.Render(m.methodList.View())

	urlStyle := blurredStyle
	if m.activePanel == urlPanel {
		urlStyle = focusedStyle
	}
	urlView := urlStyle.Render(fmt.Sprintf("%s\n%s", "URL", m.urlInput.View()))

	headersStyle := blurredStyle
	if m.activePanel == headersPanel {
		headersStyle = focusedStyle
	}
	headersView := headersStyle.Render(fmt.Sprintf("%s\n%s", "Headers", m.headersInput.View()))

	bodyStyle := blurredStyle
	if m.activePanel == bodyPanel {
		bodyStyle = focusedStyle
	}
	bodyView := bodyStyle.Render(fmt.Sprintf("%s\n%s", "Body", m.bodyInput.View()))

	responseContent := "No response yet"
	if m.loading {
		responseContent = fmt.Sprintf("%s Sending request...", m.spinner.View())
	} else if m.response.StatusCode > 0 || m.response.Error != nil {
		responseContent = m.responseView.View()
	}
	responseStyle := blurredStyle
	if m.activePanel == responsePanel {
		responseStyle = focusedStyle
	}
	responseView := responseStyle.Render(fmt.Sprintf("%s\n%s", "Response", responseContent))

	topRow := lipgloss.JoinVertical(lipgloss.Left,
		methodView,
		urlView)

	middleRow := lipgloss.JoinHorizontal(lipgloss.Top, headersView, bodyView)

	historyPanel := ""
	if m.showHistory && m.configManager != nil {
		historyContent := "No history items"
		if len(m.configManager.History) > 0 {
			var sb strings.Builder
			sb.WriteString("Recent Requests:\n")
			for i, item := range m.configManager.History {
				if i >= 10 { // Show only 10 most recent items
					break
				}
				sb.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, item.Method, item.URL))
			}
			historyContent = sb.String()
		}
		historyPanel = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Width(m.width - 4).
			Render(historyContent)
	}

	envsPanel := ""
	if m.showEnvs && m.configManager != nil {
		envsContent := "No environments configured"
		if len(m.configManager.Environments) > 0 {
			var sb strings.Builder
			sb.WriteString("Environments:\n")

			currentEnv := m.configManager.getCurrentEnvironment()
			sb.WriteString(fmt.Sprintf("Current: %s\n\n", currentEnv.Name))

			sb.WriteString("Variables:\n")
			for k, v := range currentEnv.Variables {
				sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			}
			envsContent = sb.String()
		}
		envsPanel = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Width(m.width - 4).
			Render(envsContent)
	}

	help := ""
	if m.showHelp {
		help = helpStyle.Render("\nTab: Next panel • Shift+Tab: Previous panel • Enter: Send request • Ctrl+h: History • Ctrl+e: Environments • Ctrl+s: Save • q: Quit • ?: Toggle help")
	} else {
		help = helpStyle.Render("\nPress ? for help")
	}

	view := fmt.Sprintf("%s\n%s\n%s\n%s", header, topRow, middleRow, responseView)

	if m.showHistory {
		view += "\n" + historyPanel
	}

	if m.showEnvs {
		view += "\n" + envsPanel
	}

	view += help

	return view
}

func tryAlternativeEncodings(input []byte) []byte {
	encodings := []string{"windows-1252", "iso-8859-1", "shift-jis", "gbk", "big5"}

	for _, encoding := range encodings {
		if enc, err := htmlindex.Get(encoding); err == nil {
			if decoded, _, err := transform.Bytes(enc.NewDecoder(), input); err == nil && utf8.Valid(decoded) {
				return decoded
			}
		}
	}

	return input // Return original if no encoding works
}

func parseHeaders(input string) map[string]string {
	headers := make(map[string]string)
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" {
				headers[key] = value
			}
		}
	}
	return headers
}

func main() {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
			os.Exit(1)
		}

		if !utf8.Valid(input) || bytes.Contains(input, []byte{0}) {
			fmt.Println("tweet content not found")
			os.Exit(0)
		}

		model := initialModel()
		model.bodyInput.SetValue(string(input))

		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
	} else {
		p := tea.NewProgram(initialModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
	}
}
