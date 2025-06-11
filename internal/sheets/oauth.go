package sheets

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

// OAuth2Config holds OAuth2 configuration.
type OAuth2Config struct {
	ClientID     string
	ClientSecret string
	TokenFile    string // Where to save the token
}

// AuthenticateOAuth2Interactive performs the OAuth2 flow interactively.
func AuthenticateOAuth2Interactive(ctx context.Context, config OAuth2Config) (*oauth2.Token, error) {
	// Create OAuth2 config
	oauthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8080/callback",
		Scopes:       []string{sheets.SpreadsheetsScope},
	}

	// Start local server to receive the callback
	codeChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errorChan <- fmt.Errorf("no authorization code received")
			_, _ = fmt.Fprintf(w, `<html><body>
				<h1>Authentication Failed</h1>
				<p>No authorization code received. Please try again.</p>
				<script>window.setTimeout(function(){window.close();}, 3000);</script>
			</body></html>`)
			return
		}

		codeChan <- code
		_, _ = fmt.Fprintf(w, `<html><body>
			<h1>Authentication Successful!</h1>
			<p>You can close this window and return to the terminal.</p>
			<script>window.setTimeout(function(){window.close();}, 3000);</script>
		</body></html>`)
	})

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errorChan <- fmt.Errorf("failed to start callback server: %w", err)
		}
	}()

	// Generate auth URL with offline access for refresh token
	authURL := oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	slog.Info("🔐 Google Sheets Authentication Required")
	slog.Info("Please visit this URL to authenticate", "url", authURL)
	slog.Info("Waiting for authentication...")

	// Wait for callback or timeout
	var authCode string
	select {
	case authCode = <-codeChan:
		slog.Info("Received authorization code")
	case err := <-errorChan:
		_ = server.Shutdown(ctx)
		return nil, err
	case <-time.After(5 * time.Minute):
		_ = server.Shutdown(ctx)
		return nil, fmt.Errorf("authentication timeout - no response received within 5 minutes")
	}

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		slog.Warn("Error shutting down callback server", "error", err)
	}

	// Exchange code for token
	token, err := oauthConfig.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Save token if file path provided
	if config.TokenFile != "" {
		if err := saveToken(config.TokenFile, token); err != nil {
			slog.Warn("Failed to save token to file", "error", err, "file", config.TokenFile)
		} else {
			slog.Info("Token saved successfully", "file", config.TokenFile)
		}
	}

	return token, nil
}

// LoadToken loads a token from file.
func LoadToken(tokenFile string) (*oauth2.Token, error) {
	f, err := os.Open(tokenFile) // #nosec G304
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

// saveToken saves a token to file.
func saveToken(path string, token *oauth2.Token) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	// Write token
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("failed to encode token: %w", err)
	}

	return nil
}

// RefreshTokenIfNeeded refreshes the token if it's expired.
func RefreshTokenIfNeeded(ctx context.Context, config OAuth2Config, token *oauth2.Token) (*oauth2.Token, error) {
	// Check if token is still valid
	if token.Valid() {
		return token, nil
	}

	slog.Info("Token expired, refreshing...")

	// Create OAuth2 config
	oauthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{sheets.SpreadsheetsScope},
	}

	// Get new token using refresh token
	tokenSource := oauthConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Save new token
	if config.TokenFile != "" {
		if err := saveToken(config.TokenFile, newToken); err != nil {
			slog.Warn("Failed to save refreshed token", "error", err)
		}
	}

	return newToken, nil
}

// GetOrCreateToken gets an existing token or creates a new one.
func GetOrCreateToken(ctx context.Context, config OAuth2Config) (*oauth2.Token, error) {
	// Try to load existing token
	if config.TokenFile != "" {
		token, err := LoadToken(config.TokenFile)
		if err == nil {
			slog.Info("Loaded existing token from file")
			// Refresh if needed
			return RefreshTokenIfNeeded(ctx, config, token)
		}
		slog.Info("No existing token found, starting OAuth2 flow")
	}

	// No token found, do interactive auth
	return AuthenticateOAuth2Interactive(ctx, config)
}
