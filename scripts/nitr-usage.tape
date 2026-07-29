# usage.gif — the CLI basics: nitr cpu, nitr ram. No login, no API key, no
# jq — the info commands read the local machine directly and need no server.
# Run via scripts/regen-images.sh (it builds the binary to $NITR_BIN).
#
# Viewport sizing (measured, not guessed): cpu's "Usage Each:" line is
# 156-200 chars depending on live values — it wraps at any sane width, so
# the demo pipes cpu through head -6 (visible in the command, clearly
# deliberate) and shows the six summary lines. Longest remaining line is
# 49 chars -> Width 900. 12 rendered lines (~33px/line) -> Height 480 with
# margin. Verify the last frame shows the final prompt unclipped.
#
# Identity hygiene (do not regress):
#   - Env PS1 pins the prompt to "$ " so user@host never renders.
#   - cpu/ram output is CPU model + byte counts: no hostname, IPs, keys,
#     or user paths.
#   - `nitr disks` is deliberately NOT shown: the real mount list leaks
#     host layout (WSL/Docker Desktop paths) and terminal output can't be
#     DOM-scrubbed like the web panel. Don't add it back without a generic
#     environment (e.g. a container).
#   - The Hide/Show block stages a scratch dir off-camera so the binary
#     runs as ./nitr without a build path on screen.

Set Shell bash
Set FontSize 16
Set Width 900
Set Height 320
Set Padding 20
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"
Env PS1 "$ "

Output images/usage.gif

Hide
Type "rm -rf /tmp/nitr-demo-cli && mkdir -p /tmp/nitr-demo-cli && cp $NITR_BIN /tmp/nitr-demo-cli/nitr && cd /tmp/nitr-demo-cli"
Enter
Sleep 2s
Type "clear"
Enter
Sleep 500ms
Show

Type "./nitr cpu | head -6"
Enter
Sleep 3s

Type "./nitr ram"
Enter
Sleep 3s
