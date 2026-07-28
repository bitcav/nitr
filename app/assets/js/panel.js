var qrcode;

function jsUcfirst(string) {
  return string.charAt(0).toUpperCase() + string.slice(1);
}

function humanBytes(bytes) {
  var units = ["B", "KB", "MB", "GB", "TB"];
  var i = 0;
  var n = bytes;
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
  return (i === 0 ? n : n.toFixed(1)) + " " + units[i];
}

// --- Theme ----------------------------------------------------------------

function currentTheme() {
  return document.documentElement.getAttribute("data-theme") === "dark" ? "dark" : "light";
}

function applyTheme(theme) {
  document.documentElement.setAttribute("data-theme", theme);
  try { localStorage.setItem("nitr-theme", theme); } catch (e) {}
}

// --- Charts (uPlot 1.6.31, vendored at /assets/js/uplot.iife.min.js) ------
// Data source: the /status WebSocket live-metrics stream; the last
// HISTORY_LEN samples are kept in a ring buffer per metric, so these charts
// are session history only — nothing is persisted server-side.

var HISTORY_LEN = 120; // ~6 min at the default 3s push interval
var metricHistory = { t: [], cpu: [], ram: [] };
var charts = {};

function chartThemeColors() {
  return currentTheme() === "dark"
    ? { axis: "#7d828a", grid: "rgba(255,255,255,0.08)", cpu: "#6ea8fe", ram: "#f78c6c" }
    : { axis: "#999", grid: "rgba(0,0,0,0.06)", cpu: "#1e87f0", ram: "#d84315" };
}

function makeChart(el, series, color) {
  var c = chartThemeColors();
  return new uPlot({
    width: Math.max(el.clientWidth || 0, 280),
    height: 200,
    scales: { x: { time: true }, y: { range: [0, 100] } },
    series: [
      {},
      { stroke: color, width: 2, fill: color + "22" },
    ],
    axes: [
      { stroke: c.axis, grid: { stroke: c.grid }, ticks: { stroke: c.grid } },
      { stroke: c.axis, grid: { stroke: c.grid }, ticks: { stroke: c.grid }, size: 40 },
    ],
    legend: { show: false },
    cursor: { show: false },
  }, [metricHistory.t, metricHistory[series]], el);
}

function buildCharts() {
  if (!$("#cpuChart").length) return; // not the panel page
  ["cpu", "ram"].forEach(function (key) {
    if (charts[key]) { charts[key].destroy(); }
    var el = document.getElementById(key + "Chart");
    el.innerHTML = "";
    var c = chartThemeColors();
    charts[key] = makeChart(el, key, key === "cpu" ? c.cpu : c.ram);
  });
}

function pushSample(data) {
  var cpu = (typeof data.cpuUsage === "number") ? data.cpuUsage : null;
  var ram = data.ram || {};
  var ramPct = (ram.total > 0) ? (ram.usage / ram.total) * 100 : null;
  metricHistory.t.push(Date.now() / 1000);
  metricHistory.cpu.push(cpu);
  metricHistory.ram.push(ramPct);
  if (metricHistory.t.length > HISTORY_LEN) {
    metricHistory.t.shift(); metricHistory.cpu.shift(); metricHistory.ram.shift();
  }
  if (charts.cpu) charts.cpu.setData([metricHistory.t, metricHistory.cpu]);
  if (charts.ram) charts.ram.setData([metricHistory.t, metricHistory.ram]);
}

function fitCharts() {
  ["cpu", "ram"].forEach(function (key) {
    var el = document.getElementById(key + "Chart");
    if (charts[key] && el && el.clientWidth > 0) {
      charts[key].setSize({ width: el.clientWidth, height: 200 });
    }
  });
}

$(window).on("resize", fitCharts);

// Charts are built while the Metrics tab is display:none (clientWidth 0), so
// they start clamped narrow. uk-tab toggles the connected content <li>s via
// UIkit's Togglable mixin, which fires "shown" on the revealed <li> — NOT on
// .uk-tab itself and NOT "itemshown" (those are Slider events).
$(document).on("shown", "#panelSections li", fitCharts);

