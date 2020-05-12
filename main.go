package main

import (
	b64 "encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/fiberweb/apikey"
	"github.com/juanhuttemann/nitr-api/nitrdb"

	"github.com/gofiber/fiber"
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
	bolt "go.etcd.io/bbolt"

	_ "github.com/mattn/go-sqlite3"
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

var sessionID string

func init() {
	if _, err := os.Stat("nitr.db"); err != nil {
		fmt.Println("Creating database...")
		db, err := nitrdb.SetupDB()
		defer db.Close()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Adding user...")
		user := nitrdb.User{Username: "admin", Password: "admin", Apikey: ""}
		err = nitrdb.SetUserData(db, "1", user)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func main() {
	app := fiber.New(&fiber.Settings{
		DisableStartupMessage: true,
	})

	sessions := session.New()

	app.Settings.TemplateEngine = template.Mustache()

	app.Static("/", "assets")

	app.Get("/", func(c *fiber.Ctx) {
		store := sessions.Get(c)
		if store.Get("UserID") == "1" || c.Cookies("remember") == "1" {
			c.Redirect("/panel")
		} else {
			content, err := ioutil.ReadFile("./views/login.html")
			if err != nil {
				log.Fatal(err)
			}
			bind := fiber.Map{
				"content": string(content),
			}
			c.Render("views/layout/default.mustache", bind)
		}
	})

	app.Get("/panel", func(c *fiber.Ctx) {
		store := sessions.Get(c)
		if store.Get("UserID") == "1" || c.Cookies("remember") == "1" {
			content, err := ioutil.ReadFile("./views/panel.mustache")
			if err != nil {
				log.Fatal(err)
			}

			db, err := bolt.Open("nitr.db", 0600, nil)
			defer db.Close()

			if err != nil {
				fmt.Errorf("could not open db, %v", err)
			}

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
			c.Render("views/layout/default.mustache", bind)
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

		if err != nil {
			fmt.Errorf("could not open db, %v", err)
		}

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
		c.ClearCookie()
		c.Redirect("/")
	})

	app.Post("/code", func(c *fiber.Ctx) {
		apikey := key.String(12)
		png, err := qrcode.Encode(apikey, qrcode.Medium, 256)
		uEncQr := b64.StdEncoding.EncodeToString(png)

		db, err := bolt.Open("nitr.db", 0600, nil)
		defer db.Close()

		if err != nil {
			fmt.Errorf("could not open db, %v", err)
		}

		user := nitrdb.User{Username: "admin", Password: "admin", Apikey: apikey, QrCode: uEncQr}
		err = nitrdb.SetUserData(db, "1", user)
		if err != nil {
			log.Fatal(err)
		}

		c.JSON(Key{
			Key:    apikey,
			QrCode: uEncQr,
		})
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

	fmt.Println(`                 _  __       
         ____   (_)/ /_ _____
   ____ / __ \ / // __// ___/
 _____ / / / // // /_ / /    
   __ /_/ /_//_/ \__//_/ v0.1.0b     
Go to admin panel at http://localhost:3000/
`)
	app.Listen(3000)
}
