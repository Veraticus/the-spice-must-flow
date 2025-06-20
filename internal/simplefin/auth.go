package simplefin

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// AuthState represents the saved SimpleFIN authentication state
type AuthState struct {
	AccessURL  string    `json:"access_url"`
	ClaimedAt  time.Time `json:"claimed_at"`
	ClaimToken string    `json:"claim_token_hash"` // Store hash for tracking
}

// LoadOrClaimAuth loads existing auth or claims a new token
func LoadOrClaimAuth(token string) (*AuthState, error) {
	// Get state file path
	stateFile, err := getStateFilePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get state file path: %w", err)
	}

	// Try to load existing auth
	auth, err := loadAuthState(stateFile)
	if err == nil && auth.AccessURL != "" {
		// Valid auth exists
		slog.Info("Using saved SimpleFIN access URL",
			"claimed_at", auth.ClaimedAt.Format("2006-01-02"),
			"state_file", stateFile)
		return auth, nil
	}

	// No valid auth, claim the token
	slog.Info("No saved auth found, claiming new SimpleFIN token")
	accessURL, err := claimToken(token)
	if err != nil {
		return nil, fmt.Errorf("failed to claim token: %w", err)
	}

	// Save the new auth state
	newAuth := &AuthState{
		AccessURL:  accessURL,
		ClaimedAt:  time.Now(),
		ClaimToken: hashToken(token),
	}

	if err := saveAuthState(stateFile, newAuth); err != nil {
		return nil, fmt.Errorf("failed to save auth state: %w", err)
	}

	slog.Info("Successfully claimed and saved SimpleFIN access URL",
		"state_file", stateFile)

	return newAuth, nil
}

func getStateFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Use XDG_DATA_HOME if set, otherwise ~/.local/share
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		dataDir = filepath.Join(home, ".local", "share")
	}

	spiceDir := filepath.Join(dataDir, "spice")
	if err := os.MkdirAll(spiceDir, 0700); err != nil {
		return "", err
	}

	return filepath.Join(spiceDir, "simplefin_auth.json"), nil
}

func loadAuthState(path string) (*AuthState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var auth AuthState
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, err
	}

	return &auth, nil
}

func saveAuthState(path string, auth *AuthState) error {
	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600) // Read/write for owner only
}

func hashToken(token string) string {
	// Just store first/last 8 chars for identification
	if len(token) > 16 {
		return token[:8] + "..." + token[len(token)-8:]
	}
	return "short_token"
}
