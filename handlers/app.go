package handlers

import (
	"encoding/json"
	"io/fs"
	"log"
	"sync"
	"time"

	"github.com/bitcav/nitr-core/disk"
	"github.com/bitcav/nitr-core/host"
	"github.com/bitcav/nitr-core/overview"
	db "github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/websocket/v2"
	"github.com/hoisie/mustache"
)

var sessions = session.New()

// ViewsFS holds the panel's view templates, rooted at the views directory
// (assigned from main's go:embed FS; tests substitute os.DirFS).
var ViewsFS fs.FS

// view reads a template from ViewsFS.
func view(name string) (string, error) {
	b, err := fs.ReadFile(ViewsFS, name)
	return string(b), err
}

func Login(c *fiber.Ctx) error {
	store, err := sessions.Get(c)
	if err != nil {
		return err
	}
	if store.Get("UserID") == "1" {
		return c.Redirect("/panel")
	}
	loginView, err := view("login.mustache")
	if err != nil {
		utils.LogError(err)
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	layoutView, err := view("layout/default.mustache")
	if err != nil {
		utils.LogError(err)
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Type("html")
	return c.SendString(mustache.RenderInLayout(loginView, layoutView))
}

func LoginSubmit(c *fiber.Ctx) error {
	login := new(models.Login)

	if err := c.BodyParser(login); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
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
	panelView, err := view("panel.html")
	if err != nil {
		utils.LogError(err)
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	layoutView, err := view("layout/default.mustache")
	if err != nil {
		utils.LogError(err)
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

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
	localIP, err := utils.GetLocalIP()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	hostInfo := models.HostInfo{
		Name:        host.Info().Name,
		Description: host.Info().Platform + "/" + host.Info().Arch,
		IP:          localIP,
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

	localIP, err := utils.GetLocalIP()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	hostInfo := models.HostInfo{
		Name:        host.Info().Name,
		Description: host.Info().Platform + "/" + host.Info().Arch,
		IP:          localIP,
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
	passwordView, err := view("password.html")
	if err != nil {
		utils.LogError(err)
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	layoutView, err := view("layout/default.mustache")
	if err != nil {
		utils.LogError(err)
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Type("html")
	return c.SendString(mustache.RenderInLayout(passwordView, layoutView))
}

func PasswordSubmit(c *fiber.Ctx) error {
	password := new(models.Password)

	if err := c.BodyParser(password); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
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

// handleSocketMessageFunc processes a single inbound websocket message. It is
// a package-level seam so tests can substitute a handler that panics, which is
// the only way to exercise SocketReader's recover path from outside the loop.
// Guarded by handleSocketMessageMu: test cleanup restores the original while
// handler goroutines from earlier connections may still be reading it.
var (
	handleSocketMessageMu   sync.RWMutex
	handleSocketMessageFunc = func(msg []byte) { log.Printf("%s", msg) }
)

// liveMetrics is the JSON shape pushed to the panel on each tick. It embeds
// overview.Overview (host + CPU usage + RAM, already assembled by nitr-core)
// and adds per-disk usage so the panel can render CPU/RAM/disk widgets.
type liveMetrics struct {
	overview.Overview
	Disks []disk.Disk `json:"disks"`
}

func SocketReader(c *websocket.Conn) {
	// recover.New's deferred recover runs on the request goroutine, but
	// fasthttp/websocket runs this handler on a separate hijacked-conn
	// goroutine it spawns itself. An unrecovered panic here would crash the
	// whole process, so recover explicitly: log, then let the function
	// return so the websocket library's own deferred releaseConn closes
	// the connection.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("SocketReader recovered from panic: %v", r)
		}
	}()

	// done is closed on every exit path — clean disconnect, read error, or a
	// recovered panic — because this deferred close and the recover above both
	// run during unwind. Closing it is the single signal that stops the metrics
	// writer goroutine; without it the writer would tick forever, one leaked
	// goroutine per closed socket.
	done := make(chan struct{})

	// Signalling is not enough: SocketReader must not return until the writer
	// goroutine has actually exited. gofiber's websocket middleware returns
	// this *websocket.Conn to a sync.Pool (and zeroes it) the moment this
	// handler returns; a writer still mid-WriteMessage past that point is a
	// use-after-free that can write metrics into the NEXT client's reused
	// conn. Defers run LIFO, so the Wait is registered BEFORE close(done):
	// on unwind, done closes first (writer exits), then Wait blocks until it
	// has — join, don't just signal, and never before the signal or the two
	// goroutines deadlock.
	var writerWG sync.WaitGroup
	writerWG.Add(1)
	defer writerWG.Wait()
	defer close(done)

	// Metrics writer: the only goroutine that writes to this conn (the read
	// loop below only reads), so there is no concurrent-write hazard. It stops
	// when the client disconnects (done closed) or when its own write fails.
	// Its own recover mirrors the one above: the writer runs on a separate
	// goroutine that SocketReader's recover does not cover, so a panic in
	// overview/disk collection here would otherwise crash the process.
	go func() {
		defer writerWG.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("SocketReader writer recovered from panic: %v", r)
			}
		}()
		ticker := time.NewTicker(utils.MetricsPushInterval())
		defer ticker.Stop()
		writeMetrics(c) // first reading immediately, no full-interval wait
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !writeMetrics(c) {
					return
				}
			}
		}
	}()

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			log.Println(err)
			break
		}
		handleSocketMessageMu.RLock()
		h := handleSocketMessageFunc
		handleSocketMessageMu.RUnlock()
		h(msg)
	}
}

// writeMetrics marshals and pushes one live-metrics frame. It returns false
// when the write failed (broken/closed conn), signalling the writer to stop.
func writeMetrics(c *websocket.Conn) bool {
	payload := liveMetrics{
		Overview: overview.Info(),
		Disks:    disk.Info(),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		log.Println(err)
		return true // marshal of a fixed struct shape won't fail; keep ticking
	}
	if err := c.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println(err)
		return false
	}
	return true
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
