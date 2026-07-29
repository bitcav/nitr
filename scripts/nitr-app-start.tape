# app-start.gif — starting the nitr server.
# Run via scripts/regen-images.sh: it builds the binary to $NITR_BIN and
# starts its own server first (this tape demonstrates startup on a second
# port against a scratch dir).
#
# Identity hygiene (do not regress):
#   - Env PS1 pins the prompt to "$ " so user@host never renders.
#   - The startup banner prints only the localhost URL and version.
#   - The scratch dir is /tmp so nitr.db/config.ini/nitr.log never land in
#     the repo. The Hide/Show block stages that dir off-camera.

Set Shell bash
Set FontSize 16
Set Width 900
Set Height 550
Set Padding 20
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"
Env PS1 "$ "

Output images/app-start.gif

Hide
Type "rm -rf /tmp/nitr-demo-start && mkdir -p /tmp/nitr-demo-start && cp $NITR_BIN /tmp/nitr-demo-start/nitr && cd /tmp/nitr-demo-start"
Enter
Sleep 2s
Type "clear"
Enter
Sleep 500ms
Show

Type "./nitr server --port 8472"
Enter
Sleep 5s
Ctrl+C
Sleep 2s
