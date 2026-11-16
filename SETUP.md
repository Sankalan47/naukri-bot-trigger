# Setup: from clone to running 24/7

This guide covers the full path from `git clone` to a bot that survives reboots, on:

- **Linux 64-bit**: x86_64 (most VPSes and PCs) and aarch64/arm64 (Oracle Cloud ARM, or a Raspberry Pi running 64-bit OS)
- **Linux 32-bit**: armv7l (Raspberry Pi 2/3/4 on 32-bit Raspberry Pi OS)
- **Windows 64-bit**: Windows 10/11 x64

---

## 0. Things you need on every platform

1. **A Telegram bot token.** Message [@BotFather](https://t.me/botfather), run `/newbot`, and copy the token.
2. **Each user's Telegram user ID.** Message [@userinfobot](https://t.me/userinfobot); it replies with your numeric ID.
3. **A Naukri.com account per user.** Email + password login is recommended. If the account only uses Google sign-in, use the cookie-session flow in [step 4](#4-bootstrap-naukri-sessions).
4. **Go 1.23 or newer** on the machine where you build. The target machine doesn't need Go if you cross-compile.

### Figure out which Linux section applies

On the Linux target, run:

```bash
uname -m
```

| Output | Architecture | Follow |
|---|---|---|
| `x86_64` | Linux 64-bit (amd64) | [Linux 64-bit](#linux-64-bit-x86_64--arm64) |
| `aarch64` | Linux 64-bit (arm64) | [Linux 64-bit](#linux-64-bit-x86_64--arm64) |
| `armv7l` / `armv6l` | Linux 32-bit ARM | [Linux 32-bit](#linux-32-bit-armv7--raspberry-pi) |

> A Raspberry Pi 3B/4 running **64-bit** Raspberry Pi OS reports `aarch64`. Use the 64-bit section; it's simpler because Playwright ships its own Chromium for arm64.

---

## 1. Clone and configure (all platforms)

```bash
git clone https://github.com/Sankalan47/naukri-bot-trigger.git
cd naukri-bot-trigger
```

Create the two config files from their templates:

```bash
# Linux / macOS
cp .env.example .env
cp users.json.example users.json
```

```powershell
# Windows PowerShell
Copy-Item .env.example .env
Copy-Item users.json.example users.json
```

Edit **`.env`** and set at least:

```dotenv
TELEGRAM_BOT_TOKEN=123456:ABC-your-token
HEADLESS=false
```

`HEADLESS=false` is needed in practice: Naukri's Akamai protection returns "Access Denied" to headless Chromium. Each platform section below explains how to give the browser a display.

Edit **`users.json`** with one entry per person:

```json
[
  {
    "telegram_user_id": 111111111,
    "naukri_email": "you@example.com",
    "naukri_password": "your-naukri-password"
  }
]
```

- `session_dir` is optional and defaults to `sessions/<telegram_user_id>`.
- The bot ignores any Telegram user who isn't listed here.
- Duplicate IDs or session folders make the bot refuse to start, so you'll see the error right away.

Both files are gitignored. Never commit them.

---

## Linux 64-bit (x86_64 / arm64)

### 2. Build

**Option 1: build on the server.** Install Go on the server first, then:

```bash
make build
# produces bin/naukri-bot and bin/save-session
```

**Option 2: cross-compile on your own machine and copy the binary.** Go isn't needed on the server.

```bash
# arm64 target (Oracle Cloud ARM, 64-bit Pi)
make build-linux-arm64                     # → bin/naukri-bot-linux-arm64

# x86_64 target (no Makefile target for this one)
GOOS=linux GOARCH=amd64 go build -o bin/naukri-bot-linux-amd64 ./cmd/naukri-bot
```

The server still needs Chromium, which Playwright normally installs with `go run`. To skip installing Go there, cross-compile the Playwright CLI too:

```bash
GOOS=linux GOARCH=arm64 go build -o bin/playwright-linux-arm64 github.com/mxschmitt/playwright-go/cmd/playwright
# or GOARCH=amd64 → bin/playwright-linux-amd64
```

Copy the project files to the server. The examples use `~/naukri-bot`:

```bash
ssh user@server 'mkdir -p ~/naukri-bot/bin ~/naukri-bot/sessions'
scp bin/naukri-bot-linux-arm64 bin/playwright-linux-arm64 user@server:~/naukri-bot/bin/
scp .env users.json deploy/naukri-bot.service user@server:~/naukri-bot/
```

### 3. Install Chromium and Xvfb on the server

Run these **as the same user the bot will run as**. Playwright stores Chromium in that user's `~/.cache/ms-playwright`.

```bash
cd ~/naukri-bot

# If Go is on the server:
make install-browsers

# Or, with the cross-compiled CLI from step 2:
./bin/playwright-linux-arm64 install --with-deps chromium   # sudo may be prompted for system libs

# Virtual display for headed Chromium
sudo apt update && sudo apt install -y xvfb
```

On non-Debian distros, `--with-deps` may not install the system libraries. Install your distro's Chromium dependencies by hand if the browser fails to launch.

Continue at [step 4](#4-bootstrap-naukri-sessions), then [step 5 (Linux)](#5-linux-run-as-a-systemd-service).

---

## Linux 32-bit (armv7 / Raspberry Pi)

Playwright publishes no Chromium or Node.js builds for 32-bit ARM. So the bot uses:

- the Playwright **driver** only (fetched by `install-driver`)
- the **system** Node.js and Chromium from apt, pointed at via `.env`

This is the setup the project runs on a Raspberry Pi 3B.

### 2. Build (cross-compile; building on a Pi 3B is very slow)

On your own machine:

```bash
make build-linux-arm
# → bin/naukri-bot-linux-arm        (the bot)
# → bin/install-driver-linux-arm    (downloads only the Playwright driver)
```

On Windows PowerShell, without `make`:

```powershell
$env:GOOS="linux"; $env:GOARCH="arm"; $env:GOARM="7"
go build -o bin/naukri-bot-linux-arm ./cmd/naukri-bot
go build -o bin/install-driver-linux-arm ./cmd/install-driver
Remove-Item Env:GOOS, Env:GOARCH, Env:GOARM
```

Copy the files to the Pi:

```bash
ssh pi@raspberrypi 'mkdir -p ~/naukri-bot/bin ~/naukri-bot/sessions'
scp bin/naukri-bot-linux-arm bin/install-driver-linux-arm pi@raspberrypi:~/naukri-bot/bin/
scp .env users.json deploy/naukri-bot.service pi@raspberrypi:~/naukri-bot/
```

### 3. Install system Chromium, Node.js and Xvfb on the Pi

```bash
sudo apt update
sudo apt install -y chromium nodejs xvfb
# On older Raspberry Pi OS (Bullseye and earlier) the package is chromium-browser

node --version          # needs v18 or newer for the Playwright driver
which chromium || which chromium-browser
which node
```

Add the two paths to `~/naukri-bot/.env`:

```dotenv
PLAYWRIGHT_NODEJS_PATH=/usr/bin/node
CHROMIUM_PATH=/usr/bin/chromium          # or /usr/bin/chromium-browser
HEADLESS=false
```

Download the Playwright driver. Run this from the project folder so it picks up `.env`:

```bash
cd ~/naukri-bot
./bin/install-driver-linux-arm
```

> **32-bit x86 (i386/i686):** the same approach should work. Build with `GOARCH=386` instead of `arm`/`GOARM=7`, and use system Chromium and Node. It hasn't been tested with this project.

Continue at [step 4](#4-bootstrap-naukri-sessions), then [step 5 (Linux)](#5-linux-run-as-a-systemd-service).

---

## Windows 64-bit

### 2. Install Go and build

1. Install Go from <https://go.dev/dl/> (the `windows-amd64.msi` installer), then open a **new** PowerShell window.
2. Build. `make` isn't available by default on Windows, so run the commands directly:

```powershell
cd naukri-bot-trigger
go build -o bin\naukri-bot.exe .\cmd\naukri-bot
go build -o bin\save-session.exe .\cmd\save-session
```

**Optional: install `make`.** Get [GNU Make for Windows (GnuWin32)](https://gnuwin32.sourceforge.net/packages/make.htm) and run the "Complete package, except sources" installer. Then add `C:\Program Files (x86)\GnuWin32\bin` to your `PATH` and open a new terminal. After that, `make test` works. Keep using the `go build` commands above to build: `make build` writes binaries without the `.exe` extension, and the `build-linux-*` targets use Unix-style `GOOS=...` syntax that PowerShell and cmd don't understand.

### 3. Install Chromium

```powershell
go run github.com/mxschmitt/playwright-go/cmd/playwright install chromium
```

This puts Chromium in `%USERPROFILE%\AppData\Local\ms-playwright`, under the current user. Run the bot as that same user.

On Windows you don't need Xvfb or `CHROMIUM_PATH`. With `HEADLESS=false`, a real Chrome window opens on your desktop for each action and closes when it finishes. The Windows user therefore needs to be **logged in** for the bot to work.

Continue at [step 4](#4-bootstrap-naukri-sessions), then [step 5 (Windows)](#5-windows-run-at-logon-with-task-scheduler).

---

## 4. Bootstrap Naukri sessions

### 4a. Password login (recommended)

If `naukri_email` and `naukri_password` are set in `users.json`, there's nothing to do. The bot logs in on first use, saves `sessions/<id>/auth_state.json`, and logs in again when the session expires.

### 4b. Cookie-session fallback

Use this for accounts that only sign in with Google. It opens a **visible** browser where you log in by hand, so run it on a machine with a screen:

```bash
# Linux/macOS desktop
./bin/save-session --save-cookies --session-dir sessions/<telegram_user_id>
```

```powershell
# Windows
.\bin\save-session.exe --save-cookies --session-dir sessions\<telegram_user_id>
```

Log in to Naukri in the window that opens. That writes `sessions/<telegram_user_id>/cookies.json`.

If the bot runs on a headless server or a Pi, do this on your desktop and copy the file over:

```bash
scp sessions/<telegram_user_id>/cookies.json user@server:~/naukri-bot/sessions/<telegram_user_id>/
```

Cookie sessions last about 30 days. Repeat this step when `/status` reports the session has expired.

---

## 5 (Linux). Run as a systemd service

Edit `~/naukri-bot/naukri-bot.service` and set these three lines to match your server. The shipped file targets the author's Pi.

```ini
WorkingDirectory=/home/<user>/naukri-bot
ExecStart=/usr/bin/xvfb-run -a /home/<user>/naukri-bot/bin/<binary>
User=<user>
```

`<binary>` is the bot binary for your architecture:

| Architecture | `<binary>` |
|---|---|
| x86_64, built with `make build` on the server | `naukri-bot` |
| x86_64, cross-compiled | `naukri-bot-linux-amd64` |
| arm64 | `naukri-bot-linux-arm64` |
| 32-bit ARM | `naukri-bot-linux-arm` |

`WorkingDirectory` matters because the bot reads `.env` and `users.json` and writes `sessions/` relative to it. `User` must be the user who installed Chromium in step 3.

Install the service and start it:

```bash
chmod +x ~/naukri-bot/bin/*
sudo cp ~/naukri-bot/naukri-bot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now naukri-bot
journalctl -u naukri-bot -f
```

To run it in the foreground once first, as a sanity check:

```bash
cd ~/naukri-bot && xvfb-run -a ./bin/<binary>
```

---

## 5 (Windows). Run at logon with Task Scheduler

Because the browser is headed, run the bot in your interactive session, not as a background Windows service. Task Scheduler can start it every time you sign in:

1. Open **Task Scheduler** and choose **Create Task…** (not "Basic Task").
2. **General** tab:
   - Name it `naukri-bot`.
   - Select **Run only when user is logged on**.
3. **Triggers** tab: **New…** → **At log on** → your user.
4. **Actions** tab: **New…** → **Start a program**:
   - Program: `C:\path\to\naukri-bot\bin\naukri-bot.exe`
   - **Start in:** `C:\path\to\naukri-bot`

   **Start in** is required. Without it the bot can't find `.env` or `users.json`.
5. **Settings** tab:
   - Select **If the task fails, restart every 1 minute**, up to 999 times.
   - Clear **Stop the task if it runs longer than**.
6. **Conditions** tab: clear **Start the task only if the computer is on AC power** (on laptops).

Also keep the PC awake: **Settings → System → Power → Screen and sleep → When plugged in, put my device to sleep after: Never**.

To test it by hand first:

```powershell
cd C:\path\to\naukri-bot
.\bin\naukri-bot.exe
```

To log in automatically after a reboot, without typing a password, look up Windows auto sign-in (`netplwiz`). Weigh the security trade-off for your machine first.

---

## 6. Verify (all platforms)

1. From each configured Telegram account, send `/start` to your bot. It should reply with the command list.
2. Send `/status`. It should report the Naukri session as alive. The first run with password login takes a few extra seconds to log in.
3. Try `/add_skill`, then `/remove_skill`, with a test skill to confirm a real edit works.
4. Scheduled bio touches run at **09:15, 14:30 and 17:00 IST**. Each run sends a success or failure message to Telegram.

---

## Updating

```bash
git pull
# then rebuild (step 2), copy the new binary over if you cross-compile, and restart:
sudo systemctl restart naukri-bot          # Linux
```

On Windows, end the `naukri-bot` task in Task Scheduler (or Task Manager), rebuild, and run the task again.

---

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `config: TELEGRAM_BOT_TOKEN not set` | `.env` missing or the bot started from the wrong folder. Check `WorkingDirectory` (systemd) or **Start in** (Task Scheduler). |
| `reading users.json: ... no such file` | Same as above, or set `USERS_FILE` to an absolute path in `.env`. |
| Bot doesn't reply at all | Your Telegram ID isn't in `users.json`; the bot ignores unknown users. |
| "Access Denied" from Naukri | `HEADLESS` isn't `false`, or on Linux the service isn't wrapped in `xvfb-run -a`. |
| `Missing X server or $DISPLAY` | Linux headed mode without Xvfb. Install `xvfb` and launch via `xvfb-run -a`. |
| `could not install playwright driver` / browser not found | Chromium was installed under a different user than the one running the bot. Rerun step 3 as the service user. |
| 32-bit ARM: Node or driver errors | Check that `PLAYWRIGHT_NODEJS_PATH` and `CHROMIUM_PATH` in `.env` exist (`ls -l`) and that `node --version` is 18 or newer. |
| Session expired / login fails | Password login retries by itself. For cookie sessions, redo [step 4b](#4b-cookie-session-fallback). Check `sessions/<id>/debug_last_error.png` for what the browser saw. |
| Action fails after a Naukri redesign | Selectors changed. Update `internal/naukri/selectors.go`, then test with `save-session --add-skill test` and similar flags. |
