package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Validator represents the structure of validator data from the API response
type Validator struct {
	Validator       string `json:"validator"`       // Address of the validator
	Name            string `json:"name"`            // Name of the validator
	IsJailed        bool   `json:"isJailed"`        // Indicates if validator is jailed
	IsActive        bool   `json:"isActive"`        // Indicates if validator is active
	Commission      string `json:"commission"`      // Commission rate charged by validator
	UnjailableAfter *int64 `json:"unjailableAfter"` // Timestamp when validator can be unjailed (null if not jailed)
}

// NotificationBackoff handles exponential backoff for alerts to prevent notification spam
type NotificationBackoff struct {
	LastSent      time.Time // When the last notification was sent
	BackoffFactor int       // Current backoff multiplier
}

// ValidatorState tracks validator status between checks for state change detection
type ValidatorState struct {
	IsJailed bool // Current jailed status
	IsActive bool // Current active status
	FirstRun bool // Indicates first check to prevent false recovery alerts
}

// Global variables for tracking notification state and validator status
var (
	jailedBackoff   = &NotificationBackoff{}
	inactiveBackoff = &NotificationBackoff{}
	recoveryBackoff = &NotificationBackoff{}
	validatorState  = &ValidatorState{FirstRun: true}
)

// Constants for backoff timing
const (
	initialBackoff = time.Minute      // Base backoff interval
	maxBackoff     = 15 * time.Minute // Maximum backoff interval to prevent excessive delays
)

// getEnv retrieves environment variable with fallback for CRON_INTERVAL
func getEnv(key string) string {
	val := os.Getenv(key)
	if val == "" && key == "CRON_INTERVAL" {
		return "1m" // Default check interval
	}
	if val == "" {
		log.Fatalf("ENV variable %s is required", key)
	}
	return val
}

// notifyDiscord sends alerts to Discord webhook
func notifyDiscord(webhook, message string) {
	payload := map[string]string{"content": message}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Failed to send Discord notification: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		log.Printf("Discord webhook returned status: %s", resp.Status)
	} else {
		log.Println("Sent Discord alert:", message)
	}
}

// shouldNotify determines if enough time has passed since last notification
// based on the current backoff factor
func shouldNotify(state *NotificationBackoff) bool {
	if time.Since(state.LastSent) >= time.Duration(state.BackoffFactor)*initialBackoff {
		return true
	}
	return false
}

// formatValidatorIdentifier creates a consistent identifier string with name and address
// to uniquely identify validators in logs and notifications
func formatValidatorIdentifier(name, address string) string {
	if name == "" {
		return fmt.Sprintf("%s", address)
	}
	return fmt.Sprintf("%s (%s)", name, address)
}

// updateBackoff increases the backoff factor exponentially up to a maximum value
func updateBackoff(state *NotificationBackoff) {
	state.LastSent = time.Now()
	if state.BackoffFactor == 0 {
		state.BackoffFactor = 1
	} else {
		state.BackoffFactor *= 2
		if time.Duration(state.BackoffFactor)*initialBackoff > maxBackoff {
			state.BackoffFactor = int(maxBackoff / initialBackoff)
		}
	}
}

// resetBackoff resets backoff state when conditions return to normal
func resetBackoff(state *NotificationBackoff) {
	state.BackoffFactor = 0
	state.LastSent = time.Time{}
}

