package main

import (
	"log"
	"os"

	rice "github.com/GeertJohan/go.rice"
	"github.com/bitcav/nitr/cmd"
	db "github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/handlers"
	"github.com/bitcav/nitr/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"
	"github.com/kardianos/service"
)

// server performs all non-blocking startup (config, routes, middleware,
// log wiring) and returns the assembled app ready to listen. Setup errors
// are returned rather than killing the process: server used to run entirely
// on the goroutine spawned by Start, so a log.Fatalf from utils.Logs would
// os.Exit the program from outside main's error-reporting path. The blocking
// listen is left to Start so this function stays quick.
func server() (*fiber.App, error) {
	//Set Config.ini Default Values
	utils.ConfigFileSetup()

	//Set API Server default Data
	db.SetAPIData()

	//App Config
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	//In Memory Static Assets
	app.Use("/assets", filesystem.New(filesystem.Config{
		Root: rice.MustFindBox("app/assets").HTTPBox(),
	}))

	//Checks if logs saving is activated
	if err := utils.Logs(app); err != nil {
		return nil, err
	}

	app.Use(recover.New())

	//API Config
	api := app.Group("/api")
	v1 := api.Group("/v1")

	//API Key auth middleware
	v1.Use(handlers.AuthAPI)

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

	//Prometheus /metrics endpoint, behind the same x-api-key auth as /api/v1/*.
	app.Get("/metrics", handlers.AuthAPI, handlers.Metrics)

	//Liveness and readiness probes. Registered on `app` (not `v1`, so they
	//skip the x-api-key middleware) and BEFORE app.Use(handlers.Auth) below,
	//so they answer without credentials and never redirect to the login page.
	//Health touches nothing; Ready only stats nitr.db. Both stay fast and
	//side-effect-free so Docker/Compose/Kubernetes/uptime checkers can poll
	//them without auth or backpressure.
	app.Get("/health", handlers.Health)
	app.Get("/ready", handlers.Ready)

	//Login View
	handlers.ViewsBox = rice.MustFindBox("app/views")
	app.Get("/", handlers.Login)

	//Login Submit
	app.Post("/", handlers.LoginSubmit)

	//Auth middleware
	app.Use(handlers.Auth)

	//Panel View
	app.Get("/panel", handlers.Panel)

	//Panel JSON Data
	app.Get("/content", handlers.PanelContent)

	//Panel Logout
	app.Post("/logout", handlers.Logout)

	//Generate new API Key
	app.Post("/generate", handlers.GenerateApiKey)

	//Change Password View
	app.Get("/password", handlers.Password)

	//New Password Submit
	app.Post("/password", handlers.PasswordSubmit)

	app.Get("/status", websocket.New(handlers.SocketReader))

	return app, nil
}

type program struct {
	// app holds the server started by Start so Stop can shut it down. It is
	// the minimal seam for deterministic listener teardown in tests; a full
	// graceful-shutdown design (bbolt handle lifecycle, signal handling,
	// shutdown timeouts) belongs to a separate ticket.
	app *fiber.App
}

var logger service.Logger

// Start runs server setup synchronously and backgrounds only the blocking
// listener, matching the kardianos/service contract that Start return within
// a few seconds. Setup errors (e.g. nitr.log not writable) are returned
// here so main's error-reporting path handles them, instead of being raised
// by log.Fatalf on the background goroutine.
func (p *program) Start(s service.Service) error {
	app, err := server()
	if err != nil {
		return err
	}
	p.app = app
	go utils.StartServer(app)
	return nil
}

func (p *program) Stop(s service.Service) error {
	if p.app != nil {
		return p.app.Shutdown()
	}
	return nil
}

func main() {
	if dispatch(os.Args) {
		return
	}

	s, lg, err := initService()
	if err != nil {
		log.Fatal(err)
	}
	logger = lg

	if err := s.Run(); err != nil {
		logger.Error(err)
	}
}

// dispatch runs the CLI when the process is started with extra arguments
// (e.g. "nitr version"). It returns true when the CLI handled the invocation,
// in which case the caller must not start the host service.
func dispatch(args []string) bool {
	if len(args) > 1 {
		cmd.ExecuteArgs(args[1:])
		return true
	}
	return false
}

// initService builds the Nitr host service together with its logger. It is
// extracted from main() so the construction logic can be exercised by tests.
func initService() (service.Service, service.Logger, error) {
	svcConfig := &service.Config{
		Name:        "NitrService",
		Description: "A Remote Monitoring Tool for system information gathering, making it available through a JSON API.",
	}

	s, err := service.New(&program{}, svcConfig)
	if err != nil {
		return nil, nil, err
	}

	lg, err := s.Logger(nil)
	if err != nil {
		return nil, nil, err
	}

	return s, lg, nil
}