function renderMetrics(data) {
  if (typeof data.cpuUsage === "number") {
    $("#cpuUsage").text(data.cpuUsage.toFixed(1) + "%");
  }
  var ram = data.ram || {};
  var ramUsed = ram.usage || 0;
  var ramTotal = ram.total || 0;
  var ramPct = ramTotal > 0 ? Math.round((ramUsed / ramTotal) * 100) : 0;
  $("#ramUsage").html(humanBytes(ramUsed) + " / " + humanBytes(ramTotal) +
    "<br><span class=\"uk-text-small uk-text-muted\">" + ramPct + "%</span>");
  var disks = data.disks || [];
  var html = disks.map(function (d) {
    var pct = (typeof d.percent === "number") ? d.percent : (d.size > 0 ? (d.used / d.size) * 100 : 0);
    return "<div class=\"disk-row\">" +
      "<span class=\"disk-mount uk-text-small uk-text-muted\">" + (d.mountPoint || "") + "</span>" +
      "<span class=\"disk-nums\">" + humanBytes(d.used) + " / " + humanBytes(d.size) +
      " <span class=\"uk-text-small\">" + pct.toFixed(0) + "%</span></span></div>";
  }).join("");
  $("#diskUsage").html(html || "--");
}

$(document).ready(function () {
  buildCharts();
  fitCharts(); // no-op while Metrics is hidden; covers Metrics active at first paint
  $("#themeToggle").click(function () {
    applyTheme(currentTheme() === "dark" ? "light" : "dark");
    buildCharts(); // rebuild picks up theme colors; sizes from live clientWidth
  });

  fetch("/content")
    .then(function (response) {
      return response.json()
    })
    .then(function (data) {
      $("#hostname").val(data.name)
      $("#platform").val(jsUcfirst(data.description));
      $("#ip").val(data.ip);
      $("#port").val(data.port);
      $("#key").val(data.key);
      qrcode = new QRCode("qrcode", {
        text: data.qrCode,
        width: 240,
        height: 240,
        colorDark: "#000000",
        colorLight: "#ffffff",
        correctLevel: QRCode.CorrectLevel.H,
      });
    });

  var clipboard = new ClipboardJS(".copy");

  clipboard.on("success", function (e) {
    e.clearSelection();
    UIkit.notification({
      message: "API key copied to clipboard.",
      status: "success",
      pos: "bottom-center",
      timeout: 2000,
    });
  });

  $("#genBtn").click(function (e) {
    e.preventDefault();
    UIkit.modal.confirm('<h2 class="uk-text-danger uk-text-center">Are you sure?</h2><p class="uk-text-justify">This process is <b>irreversible</b> and will leave all clients using the <b>old key</b> without access!</p><p>Press OK to generate a new one.</p>').then(function () {
      fetch("/generate", {
        method: "POST",
      })
        .then(function (response) {
          return response.json();
        })
        .then(function (data) {
          console.log(data)
          $("#key").val(data.key);
          $("#qrcode").empty();
                qrcode = new QRCode("qrcode", {
                  text: data.qrCode,
                  width: 240,
                  height: 240,
                  colorDark: "#000000",
                  colorLight: "#ffffff",
                  correctLevel: QRCode.CorrectLevel.H,
                });

        });
      var btn = $("#genBtn");
      btn.prop("disabled", true).attr("uk-spinner", true);
      setTimeout(function () {
        btn.prop("disabled", false).removeAttr("uk-spinner");
      }, 500);
    }, function(){

    });


  });

  $("#mainOverlay").hide()
  const url = window.location.href
  const arr = url.split("/");
  const server = arr[2]

  let attempts = 0

  function connect() {
    var ws = new WebSocket(`ws://${server}/status`);

    ws.onopen = function () {
      if (attempts > 0) {
        $("#mainOverlay").hide();
        UIkit.notification({
          message: "Agent connected.",
          status: "success",
          pos: "top-center",
          timeout: 4000,
        });
        ws.send(`Client connected to ws from ${server}`);
      } else {
        ws.send(`Client connected to ws from ${server}`);
      }
    }

    ws.onmessage = function (e) {
      var data;
      try { data = JSON.parse(e.data); } catch (err) { console.log("Server:", e.data); return; }
      renderMetrics(data);
      pushSample(data);
    };

    ws.onclose = function (e) {
      $("#mainOverlay").show()
      UIkit.notification({
        message: "Agent disconnected. \nRetrying in 5 seconds",
        status: "danger",
        pos: "top-center",
        timeout: 4000,
      });

      console.log('Socket is closed. Reconnect will be attempted in 1 second.', e.reason);
      setTimeout(function () {
        attempts = attempts + 1
        connect();
      }, 5000);
    };

    ws.onerror = function (err) {
      console.error('Socket encountered error: ', err.message, 'Closing socket');
      ws.close();
    };
  }

  if (arr[3]==="panel") {
    console.log("connecting to ws...")
    connect();
  }

});
