package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	rice "github.com/GeertJohan/go.rice"
	ndb "github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/handlers"
	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/fiberweb/apikey"

	"github.com/gofiber/embed"
	"github.com/gofiber/fiber"
	"github.com/gofiber/logger"
	"github.com/gofiber/recover"
	"github.com/gofiber/websocket"

	"github.com/skip2/go-qrcode"

	"github.com/bitcav/nitr-core/host"
	"github.com/spf13/viper"
)

func init() {
	//Config file initial setup
	if _, err := os.Stat("config.ini"); err != nil {
		configFile, err := os.OpenFile("config.ini", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		utils.LogError(err)
		defer configFile.Close()

		defaultConfigOpts := []string{
			"port: 8000",
			"open_browser_on_startup: true",
			"save_logs: false",
			"ssl_enabled: false",
			"# ssl_certificate: /path/to/file.crt ",
			"# ssl_certificate_key: /path/to/file.key",
		}

		defaultConfig := strings.Join(defaultConfigOpts, "\n")
		_, err = configFile.WriteString(defaultConfig)
		utils.LogError(err)
	}

	runPath, err := os.Getwd()
	utils.LogError(err)

	viper.SetConfigName("config.ini")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(runPath)
	err = viper.ReadInConfig()
	if err != nil {
		utils.LogError(err)
	}

	//DB Setup
	if _, err := os.Stat("nitr.db"); err != nil {
		log.Println("Database created")
		err := ndb.SetupDB()
		utils.LogError(err)

		log.Println("Adding default user")

		APIKey := utils.RandString(10)

		port := viper.GetString("port")
		if port == "" {
			port = "3000"
		}

		qr := models.QR{
			Name:        host.Info().Name,
			Description: host.Info().Platform,
			Port:        port,
			Key:         APIKey,
		}

		qrJSON, err := json.Marshal(qr)
		if err != nil {
			utils.LogError(err)
		}

		png, err := qrcode.Encode(string(qrJSON), qrcode.Medium, 256)
		uEncQr := base64.StdEncoding.EncodeToString(png)
		user := models.User{Username: "admin", Password: "admin", Apikey: APIKey, QrCode: uEncQr}
		err = ndb.SetUserData("1", user)
		utils.LogError(err)
	}
}

func main() {
	//App Config
	app := fiber.New(&fiber.Settings{
		DisableStartupMessage: true,
	})

	//In Memory Static Assets
	app.Use("/assets", embed.New(embed.Config{
		Root: rice.MustFindBox("app/assets").HTTPBox(),
	}))

	//Checks if logs saving is activated
	saveLogs := viper.GetBool("save_logs")
	if saveLogs {
		logFile, err := os.OpenFile("nitr.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer logFile.Close()
		log.SetOutput(logFile)

		cfg := logger.Config{
			Output:     logFile,
			TimeFormat: "2006/01/02 15:04:05",
			Format:     "${time} - ${method} ${path} - ${ip}\n",
		}

		app.Use(logger.New(cfg))
	}

	app.Use(recover.New(recover.Config{
		Handler: func(c *fiber.Ctx, err error) {
			c.SendString(err.Error())
			c.SendStatus(500)
		},
	}))

	//API Config
	api := app.Group("/api")
	v1 := api.Group("/v1")
	v1.Use(apikey.New(apikey.Config{Key: ndb.GetApiKey()}))

	//nitr API Endpoints
	v1.Get("/", handlers.Overview)
	v1.Get("/cpu", handlers.CPU)
	v1.Get("/bios", handlers.Bios)
	v1.Get("/bandwidth", handlers.Bandwidth)
	v1.Get("/chassis", handlers.Chassis)
	v1.Get("/disks", handlers.Disk)
	v1.Get("/drives", handlers.Drive)
	v1.Get("/devices", handlers.Devices)
	v1.Get("/gpu", handlers.GPU)
	v1.Get("/host", handlers.Host)
	v1.Get("/isp", handlers.ISP)
	v1.Get("/network", handlers.Network)
	v1.Get("/processes", handlers.Process)
	v1.Get("/ram", handlers.RAM)
	v1.Get("/baseboard", handlers.Baseboard)
	v1.Get("/product", handlers.Product)
	v1.Get("/memory", handlers.Memory)

	//Login View
	app.Get("/", handlers.Login)

	//Login Submit
	app.Post("/", handlers.LoginSubmit)

	//Auth middleware
	app.Use(handlers.Auth)

	//Panel View
	app.Get("/panel", handlers.Panel)

	//Panel Logout
	app.Post("/logout", handlers.Logout)

	//Generate new API Key
	app.Post("/generate", handlers.GenerateApiKey)

	//Change Password View
	app.Get("/password", handlers.Password)

	//New Password Submit
	app.Post("/password", handlers.PasswordSubmit)

	app.Get("/status", websocket.New(handlers.SocketReader))

	//Checks if custom port was set, otherwise sets default port
	port := viper.GetString("port")
	if port == "" {
		port = "3000"
	}

	//Server startup
	sslEnabled := viper.GetBool("ssl_enabled")
	if sslEnabled {
		cert := viper.GetString("ssl_certificate")
		key := viper.GetString("ssl_certificate_key")

		cer, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			log.Println("Invalid ssl certificate")
			utils.LogError(err)
		}

		config := &tls.Config{Certificates: []tls.Certificate{cer}}
		utils.StartMessage("https", port)

		openBrowser := viper.GetBool("open_browser_on_startup")
		if openBrowser {
			utils.OpenBrowser("https://localhost", port)
		}

		log.Println("Starting server")

		err = app.Listen(port, config)
		if err != nil {
			fmt.Println(err, "\nCheck settings at config.ini file")
		}
		utils.LogError(err)

	} else {
		utils.StartMessage("http", port)
		openBrowser := viper.GetBool("open_browser_on_startup")
		if openBrowser {
			utils.OpenBrowser("http://localhost", port)
		}

		log.Println("Starting server")

		err := app.Listen(port)
		if err != nil {
			fmt.Println(err, "\nCheck settings at config.ini file")
		}
		utils.LogError(err)
	}

}
