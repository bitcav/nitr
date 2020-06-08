// go: generate goversioninfo -icon=images/app.ico
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
	"github.com/fiberweb/apikey"
	"github.com/hoisie/mustache"
	"github.com/juanhuttemann/nitr-agent/bandwidth"
	"github.com/juanhuttemann/nitr-agent/baseboard"
	"github.com/juanhuttemann/nitr-agent/devices"
	"github.com/juanhuttemann/nitr-agent/internet"
	"github.com/juanhuttemann/nitr-agent/network"
	"github.com/juanhuttemann/nitr-agent/nitrdb"
	"github.com/juanhuttemann/nitr-agent/overview"
	"github.com/juanhuttemann/nitr-agent/product"
	"github.com/juanhuttemann/nitr-agent/system"
	"github.com/juanhuttemann/nitr-agent/utils"

	"github.com/gofiber/embed"
	"github.com/gofiber/fiber"
	"github.com/gofiber/logger"
	"github.com/gofiber/recover"
	"github.com/gofiber/session"

	"github.com/gofiber/template"
	"github.com/skip2/go-qrcode"

	"github.com/juanhuttemann/nitr-agent/bios"
	"github.com/juanhuttemann/nitr-agent/chassis"
	"github.com/juanhuttemann/nitr-agent/cpu"
	"github.com/juanhuttemann/nitr-agent/disk"
	"github.com/juanhuttemann/nitr-agent/drive"
	"github.com/juanhuttemann/nitr-agent/gpu"
	"github.com/juanhuttemann/nitr-agent/host"
	"github.com/juanhuttemann/nitr-agent/process"
	"github.com/juanhuttemann/nitr-agent/ram"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
)

type loginForm struct {
	Username string `form:"username"`
	Password string `form:"password"`
	Remember string `form:"remember"`
}

type passwordForm struct {
	CurrentPassword    string `form:"currentPassword"`
	NewPassword        string `form:"newPassword"`
	RepeateNewPassword string `form:"repeatNewPassword"`
}

type apiKeyForm struct {
	Key    string `json:"key"`
	QrCode string `json:"qrCode"`
}

type qr struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Port        string `json:"port"`
	Key         string `json:"key"`
}

