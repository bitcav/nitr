package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bitcav/nitr/version"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/spf13/viper"
)

// ConfigFileSetup loads configuration into viper, creating a default config
// file when none exists. Every key resolves as:
//
//	--flag > NITR_* env var > config file > built-in default
//
// The config file is config.ini in the working directory unless the config
// key (--config flag / NITR_CONFIG env) points elsewhere. Despite the .ini
// extension it is parsed as YAML.
func ConfigFileSetup() {
	viper.SetEnvPrefix("NITR")
	viper.AutomaticEnv()

	if cfgFile := viper.GetString("config"); cfgFile != "" {
		if _, err := os.Stat(cfgFile); err != nil {
			writeDefaultConfig(cfgFile)
		}
		viper.SetConfigFile(cfgFile)
	} else {
		if _, err := os.Stat("config.ini"); err != nil {
			writeDefaultConfig("config.ini")
		}
		runPath, err := os.Getwd()
		LogError(err)
		viper.SetConfigName("config.ini")
		viper.AddConfigPath(runPath)
	}
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		LogError(err)
	}
}

// writeDefaultConfig creates path with the default settings. The header
// states plainly that the syntax is YAML despite the .ini extension — real
// INI (key=value, [sections]) is silently ignored by the YAML parser.
func writeDefaultConfig(path string) {
	configFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		LogError(err)
		return
	}
	defer configFile.Close()

	defaultConfigOpts := []string{
		"# Nitr configuration. Parsed as YAML despite the .ini extension:",
		"# write `key: value` lines, not INI `key=value` or [sections].",
		"# Precedence: --flags > NITR_* env vars (NITR_PORT, NITR_BIND_ADDRESS,",
		"# NITR_DATA_DIR, ...) > this file > built-in defaults.",
		"port: 8000",
		"# address to bind: 0.0.0.0 is all interfaces, 127.0.0.1 localhost only",
		"bind_address: 0.0.0.0",
		"open_browser_on_startup: true",
		"save_logs: false",
		"ssl_enabled: false",
		"# ssl_certificate: /path/to/file.crt ",
		"# ssl_certificate_key: /path/to/file.key",
		"# directory holding nitr.db (default: nitr's working directory)",
		"# data_dir: /var/lib/nitr",
		"# seconds between live-metrics pushes over the /status socket (default 3)",
		"metrics_push_interval: 3",
		"# max requests per minute per IP on the login POST (default 20)",
		"# rate_limit_login_max: 20",
		"# max requests per minute per IP on the /api endpoints (default 300)",
		"# rate_limit_api_max: 300",
		"# comma-separated browser origins allowed cross-origin API access;",
		"# empty (the default) denies all cross-origin access",
		"# cors_origins: https://grafana.example.com, https://dash.example.com",
	}

	defaultConfig := strings.Join(defaultConfigOpts, "\n")
	_, err = configFile.WriteString(defaultConfig)
	LogError(err)
}

// OpenBrowser opens default web browser in specific domain
func OpenBrowser(domain, port string) error {
	url := domain + ":" + port
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		return err
	}
	return nil
}

// openBrowserFunc is a seam so tests can stub browser opening without
// spawning a real process. It points at OpenBrowser in production.
var openBrowserFunc = OpenBrowser

// ListenFunc is a seam so tests can serve on a pre-bound listener (closing
// the TOCTOU window of bind-close-rebind across the separate OS processes
// that `go test ./...` spawns per package) or force StartServer down its
// listen-error path without depending on OS socket-binding semantics, which
// differ between Linux and Windows (Windows permits some double-binds Linux
// refuses). It calls app.Listen in production.
var ListenFunc = func(app *fiber.App, addr string) error {
	return app.Listen(addr)
}

const charset = "abcdefghijkmnpqrstuvwxyz" +
	"123456789"

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		// package-level rand.IntN is goroutine-safe and auto-seeded
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}

// RandString returns random string with specific length
func RandString(length int) string {
	return stringWithCharset(length, charset)
}

// StartMessage displays message on server start up
func StartMessage(protocol, port string) {
	fmt.Printf(`       
     _____________
    /            /\          _  __    
   /   /    /   / /   ___   (_)/ /_ ____
  /   /    /   / /   / _ \ / // __// __/    
 /            / /   /_//_//_/ \__//_/
/____________/ / 	    
\____________\/     v%v

Go to admin panel at %v://localhost:%v

`, version.Version, protocol, port)
}

