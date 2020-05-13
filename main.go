package main

import (
	b64 "encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	rice "github.com/GeertJohan/go.rice"
	"github.com/fiberweb/apikey"
	"github.com/hoisie/mustache"
	"github.com/juanhuttemann/nitr-api/nitrdb"

	"github.com/gofiber/fiber"
	"github.com/gofiber/logger"
	"github.com/gofiber/session"

	"github.com/gofiber/template"
	"github.com/skip2/go-qrcode"

	"github.com/juanhuttemann/nitr-api/bios"
	"github.com/juanhuttemann/nitr-api/chassis"
	"github.com/juanhuttemann/nitr-api/cpu"
	"github.com/juanhuttemann/nitr-api/disk"
	"github.com/juanhuttemann/nitr-api/drive"
	"github.com/juanhuttemann/nitr-api/gpu"
	"github.com/juanhuttemann/nitr-api/host"
	"github.com/juanhuttemann/nitr-api/key"
	"github.com/juanhuttemann/nitr-api/network"
	"github.com/juanhuttemann/nitr-api/process"
	"github.com/juanhuttemann/nitr-api/ram"
	"github.com/juanhuttemann/nitr-api/system"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
)

type LoginForm struct {
	Username string `form:"username" query:"username"`
	Password string `form:"password" query:"password"`
	Remember string `form:"remember" query:"remember"`
}

type Key struct {
	Key    string `json:"key"`
	QrCode string `json:"qrCode"`
}

func init() {
	logFile, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer logFile.Close()

	log.SetOutput(logFile)

	if _, err := os.Stat("config.ini"); err != nil {
		configFile, err := os.OpenFile("config.ini", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		logError(err)
		defer configFile.Close()

		_, err = configFile.WriteString(`# agent port
port: 3000`)

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

	if _, err := os.Stat("nitr.db"); err != nil {
		log.Println("Database created")
		db, err := nitrdb.SetupDB()
		defer db.Close()
		logError(err)

		log.Println("Adding default user")
		user := nitrdb.User{Username: "admin", Password: "admin", Apikey: ""}
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
	app := fiber.New(&fiber.Settings{
		DisableStartupMessage: true,
	})

	logFile, err := os.OpenFile("log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
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

	sessions := session.New()

	app.Settings.TemplateEngine = template.Mustache()

	if err != nil {
		fmt.Println(err)
	}
	app.Static("/", "assets")

	app.Get("/favicon", func(c *fiber.Ctx) {
		favicon, err := rice.MustFindBox("app/assets/images/").HTTPBox().String("favicon.png")
		logError(err)
		c.Send(favicon)
	})

	app.Get("/logo", func(c *fiber.Ctx) {
		logo, err := rice.MustFindBox("app/assets/images/").HTTPBox().String("logo.png")
		logError(err)
		c.Send(logo)
	})

	app.Get("/", func(c *fiber.Ctx) {
		store := sessions.Get(c)
		if store.Get("UserID") == "1" || c.Cookies("remember") == "1" {
			c.Redirect("/panel")
		} else {
			content, err := rice.MustFindBox("app/views").HTTPBox().String("login.html")
			logError(err)

			layout, err := rice.MustFindBox("app/views/layout").HTTPBox().String("default.mustache")
			logError(err)

			bind := fiber.Map{
				"content": string(content),
			}
			c.Type("html")
			c.Send(mustache.Render(layout, bind))
		}
	})

	app.Get("/panel", func(c *fiber.Ctx) {
		store := sessions.Get(c)
		if store.Get("UserID") == "1" || c.Cookies("remember") == "1" {
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
		} else {
			c.Redirect("/")
		}
	})
	app.Post("/", func(c *fiber.Ctx) {
		login := new(LoginForm)

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

	app.Post("/logout", func(c *fiber.Ctx) {
		store := sessions.Get(c)
		if store.Get("UserID") == "1" || c.Cookies("remember") == "1" {
			c.ClearCookie()
			c.Redirect("/")
		}
	})

	app.Post("/code", func(c *fiber.Ctx) {
		store := sessions.Get(c)
		if store.Get("UserID") == "1" || c.Cookies("remember") == "1" {
			apikey := key.String(12)
			png, err := qrcode.Encode(apikey, qrcode.Medium, 256)
			uEncQr := b64.StdEncoding.EncodeToString(png)

			db, err := bolt.Open("nitr.db", 0600, nil)
			defer db.Close()

			logError(err)

			user := nitrdb.User{Username: "admin", Password: "admin", Apikey: apikey, QrCode: uEncQr}
			err = nitrdb.SetUserData(db, "1", user)
			logError(err)

			c.JSON(Key{
				Key:    apikey,
				QrCode: uEncQr,
			})
		}
	})

	api := app.Group("/api")
	api.Use(apikey.New(apikey.Config{Key: nitrdb.GetApiKey()}))

	v1 := api.Group("/v1")

	v1.Get("/", system.Data)
	v1.Get("/cpu", cpu.Data)
	v1.Get("/bios", bios.Data)
	v1.Get("/chassis", chassis.Data)
	v1.Get("/disks", disk.Data)
	v1.Get("/drives", drive.Data)
	v1.Get("/gpu", gpu.Data)
	v1.Get("/host", host.Data)
	v1.Get("/network", network.Data)
	v1.Get("/processes", process.Data)
	v1.Get("/ram", ram.Data)

	port := viper.Get("port")
	if port == nil {
		port = 3000
	}

	fmt.Printf(`                 _  __       
         ____   (_)/ /_ _____
   ____ / __ \ / // __// ___/
 _____ / / / // // /_ / /    
   __ /_/ /_//_/ \__//_/ v0.1.0b     

Go to admin panel at http://localhost:%v

`, port)
	err = app.Listen(port)
	if err != nil {
		fmt.Println(err, "\nCheck the port settings at config.ini file")
	}
	logError(err)
}