// runCheck performs a single validation check cycle
func runCheck(apiEndpoint, validatorAddress, discordWebhook string) {
	startTime := time.Now()
	log.Printf("Fetching validator status for address: %s", validatorAddress)

	validator, err := fetchValidatorData(apiEndpoint, validatorAddress)
	if err != nil {
		log.Printf("Error fetching validator data: %v", err)
		return
	}

	validatorName := validator.Name
	if validatorName == "" {
		validatorName = validatorAddress[:10] + "..." // Use truncated address if name is not available
	}

	validatorIdentifier := formatValidatorIdentifier(validatorName, validatorAddress)

	// Log detailed validator information
	log.Printf("Validator %s status: active=%v, jailed=%v, commission=%s",
		validatorIdentifier, validator.IsActive, validator.IsJailed, validator.Commission)

	// Recovery detection - only after first run completed
	if !validatorState.FirstRun {
		// Check for jailed -> not jailed transition
		if validatorState.IsJailed && !validator.IsJailed {
			message := fmt.Sprintf("✅ Validator %s has RECOVERED from jailed state", validatorIdentifier)
			notifyDiscord(discordWebhook, message)
			log.Printf("Recovery detected: %s", message)
			resetBackoff(recoveryBackoff)
		}

		// Check for inactive -> active transition
		if !validatorState.IsActive && validator.IsActive {
			message := fmt.Sprintf("✅ Validator %s is now ACTIVE", validatorIdentifier)
			notifyDiscord(discordWebhook, message)
			log.Printf("Recovery detected: %s", message)
			resetBackoff(recoveryBackoff)
		}
	}

	// Update state for next comparison
	validatorState.IsJailed = validator.IsJailed
	validatorState.IsActive = validator.IsActive
	validatorState.FirstRun = false

	// Handle jailed status alerts with backoff
	if validator.IsJailed {
		if shouldNotify(jailedBackoff) {
			unjailMsg := ""
			if validator.UnjailableAfter != nil {
				unjailTime := time.Unix(*validator.UnjailableAfter/1000, 0)
				unjailMsg = fmt.Sprintf(" (unjailable after %s)", unjailTime.Format(time.RFC3339))
			}

			message := fmt.Sprintf("🚨 Validator %s is JAILED%s", validatorIdentifier, unjailMsg)
			notifyDiscord(discordWebhook, message)
			log.Printf("Alert: %s", message)
			updateBackoff(jailedBackoff)
		}
	} else {
		resetBackoff(jailedBackoff)
	}

	// Handle inactive status alerts with backoff
	if !validator.IsActive {
		if shouldNotify(inactiveBackoff) {
			message := fmt.Sprintf("🚨 Validator %s is INACTIVE", validatorIdentifier)
			notifyDiscord(discordWebhook, message)
			log.Printf("Alert: %s", message)
			updateBackoff(inactiveBackoff)
		}
	} else {
		resetBackoff(inactiveBackoff)
	}

	elapsed := time.Since(startTime)
	log.Printf("Validator %s monitor check complete (took %dms)", validatorIdentifier, elapsed.Milliseconds())
}

// fetchValidatorData retrieves validator data from the API and finds the requested validator
// Case-insensitive comparison is used for addresses to prevent configuration errors
func fetchValidatorData(apiEndpoint string, validatorAddress string) (*Validator, error) {
	startTime := time.Now()

	payload := []byte(`{"type":"validatorSummaries"}`)
	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-200 status: %s", resp.Status)
	}

	var allValidators []Validator
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if err := json.Unmarshal(body, &allValidators); err != nil {
		return nil, fmt.Errorf("error parsing API response: %w", err)
	}

	log.Printf("API returned data for %d validators (took %dms)", len(allValidators), time.Since(startTime).Milliseconds())

	lowercaseInputAddress := strings.ToLower(validatorAddress)
	for _, val := range allValidators {
		if strings.ToLower(val.Validator) == lowercaseInputAddress {
			return &val, nil
		}
	}

	return nil, fmt.Errorf("validator with address '%s' not found among %d validators",
		validatorAddress, len(allValidators))
}

// main initializes the application and starts the monitoring loop
func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
	log.Printf("Validator Monitor starting up...")

	apiEndpoint := getEnv("API_ENDPOINT")
	validatorAddress := getEnv("VALIDATOR_ADDRESS")
	discordWebhook := getEnv("DISCORD_WEBHOOK")
	cronInterval := getEnv("CRON_INTERVAL")

	duration, err := time.ParseDuration(cronInterval)
	if err != nil {
		log.Fatalf("Invalid CRON_INTERVAL '%s': %v", cronInterval, err)
	}

	log.Printf("Configuration loaded - API: %s, Address: %s, Interval: %s",
		apiEndpoint, validatorAddress, duration)

	// Initial notification to confirm monitoring has started
	notifyDiscord(discordWebhook, fmt.Sprintf("🔄 Validator monitoring started for %s (checking every %s)",
		validatorAddress, duration))

	// Main monitoring loop
	for {
		runCheck(apiEndpoint, validatorAddress, discordWebhook)
		log.Printf("Sleeping for %s before next check", duration)
		time.Sleep(duration)
	}
}
