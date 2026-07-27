package handlers

import (
	"encoding/json"
	"log"

	rice "github.com/GeertJohan/go.rice"
	"github.com/bitcav/nitr-core/host"
	db "github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/websocket/v2"
	"github.com/hoisie/mustache"
)

var sessions = session.New()

var ViewsBox *rice.Box

func Login(c *fiber.Ctx) error {
	store, err := sessions.Get(c)
	if err != nil {
		return err
	}
	if store.Get("UserID") == "1" {
		return c.Redirect("/panel")
	}
	loginView, err := ViewsBox.String("login.mustache")
	utils.LogError(err)

	layoutView, err := ViewsBox.String("layout/default.mustache")
	utils.LogError(err)

	c.Type("html")
	return c.SendString(mustache.RenderInLayout(loginView, layoutView))
}

func LoginSubmit(c *fiber.Ctx) error {
	login := new(models.Login)

	if err := c.BodyParser(login); err != nil {
		log.Fatal(err)
	}

	nitrUser, err := db.GetUserByID("1")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	if utils.PasswordHash(login.Password) == nitrUser.Password {
		store, err := sessions.Get(c)
		if err != nil {
			return err
		}
		defer func() {
			utils.LogError(store.Save())
		}()
		store.Set("UserID", "1")
		return c.Redirect("/panel")
	}
	return c.Redirect("/")
}

func Panel(c *fiber.Ctx) error {
	panelView, err := ViewsBox.String("panel.html")
	utils.LogError(err)

	layoutView, err := ViewsBox.String("layout/default.mustache")
	utils.LogError(err)

	c.Type("html")
	err = c.SendString(mustache.RenderInLayout(panelView, layoutView))
	utils.LogError(err)

	log.Println("Session started")
	return nil
}

func PanelContent(c *fiber.Ctx) error {
	apiKey, err := db.GetApiKey()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	hostInfo := models.HostInfo{
		Name:        host.Info().Name,
		Description: host.Info().Platform + "/" + host.Info().Arch,
		IP:          utils.GetLocalIP(),
		Port:        utils.GetLocalPort(),
		Key:         apiKey,
	}

	hostInfoJSON, err := json.Marshal(hostInfo)
	if err != nil {
		utils.LogError(err)
	}

	hostInfo.QrCode = string(hostInfoJSON)

	return c.JSON(hostInfo)
}

func GenerateApiKey(c *fiber.Ctx) error {
	newAPIKey := utils.RandString(10)

	hostInfo := models.HostInfo{
		Name:        host.Info().Name,
		Description: host.Info().Platform + "/" + host.Info().Arch,
		IP:          utils.GetLocalIP(),
		Port:        utils.GetLocalPort(),
		Key:         newAPIKey,
	}

	hostInfoJSON, err := json.Marshal(hostInfo)
	if err != nil {
		utils.LogError(err)
	}

	nitrUser, err := db.GetUserByID("1")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	user := models.User{Password: nitrUser.Password, Apikey: newAPIKey}
	err = db.SetUserData("1", user)
	utils.LogError(err)

	err = c.JSON(models.ApiKey{
		Key:    newAPIKey,
		QrCode: string(hostInfoJSON),
	})
	utils.LogError(err)

	log.Println("New Api key generated")
	return nil
}

func Password(c *fiber.Ctx) error {
	passwordView, err := ViewsBox.String("password.html")
	utils.LogError(err)

	layoutView, err := ViewsBox.String("layout/default.mustache")
	utils.LogError(err)

	c.Type("html")
	return c.SendString(mustache.RenderInLayout(passwordView, layoutView))
}

func PasswordSubmit(c *fiber.Ctx) error {
	password := new(models.Password)

	if err := c.BodyParser(password); err != nil {
		log.Fatal(err)
	}

	nitrUser, err := db.GetUserByID("1")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	if utils.PasswordHash(password.CurrentPassword) == nitrUser.Password {
		user := models.User{Password: utils.PasswordHash(password.NewPassword), Apikey: nitrUser.Apikey}
		err := db.SetUserData("1", user)
		utils.LogError(err)
		log.Println("Password changed")
		return c.SendStatus(fiber.StatusOK)
	}
	return c.SendStatus(fiber.StatusNotModified)
}

func SocketReader(c *websocket.Conn) {
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			log.Println(err)
			break
		}
		log.Printf("%s", msg)
	}

}

// Auth Middleware
func Auth(c *fiber.Ctx) error {
	store, err := sessions.Get(c)
	if err != nil {
		return err
	}
	if store.Get("UserID") == "1" {
		return c.Next()
	}
	return c.Redirect("/")
}

func Logout(c *fiber.Ctx) error {
	c.ClearCookie()
	log.Println("Session closed")
	return c.Redirect("/")
}