// Logs wires request logging to nitr.log when save_logs is enabled. It
// returns an error rather than terminating the process: Logs is reached
// during startup (program.Start -> server), where a fatal log would
// os.Exit the whole program from outside main's error-reporting path.
//
// nitr.log lives under data_dir (the same --data-dir / NITR_DATA_DIR key
// nitr.db uses) rather than under its own key: a separate log_file key
// would need the same --flag / NITR_ env / config-file plumbing the other
// keys have, and shipping a config key with no flag or env counterpart
// would itself be an inconsistency. A single mounted directory is also the
// simpler container story — the natural deployment for save_logs is a
// container with one persisted volume. If log retention versus DB growth
// ever needs to diverge, add a log_file key with the full flag/env/config
// treatment at that point; it is not a reason to keep nitr.log drifting
// from nitr.db in the meantime.
func Logs(app *fiber.App) error {
	saveLogs := viper.GetBool("save_logs")
	if saveLogs {
		logPath := LogPath()
		if dir := filepath.Dir(logPath); dir != "." {
			// A configured data dir may not exist yet (fresh Docker volume);
			// the cwd default always does. Errors surface at OpenFile with
			// context, matching database.SetupDB's handling of the same case.
			_ = os.MkdirAll(dir, 0755)
		}
		logFile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("opening %s: %w", logPath, err)
		}
		//defer logFile.Close()

		log.SetOutput(logFile)

		cfg := logger.Config{
			Output:     logFile,
			TimeFormat: "2006/01/02 15:04:05",
			// requestid middleware sets locals("requestid"); logging it
			// correlates an access-log line with error reports. The logger
			// writes after c.Next returns, so the value is set even though
			// logger is registered after requestid.
			Format: "${time} - ${method} ${path} - ${ip} - ${locals:requestid}\n",
		}

		app.Use(logger.New(cfg))
	}

	return nil
}

// LogPath is the single resolution point for nitr.log's location, mirroring
// database.DBPath for nitr.db: the data_dir key (--data-dir flag /
// NITR_DATA_DIR env / data_dir in the config file) joined with the file
// name, or the bare file name — i.e. the working directory, exactly as
// before — when data_dir is unset. utils cannot import database (database
// imports utils), so this is the sibling, not a caller, of DBPath; the two
// share the data_dir resolution pattern but each owns its own literal, so
// a rename of either file touches exactly one place.
func LogPath() string {
	if dir := viper.GetString("data_dir"); dir != "" {
		return filepath.Join(dir, "nitr.log")
	}
	return "nitr.log"
}

// RateLimitMax returns the per-IP requests-per-minute cap for a rate limiter,
// read from config.ini under key, falling back to def when unset or invalid.
func RateLimitMax(key string, def int) int {
	n := viper.GetInt(key)
	if n < 1 {
		n = def
	}
	return n
}

// CORSOrigins returns the comma-separated browser origins allowed cross-origin
// API access. Empty (the default) means none: the caller must then skip
// registering fiber's CORS middleware entirely, because fiber substitutes "*"
// for an empty AllowOrigins, which would open rather than deny.
func CORSOrigins() string {
	return viper.GetString("cors_origins")
}

func LogError(e error) {
	if e != nil {
		log.Println(e)
	}
}

// GetLocalIP returns the host's outbound IP. It returns an error rather than
// terminating the process: the dial fails on any host with no default route
// (air-gapped box, restricted container, egress-filtered network), and this
// runs on the panel request path, where exiting would kill the server.
func GetLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("determining local IP: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return fmt.Sprint(localAddr.IP), nil
}

func GetLocalPort() string {
	port := viper.GetString("port")
	if port == "" {
		port = "8000"
	}
	return port
}

// BindAddress returns the interface address StartServer binds to: the
// bind_address key (--host/--bind flag, NITR_BIND_ADDRESS env), defaulting
// to 0.0.0.0 (all interfaces) so behaviour without configuration is
// unchanged. Set 127.0.0.1 to serve localhost only.
func BindAddress() string {
	addr := viper.GetString("bind_address")
	if addr == "" {
		addr = "0.0.0.0"
	}
	return addr
}

// MetricsPushInterval returns the cadence at which the /status socket streams
// live host metrics to the panel. It reads metrics_push_interval (seconds)
// from config.ini, defaulting to 3s and clamping to a 1s floor so a missing
// or bogus value cannot busy-loop the socket and starve a slow client.
func MetricsPushInterval() time.Duration {
	seconds := viper.GetInt("metrics_push_interval")
	if seconds < 1 {
		seconds = 3
	}
	return time.Duration(seconds) * time.Second
}

func StartServer(app *fiber.App) {
	port := GetLocalPort()
	addr := net.JoinHostPort(BindAddress(), port)
	sslEnabled := viper.GetBool("ssl_enabled")

	// The file settings actually came from, for the listen-error hint —
	// "config.ini" is wrong when --config pointed elsewhere.
	cfgFile := viper.ConfigFileUsed()
	if cfgFile == "" {
		cfgFile = "config.ini"
	}
	if sslEnabled {
		cert := viper.GetString("ssl_certificate")
		key := viper.GetString("ssl_certificate_key")

		StartMessage("https", port)

		openBrowser := viper.GetBool("open_browser_on_startup")
		if openBrowser {
			LogError(openBrowserFunc("https://localhost", port))
		}

		log.Println("Starting server")

		err := app.ListenTLS(addr, cert, key)
		if err != nil {
			fmt.Println(err, "\nCheck settings at "+cfgFile)
		}
		LogError(err)

	} else {
		StartMessage("http", port)
		openBrowser := viper.GetBool("open_browser_on_startup")
		if openBrowser {
			LogError(openBrowserFunc("http://localhost", port))
		}

		log.Println("Starting server")

		err := ListenFunc(app, addr)
		if err != nil {
			fmt.Println(err, "\nCheck settings at "+cfgFile)
		}
		LogError(err)
	}
}

func PasswordHash(password string) string {
	h := sha256.New()
	h.Write([]byte(password))
	sha1_hash := hex.EncodeToString(h.Sum(nil))

	return sha1_hash
}
