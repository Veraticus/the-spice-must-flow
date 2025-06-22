package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/certs"
	"github.com/Veraticus/the-spice-must-flow/internal/plaid"
	"github.com/Veraticus/the-spice-must-flow/internal/sheets"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with external services",
		Long:  `Authenticate with external services like Plaid and Google Sheets.`,
	}

	cmd.AddCommand(authPlaidCmd())
	cmd.AddCommand(authSheetsCmd())

	return cmd
}

func authPlaidCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plaid",
		Short: "Connect bank accounts via Plaid",
		Long: `Connect your bank accounts using Plaid Link.

This command will:
1. Start a local web server
2. Open Plaid Link in your browser
3. Let you connect one or more bank accounts
4. Save the access tokens for future use

You can run this multiple times to add more accounts.`,
		RunE: runAuthPlaid,
	}

	cmd.Flags().String("env", "", "Plaid environment (sandbox/production)")
	cmd.Flags().Bool("update-primary", false, "Update the primary account after linking")

	return cmd
}

func runAuthPlaid(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Get Plaid configuration
	clientID := viper.GetString("plaid.client_id")
	secret := viper.GetString("plaid.secret")
	environment := viper.GetString("plaid.environment")

	// Override with flag if provided
	if flagEnv, _ := cmd.Flags().GetString("env"); flagEnv != "" {
		environment = flagEnv
	}

	// Check environment variables as fallback
	if clientID == "" {
		clientID = os.Getenv("PLAID_CLIENT_ID")
	}
	if secret == "" {
		secret = os.Getenv("PLAID_SECRET")
	}
	if environment == "" {
		environment = os.Getenv("PLAID_ENV")
		if environment == "" {
			environment = "production" // default - for real bank data
		}
	}

	if clientID == "" || secret == "" {
		return fmt.Errorf("plaid credentials missing. Please add your Client ID and Secret to the config file or set PLAID_CLIENT_ID and PLAID_SECRET environment variables")
	}

	slog.Info("Starting Plaid Link flow", "environment", environment)

	// Create Plaid client
	plaidClient, err := plaid.NewClient(plaid.Config{
		ClientID:    clientID,
		Secret:      secret,
		Environment: environment,
	})
	if err != nil {
		return fmt.Errorf("failed to create Plaid client: %w", err)
	}

	// Create link token
	linkToken, err := plaidClient.CreateLinkToken(ctx)
	if err != nil {
		// If it fails due to redirect URI, provide helpful message
		if strings.Contains(err.Error(), "redirect") || strings.Contains(err.Error(), "OAuth") {
			slog.Error("OAuth setup required for this bank. Please follow the instructions below:")
			slog.Info("")
			slog.Info("This error occurs when the redirect URI isn't configured in Plaid Dashboard")
			slog.Info("")
			slog.Info("To fix this:")
			slog.Info("1. Log into https://dashboard.plaid.com")
			slog.Info("2. Go to Team Settings → API")
			slog.Info("3. Add to Allowed redirect URIs: https://localhost:8080/")
			slog.Info("4. Save changes")
			slog.Info("")
			slog.Info("Until then, you can:")
			slog.Info("• Use sandbox mode: spice auth plaid --env sandbox")
			slog.Info("• Search for non-OAuth banks: spice institutions search [bank]")
		}
		return fmt.Errorf("failed to create link token: %w", err)
	}

	// Set up channels for communication
	successChan := make(chan plaidLinkSuccess, 1)
	errorChan := make(chan error, 1)

	// Start local server
	server := &http.Server{Addr: ":8080"}

	// Serve the Plaid Link page
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Connect Your Bank Account - Spice</title>
    <script src="https://cdn.plaid.com/link/v2/stable/link-initialize.js"></script>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; 
               display: flex; align-items: center; justify-content: center; height: 100vh; 
               margin: 0; background-color: #f5f5f5; }
        .container { text-align: center; background: white; padding: 40px; border-radius: 8px; 
                    box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; margin-bottom: 20px; }
        button { background-color: #4CAF50; color: white; padding: 12px 24px; 
                font-size: 16px; border: none; border-radius: 4px; cursor: pointer; }
        button:hover { background-color: #45a049; }
        .error { color: #d32f2f; margin-top: 20px; }
        .success { color: #388e3c; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🌶️ Connect Your Bank Account</h1>
        <p>Click the button below to securely connect your bank account through Plaid.</p>
        <button id="link-button">Connect Bank Account</button>
        <div id="message"></div>
    </div>
    
    <script>
    const linkHandler = Plaid.create({
        token: '%s',
        onSuccess: (public_token, metadata) => {
            document.getElementById('message').innerHTML = 
                '<div class="success">🔄 Processing connection...</div>';
            
            fetch('/exchange', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ public_token, metadata })
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    document.getElementById('message').innerHTML = 
                        '<div class="success">✅ Account connected successfully! You can close this window.</div>';
                    // For desktop flow, give user time to see the message
                    setTimeout(() => {
                        if (!window.closed) {
                            document.getElementById('message').innerHTML += 
                                '<div class="success">You can now close this browser tab.</div>';
                        }
                    }, 3000);
                } else {
                    document.getElementById('message').innerHTML = 
                        '<div class="error">❌ ' + (data.error || 'Connection failed') + '</div>';
                }
            })
            .catch(error => {
                document.getElementById('message').innerHTML = 
                    '<div class="error">❌ Network error: ' + error + '</div>';
            });
        },
        onExit: (err, metadata) => {
            if (err != null) {
                document.getElementById('message').innerHTML = 
                    '<div class="error">Connection canceled or failed.</div>';
            }
        }
    });
    
    document.getElementById('link-button').onclick = () => {
        linkHandler.open();
    };
    </script>
