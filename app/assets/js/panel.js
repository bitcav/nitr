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
    var btn = $(this);
    btn.prop("disabled", true).attr("uk-spinner", true);
    setTimeout(function () {
      btn.prop("disabled", false).removeAttr("uk-spinner");
    }, 500);
  });
});
