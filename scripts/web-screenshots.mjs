#!/usr/bin/env node
// web-screenshots.mjs — capture images/login-web.png, images/panel-web.png
// (Overview tab, dark) and images/panel-metrics-web.png (Metrics tab, dark,
// charts populated) from a running nitr server, with all identity-bearing
// fields scrubbed.
//
// Prerequisites: node >= 22 (uses the global WebSocket/fetch — zero npm
// dependencies, drives Chrome over the DevTools Protocol) and google-chrome
// or chromium (override with CHROME_BIN=/path/to/browser).
//
// Normally run by scripts/regen-images.sh, which builds the binary and
// starts the server. Standalone:
//   NITR_PORT=8471 node scripts/web-screenshots.mjs
//
// SANITIZE — every identity-bearing value the panel renders, replaced in the
// DOM before the screenshot is taken. Adding a field to the panel? Add its
// scrub here. After capture the page is verified to contain NONE of the real
// values reported by /content (hostname, IP, API key) — the script exits
// non-zero if any survive, so an unsanitized image can never be committed
// silently.
import { spawn } from "node:child_process";
import { writeFileSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const PORT = process.env.NITR_PORT || "8471";
const BASE = `http://127.0.0.1:${PORT}`;
const PASSWORD = process.env.NITR_PASSWORD || "123456"; // default scratch-db password
const OUT_DIR = new URL("../images/", import.meta.url).pathname;

// Demo replacements. IPs come from RFC 5737 documentation ranges.
// diskMounts are assigned positionally in render order (each row gets a
// distinct, plausible label); extra rows fall back to /mnt/dataN. Labels
// stay <=4 chars: .disk-mount ellipsizes anything longer on narrow widgets.
const DEMO = {
  hostname: "nitr-demo",
  ip: "192.0.2.10",
  key: "DEMOKEY123",
  diskMounts: ["/", "/var", "/tmp", "/opt"],
};

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// --- Minimal CDP client -----------------------------------------------------

function launchChrome() {
  const bin = process.env.CHROME_BIN || "google-chrome";
  const profile = mkdtempSync(join(tmpdir(), "nitr-chrome-"));
  const proc = spawn(bin, [
    "--headless=new",
    "--remote-debugging-port=0",
    `--user-data-dir=${profile}`,
    "--no-first-run",
    "--window-size=1280,900",
    "--hide-scrollbars",
    "about:blank",
  ], { stdio: ["ignore", "ignore", "pipe"] });
  const wsUrl = new Promise((resolve, reject) => {
    let buf = "";
    proc.stderr.on("data", (d) => {
      buf += d;
      const m = buf.match(/DevTools listening on (ws:\/\/\S+)/);
      if (m) resolve(m[1]);
    });
    proc.on("exit", () => reject(new Error("chrome exited early:\n" + buf)));
  });
  return { proc, profile, wsUrl };
}

// startLoad induces real, moderate CPU and RAM load so the metrics charts
// move during the capture window (an idle machine produces flat lines that
// read as broken). The metrics are genuine measurements of this load — no
// fabricated data. Returns a stop function; every process is SIGKILLed.
function startLoad() {
  const procs = [];
  for (let i = 0; i < 3; i++) {
    procs.push(spawn(process.execPath, ["-e", "while(true){}"], { stdio: "ignore" }));
  }
  procs.push(spawn(process.execPath, ["-e",
    "const a=[];setInterval(()=>{a.push(Buffer.alloc(64*1024*1024,1));if(a.length>8)a.shift()},200)"],
    { stdio: "ignore" }));
  return () => procs.forEach((p) => p.kill("SIGKILL"));
}

class CDP {
  constructor(ws) { this.ws = ws; this.id = 0; this.pending = new Map(); this.handlers = []; }
  static async connect(wsUrl) {
    const ws = new WebSocket(wsUrl);
    await new Promise((res, rej) => { ws.onopen = res; ws.onerror = rej; });
    const cdp = new CDP(ws);
    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data);
      if (msg.id && cdp.pending.has(msg.id)) {
        const { resolve, reject } = cdp.pending.get(msg.id);
        cdp.pending.delete(msg.id);
        msg.error ? reject(new Error(msg.error.message)) : resolve(msg.result);
      } else if (msg.method) {
        cdp.handlers.forEach((h) => h(msg));
      }
    };
    return cdp;
  }
  send(method, params = {}) {
    const id = ++this.id;
    this.ws.send(JSON.stringify({ id, method, params }));
    return new Promise((resolve, reject) => this.pending.set(id, { resolve, reject }));
  }
  once(method) {
    return new Promise((resolve) => {
      const h = (msg) => {
        if (msg.method === method) {
          this.handlers = this.handlers.filter((x) => x !== h);
          resolve(msg.params);
        }
      };
      this.handlers.push(h);
    });
  }
  // Evaluate an expression in the page, returning the value.
  async eval(expression) {
    const r = await this.send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
    if (r.exceptionDetails) throw new Error("page eval failed: " + JSON.stringify(r.exceptionDetails));
    return r.result.value;
  }
  async waitFor(expression, timeoutMs = 15000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      if (await this.eval(expression)) return;
      await sleep(250);
    }
    throw new Error("timed out waiting for: " + expression);
  }
}

