package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	configDir        = ".api-client-tui"
	envFile          = "environments.json"
	collectionsFile  = "collections.json"
	historyFile      = "history.json"
	configFile       = "config.json"
	defaultHistLimit = 100
)

type RequestItem struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	CreatedAt   time.Time         `json:"created_at"`
	LastUsed    time.Time         `json:"last_used"`
	Collections []string          `json:"collections,omitempty"`
}

type Collection struct {
	Name     string        `json:"name"`
	Requests []RequestItem `json:"requests"`
}

type Environment struct {
	Name      string            `json:"name"`
	Variables map[string]string `json:"variables"`
}

type Config struct {
	Theme             string `json:"theme"`
	Timeout           int    `json:"timeout"`
	HistoryLimit      int    `json:"history_limit"`
	AutoFormatJSON    bool   `json:"auto_format_json"`
	SaveHistory       bool   `json:"save_history"`
	CurrentEnv        string `json:"current_env"`
	ShowResponseTime  bool   `json:"show_response_time"`
	TruncateResponse  int    `json:"truncate_response"`
	SyntaxHighlighting bool  `json:"syntax_highlighting"`
}

type ConfigManager struct {
	Config      Config
	History     []RequestItem
	Collections map[string]Collection
	Environments map[string]Environment
	configDir   string
}

func NewConfigManager() (*ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, configDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	cm := &ConfigManager{
		configDir:    configDir,
		Collections:  make(map[string]Collection),
		Environments: make(map[string]Environment),
		Config: Config{
			Theme:             "dark",
			Timeout:           30,
			HistoryLimit:      defaultHistLimit,
			AutoFormatJSON:    true,
			SaveHistory:       true,
			ShowResponseTime:  true,
			TruncateResponse:  1000,
			SyntaxHighlighting: true,
		},
	}

	cm.loadConfig()
	cm.loadHistory()
	cm.loadCollections()
	cm.loadEnvironments()

	return cm, nil
}

func (cm *ConfigManager) loadConfig() error {
	configPath := filepath.Join(cm.configDir, configFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cm.saveConfig()
	}

	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, &cm.Config)
}

func (cm *ConfigManager) saveConfig() error {
	configPath := filepath.Join(cm.configDir, configFile)
	bytes, err := json.MarshalIndent(cm.Config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, bytes, 0644)
}

func (cm *ConfigManager) loadHistory() error {
	historyPath := filepath.Join(cm.configDir, historyFile)
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		cm.History = []RequestItem{}
		return nil
	}

	file, err := os.Open(historyPath)
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, &cm.History)
}

func (cm *ConfigManager) saveHistory() error {
	if !cm.Config.SaveHistory {
		return nil
	}

	if len(cm.History) > cm.Config.HistoryLimit {
		cm.History = cm.History[:cm.Config.HistoryLimit]
	}

	historyPath := filepath.Join(cm.configDir, historyFile)
	bytes, err := json.MarshalIndent(cm.History, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(historyPath, bytes, 0644)
}

func (cm *ConfigManager) addToHistory(req RequestItem) error {
	for i, item := range cm.History {
		if item.URL == req.URL && item.Method == req.Method {
			cm.History[i].LastUsed = time.Now()
			if i > 0 {
				cm.History = append([]RequestItem{cm.History[i]}, append(cm.History[:i], cm.History[i+1:]...)...)
			}
			return cm.saveHistory()
		}
	}

	req.CreatedAt = time.Now()
	req.LastUsed = time.Now()
	cm.History = append([]RequestItem{req}, cm.History...)

	return cm.saveHistory()
}

func (cm *ConfigManager) loadCollections() error {
	collectionsPath := filepath.Join(cm.configDir, collectionsFile)
	if _, err := os.Stat(collectionsPath); os.IsNotExist(err) {
		cm.Collections = make(map[string]Collection)
		return nil
	}

	file, err := os.Open(collectionsPath)
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, &cm.Collections)
}

func (cm *ConfigManager) saveCollections() error {
	collectionsPath := filepath.Join(cm.configDir, collectionsFile)
	bytes, err := json.MarshalIndent(cm.Collections, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(collectionsPath, bytes, 0644)
}

func (cm *ConfigManager) addToCollection(collectionName string, req RequestItem) error {
	collection, exists := cm.Collections[collectionName]
	if !exists {
		collection = Collection{
			Name:     collectionName,
			Requests: []RequestItem{},
		}
	}

	for i, item := range collection.Requests {
		if item.URL == req.URL && item.Method == req.Method {
			collection.Requests[i] = req
			cm.Collections[collectionName] = collection
			return cm.saveCollections()
		}
	}

	collection.Requests = append(collection.Requests, req)
	cm.Collections[collectionName] = collection

	return cm.saveCollections()
}

func (cm *ConfigManager) loadEnvironments() error {
	envPath := filepath.Join(cm.configDir, envFile)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		cm.Environments = map[string]Environment{
			"development": {
				Name: "development",
				Variables: map[string]string{
					"BASE_URL": "http://localhost:3000",
					"API_KEY":  "dev-key-123",
				},
			},
			"production": {
				Name: "production",
				Variables: map[string]string{
					"BASE_URL": "https://api.example.com",
					"API_KEY":  "prod-key-789",
				},
			},
		}
		return cm.saveEnvironments()
	}

	file, err := os.Open(envPath)
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, &cm.Environments)
}

func (cm *ConfigManager) saveEnvironments() error {
	envPath := filepath.Join(cm.configDir, envFile)
	bytes, err := json.MarshalIndent(cm.Environments, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(envPath, bytes, 0644)
}

func (cm *ConfigManager) getCurrentEnvironment() Environment {
	env, exists := cm.Environments[cm.Config.CurrentEnv]
	if !exists && len(cm.Environments) > 0 {
		for _, e := range cm.Environments {
			return e
		}
	}
	return env
}

func (cm *ConfigManager) replaceEnvVars(input string) string {
	env := cm.getCurrentEnvironment()
	if env.Variables == nil {
		return input
	}

	result := input
	for key, value := range env.Variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}
