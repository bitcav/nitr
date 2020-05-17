package main

import (
	b64 "encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"

	rice "github.com/GeertJohan/go.rice"
	"github.com/fiberweb/apikey"
	"github.com/hoisie/mustache"
	"github.com/juanhuttemann/nitr-api/baseboard"
	"github.com/juanhuttemann/nitr-api/nitrdb"
	"github.com/juanhuttemann/nitr-api/overview"
	"github.com/juanhuttemann/nitr-api/product"
	"github.com/juanhuttemann/nitr-api/system"

	"github.com/gofiber/embed"
	"github.com/gofiber/fiber"
	"github.com/gofiber/logger"
	"github.com/gofiber/recover"
	"github.com/gofiber/session"
	"github.com/gofiber/websocket"

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
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
)

type LoginForm struct {
	Username string `form:"username"`
	Password string `form:"password"`
	Remember string `form:"remember"`
}

type PasswordForm struct {
	CurrentPassword    string `form:"currentPassword"`
	NewPassword        string `form:"newPassword"`
	RepeateNewPassword string `form:"repeatNewPassword"`
}

type Key struct {
	Key    string `json:"key"`
	QrCode string `json:"qrCode"`
}

func init() {
	if _, err := os.Stat("config.ini"); err != nil {
		configFile, err := os.OpenFile("config.ini", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		logError(err)
		defer configFile.Close()

		_, err = configFile.WriteString(`port: 8000
openBrowserOnStartUp: true`)

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

func openbrowser(domain, port string) {
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
		log.Fatal(err)
	}
}

func main() {
	//App Config
	app := fiber.New(&fiber.Settings{
		DisableStartupMessage: true,
	})

	app.Use("/assets", embed.New(embed.Config{
		Root: rice.MustFindBox("app/assets").HTTPBox(),
	}))

	saveLogs := viper.GetBool("saveLogs")
	if saveLogs {
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
	}

	app.Settings.TemplateEngine = template.Mustache()
	sessions := session.New()

	app.Use(recover.New(recover.Config{
		Handler: func(c *fiber.Ctx, err error) {
			c.SendString(err.Error())
			c.SendStatus(500)
		},
	}))

	app.Get("/shutdown", func(c *fiber.Ctx) {
		app.Shutdown()
	})

	//API Config
	api := app.Group("/api")
	v1 := api.Group("/v1")
	v1.Use(apikey.New(apikey.Config{Key: nitrdb.GetApiKey()}))

	//nitr API Endpoints
	v1.Get("/", overview.Data)
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
		newAPIKey := key.String(12)
		png, err := qrcode.Encode(newAPIKey, qrcode.Medium, 256)
		uEncQr := b64.StdEncoding.EncodeToString(png)

		db, err := bolt.Open("nitr.db", 0600, nil)
		defer db.Close()
		logError(err)

		nitrUser := nitrdb.GetUserByID(db, "1")
		user := nitrdb.User{Username: nitrUser.Username, Password: nitrUser.Password, Apikey: newAPIKey, QrCode: uEncQr}
		err = nitrdb.SetUserData(db, "1", user)
		logError(err)

		c.JSON(Key{
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
		password := new(PasswordForm)

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

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", msg)
			err = c.WriteMessage(mt, msg)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	}))

	port := viper.GetString("port")
	if port == "" {
		port = "3000"
	}

	openBrowser := viper.GetBool("openBrowserOnStartUp")
	if openBrowser {
		openbrowser("http://localhost", port)
	}

	fmt.Printf(`                 _  __       
         ____   (_)/ /_ _____
   ____ / __ \ / // __// ___/
 _____ / / / // // /_ / /    
   __ /_/ /_//_/ \__//_/ v0.1.0b     

Go to admin panel at http://localhost:%v

`, port)

	err := app.Listen(port)
	log.Println("Starting server")

	if err != nil {
		fmt.Println(err, "\nCheck the port settings at config.ini file")
	}
	logError(err)
}
