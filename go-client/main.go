package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Configuration holds application settings
type Configuration struct {
	ServerURL           string
	ClientID            string
	SessionID           string
	MongoDB             string
	DatabaseName        string
	Debug               bool
	AutoExecute         bool
	ConfidenceThreshold float64
}

// CommandResponse represents the structure of the prediction response
type CommandResponse struct {
	Command             string   `json:"command"`
	Args                []string `json:"args"`
	Explanation         string   `json:"explanation"`
	Confidence          float64  `json:"confidence"`
	RequestID           string   `json:"request_id"`
	Error               string   `json:"error,omitempty"`
	NextWordSuggestions []string `json:"next_word_suggestions,omitempty"`
}

// CommandRequest represents the request payload
type CommandRequest struct {
	Query     string `json:"query"`
	ClientID  string `json:"client_id"`
	SessionID string `json:"session_id"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	EventType string                 `bson:"event_type"`
	Message   string                 `bson:"message"`
	Timestamp time.Time              `bson:"timestamp"`
	Metadata  map[string]interface{} `bson:"metadata"`
}

var config Configuration
var mongoClient *mongo.Client
var auditCollection *mongo.Collection
var sessionStartTime time.Time
var cmdHistory []string

type DynamicCompleter struct{}

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	Bold        = "\033[1m"
)

func init() {
	// Generate a unique session ID
	sessionID := fmt.Sprintf("session-%d", time.Now().Unix())

	// Get current user information
	currentUser, err := user.Current()
	clientID := "unknown"
	if err == nil {
		hostname, _ := os.Hostname()
		clientID = fmt.Sprintf("%s@%s", currentUser.Username, hostname)
	}

	// Parse command line flags
	flag.StringVar(&config.ServerURL, "server", "http://localhost:5000", "Server URL")
	flag.StringVar(&config.ClientID, "client", clientID, "Client ID")
	flag.StringVar(&config.SessionID, "session", sessionID, "Session ID")
	flag.StringVar(&config.MongoDB, "mongodb", "mongodb://localhost:27017", "MongoDB connection string")
	flag.StringVar(&config.DatabaseName, "db", "terminal_assistant", "MongoDB database name")
	flag.BoolVar(&config.Debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&config.AutoExecute, "auto", false, "Auto-execute commands above threshold")
	flag.Float64Var(&config.ConfidenceThreshold, "threshold", 0.8, "Confidence threshold for auto-execution")

	sessionStartTime = time.Now()
}

func connectToMongoDB() error {
	// Set client options
	clientOptions := options.Client().ApplyURI(config.MongoDB)

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return err
	}

	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return err
	}

	fmt.Printf("%sConnected to MongoDB!%s\n", ColorGreen, ColorReset)
	mongoClient = client

	// Get collection
	auditCollection = client.Database(config.DatabaseName).Collection("terminal_client_server_audit")

	return nil
}

func logAudit(eventType, message string, metadata map[string]interface{}) {
	// Don't attempt to log if MongoDB is not connected
	if auditCollection == nil {
		if config.Debug {
			fmt.Printf("%s[DEBUG] MongoDB not connected for audit logging%s\n", ColorYellow, ColorReset)
		}
		return
	}

	// Ensure metadata exists
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	// Add standard fields
	metadata["client_id"] = config.ClientID
	metadata["session_id"] = config.SessionID

	// Create audit document
	audit := AuditLog{
		EventType: eventType,
		Message:   message,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	// Insert into MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := auditCollection.InsertOne(ctx, audit)
	if err != nil && config.Debug {
		fmt.Printf("%s[DEBUG] Failed to log audit: %v%s\n", ColorYellow, err, ColorReset)
	}
}

func checkServer() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/ping", config.ServerURL), nil)
	if err != nil {
		fmt.Printf("%s✗ Failed to create request: %v%s\n", ColorRed, err, ColorReset)
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("%s✗ Server unavailable: %v%s\n", ColorRed, err, ColorReset)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("%s✗ Server returned error status: %d%s\n", ColorRed, resp.StatusCode, ColorReset)
		return false
	}

	return true
}

func checkServerHealth() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/health", config.ServerURL), nil)
	if err != nil {
		fmt.Printf("%s✗ Failed to create health request: %v%s\n", ColorRed, err, ColorReset)
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("%s✗ Health check failed: %v%s\n", ColorRed, err, ColorReset)
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s✗ Failed to read health response: %v%s\n", ColorRed, err, ColorReset)
		return false
	}

	var healthData map[string]interface{}
	if err := json.Unmarshal(body, &healthData); err != nil {
		fmt.Printf("%s✗ Failed to parse health response: %v%s\n", ColorRed, err, ColorReset)
		return false
	}

	// Print health check if in debug mode
	if config.Debug {
		fmt.Printf("%s[DEBUG] Health check response: %v%s\n", ColorYellow, healthData, ColorReset)
	}

	status, ok := healthData["status"].(string)
	if !ok || status != "healthy" {
		fmt.Printf("%s✗ Server reports unhealthy status: %v%s\n", ColorRed, status, ColorReset)

		// Show more detail about checks
		if checks, ok := healthData["checks"].(map[string]interface{}); ok {
			for check, status := range checks {
				statusStr := fmt.Sprintf("%v", status)
				if status == true {
					fmt.Printf("  %s✓ %s: %s%s\n", ColorGreen, check, statusStr, ColorReset)
				} else {
					fmt.Printf("  %s✗ %s: %s%s\n", ColorRed, check, statusStr, ColorReset)
				}
			}
		}

		return false
	}

	return true
}

func predictCommand(query string) (*CommandResponse, error) {
	// Create request payload
	reqPayload := CommandRequest{
		Query:     query,
		ClientID:  config.ClientID,
		SessionID: config.SessionID,
	}

	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Set timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create request
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		fmt.Sprintf("%s/predict", config.ServerURL),
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response
	var result CommandResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for errors
	if result.Error != "" {
		return &result, fmt.Errorf("%s", result.Error)
	}

	return &result, nil
}

func predictNextWord(query string) (*CommandResponse, error) {
	// Create request payload
	reqPayload := CommandRequest{
		Query:     query,
		ClientID:  config.ClientID,
		SessionID: config.SessionID,
	}

	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Set timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create request
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		fmt.Sprintf("%s/predictNextWord", config.ServerURL),
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response
	var result CommandResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for errors
	if result.Error != "" {
		return &result, fmt.Errorf("%s", result.Error)
	}

	return &result, nil
}

func executeCommand(cmdStr string, args []string) error {
	// Create command
	cmd := exec.Command(cmdStr, args...)

	// Set input/output
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run command
	return cmd.Run()
}

func displayWelcome() {
	fmt.Printf("%s%s╔════════════════════════════════════════════╗%s\n", Bold, ColorBlue, ColorReset)
	fmt.Printf("%s%s║  Smart Terminal Assistant                  ║%s\n", Bold, ColorBlue, ColorReset)
	fmt.Printf("%s%s║  Type your command or ask in plain English ║%s\n", Bold, ColorBlue, ColorReset)
	fmt.Printf("%s%s║  Type 'exit' or 'quit' to exit             ║%s\n", Bold, ColorBlue, ColorReset)
	fmt.Printf("%s%s╚════════════════════════════════════════════╝%s\n", Bold, ColorBlue, ColorReset)
	fmt.Printf("Client ID: %s\n", config.ClientID)
	fmt.Printf("Session ID: %s\n", config.SessionID)
	fmt.Printf("Server: %s\n", config.ServerURL)
	fmt.Printf("\n")
}

func getPrompt() string {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown"
	}

	// Get just the base directory name
	dirName := filepath.Base(cwd)

	// Create a colored prompt
	return fmt.Sprintf("%s%s%s:%s%s%s$ ", Bold, ColorBlue, config.ClientID, ColorGreen, dirName, ColorReset)
}

func runTerminal() {
	// Check if server is ready
	fmt.Printf("Checking server connection... ")
	if !checkServer() {
		fmt.Printf("%sFATAL: Server is not available or not responding%s\n", ColorRed, ColorReset)
		logAudit("startup_error", "Server not available", map[string]interface{}{
			"server_url": config.ServerURL,
		})
		os.Exit(1)
	}
	fmt.Printf("%s✓ Connected!%s\n", ColorGreen, ColorReset)

	// Check server health
	fmt.Printf("Checking server health... ")
	if !checkServerHealth() {
		fmt.Printf("%sWARNING: Server health check failed%s\n", ColorYellow, ColorReset)
		logAudit("health_warning", "Server health check failed", nil)
	} else {
		fmt.Printf("%s✓ Healthy!%s\n", ColorGreen, ColorReset)
	}

	// Display welcome message
	displayWelcome()

	// Log session start
	logAudit("session_start", "Terminal session started", map[string]interface{}{
		"auto_execute": config.AutoExecute,
		"threshold":    config.ConfidenceThreshold,
	})

	// Set up readline with tab completion
	rl, err := setupTabCompletion()
	if err != nil {
		fmt.Printf("%sError setting up terminal: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}
	defer rl.Close()

	// Main loop
	for {
		// Update prompt (in case it changes dynamically)
		rl.SetPrompt(getPrompt())

		// Read line with tab completion support
		input, err := rl.Readline()
		if err != nil { // EOF or interrupted
			break
		}

		// Trim whitespace
		input = strings.TrimSpace(input)

		// Check for exit commands
		if input == "exit" || input == "quit" {
			fmt.Println("Exiting terminal...")
			break
		}

		// Skip empty input
		if input == "" {
			continue
		}

		// Add to history
		cmdHistory = append(cmdHistory, input)

		// Check if it starts with ! for direct execution
		if strings.HasPrefix(input, "!") {
			// Remove ! and execute directly
			cmdParts := strings.Fields(input[1:])
			if len(cmdParts) > 0 {
				cmd := cmdParts[0]
				args := cmdParts[1:]

				logAudit("direct_command", "Executing direct command", map[string]interface{}{
					"command": cmd,
					"args":    args,
				})

				if err := executeCommand(cmd, args); err != nil {
					fmt.Printf("%sExecution error: %v%s\n", ColorRed, err, ColorReset)
				}
			}
			continue
		}

		// Handle special commands
		if strings.HasPrefix(input, "#") {
			handleSpecialCommand(input)
			continue
		}

		// Process as natural language
		logAudit("query", "Processing natural language input", map[string]interface{}{
			"query": input,
		})

		// Predict command
		result, err := predictCommand(input)
		if err != nil {
			fmt.Printf("%sError: %v%s\n", ColorRed, err, ColorReset)
			logAudit("prediction_error", "Error predicting command", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		// Format command string
		cmdString := fmt.Sprintf("%s %s", result.Command, strings.Join(result.Args, " "))
		cmdString = strings.TrimSpace(cmdString)

		// Display prediction
		fmt.Printf("%s%s╔═ Command ═══════════════════════════════════%s\n", Bold, ColorCyan, ColorReset)
		fmt.Printf("%s%s║%s %s%s\n", Bold, ColorCyan, ColorReset, cmdString, strings.Repeat(" ", max(0, 47-len(cmdString))))
		fmt.Printf("%s%s╠═ Explanation ═══════════════════════════════%s\n", Bold, ColorCyan, ColorReset)

		// Split explanation into lines for better formatting
		explanationLines := splitString(result.Explanation, 45)
		for _, line := range explanationLines {
			fmt.Printf("%s%s║%s %s%s\n", Bold, ColorCyan, ColorReset, line, strings.Repeat(" ", max(0, 45-len(line))))
		}

		fmt.Printf("%s%s╠═ Confidence ════════════════════════════════%s\n", Bold, ColorCyan, ColorReset)
		confidenceBar := createConfidenceBar(result.Confidence, 45)
		fmt.Printf("%s%s║%s %s %s%.1f%%%s\n", Bold, ColorCyan, ColorReset, confidenceBar, Bold, result.Confidence*100, ColorReset)
		fmt.Printf("%s%s╚═════════════════════════════════════════════%s\n", Bold, ColorCyan, ColorReset)

		// Log the prediction
		logAudit("prediction", "Command predicted", map[string]interface{}{
			"query":      input,
			"command":    result.Command,
			"args":       result.Args,
			"confidence": result.Confidence,
			"request_id": result.RequestID,
		})

		// Decide whether to auto-execute
		shouldExecute := false

		if config.AutoExecute && result.Confidence >= config.ConfidenceThreshold {
			fmt.Printf("%sAuto-executing command (confidence %.1f%% > threshold %.1f%%)%s\n",
				ColorGreen, result.Confidence*100, config.ConfidenceThreshold*100, ColorReset)
			shouldExecute = true
		} else if !config.AutoExecute {
			// Ask user if they want to execute
			fmt.Printf("Execute this command? [y/N]: ")
			var response string
			fmt.Scanln(&response)

			shouldExecute = strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
		}

		if shouldExecute {
			fmt.Printf("%sExecuting: %s%s\n", ColorGreen, cmdString, ColorReset)
			logAudit("execute", "Executing predicted command", map[string]interface{}{
				"command": result.Command,
				"args":    result.Args,
			})

			// Execute command
			if err := executeCommand(result.Command, result.Args); err != nil {
				fmt.Printf("%sExecution error: %v%s\n", ColorRed, err, ColorReset)
				logAudit("execution_error", "Error executing command", map[string]interface{}{
					"error":   err.Error(),
					"command": result.Command,
					"args":    result.Args,
				})
			}
		}
	}

	// Log session end
	sessionDuration := time.Since(sessionStartTime)
	logAudit("session_end", "Terminal session ended", map[string]interface{}{
		"duration_seconds":  sessionDuration.Seconds(),
		"commands_executed": len(cmdHistory),
	})

	// Close MongoDB connection
	if mongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			fmt.Printf("%sError disconnecting from MongoDB: %v%s\n", ColorRed, err, ColorReset)
		}
	}
}

// Helper function to create a confidence bar visualization
func createConfidenceBar(confidence float64, width int) string {
	barWidth := int(confidence * float64(width))

	// Determine color based on confidence level
	var barColor string
	if confidence < 0.3 {
		barColor = ColorRed
	} else if confidence < 0.7 {
		barColor = ColorYellow
	} else {
		barColor = ColorGreen
	}

	// Create bar
	bar := fmt.Sprintf("%s%s%s%s",
		barColor,
		strings.Repeat("█", barWidth),
		ColorReset,
		strings.Repeat("░", width-barWidth))

	return bar
}

// Helper function to split a string into lines with max length
func splitString(s string, maxLen int) []string {
	var lines []string
	words := strings.Fields(s)
	currentLine := ""

	for _, word := range words {
		if len(currentLine)+len(word)+1 > maxLen {
			lines = append(lines, currentLine)
			currentLine = word
		} else {
			if currentLine == "" {
				currentLine = word
			} else {
				currentLine += " " + word
			}
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// Helper function for max of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Handle special commands starting with #
func handleSpecialCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	command := parts[0]

	switch command {
	case "#help":
		displayHelp()
	case "#history":
		displayHistory()
	case "#debug":
		toggleDebug()
	case "#auto":
		toggleAutoExecute()
	case "#threshold":
		if len(parts) > 1 {
			setThreshold(parts[1])
		} else {
			fmt.Printf("%sMissing threshold value%s\n", ColorRed, ColorReset)
		}
	case "#config":
		displayConfig()
	default:
		fmt.Printf("%sUnknown special command: %s%s\n", ColorRed, command, ColorReset)
	}
}

// Display help information
func displayHelp() {
	fmt.Printf("%s%s╔════════════════════════════════════════════╗%s\n", Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║  Terminal Assistant Help                   ║%s\n", Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s╠════════════════════════════════════════════╣%s\n", Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s Special Commands:                         %s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s  #help      - Show this help              %s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s  #history   - Show command history        %s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s  #debug     - Toggle debug mode           %s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s  #auto      - Toggle auto-execute         %s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s  #threshold <val> - Set confidence threshold%s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s  #config    - Show current configuration  %s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s                                           %s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s Shortcuts:                                %s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s  !<command> - Execute command directly    %s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s  exit/quit  - Exit the terminal           %s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s╚════════════════════════════════════════════╝%s\n", Bold, ColorPurple, ColorReset)
}

// Display command history
func displayHistory() {
	fmt.Printf("%s%s╔════════════════════════════════════════════╗%s\n", Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║  Command History                           ║%s\n", Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s╠════════════════════════════════════════════╣%s\n", Bold, ColorPurple, ColorReset)

	for i, cmd := range cmdHistory {
		if len(cmd) > 45 {
			cmd = cmd[:42] + "..."
		}
		fmt.Printf("%s%s║%s %2d. %s%s\n", Bold, ColorPurple, ColorReset, i+1, cmd,
			strings.Repeat(" ", max(0, 42-len(cmd))))
	}

	fmt.Printf("%s%s╚════════════════════════════════════════════╝%s\n", Bold, ColorPurple, ColorReset)
}

// Toggle debug mode
func toggleDebug() {
	config.Debug = !config.Debug
	var debugStatus string
	if config.Debug {
		debugStatus = ColorGreen + "ON"
	} else {
		debugStatus = ColorRed + "OFF"
	}
	fmt.Printf("Debug mode: %s%s%s\n", Bold, debugStatus, ColorReset)

	logAudit("config_change", "Debug mode toggled", map[string]interface{}{
		"debug": config.Debug,
	})
}

// Toggle auto-execute mode
func toggleAutoExecute() {
	config.AutoExecute = !config.AutoExecute
	var autoExecuteStatus string
	if config.AutoExecute {
		autoExecuteStatus = ColorGreen + "ON"
	} else {
		autoExecuteStatus = ColorRed + "OFF"
	}
	fmt.Printf("Auto-execute mode: %s%s%s\n", Bold, autoExecuteStatus, ColorReset)

	logAudit("config_change", "Auto-execute mode toggled", map[string]interface{}{
		"auto_execute": config.AutoExecute,
	})
}

// Set confidence threshold
func setThreshold(thresholdStr string) {
	var threshold float64
	_, err := fmt.Sscanf(thresholdStr, "%f", &threshold)
	if err != nil || threshold < 0 || threshold > 1 {
		fmt.Printf("%sInvalid threshold value. Must be between 0.0 and 1.0%s\n", ColorRed, ColorReset)
		return
	}

	oldThreshold := config.ConfidenceThreshold
	config.ConfidenceThreshold = threshold

	fmt.Printf("Confidence threshold updated: %.1f%% -> %.1f%%\n",
		oldThreshold*100, threshold*100)

	logAudit("config_change", "Confidence threshold changed", map[string]interface{}{
		"old_threshold": oldThreshold,
		"new_threshold": threshold,
	})
}

// Display current configuration
func displayConfig() {
	fmt.Printf("%s%s╔════════════════════════════════════════════╗%s\n", Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║  Current Configuration                     ║%s\n", Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s╠════════════════════════════════════════════╣%s\n", Bold, ColorPurple, ColorReset)
	fmt.Printf("%s%s║%s Server URL: %s%-28s%s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, config.ServerURL, ColorReset, Bold, ColorReset)
	fmt.Printf("%s%s║%s Client ID: %s%-29s%s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, config.ClientID, ColorReset, Bold, ColorReset)
	fmt.Printf("%s%s║%s Session ID: %s%-28s%s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, config.SessionID, ColorReset, Bold, ColorReset)
	fmt.Printf("%s%s║%s MongoDB: %s%-31s%s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, config.MongoDB, ColorReset, Bold, ColorReset)
	fmt.Printf("%s%s║%s Database: %s%-30s%s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, config.DatabaseName, ColorReset, Bold, ColorReset)

	debugStatus := fmt.Sprintf("%s%s", func() string {
		if config.Debug {
			return ColorGreen + "ON"
		}
		return ColorRed + "OFF"
	}(), ColorReset)
	autoStatus := fmt.Sprintf("%s%s", func() string {
		if config.AutoExecute {
			return ColorGreen + "ON"
		}
		return ColorRed + "OFF"
	}(), ColorReset)
	threshold := fmt.Sprintf("%.1f%%", config.ConfidenceThreshold*100)

	fmt.Printf("%s%s║%s Debug Mode: %s%-28s%s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, debugStatus, ColorReset, Bold, ColorReset)
	fmt.Printf("%s%s║%s Auto-Execute: %s%-26s%s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, autoStatus, ColorReset, Bold, ColorReset)
	fmt.Printf("%s%s║%s Confidence Threshold: %s%-20s%s%s║%s\n", Bold, ColorPurple, ColorReset, Bold, threshold, ColorReset, Bold, ColorReset)
	fmt.Printf("%s%s╚════════════════════════════════════════════╝%s\n", Bold, ColorPurple, ColorReset)
}

// Do implements the readline.AutoCompleter interface
func (c *DynamicCompleter) Do(line []rune, pos int) ([][]rune, int) {
	if len(line) == 0 {
		return nil, 0
	}

	// Convert current input to string
	currentInput := string(line[:pos])

	// Skip completion for direct commands and special commands
	if strings.HasPrefix(currentInput, "!") || strings.HasPrefix(currentInput, "#") {
		return nil, 0
	}

	// Call predictNextWord
	suggestions, err := getNextWordSuggestions(currentInput)
	if err != nil {
		// Just return no suggestions on error
		return nil, 0
	}

	// Format suggestions
	var results [][]rune

	// Extract the last word - this is what we're completing
	words := strings.Fields(currentInput)
	lastWord := ""
	if len(words) > 0 && !strings.HasSuffix(currentInput, " ") {
		lastWord = words[len(words)-1]
	}

	// Create completion options
	for _, suggestion := range suggestions {
		results = append(results, []rune(suggestion))
	}

	// Return the completion options and the length of text to replace
	return results, len(lastWord)
}

// setupTabCompletion configures readline with tab completion
func setupTabCompletion() (*readline.Instance, error) {
	// Configure readline with custom completer
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          getPrompt(),
		HistoryFile:     ".command_history",
		AutoComplete:    &DynamicCompleter{},
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})

	return rl, err
}

// getNextWordSuggestions calls predictNextWord and formats the results
func getNextWordSuggestions(input string) ([]string, error) {
	// Call the predict function
	result, err := predictNextWord(input)
	if err != nil {
		return nil, err
	}

	// Extract suggestions from the result
	// This assumes the predictNextWord returns predictions in some format
	// You'll need to adapt this based on the actual structure of CommandResponse
	suggestions := []string{}

	// If NextWordSuggestions field exists in your CommandResponse
	if len(result.NextWordSuggestions) > 0 {
		suggestions = result.NextWordSuggestions
	} else {
		// Fallback: use the command itself as a suggestion
		if result.Command != "" {
			// If we have a command result but no explicit next word suggestions,
			// we'll use the command itself as a suggestion
			cmdParts := strings.Fields(result.Command)
			if len(cmdParts) > 0 {
				suggestions = append(suggestions, cmdParts[0])
			}

			// Add args as additional suggestions if available
			for _, arg := range result.Args {
				if arg != "" {
					suggestions = append(suggestions, arg)
				}
			}
		}
	}

	return suggestions, nil
}

// Main function
func main() {
	// Parse command line flags
	flag.Parse()

	// Try to connect to MongoDB for audit logging
	err := connectToMongoDB()
	if err != nil {
		fmt.Printf("%sWarning: Could not connect to MongoDB: %v%s\n", ColorYellow, err, ColorReset)
		fmt.Printf("%sAudit logging will be disabled%s\n", ColorYellow, ColorReset)
		auditCollection = nil
	}
	// Run the terminal
	runTerminal()
	// Close MongoDB connection if it was opened
	if mongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			fmt.Printf("%sError disconnecting from MongoDB: %v%s\n", ColorRed, err, ColorReset)
		}
	}
	// Exit the program
	fmt.Println("Goodbye!")
}