</body>
</html>`, linkToken)

		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, html)
	})

	// Handle token exchange
	http.HandleFunc("/exchange", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PublicToken string `json:"public_token"`
			Metadata    struct {
				Institution struct {
					Name string `json:"name"`
					ID   string `json:"institution_id"`
				} `json:"institution"`
				Accounts []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"accounts"`
			} `json:"metadata"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Invalid request",
			})
			return
		}

		// Exchange public token for access token
		accessToken, itemID, err := plaidClient.ExchangePublicToken(ctx, req.PublicToken)
		if err != nil {
			errorChan <- fmt.Errorf("failed to exchange token: %w", err)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Failed to exchange token",
			})
			return
		}

		successChan <- plaidLinkSuccess{
			AccessToken:     accessToken,
			ItemID:          itemID,
			InstitutionName: req.Metadata.Institution.Name,
			InstitutionID:   req.Metadata.Institution.ID,
			Accounts:        req.Metadata.Accounts,
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
		})
	})

	// Configure HTTPS for production environment
	var browserURL string
	if environment == "production" {
		// Get certificate directory
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			configDir = filepath.Join(home, ".config")
		}
		certDir := filepath.Join(configDir, "spice", "certs")

		// Get or create certificate
		certManager := certs.NewFileManager(certDir)
		cert, err := certManager.GetOrCreateCertificate()
		if err != nil {
			return fmt.Errorf("failed to get/create certificate: %w", err)
		}

		// Configure TLS
		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		browserURL = "https://localhost:8080"

		// Start HTTPS server
		go func() {
			if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
				errorChan <- fmt.Errorf("failed to start HTTPS server: %w", err)
			}
		}()

		slog.Info("🏦 Plaid Account Connection (Production)")
		slog.Info("Starting secure HTTPS server...")
		slog.Info("")
		slog.Info("⚠️  BROWSER SECURITY WARNING EXPECTED")
		slog.Info("Your browser will show a security warning about the certificate.")
		slog.Info("This is normal for local development. To proceed:")
		slog.Info("  1. Click 'Advanced' or 'Show Details'")
		slog.Info("  2. Click 'Proceed to localhost' or 'Visit this website'")
		slog.Info("")
	} else {
		// Sandbox mode uses HTTP
		browserURL = "http://localhost:8080"

		go func() {
			if err := server.ListenAndServe(); err != http.ErrServerClosed {
				errorChan <- fmt.Errorf("failed to start server: %w", err)
			}
		}()

		slog.Info("🏦 Plaid Account Connection (Sandbox)")
		slog.Info("Starting server...")
	}

	slog.Info("Opening your browser to connect bank accounts...")
	slog.Info("If the browser doesn't open, visit:", "url", browserURL)

	// Try to open browser
	openBrowser(browserURL)

	// Wait for result
	var result plaidLinkSuccess
	select {
	case result = <-successChan:
		slog.Info("Successfully linked account", "institution", result.InstitutionName)
	case err := <-errorChan:
		_ = server.Shutdown(ctx)
		return err
	case <-time.After(10 * time.Minute):
		_ = server.Shutdown(ctx)
		return fmt.Errorf("timeout waiting for account connection")
	}

	// Shutdown server
	_ = server.Shutdown(ctx)

	// Save the access token
	if err := savePlaidConnection(result); err != nil {
		return fmt.Errorf("failed to save connection: %w", err)
	}

	slog.Info("Successfully connected to bank", "institution", result.InstitutionName, "accounts", len(result.Accounts))

	// List accounts
	if len(result.Accounts) > 0 {
		slog.Info("Connected accounts:")
		for _, acc := range result.Accounts {
			slog.Info("  Account", "name", acc.Name, "type", acc.Type)
		}
	}

	updatePrimary, _ := cmd.Flags().GetBool("update-primary")
	if updatePrimary || viper.GetString("plaid.access_token") == "" {
		// Update primary access token if requested or if none exists
		viper.Set("plaid.access_token", result.AccessToken)
		if err := saveConfig(); err != nil {
			slog.Warn("Failed to update config file", "error", err)
		} else {
			slog.Info("📝 Updated primary access token in config")
		}
	}

	slog.Info("🎉 Your bank account is now connected!")
	slog.Info("Run 'spice import' to import transactions")

	return nil
}

func authSheetsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sheets",
		Short: "Authenticate with Google Sheets",
		Long: `Authenticate with Google Sheets using OAuth2.

