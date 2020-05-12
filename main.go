package main

import (
	"database/sql"
	b64 "encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/gofiber/fiber"
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

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	Username string `form:"username" query:"username"`
	Password string `form:"password" query:"password"`
}

type Key struct {
	Key    string `json:"key"`
	QrCode string `json:"qrCode"`
}

func init() {
	db, err := nitrdb.setupDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	user := User{Username: "admin", Password: "admin"}
	err = nitrdb.addUser(db, "1", user)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	app := fiber.New(&fiber.Settings{
		DisableStartupMessage: true,
	})

	app.Settings.TemplateEngine = template.Mustache()

	app.Static("/", "assets")

	app.Get("/", func(c *fiber.Ctx) {
		if c.Cookies("admin") == "admin" {
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
		if c.Cookies("admin") == "admin" {
			content, err := ioutil.ReadFile("./views/panel.mustache")
			if err != nil {
				log.Fatal(err)
			}

			database, _ := sql.Open("sqlite3", "./nitr.db")
			rows, _ := database.Query("SELECT apikey, qrcode FROM users where username=?", "admin")
			var apikey string
			var qrcode string
			for rows.Next() {
				rows.Scan(&apikey, &qrcode)
			}

			bind := fiber.Map{
				"content":  string(content),
				"host":     host.Check().Name,
				"os":       host.Check().OS,
				"platform": host.Check().Platform,
				"arch":     host.Check().Arch,
				"apikey":   apikey,
				"qrcode":   qrcode,
			}
			c.Render("views/layout/default.mustache", bind)
		} else {
			c.Redirect("/")
		}
	})

	app.Post("/", func(c *fiber.Ctx) {
		u := new(User)

		if err := c.BodyParser(u); err != nil {
			log.Fatal(err)
		}

		database, _ := sql.Open("sqlite3", "./nitr.db")
		rows, _ := database.Query("SELECT id, username, password FROM users where username=?", u.Username)
		var id int
		var username string
		var password string
		for rows.Next() {
			rows.Scan(&id, &username, &password)
		}

		if (u.Username == username) && (u.Password == password) {
			cookie := new(fiber.Cookie)
			cookie.Name = "admin"
			cookie.Value = "admin"
			cookie.Expires = time.Now().Add(24 * time.Hour)
			c.Cookie(cookie)
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
		database, _ := sql.Open("sqlite3", "./nitr.db")
		statement, _ := database.Prepare("UPDATE users SET apikey=(?), qrcode=(?) where username=(?)")
		statement.Exec(apikey, uEncQr, "admin")

		if err != nil {
			fmt.Println(err)
		}
		c.JSON(Key{
			Key:    apikey,
			QrCode: uEncQr,
		})
	})

	api := app.Group("/api")

	/* 	database, _ := sql.Open("sqlite3", "./nitr.db")
	   	rows, _ := database.Query("SELECT apikey FROM users where username=?", "admin")
	   	var apiKey string
	   	for rows.Next() {
	   		rows.Scan(&apiKey)
	   	}

	   	api.Use(apikey.New(apikey.Config{Key: apiKey})) */

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
