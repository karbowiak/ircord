package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"ircord/cli/internal/adminclient"
	"ircord/cli/internal/credentials"
)

type remoteFlags struct {
	server, tokenFile, config string
	timeout                   time.Duration
}

// Config files contain locations, never raw credentials. Explicit flags beat
// environment, then the optional config file, then defaults.
type remoteConfig struct {
	ServerURL      string `json:"serverUrl"`
	AdminTokenFile string `json:"adminTokenFile"`
	Timeout        string `json:"timeout"`
}

func readConfig(path string) (remoteConfig, error) {
	var config remoteConfig
	f, err := os.Open(path)
	if err != nil {
		return config, errors.New("cannot open CLI configuration file")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(b) > 65536 {
		return config, errors.New("cannot read CLI configuration file (maximum 65536 bytes)")
	}
	// Reject unknown keys so misspelled locations or accidental inline secrets
	// are not silently ignored. The raw content never enters diagnostics.
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&config) != nil || d.Decode(new(any)) != io.EOF {
		return remoteConfig{}, errors.New("invalid CLI configuration; expected serverUrl, adminTokenFile, and timeout")
	}
	if config.AdminTokenFile != "" && !filepath.IsAbs(config.AdminTokenFile) {
		config.AdminTokenFile = filepath.Join(filepath.Dir(path), config.AdminTokenFile)
	}
	return config, nil
}

func (f remoteFlags) client(cmd *cobra.Command, getenv func(string) string) (*adminclient.Client, error) {
	var config remoteConfig
	configFile := getenv("IRCORD_CLI_CONFIG")
	if cmd.Flags().Changed("config") {
		configFile = f.config
	}
	if configFile != "" {
		var err error
		config, err = readConfig(configFile)
		if err != nil {
			return nil, err
		}
	}
	server := config.ServerURL
	if v := getenv("IRCORD_SERVER_URL"); v != "" {
		server = v
	}
	if cmd.Flags().Changed("server") {
		server = f.server
	} else if server == "" {
		server = "http://127.0.0.1:8080"
	}
	timeout := 10 * time.Second
	duration := config.Timeout
	if v := getenv("IRCORD_CLI_TIMEOUT"); v != "" {
		duration = v
	}
	if cmd.Flags().Changed("timeout") {
		timeout = f.timeout
	} else if duration != "" {
		var err error
		timeout, err = time.ParseDuration(duration)
		if err != nil {
			return nil, errors.New("invalid administration timeout; use a duration such as 10s")
		}
	}
	value, file := "", config.AdminTokenFile
	if v, p := getenv("IRCORD_ADMIN_TOKEN"), getenv("IRCORD_ADMIN_TOKEN_FILE"); v != "" || p != "" {
		value, file = v, p
	}
	if cmd.Flags().Changed("admin-token-file") {
		value, file = "", f.tokenFile
	}
	token, err := credentials.LoadToken(value, file)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, errors.New("admin token required; set IRCORD_ADMIN_TOKEN_FILE, IRCORD_ADMIN_TOKEN, or --admin-token-file")
	}
	return adminclient.New(server, token, timeout)
}
