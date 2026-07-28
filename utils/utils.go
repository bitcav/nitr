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
	"runtime"
	"strings"
	"time"

	"github.com/bitcav/nitr/version"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/spf13/viper"
)

func ConfigFileSetup() {
	if _, err := os.Stat("config.ini"); err != nil {
		configFile, err := os.OpenFile("config.ini", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		LogError(err)
		defer configFile.Close()

		defaultConfigOpts := []string{
			"port: 8000",
			"open_browser_on_startup: true",
			"save_logs: false",
			"ssl_enabled: false",
			"# ssl_certificate: /path/to/file.crt ",
			"# ssl_certificate_key: /path/to/file.key",
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

	runPath, err := os.Getwd()
	LogError(err)

	viper.SetConfigName("config.ini")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(runPath)
	err = viper.ReadInConfig()
	if err != nil {
		LogError(err)
	}
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
func Logs(app *fiber.App) error {
	saveLogs := viper.GetBool("save_logs")
	if saveLogs {
		logFile, err := os.OpenFile("nitr.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("opening nitr.log: %w", err)
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
	addr := ":" + port
	sslEnabled := viper.GetBool("ssl_enabled")
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
			fmt.Println(err, "\nCheck settings at config.ini file")
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
			fmt.Println(err, "\nCheck settings at config.ini file")
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