func init() {
	//Config file initial setup
	if _, err := os.Stat("config.ini"); err != nil {
		configFile, err := os.OpenFile("config.ini", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		logError(err)
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
		logError(err)
	}

	runPath, err := os.Getwd()
	logError(err)

	viper.SetConfigName("config.ini")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(runPath)
	err = viper.ReadInConfig()
	if err != nil {
		logError(err)
	}

	//DB Setup
	if _, err := os.Stat("nitr.db"); err != nil {
		log.Println("Database created")
		db, err := nitrdb.SetupDB()
		defer db.Close()
		logError(err)

		log.Println("Adding default user")

		APIKey := utils.RandString(10)

		port := viper.GetString("port")
		if port == "" {
			port = "3000"
		}

		qr := qr{
			Name:        host.Check().Name,
			Description: host.Check().Platform,
			Port:        port,
			Key:         APIKey,
		}

		qrJSON, err := json.Marshal(qr)
		if err != nil {
			logError(err)
		}

		png, err := qrcode.Encode(string(qrJSON), qrcode.Medium, 256)
		uEncQr := b64.StdEncoding.EncodeToString(png)
		user := nitrdb.User{Username: "admin", Password: "admin", Apikey: APIKey, QrCode: uEncQr}
		err = nitrdb.SetUserData(db, "1", user)
		logError(err)
	}
}

func logError(e error) {
	if e != nil {
		log.Println(e)
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

	app.Settings.TemplateEngine = template.Mustache()

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
	v1.Use(apikey.New(apikey.Config{Key: nitrdb.GetApiKey()}))

	//nitr API Endpoints
	v1.Get("/", overview.Data)
	v1.Get("/cpu", cpu.Data)
	v1.Get("/bios", bios.Data)
	v1.Get("/bandwidth", bandwidth.Data)
	v1.Get("/chassis", chassis.Data)
	v1.Get("/disks", disk.Data)
	v1.Get("/drives", drive.Data)
	v1.Get("/devices", devices.Data)
	v1.Get("/gpu", gpu.Data)
	v1.Get("/host", host.Data)
	v1.Get("/isp", internet.Data)
	v1.Get("/network", network.Data)
	v1.Get("/processes", process.Data)
	v1.Get("/ram", ram.Data)
	v1.Get("/baseboard", baseboard.Data)
	v1.Get("/product", product.Data)
	v1.Get("/system", system.Data)

	//Login View
	app.Get("/", func(c *fiber.Ctx) {
		store := sessions.Get(c)
		if store.Get("UserID") == "1" || c.Cookies("remember") == "1" {
			c.Redirect("/panel")
		} else {
			content, err := rice.MustFindBox("app/views").HTTPBox().String("login.mustache")
			logError(err)

			layout, err := rice.MustFindBox("app/views/layout").HTTPBox().String("default.mustache")
			logError(err)

			c.Type("html")
			c.Send(mustache.RenderInLayout(content, layout))
		}
	})

	//Login Submit
	app.Post("/", func(c *fiber.Ctx) {
		login := new(loginForm)

		if err := c.BodyParser(login); err != nil {
			log.Fatal(err)
		}
		db, err := bolt.Open("nitr.db", 0600, nil)
		defer db.Close()

		logError(err)

		nitrUser := nitrdb.GetUserByID(db, "1")
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
		logError(err)
		layout, err := rice.MustFindBox("app/views/layout").HTTPBox().String("default.mustache")
		logError(err)

		db, err := bolt.Open("nitr.db", 0600, nil)
		defer db.Close()

		logError(err)

		nitrUser := nitrdb.GetUserByID(db, "1")

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

		qr := qr{
			Name:        host.Check().Name,
			Description: host.Check().Platform,
			Port:        port,
			Key:         newAPIKey,
		}

		qrJSON, err := json.Marshal(qr)
		if err != nil {
			logError(err)
		}
		png, err := qrcode.Encode(string(qrJSON), qrcode.Medium, 256)
		uEncQr := b64.StdEncoding.EncodeToString(png)

		db, err := bolt.Open("nitr.db", 0600, nil)
		defer db.Close()
		logError(err)

		nitrUser := nitrdb.GetUserByID(db, "1")
		user := nitrdb.User{Username: nitrUser.Username, Password: nitrUser.Password, Apikey: newAPIKey, QrCode: uEncQr}
		err = nitrdb.SetUserData(db, "1", user)
		logError(err)

		c.JSON(apiKeyForm{
			Key:    newAPIKey,
			QrCode: uEncQr,
		})

		log.Println("New Api key generated")
	})

	//Change Password View
	app.Get("/password", func(c *fiber.Ctx) {
		content, err := rice.MustFindBox("app/views").HTTPBox().String("password.html")
		logError(err)
		layout, err := rice.MustFindBox("app/views/layout").HTTPBox().String("default.mustache")
		logError(err)

		c.Type("html")
		c.Send(mustache.RenderInLayout(content, layout))
	})

	//New Password Submit
	app.Post("/password", func(c *fiber.Ctx) {
		password := new(passwordForm)

		if err := c.BodyParser(password); err != nil {
			log.Fatal(err)
		}

		db, err := bolt.Open("nitr.db", 0600, nil)
		defer db.Close()

		logError(err)

		nitrUser := nitrdb.GetUserByID(db, "1")
		if password.CurrentPassword == nitrUser.Password {
			logError(err)
			user := nitrdb.User{Username: nitrUser.Username, Password: password.NewPassword, Apikey: nitrUser.Apikey, QrCode: nitrUser.QrCode}
			err = nitrdb.SetUserData(db, "1", user)
			logError(err)
			c.SendStatus(200)
			log.Println("Password changed")
		} else {
			c.SendStatus(304)
		}
	})

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
			logError(err)
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
		logError(err)

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
		logError(err)
	}

}
