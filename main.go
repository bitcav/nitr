package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bitcav/nitr/cmd"
	db "github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/handlers"
	"github.com/bitcav/nitr/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/websocket/v2"
	"github.com/kardianos/service"
)

//go:embed app/assets
var assetsFS embed.FS

//go:embed app/views
var viewsFS embed.FS

// subFS strips the embed-retained directory prefix (e.g. "app/assets") so the
// FS is rooted where go.rice's box used to root it. The sub directory exists
// by construction (it is embedded at build time), so err is always nil.
func subFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err) // unreachable: dir is embedded at build time
	}
	return sub
}

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

	//recover stays outermost so panics from any middleware or handler below
	//are still caught.
	app.Use(recover.New())

	//requestid early so every response carries X-Request-Id and the access
	//log (wired by utils.Logs below) can record it.
	app.Use(requestid.New())

	//helmet's baseline security headers apply to every response, static
	//assets included. CORS defaults to deny: the middleware is only
	//registered when config.ini lists origins (cors_origins), because fiber
	//substitutes "*" for an empty AllowOrigins — wiring it with no origins
	//configured would open cross-origin access to host telemetry instead of
	//denying it.
	app.Use(helmet.New())
	if origins := utils.CORSOrigins(); origins != "" {
		app.Use(cors.New(cors.Config{
			AllowOrigins: origins,
		}))
	}

	//compress and etag wrap all bodies below: /processes and /devices are
	//large JSON payloads, and the slow-moving endpoints (bios, baseboard,
	//product, chassis) let polling clients skip unchanged bodies via 304.
	app.Use(compress.New())
	app.Use(etag.New())

	//In Memory Static Assets
	app.Use("/assets", filesystem.New(filesystem.Config{
		Root: http.FS(subFS(assetsFS, "app/assets")),
	}))

	//Checks if logs saving is activated
	if err := utils.Logs(app); err != nil {
		return nil, err
	}

	//API Config
	api := app.Group("/api")
	v1 := api.Group("/v1")

	//Rate limit the API per client IP (rate_limit_api_max in config.ini,
	//default 300/min). Registered before AuthAPI so unauthenticated
	//API-key guessing is throttled too.
	v1.Use(limiter.New(limiter.Config{
		Max:        utils.RateLimitMax("rate_limit_api_max", 300),
		Expiration: time.Minute,
	}))

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

	//Prometheus /metrics endpoint, behind the same x-api-key auth as /api/v1/*
	//and the same rate limit, since it is equally brute-forceable.
	app.Get("/metrics", limiter.New(limiter.Config{
		Max:        utils.RateLimitMax("rate_limit_api_max", 300),
		Expiration: time.Minute,
	}), handlers.AuthAPI, handlers.Metrics)

	//Liveness and readiness probes. Registered on `app` (not `v1`, so they
	//skip the x-api-key middleware) and BEFORE app.Use(handlers.Auth) below,
	//so they answer without credentials and never redirect to the login page.
	//Health touches nothing; Ready only stats nitr.db. Both stay fast and
	//side-effect-free so Docker/Compose/Kubernetes/uptime checkers can poll
	//them without auth or backpressure.
	app.Get("/health", handlers.Health)
	app.Get("/ready", handlers.Ready)

	//Login View
	handlers.ViewsFS = subFS(viewsFS, "app/views")
	app.Get("/", handlers.Login)

	//Login Submit, rate-limited per client IP (rate_limit_login_max in
	//config.ini, default 20/min) against password brute-forcing. Scoped to
	//this route so panel polling is unaffected.
	app.Post("/", limiter.New(limiter.Config{
		Max:        utils.RateLimitMax("rate_limit_login_max", 20),
		Expiration: time.Minute,
	}), handlers.LoginSubmit)

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
	// Bare `nitr` falls through dispatch and runs the service below;
	// arg- or flag-carrying invocations (`nitr server`, `nitr --port 9000`)
	// are routed by dispatch into cobra, which reaches this same path via
	// cmd.RunServer (the server subcommand and the root RunE both call it).
	cmd.RunServer = runService

	if dispatch(os.Args) {
		return
	}

	if err := runService(); err != nil {
		log.Fatal(err)
	}
}

// runService builds and runs the Nitr host service. It is the single server
// entry point, reached directly for bare `nitr` and through cmd.RunServer
// when cobra handles an invocation that carries flags. Init failures are
// returned (main reports them via log.Fatal); a run failure is logged
// through the service logger, matching the previous main() behaviour.
func runService() error {
	s, lg, err := initService()
	if err != nil {
		return err
	}
	logger = lg

	if err := s.Run(); err != nil {
		logger.Error(err)
	}
	return nil
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
// The Config is shared with the cmd lifecycle subcommands via cmd.ServiceConfig
// so the running service and the installer always agree on Name/Description.
func initService() (service.Service, service.Logger, error) {
	s, err := service.New(&program{}, cmd.ServiceConfig())
	if err != nil {
		return nil, nil, err
	}

	lg, err := s.Logger(nil)
	if err != nil {
		return nil, nil, err
	}

	return s, lg, nil
}