This command will:
1. Open your browser to authenticate with Google
2. Save the refresh token for future use
3. Update your config file with the token

You'll need to run this once to set up Google Sheets integration.`,
		RunE: runAuthSheets,
	}

	cmd.Flags().String("client-id", "", "OAuth2 Client ID (overrides config)")
	cmd.Flags().String("client-secret", "", "OAuth2 Client Secret (overrides config)")

	return cmd
}

func runAuthSheets(cmd *cobra.Command, _ []string) error {
	// This is essentially the same as the previous sheetsAuthCmd
	// Just moved here for consistency
	ctx := cmd.Context()

	// Get OAuth2 config
	clientID := viper.GetString("sheets.client_id")
	clientSecret := viper.GetString("sheets.client_secret")

	// Override with flags if provided
	if flagID, _ := cmd.Flags().GetString("client-id"); flagID != "" {
		clientID = flagID
	}
	if flagSecret, _ := cmd.Flags().GetString("client-secret"); flagSecret != "" {
		clientSecret = flagSecret
	}

	// Check for environment variables as fallback
	if clientID == "" {
		clientID = os.Getenv("GOOGLE_SHEETS_CLIENT_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("GOOGLE_SHEETS_CLIENT_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("OAuth2 credentials not found. Please set sheets.client_id and sheets.client_secret in config or use --client-id and --client-secret flags")
	}

	// Determine token file location
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(home, ".config")
	}
	tokenFile := filepath.Join(configDir, "spice", "sheets-token.json")

	slog.Info("Starting Google Sheets authentication", "token_file", tokenFile)

	// Perform OAuth2 flow
	config := sheets.OAuth2Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenFile:    tokenFile,
	}

	token, err := sheets.AuthenticateOAuth2Interactive(ctx, config)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Update config file with refresh token
	viper.Set("sheets.refresh_token", token.RefreshToken)

	if err := saveConfig(); err != nil {
		slog.Warn("Failed to update config file with refresh token", "error", err)
		slog.Warn("⚠️  Could not save refresh token to config file")
		slog.Info("Please add this to your config.yaml manually:")
		slog.Info(fmt.Sprintf("sheets:\n  refresh_token: \"%s\"", token.RefreshToken))
	} else {
		slog.Info("Updated config file with refresh token")
		slog.Info("✅ Authentication successful!")
	}

	slog.Info("📊 Google Sheets is now configured and ready to use.")
	slog.Info("Run 'spice export' to generate reports.")

	return nil
}

type plaidLinkSuccess struct {
	AccessToken     string
	ItemID          string
	InstitutionName string
	InstitutionID   string
	Accounts        []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
}

func savePlaidConnection(conn plaidLinkSuccess) error {
	// Load existing connections
	connections := viper.GetStringMap("plaid.connections")
	if connections == nil {
		connections = make(map[string]any)
	}

	// Add new connection
	connections[conn.ItemID] = map[string]any{
		"access_token":     conn.AccessToken,
		"institution_name": conn.InstitutionName,
		"institution_id":   conn.InstitutionID,
		"connected_at":     time.Now().Format(time.RFC3339),
		"accounts":         conn.Accounts,
	}

	viper.Set("plaid.connections", connections)
	return saveConfig()
}

func saveConfig() error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		configFile = filepath.Join(home, ".config", "spice", "config.yaml")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configFile), 0750); err != nil {
		return err
	}

	return viper.WriteConfigAs(configFile)
}

// openBrowser tries to open the URL in the default browser.
func openBrowser(url string) {
	var err error
	switch os := runtime.GOOS; os {
	case "linux":
		err = exec.Command("xdg-open", url).Start() //nolint:gosec,forbidigo
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() //nolint:gosec,forbidigo
	case "darwin":
		err = exec.Command("open", url).Start() //nolint:gosec,forbidigo
	}
	if err != nil {
		slog.Debug("Failed to open browser", "error", err)
	}
}
