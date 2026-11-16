# Naukri Bot

A Telegram bot that remotely updates your Naukri.com profile (bio, headline, skills) via browser automation — trigger it from your phone, it runs 24/7 on a small home/cloud server. Supports multiple users, each against their own Naukri account.

## Why

Naukri profiles get more recruiter visibility when they look "recently active," but updating one usually means sitting down at a browser. This bot lets you do it from Telegram, from anywhere. Naukri logs in via Google OAuth, which blocks direct automation — so instead of automating the login, the bot saves a logged-in session once (cookies/local storage or email+password) and reuses it for every future action.

## Features

Chat commands:

| Command | Description |
|---|---|
| `/start` | Welcome message, lists available commands |
| `/update_bio` | Prompts for new bio text, updates it |
| `/update_headline` | Prompts for new headline text, updates it |
| `/add_skill` | Prompts for a skill name, adds it to the skills section |
| `/remove_skill` | Prompts for a skill name, removes it from the skills section |
| `/status` | Checks whether the saved Naukri session is still alive |
| `/refresh_session` | Instructions to renew an expired session |
| `/cancel` | Cancels the current multi-step operation |

The four multi-step commands are backed by a per-chat conversation state machine with a 120-second timeout; sending another command interrupts an in-progress one.

There's also a background job: `internal/scheduler` fires a trivial bio "touch" (toggles a trailing period) at 09:15, 14:30, and 17:00 IST daily, so the profile shows recent activity even without manual input. Each run reports success/failure back to Telegram.

## Tech stack

