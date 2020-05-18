$(document).ready(function () {
  $("#passwordForm").keyup(function () {
    var currentPassword = $("#current-password").val();
    var newPassword = $("#new-password").val();
    var repeatNewPassword = $("#repeat-new-password").val();

    if (currentPassword == "" || newPassword == "" || repeatNewPassword == "") {
      $("#saveBtn").prop("disabled", true);
    } else {
      $("#saveBtn").prop("disabled", false);
    }
  });

  $("#saveBtn").click(function (e) {
    e.preventDefault();
    var currentPassword = $("#current-password").val();
    var newPassword = $("#new-password").val();
    var repeatNewPassword = $("#repeat-new-password").val();

    if (currentPassword == newPassword) {
      $("#saveBtn").prop("disabled", true);
      UIkit.notification({
        message: "The new password must be different to the current one.",
        status: "danger",
        pos: "bottom-center",
        timeout: 5000,
      });
    } else if (newPassword == repeatNewPassword) {
      const passwordForm = document.getElementById("passwordForm");
      const formData = new FormData(passwordForm);
      fetch("/password", {
        method: "POST",
        body: formData,
      }).then(function (response) {
        if (response.ok) {
          $("#passwordForm").trigger("reset");
          $("#saveBtn").prop("disabled", true);
          UIkit.notification({
            message: "Password changed successfully.",
            status: "success",
            pos: "bottom-center",
            timeout: 5000,
          });
        } else {
          $("#saveBtn").prop("disabled", true);
          UIkit.notification({
            message: "Your current password is incorrect.",
            status: "danger",
            pos: "bottom-center",
            timeout: 5000,
          });
        }
      });
      var btn = $(this);
      btn.prop("disabled", true).attr("uk-spinner", true);
      setTimeout(function () {
        btn.prop("disabled", false).removeAttr("uk-spinner");
      }, 500);
    } else {
      UIkit.notification({
        message: "Passwords don't match.",
        status: "danger",
        pos: "bottom-center",
        timeout: 5000,
      });
    }
  });
});
