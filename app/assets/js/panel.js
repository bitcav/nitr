$(document).ready(function () {
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
          $("#key").val(data.key);
          $("#qr")
            .removeAttr("src")
            .attr("src", "data:image/png;base64, " + data.qrCode)
            .show();
          UIkit.notification({
            message: "Restart the agent service to apply these changes.",
            status: "warning",
            pos: "bottom-center",
            timeout: 5000,
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
  console.log(arr)
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
      console.log('Server:', e.data);
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
