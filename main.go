package main

import (
	"crypto/tls"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	rice "github.com/GeertJohan/go.rice"
	ndb "github.com/bitcav/nitr-agent/database"
	"github.com/bitcav/nitr-agent/handlers"
	"github.com/bitcav/nitr-agent/models"
	"github.com/bitcav/nitr-agent/utils"
	"github.com/fiberweb/apikey"
	"github.com/hoisie/mustache"

	"github.com/gofiber/embed"
	"github.com/gofiber/fiber"
	"github.com/gofiber/logger"
	"github.com/gofiber/recover"
	"github.com/gofiber/session"
	"github.com/gofiber/websocket"

	"github.com/skip2/go-qrcode"

	"github.com/bitcav/nitr-agent/host"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
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
		db, err := ndb.SetupDB()
		defer db.Close()
		utils.LogError(err)

		log.Println("Adding default user")

		APIKey := utils.RandString(10)

		port := viper.GetString("port")
		if port == "" {
			port = "3000"
		}

		qr := models.QR{
			Name:        host.Check().Name,
			Description: host.Check().Platform,
			Port:        port,
			Key:         APIKey,
		}

		qrJSON, err := json.Marshal(qr)
		if err != nil {
			utils.LogError(err)
		}

		png, err := qrcode.Encode(string(qrJSON), qrcode.Medium, 256)
		uEncQr := b64.StdEncoding.EncodeToString(png)
		user := models.User{Username: "admin", Password: "admin", Apikey: APIKey, QrCode: uEncQr}
		err = ndb.SetUserData(db, "1", user)
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

	sessions := session.New()

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
	v1.Get("/system", handlers.System)

	//Login View
	app.Get("/", func(c *fiber.Ctx) {
		store := sessions.Get(c)
		if store.Get("UserID") == "1" || c.Cookies("remember") == "1" {
			c.Redirect("/panel")
		} else {
			content, err := rice.MustFindBox("app/views").HTTPBox().String("login.mustache")
			utils.LogError(err)

			layout, err := rice.MustFindBox("app/views/layout").HTTPBox().String("default.mustache")
			utils.LogError(err)

			c.Type("html")
			c.Send(mustache.RenderInLayout(content, layout))
		}
	})

	//Login Submit
	app.Post("/", func(c *fiber.Ctx) {
		login := new(models.Login)

		if err := c.BodyParser(login); err != nil {
			log.Fatal(err)
		}
		db, err := bolt.Open("nitr.db", 0600, nil)
		defer db.Close()

		utils.LogError(err)

		nitrUser := ndb.GetUserByID(db, "1")
		if (login.Username == nitrUser.Username) && (login.Password == nitrUser.Password) {
			store := sessions.Get(c)
			defer store.Save()
			store.Set("UserID", "1")
			if login.Remember == "on" {
				cookie := new(fiber.Cookie)
				cookie.Name = "remember"
				cookie.Value = "1"
				cookie.Expires = time.Now().Add(48 * time.Hour)
				c.Cookie(cookie)
			}
			c.Redirect("/panel")
		} else {
			c.Redirect("/")
		}
	})

	//Auth middleware
	app.Use(func(c *fiber.Ctx) {
		store := sessions.Get(c)
		if store.Get("UserID") == "1" || c.Cookies("remember") == "1" {
			c.Next()
		} else {
			c.Redirect("/")
		}
	})

	//Panel View
	app.Get("/panel", func(c *fiber.Ctx) {
		content, err := rice.MustFindBox("app/views").HTTPBox().String("panel.html")
		utils.LogError(err)
		layout, err := rice.MustFindBox("app/views/layout").HTTPBox().String("default.mustache")
		utils.LogError(err)

		db, err := bolt.Open("nitr.db", 0600, nil)
		defer db.Close()

		utils.LogError(err)

		nitrUser := ndb.GetUserByID(db, "1")

		bind := fiber.Map{
			"content":  string(content),
			"host":     host.Check().Name,
			"os":       host.Check().OS,
			"platform": host.Check().Platform,
			"arch":     host.Check().Arch,
			"apikey":   nitrUser.Apikey,
			"qrCode":   nitrUser.QrCode,
		}

		c.Type("html")
		c.Send(mustache.Render(layout, bind))
		log.Println("Session started")
	})

	//Panel Logout
	app.Post("/logout", func(c *fiber.Ctx) {
		c.ClearCookie()
		c.Redirect("/")
		log.Println("Session closed")
	})

	//Generate new API Key
	app.Post("/generate", func(c *fiber.Ctx) {
		newAPIKey := utils.RandString(10)

		port := viper.GetString("port")
		if port == "" {
			port = "3000"
		}

		qr := models.QR{
			Name:        host.Check().Name,
			Description: host.Check().Platform,
			Port:        port,
			Key:         newAPIKey,
		}

		qrJSON, err := json.Marshal(qr)
		if err != nil {
			utils.LogError(err)
		}
		png, err := qrcode.Encode(string(qrJSON), qrcode.Medium, 256)
		uEncQr := b64.StdEncoding.EncodeToString(png)

		db, err := bolt.Open("nitr.db", 0600, nil)
		defer db.Close()
		utils.LogError(err)

		nitrUser := ndb.GetUserByID(db, "1")
		user := models.User{Username: nitrUser.Username, Password: nitrUser.Password, Apikey: newAPIKey, QrCode: uEncQr}
		err = ndb.SetUserData(db, "1", user)
		utils.LogError(err)

		c.JSON(models.ApiKey{
			Key:    newAPIKey,
			QrCode: uEncQr,
		})

		log.Println("New Api key generated")
	})

	//Change Password View
	app.Get("/password", func(c *fiber.Ctx) {
		content, err := rice.MustFindBox("app/views").HTTPBox().String("password.html")
		utils.LogError(err)
		layout, err := rice.MustFindBox("app/views/layout").HTTPBox().String("default.mustache")
		utils.LogError(err)

		c.Type("html")
		c.Send(mustache.RenderInLayout(content, layout))
	})

	//New Password Submit
	app.Post("/password", func(c *fiber.Ctx) {
		password := new(models.Password)

		if err := c.BodyParser(password); err != nil {
			log.Fatal(err)
		}

		db, err := bolt.Open("nitr.db", 0600, nil)
		defer db.Close()

		utils.LogError(err)

		nitrUser := ndb.GetUserByID(db, "1")
		if password.CurrentPassword == nitrUser.Password {
			utils.LogError(err)
			user := models.User{Username: nitrUser.Username, Password: password.NewPassword, Apikey: nitrUser.Apikey, QrCode: nitrUser.QrCode}
			err = ndb.SetUserData(db, "1", user)
			utils.LogError(err)
			c.SendStatus(200)
			log.Println("Password changed")
		} else {
			c.SendStatus(304)
		}
	})

	app.Get("/status", websocket.New(func(c *websocket.Conn) {
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				log.Println(err)
				break
			}
			log.Printf("%s", msg)
		}

	}))

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
