package handlers

import (
	"net"
	"testing"
	"time"

	db "github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ws "github.com/fasthttp/websocket"
	websocket "github.com/gofiber/websocket/v2"
)

func TestLoginRendersWhenUnauthenticated(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/", Login)

	resp := get(t, app, "/")
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, body(t, resp), "password")
}

func TestLoginRedirectsOnRememberCookie(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/", Login)

	resp := getWithCookie(t, app, "/", "remember=1")
	assert.Equal(t, 302, resp.StatusCode)
	loc := resp.Header.Get("Location")
	assert.Equal(t, "/panel", loc)
}

func TestLoginRedirectsOnSession(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Post("/", LoginSubmit)
	app.Get("/", Login)

	// authenticate first
	resp := post(t, app, "/", "password=123456")
	assert.Equal(t, 302, resp.StatusCode)
	cookie := sessionCookie(resp)
	require.NotEmpty(t, cookie, "expected a session cookie to be set")

	// now Login should see UserID==1 and redirect to /panel
	resp = getWithCookie(t, app, "/", cookie)
	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/panel", resp.Header.Get("Location"))
}

func TestLoginSubmitCorrectPassword(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Post("/", LoginSubmit)

	resp := post(t, app, "/", "password=123456")
	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/panel", resp.Header.Get("Location"))
	assert.NotEmpty(t, sessionCookie(resp))
}

func TestLoginSubmitWrongPassword(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Post("/", LoginSubmit)

	resp := post(t, app, "/", "password=nope")
	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
	assert.Empty(t, sessionCookie(resp))
}

func TestPanelRender(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/panel", Panel)

	resp := get(t, app, "/panel")
	assert.Equal(t, 200, resp.StatusCode)
	assert.NotEmpty(t, body(t, resp))
}

func TestPanelContent(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/content", PanelContent)

	resp := get(t, app, "/content")
	assert.Equal(t, 200, resp.StatusCode)
	bd := body(t, resp)
	assert.Contains(t, bd, "testapikey")
	assert.Contains(t, bd, "qrCode")
}

func TestGenerateApiKey(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Post("/generate", GenerateApiKey)

	resp := post(t, app, "/generate", "")
	assert.Equal(t, 200, resp.StatusCode)
	bd := body(t, resp)
	assert.Contains(t, bd, "key")

	// the new key must have been persisted
	newKey, err := db.GetApiKey()
	require.NoError(t, err)
	assert.Len(t, newKey, 10)
	assert.Contains(t, bd, newKey)
}

func TestPasswordView(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/password", Password)

	resp := get(t, app, "/password")
	assert.Equal(t, 200, resp.StatusCode)
	assert.NotEmpty(t, body(t, resp))
}

func TestPasswordSubmitCorrect(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Post("/password", PasswordSubmit)

	resp := post(t, app, "/password", "currentPassword=123456&newPassword=newpass&repeatNewPassword=newpass")
	assert.Equal(t, 200, resp.StatusCode)

	// password actually changed in db
	u, err := db.GetUserByID("1")
	require.NoError(t, err)
	assert.Equal(t, utils.PasswordHash("newpass"), u.Password)
}

func TestPasswordSubmitWrongCurrent(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Post("/password", PasswordSubmit)

	resp := post(t, app, "/password", "currentPassword=bad&newPassword=newpass&repeatNewPassword=newpass")
	assert.Equal(t, 304, resp.StatusCode)
}

func TestAuthMiddlewareAllowsSession(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Post("/", LoginSubmit)
	app.Get("/panel", Auth, func(c *fiber.Ctx) error { return c.SendString("granted") })

	resp := post(t, app, "/", "password=123456")
	cookie := sessionCookie(resp)
	require.NotEmpty(t, cookie)

	resp = getWithCookie(t, app, "/panel", cookie)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "granted", body(t, resp))
}

func TestAuthMiddlewareAllowsRememberCookie(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/panel", Auth, func(c *fiber.Ctx) error { return c.SendString("granted") })

	resp := getWithCookie(t, app, "/panel", "remember=1")
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "granted", body(t, resp))
}

func TestAuthMiddlewareRedirectsWhenUnauthenticated(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Get("/panel", Auth, func(c *fiber.Ctx) error { return c.SendString("granted") })

	resp := get(t, app, "/panel")
	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
}

func TestLogout(t *testing.T) {
	setupEnv(t)
	app := newTestApp()
	app.Post("/logout", Logout)

	resp := post(t, app, "/logout", "")
	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
}

func TestRecoverHandler(t *testing.T) {
	setupEnv(t)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(recover.New())
	app.Get("/boom", func(c *fiber.Ctx) error { panic("kaboom") })

	resp := get(t, app, "/boom")
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, "kaboom", body(t, resp))
}

func TestSocketReader(t *testing.T) {
	setupEnv(t)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/ws", websocket.New(SocketReader))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	dialer := ws.Dialer{HandshakeTimeout: 2 * time.Second}
	var c *ws.Conn
	// retry until the server accepts connections
	for i := 0; i < 50; i++ {
		conn, _, derr := dialer.Dial("ws://"+addr+"/ws", nil)
		if derr == nil {
			c = conn
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NotNil(t, c, "could not dial websocket server")
	require.NoError(t, c.WriteMessage(ws.TextMessage, []byte("hello")))
	time.Sleep(100 * time.Millisecond)
	// closing the client makes the server's ReadMessage error and break the loop
	require.NoError(t, c.Close())
	time.Sleep(150 * time.Millisecond)
}

func TestSocketReaderRecoversFromPanic(t *testing.T) {
	setupEnv(t)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/ws", websocket.New(SocketReader))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	// Swap the per-message seam for one that panics. SocketReader runs on the
	// goroutine fasthttp/websocket spawns after the connection hijack, which
	// recover.New does not cover, so without the explicit recover in
	// SocketReader this panic escapes and kills the whole test process.
	orig := handleSocketMessageFunc
	handleSocketMessageFunc = func(_ []byte) { panic("boom from message handler") }
	t.Cleanup(func() { handleSocketMessageFunc = orig })

	dialer := ws.Dialer{HandshakeTimeout: 2 * time.Second}
	var c *ws.Conn
	for i := 0; i < 50; i++ {
		conn, _, derr := dialer.Dial("ws://"+addr+"/ws", nil)
		if derr == nil {
			c = conn
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NotNil(t, c, "could not dial websocket server")
	require.NoError(t, c.WriteMessage(ws.TextMessage, []byte("trigger")))
	require.NoError(t, c.Close())

	// Reaching this dial at all proves the test process survived the panic.
	// If the recover were absent, the panicked goroutine would have crashed
	// `go test` before this line ran; the listener would be gone and the
	// dial would never succeed.
	var c2 *ws.Conn
	for i := 0; i < 50; i++ {
		conn, _, derr := dialer.Dial("ws://"+addr+"/ws", nil)
		if derr == nil {
			c2 = conn
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NotNil(t, c2, "server not accepting new connections after a handler panic")
	require.NoError(t, c2.Close())
}