// --- Real values, for post-capture verification -----------------------------

async function realHostValues() {
  const login = await fetch(`${BASE}/`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: `password=${encodeURIComponent(PASSWORD)}`,
    redirect: "manual",
  });
  const cookie = (login.headers.get("set-cookie") || "").split(";")[0];
  if (!cookie) throw new Error("login failed — is the server running with the default password?");
  const content = await (await fetch(`${BASE}/content`, { headers: { cookie } })).json();
  return [content.name, content.ip, content.key].filter(Boolean);
}

// --- Main -------------------------------------------------------------------

const chrome = launchChrome();
let stopLoad = () => {};
try {
  const browserWs = await chrome.wsUrl;
  const port = new URL(browserWs).port;
  const targets = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
  const page = targets.find((t) => t.type === "page");
  const cdp = await CDP.connect(page.webSocketDebuggerUrl);
  await cdp.send("Page.enable");

  // 1. Login page — renders no identity, screenshot as-is.
  let loaded = cdp.once("Page.loadEventFired");
  await cdp.send("Page.navigate", { url: `${BASE}/` });
  await loaded;
  await sleep(800); // let UIkit finish painting
  let shot = await cdp.send("Page.captureScreenshot", { format: "png" });
  writeFileSync(join(OUT_DIR, "login-web.png"), Buffer.from(shot.data, "base64"));
  console.log("wrote images/login-web.png");

  // 2. Log in, open the panel.
  await cdp.eval(`fetch("/", {method: "POST", headers: {"Content-Type": "application/x-www-form-urlencoded"},
    body: "password=${PASSWORD}"}).then(r => r.ok || r.redirected)`);
  loaded = cdp.once("Page.loadEventFired");
  await cdp.send("Page.navigate", { url: `${BASE}/panel` });
  await loaded;

  // Wait for host info, the QR code, and the first live-metrics sample.
  await cdp.waitFor(`document.querySelector("#hostname").value !== ""`);
  await cdp.waitFor(`document.querySelector("#qrcode img, #qrcode canvas") !== null`);
  await cdp.waitFor(`!document.querySelector("#cpuUsage").textContent.includes("--")`);

  // Scrub identity-bearing fields (see SANITIZE note at top). The Metrics
  // widgets re-render from every WebSocket sample, so the disk-mount scrub
  // must be re-applied right before each capture — hence a function, run
  // before every screenshot. withQr rebuilds the QR code so it encodes the
  // scrubbed payload, not real data.
  const scrub = (withQr) => cdp.eval(`(() => {
    document.getElementById("hostname").value = ${JSON.stringify(DEMO.hostname)};
    document.getElementById("ip").value = ${JSON.stringify(DEMO.ip)};
    document.getElementById("key").value = ${JSON.stringify(DEMO.key)};
    document.querySelectorAll(".disk-mount").forEach((e, i) => {
      const m = ${JSON.stringify(DEMO.diskMounts)};
      e.textContent = i < m.length ? m[i] : "/mnt/data" + (i - m.length + 1);
    });
    if (${withQr}) {
      const qr = {
        name: ${JSON.stringify(DEMO.hostname)},
        description: document.getElementById("platform").value,
        ip: ${JSON.stringify(DEMO.ip)},
        port: document.getElementById("port").value,
        key: ${JSON.stringify(DEMO.key)},
      };
      document.getElementById("qrcode").innerHTML = "";
      new QRCode("qrcode", { text: JSON.stringify(qr), width: 240, height: 240,
        colorDark: "#000000", colorLight: "#ffffff", correctLevel: QRCode.CorrectLevel.H });
    }
    return true;
  })()`);

  // Fail-closed check: no real hostname / IP / API key anywhere in the page
  // (text, input values, attributes, or title) before pixels are captured.
  const secrets = await realHostValues();
  const verifyScrubbed = async () => {
    const pageText = await cdp.eval(`document.title + "\\n" + document.documentElement.outerHTML`);
    const leaked = secrets.filter((s) => pageText.includes(s));
    if (leaked.length) {
      throw new Error(`sanitization failed — real values still on page: ${leaked.map(() => "<redacted>").join(", ")}`);
    }
  };

  // Clip to the real content bottom (navbar, container, footer are the only
  // in-flow blocks) so captures have no dead whitespace band. Width stays
  // the fixed window width.
  const contentHeight = () => cdp.eval(`Math.ceil(Math.max(
    ...[...document.querySelectorAll(".uk-navbar-container, .uk-container, body > div.uk-text-center")]
      .map((e) => e.getBoundingClientRect().bottom)) + 16)`);
  const capture = async (file) => {
    const height = Math.min(await contentHeight(), 1600);
    const shot = await cdp.send("Page.captureScreenshot", {
      format: "png",
      clip: { x: 0, y: 0, width: 1280, height, scale: 1 },
      captureBeyondViewport: true,
    });
    writeFileSync(join(OUT_DIR, file), Buffer.from(shot.data, "base64"));
    console.log(`wrote images/${file} (sanitized: ${secrets.length} real values verified absent)`);
  };

  // 3. Dark mode for both panel captures. applyTheme sets data-theme and
  // persists it; buildCharts() rebuilds so chart colors come from the dark
  // theme, not light-theme colors on a dark background (panel.js:128).
  // Load starts now so chart samples accumulate movement from here on.
  stopLoad = startLoad();
  await scrub(true);
  await cdp.eval(`applyTheme("dark"), buildCharts(), true`);
  await sleep(500);
  await verifyScrubbed();
  await capture("panel-web.png");

  // 4. Metrics tab: charts need a real span of samples — wait for ~10
  // (one every ~3s, so ~30s under induced load). A two-point flat line
  // reads as a broken chart.
  await cdp.eval(`document.querySelectorAll(".uk-tab li a")[1].click(), true`);
  await cdp.waitFor(`typeof metricHistory !== "undefined" && metricHistory.t.length >= 10`, 60000);
  await sleep(800); // let the "shown" handler fit the charts to the tab
  await scrub(false); // re-scrub: samples re-render the disk mounts
  await verifyScrubbed();
  await capture("panel-metrics-web.png");
  stopLoad(); stopLoad = () => {};

  chrome.proc.kill("SIGKILL");
} finally {
  stopLoad();
  rmSync(chrome.profile, { recursive: true, force: true });
}