| Layer | Choice | Reason |
|---|---|---|
| Language | Go 1.23+ | Single static binary, nothing to install on the server |
| Telegram | [go-telegram-bot-api/v5](https://github.com/go-telegram-bot-api/telegram-bot-api) | Thin Bot API wrapper; conversation state is hand-rolled |
| Browser automation | [playwright-go](https://github.com/mxschmitt/playwright-go) | Drives real Chromium against Naukri's JS-heavy SPA |
| Process manager | systemd | Restart-on-failure / boot-on-reboot for a compiled binary |
| Secrets | [godotenv](https://github.com/joho/godotenv) | Loads `.env`; credentials are never hardcoded |
| Infra | Raspberry Pi 3B (current) / Oracle Cloud Free Tier ARM VM (previous) | Cheap or free always-on hosting |

## Project structure

```
naukri-bot-trigger/
├── go.mod / go.sum
├── Makefile                          # build, build-linux-arm64, build-linux-arm, install-browsers, test, run, run-save-session
├── .env.example                       # template for .env (process-wide secrets, gitignored)
├── users.json.example                 # template for users.json (per-user Naukri accounts, gitignored)
├── sessions/                          # per-user session files, one dir per Telegram user ID (gitignored)
├── SETUP.md                           # clone-to-deploy guide for Linux (64/32-bit) and Windows
├── deploy/
│   └── naukri-bot.service             # systemd unit
├── cmd/
│   ├── naukri-bot/main.go             # production entrypoint
│   ├── save-session/main.go           # dev tool: headed login, --save-cookies, flag-gated action testing
│   └── install-driver/main.go         # fetches the Playwright driver only (for 32-bit ARM targets)
└── internal/
    ├── config/
    │   └── config.go                  # env + users.json loading and validation
    ├── naukri/
    │   ├── client.go                  # Client: Start/Stop/Login/CheckStatus/UpdateHeadline/UpdateBio/AddSkill/RemoveSkill
    │   ├── selectors.go                # Naukri DOM selectors (update when Naukri ships a new frontend)
    │   ├── text.go / text_test.go      # pure text helpers (e.g. bio-toggle logic) + tests
    │   └── errors.go                   # Error type + ErrorKind enum (Auth/SessionExpired/Selector/Timeout)
    ├── scheduler/
    │   └── scheduler.go / scheduler_test.go  # daily scheduled per-user bio-touch jobs + tests
    └── telegram/
        ├── bot.go                       # update loop, per-user dispatch, lifecycle
        ├── handlers.go                  # /start, /status, /refresh_session, /cancel, conversation entry points
        ├── conversation.go              # per-chat state machine, 120s timeout
        └── errors.go                    # naukri.Error → Telegram-facing copy
```

Each user's `auth_state.json`/`cookies.json` (gitignored) live under `sessions/<telegram_user_id>/` and hold that user's saved Naukri session.

## Getting started

For a full clone-to-deploy walkthrough on Linux (64-bit and 32-bit) and Windows, see **[SETUP.md](SETUP.md)**.

**Prerequisites:** Go 1.23+, one Naukri.com account per user, and a Telegram bot token from [@BotFather](https://t.me/botfather).

```bash
git clone https://github.com/Sankalan47/naukri-bot-trigger.git
cd naukri-bot-trigger
cp .env.example .env             # fill in TELEGRAM_BOT_TOKEN, see Configuration below
cp users.json.example users.json # add one entry per Telegram user + their Naukri account
make build
make install-browsers            # downloads the Playwright driver + Chromium
```

Onboarding a new user is manual: add their `telegram_user_id`/`naukri_email`/`naukri_password` to `users.json` and redeploy — there's no self-service `/register` flow.

Bootstrap each user's Naukri session before the bot can act on their behalf — either:

- **Password login** (recommended): set that user's `naukri_email`/`naukri_password` in `users.json`; the bot logs in automatically and re-logs in when the session expires.
- **Cookie session** (fallback): run `./bin/save-session --save-cookies --session-dir sessions/<their_telegram_user_id>`, log in manually in the browser window that opens. This saves that user's `cookies.json`, loaded on every future run. It expires after ~30 days, at which point you repeat this step (or use `/refresh_session`).

Then run the bot:

```bash
make run
```

Message your bot on Telegram with `/start` (from each configured user's account) to confirm it's alive.

## Configuration

Process-wide environment variables, read via `internal/config/config.go`:

| Variable | Required | Purpose |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | Yes | From BotFather; process fails fast at startup if missing |
| `USERS_FILE` | No | Path to the per-user roster (default `users.json`) |
| `PLAYWRIGHT_NODEJS_PATH` / `CHROMIUM_PATH` | No | Point at apt-installed system Node/Chromium — needed on 32-bit ARM (e.g. Raspberry Pi 3B), where Playwright has no prebuilt binaries |
| `HEADLESS` | No | Set to `false` to run a headed browser (see Deployment — required on Naukri due to bot detection on some setups) |
| `NAUKRI_EMAIL` / `NAUKRI_PASSWORD` | No | Only read by the `save-session` dev tool, which drives a single account independent of `users.json` |

See `.env.example` for the full template.

Per-user Naukri accounts live in `users.json` (gitignored — see `users.json.example`), a JSON array of:

| Field | Required | Purpose |
|---|---|---|
| `telegram_user_id` | Yes | That user's Telegram ID (from [@userinfobot](https://t.me/userinfobot)); the bot silently ignores messages from anyone not listed |
| `naukri_email` / `naukri_password` | No | Enables automatic (re-)login when that user's session expires |
| `session_dir` | No | Where that user's `auth_state.json`/`cookies.json`/debug screenshot live (default `sessions/<telegram_user_id>`) |

`telegram_user_id` and the resolved `session_dir` must each be unique across the file; the process fails fast at startup otherwise.

## Development

```bash
make test               # go test ./...
make run-save-session    # build + run the dev tool (headed browser)
```

`cmd/save-session` doubles as a way to exercise real Naukri actions without going through Telegram:

| Flag | Effect |
|---|---|
| `--save-cookies` | Log in manually in a visible browser, save `cookies.json` |
| `--update-bio "text"` | Call `UpdateBio` with this text and exit |
| `--update-headline "text"` | Call `UpdateHeadline` with this text and exit |
| `--add-skill "name"` | Call `AddSkill` with this skill and exit |
| `--remove-skill "name"` | Call `RemoveSkill` with this skill and exit |
| `--touch-bio` | Call `TouchBio` (toggle trailing full stop) and exit |
| `--session-dir <dir>` | Directory to load/save session files (default `.`) — point it at `sessions/<telegram_user_id>` to debug a specific user's session |

`save-session` reads `NAUKRI_EMAIL`/`NAUKRI_PASSWORD` directly from the environment/`.env`, independent of `users.json`. Use this to validate selectors and behavior against the live site before trusting a scheduled or Telegram-triggered run.

## Deployment

Cross-compile locally and copy the binary over — no Go toolchain needed on the target:

```bash
make build-linux-arm64   # Oracle Cloud Free Tier ARM VM (Ubuntu 22.04, headless Chromium)
make build-linux-arm     # 32-bit ARM, e.g. Raspberry Pi 3B
```

Install `deploy/naukri-bot.service` as a systemd unit on the target so the bot restarts on failure/boot:

```bash
sudo cp deploy/naukri-bot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now naukri-bot
journalctl -u naukri-bot -f   # tail logs
```

**Raspberry Pi (32-bit ARM) specifics:** Playwright has no prebuilt Node.js/Chromium for this architecture, so the bot uses apt-installed system binaries via `PLAYWRIGHT_NODEJS_PATH`/`CHROMIUM_PATH`, and `install-driver-linux-arm` (built by `make build-linux-arm`) fetches only the platform-independent driver package. Naukri's Akamai bot protection also returns "Access Denied" to headless Chromium — the fix is `HEADLESS=false` with the systemd unit launching via `xvfb-run -a`, so the browser is genuinely headed, just pointed at a virtual display (`sudo apt install -y xvfb`) instead of a physical monitor. This is a standard way to run headed browser automation on a display-less server, not a detection-evasion technique.

## Gotchas

- **Sessions expire** after ~30 days; `/status` detects this early, `/refresh_session` explains how to renew.
- **Selectors may break** if Naukri changes its frontend — all selectors live in `internal/naukri/selectors.go`; validate with `playwright codegen https://www.naukri.com/mnjuser/profile`.
- **Manual onboarding only** — the bot only responds to Telegram users listed in `users.json`; adding someone requires editing that file and redeploying, there's no self-service `/register`.
- **One Chromium at a time, for everyone** — a package-level lock serializes every user's Naukri actions, and each action launches a fresh browser, uses it, persists that user's session, and closes it again (rather than keeping one browser alive per user). This keeps RAM usage low enough for a Raspberry Pi 3B at the cost of a few seconds of cold-start latency per action; concurrent Telegram commands from different users queue rather than run in parallel, but Telegram itself stays responsive.

## License

MIT — see [LICENSE](LICENSE).
