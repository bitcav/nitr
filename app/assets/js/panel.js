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
    return "<div class=\"uk-text-left\">" +
      "<span class=\"uk-text-small uk-text-muted\">" + (d.mountPoint || "") + "</span> " +
      humanBytes(d.used) + " / " + humanBytes(d.size) +
      " <span class=\"uk-text-small\">" + pct.toFixed(0) + "%</span></div>";
  }).join("");
  $("#diskUsage").html(html || "--");
}

$(document).ready(function () {
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
