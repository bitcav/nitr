# usage.gif — curl + jq against the nitr JSON API.
# Run via scripts/regen-images.sh: it builds the binary, starts the server
# on port 8471 with a scratch --data-dir, then runs this tape.
#
# Identity hygiene (do not regress):
#   - Env PS1 pins the prompt to "$ " so user@host never renders.
#   - Only endpoints whose payloads carry no host/network/user identity are
#     shown (cpu, ram). Do NOT add host/network/isp/disks here — their
#     payloads can contain hostname, IPs, MACs, or user-owned mount paths.
#   - The API key is the throwaway key from the scratch data-dir and stays
#     inside the KEY shell variable — never echoed to the terminal.

Set Shell bash
Set FontSize 16
Set Width 900
Set Height 550
Set Padding 20
Set TypingSpeed 60ms
Set Theme "Catppuccin Mocha"
Env PS1 "$ "

Output images/usage.gif

Type "curl -s -X POST -d password=123456 -c /tmp/nitr-demo-cookies http://localhost:8471/ -o /dev/null"
Enter
Sleep 1s

Type "KEY=$(curl -s -b /tmp/nitr-demo-cookies http://localhost:8471/content | jq -r .key)"
Enter
Sleep 1s

Type "curl -s -H x-api-key:$KEY http://localhost:8471/api/v1/cpu | jq"
Enter
Sleep 3s

Type "curl -s -H x-api-key:$KEY http://localhost:8471/api/v1/ram | jq"
Enter
Sleep 3s
