package naukri

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"
)

const (
	authStateFile   = "auth_state.json"
	cookiesFile     = "cookies.json"
	debugScreenshot = "debug_last_error.png"

	naukriLoginURL = "https://www.naukri.com/nlogin/login"
)

var (
	defaultUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	headedUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	defaultViewport  = playwright.Size{Width: 1280, Height: 800}
)

// StartOptions configures how Start launches the browser. Production use is
// Client.Start() (headless, loads a saved session); dev tooling uses
// HeadedStartOptions() (visible, slowed down, no saved session — mirrors
// test_local.py's monkeypatched start()).
type StartOptions struct {
	Headless    bool
	SlowMoMs    float64
	UserAgent   string
	LoadSession bool
}

func defaultStartOptions() StartOptions {
	// Naukri's Akamai bot protection returns "Access Denied" to headless
	// Chromium regardless of UA/viewport spoofing, but not to headed Chromium.
	// HEADLESS=false (paired with a virtual display like Xvfb on a server)
	// works around this without any fingerprint evasion.
	headless := os.Getenv("HEADLESS") != "false"
	return StartOptions{Headless: headless, UserAgent: defaultUserAgent, LoadSession: true}
}

// HeadedStartOptions returns start options for local, visible debugging.
func HeadedStartOptions() StartOptions {
	return StartOptions{Headless: false, SlowMoMs: 500, UserAgent: headedUserAgent, LoadSession: false}
}

// StatusResult is the outcome of CheckStatus. It never carries an error —
// any failure is reported via Reason, matching the Python check_status()
// contract of never raising.
type StatusResult struct {
	LoggedIn    bool
	ProfileName string
	Reason      string
}

// browserMu serializes Naukri automation across every user's Client: only
// one Chromium instance is ever alive at a time (see withBrowser), which is
// what keeps this bot usable on a 1GB Raspberry Pi with multiple users.
var browserMu sync.Mutex

// Client is an automation wrapper around a single Playwright page for one
// user's naukri.com profile. Exported action methods run inside withBrowser,
// which launches a fresh browser, loads this Client's session, runs the
// action, persists session state, and closes the browser again — so the
// browser only exists for the few seconds an action takes.
type Client struct {
	email      string
	password   string
	sessionDir string

	pw      *playwright.Playwright
	browser playwright.Browser
	context playwright.BrowserContext
	page    playwright.Page
}

// NewClient creates a Client. email/password are used for the initial login
// and for automatic re-login when a session expires; both may be empty if
// only a saved session (auth_state.json/cookies.json) will ever be used.
// sessionDir holds this user's session files and is created if missing.
func NewClient(email, password, sessionDir string) *Client {
	if sessionDir == "" {
		sessionDir = "."
	}
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		log.Printf("creating session dir %s: %v", sessionDir, err)
	}
	return &Client{email: email, password: password, sessionDir: sessionDir}
}

func (c *Client) authStatePath() string {
	return filepath.Join(c.sessionDir, authStateFile)
}

func (c *Client) cookiesPath() string {
	return filepath.Join(c.sessionDir, cookiesFile)
}

func (c *Client) debugScreenshotPath() string {
	return filepath.Join(c.sessionDir, debugScreenshot)
}

// withBrowser runs fn with a running browser available on c. If a browser is
// already running (cmd/save-session's long-lived dev workflow, started via
// Start/StartWithOptions), it's reused as-is and left running for the caller
// to Stop() explicitly. Otherwise withBrowser launches one, serialized on the
// package-level browserMu, and persists session state + closes it again once
// fn returns.
func (c *Client) withBrowser(fn func() error) error {
	browserMu.Lock()
	defer browserMu.Unlock()

	if c.browser != nil {
		return fn()
	}

	if err := c.start(defaultStartOptions()); err != nil {
		return err
	}
	defer func() {
		if err := c.persistSession(); err != nil {
			log.Printf("persisting session: %v", err)
		}
		if err := c.Stop(); err != nil {
			log.Printf("closing browser: %v", err)
		}
	}()

	return fn()
}

// WaitIdle blocks until no Naukri browser action is in flight. Used during
// shutdown so an in-flight action gets to persist its session and close its
// browser before the process exits.
func WaitIdle() {
	browserMu.Lock()
	browserMu.Unlock() //nolint:staticcheck // intentional lock/unlock to drain in-flight work
}

// Page and Context expose the underlying Playwright handles for dev tooling
// (cmd/save-session) that needs to drive the browser outside the normal
// exported-method surface, e.g. to capture cookies during a manual login.
func (c *Client) Page() playwright.Page              { return c.page }
func (c *Client) Context() playwright.BrowserContext { return c.context }

// Start launches a headless browser and loads a saved session. Called once
// at bot startup.
func (c *Client) Start() error {
	return c.start(defaultStartOptions())
}

// StartWithOptions launches the browser with custom options (see
// HeadedStartOptions for the dev-tooling case).
func (c *Client) StartWithOptions(opts StartOptions) error {
	return c.start(opts)
}

func (c *Client) start(opts StartOptions) error {
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("starting playwright: %w", err)
	}
	c.pw = pw

	launchOpts := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(opts.Headless),
		Args:     []string{"--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"},
		SlowMo:   playwright.Float(opts.SlowMoMs),
	}
	// CHROMIUM_PATH points at a system-installed browser on platforms with no
	// bundled Playwright Chromium build (e.g. 32-bit ARM / Raspberry Pi).
	if p := os.Getenv("CHROMIUM_PATH"); p != "" {
		launchOpts.ExecutablePath = playwright.String(p)
	}
	browser, err := pw.Chromium.Launch(launchOpts)
	if err != nil {
		return fmt.Errorf("launching chromium: %w", err)
	}
	c.browser = browser

	contextOpts := playwright.BrowserNewContextOptions{
		Viewport:  &defaultViewport,
		UserAgent: playwright.String(opts.UserAgent),
	}

	hasAuthState := opts.LoadSession && fileExists(c.authStatePath())
	hasCookies := opts.LoadSession && !hasAuthState && fileExists(c.cookiesPath())

	if hasAuthState {
		log.Println("Loading session from auth_state.json")
		contextOpts.StorageStatePath = playwright.String(c.authStatePath())
	}

	context, err := browser.NewContext(contextOpts)
	if err != nil {
		return fmt.Errorf("creating browser context: %w", err)
	}
	c.context = context

	if hasCookies {
		log.Println("Loading session from cookies.json")
		raw, err := os.ReadFile(c.cookiesPath())
		if err != nil {
			return fmt.Errorf("reading cookies.json: %w", err)
		}
		var cookies []playwright.OptionalCookie
		if err := json.Unmarshal(raw, &cookies); err != nil {
			return fmt.Errorf("parsing cookies.json: %w", err)
		}
		if err := context.AddCookies(cookies); err != nil {
			return fmt.Errorf("adding cookies: %w", err)
		}
	}

	if opts.LoadSession && !hasAuthState && !hasCookies {
		log.Println("No saved session found — will login with credentials")
	}

	page, err := context.NewPage()
	if err != nil {
		return fmt.Errorf("opening page: %w", err)
	}
	c.page = page

	if opts.LoadSession && !hasAuthState && !hasCookies {
		if c.email != "" && c.password != "" {
			if err := c.loginLocked(); err != nil {
				return err
			}
		} else {
			log.Println("No session file and no credentials in .env — bot will fail on first profile action.")
		}
	}

	return nil
}

// Stop closes the browser and resets c to a not-started state, so a
// subsequent withBrowser call knows to launch a fresh one.
func (c *Client) Stop() error {
	var errs []error
	if c.browser != nil {
		if err := c.browser.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing browser: %w", err))
		}
	}
	if c.pw != nil {
		if err := c.pw.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("stopping playwright: %w", err))
		}
	}
	c.pw, c.browser, c.context, c.page = nil, nil, nil, nil
	return errors.Join(errs...)
}

// Login logs in with email + password. Saves auth_state.json on success.
func (c *Client) Login() error {
	return c.withBrowser(c.loginLocked)
}

// loginLocked assumes a browser is already running (withBrowser guarantees
// this). Only call this directly from within another exported method (e.g.
// ensureLoggedIn) that's already running inside withBrowser.
func (c *Client) loginLocked() error {
	if c.email == "" || c.password == "" {
		return newAuthError("NAUKRI_EMAIL or NAUKRI_PASSWORD not set in .env")
	}

	log.Println("Logging in to Naukri with credentials")

	if _, err := c.page.Goto(naukriLoginURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(20_000),
	}); err != nil {
		return newTimeoutError(err, "Timed out loading Naukri login page")
	}
	if _, err := c.page.WaitForSelector(loginEmailField, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10_000),
	}); err != nil {
		return newTimeoutError(err, "Timed out loading Naukri login page")
	}

	if err := c.page.Fill(loginEmailField, c.email); err != nil {
		return fmt.Errorf("filling email field: %w", err)
	}
	if err := c.page.Fill(loginPasswordField, c.password); err != nil {
		return fmt.Errorf("filling password field: %w", err)
	}

	if err := c.page.Click(loginSubmitBtn); err != nil {
		return fmt.Errorf("clicking login submit: %w", err)
	}
	if err := c.page.WaitForURL("**/"+loginSuccessURL+"/**", playwright.PageWaitForURLOptions{
		Timeout: playwright.Float(20_000),
	}); err != nil {
		c.takeDebugScreenshot()
		return newAuthError("Login failed — check NAUKRI_EMAIL and NAUKRI_PASSWORD in .env")
	}

	c.closePopups()
	if err := c.persistSession(); err != nil {
		return err
	}
	log.Println("Login successful; session saved to auth_state.json")
	return nil
}

// CheckStatus reports session health. It never returns an error — any
// failure (including one launching the browser) is reported via
// StatusResult.Reason.
func (c *Client) CheckStatus() StatusResult {
	var result StatusResult
	if err := c.withBrowser(func() error {
		result = c.checkStatus()
		return nil
	}); err != nil {
		return StatusResult{Reason: err.Error()}
	}
	return result
}

func (c *Client) checkStatus() StatusResult {
	if _, err := c.page.Goto(profileURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(20_000),
	}); err != nil {
		if isTimeout(err) {
			return StatusResult{Reason: "timed out loading profile page"}
		}
		return StatusResult{Reason: err.Error()}
	}

	if strings.Contains(c.page.URL(), "nlogin") {
		return StatusResult{Reason: "redirected to login page"}
	}

	el, err := c.page.WaitForSelector(profileName, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10_000),
	})
	if err != nil {
		if isTimeout(err) {
			return StatusResult{Reason: "timed out loading profile page"}
		}
		return StatusResult{Reason: err.Error()}
	}

	name, err := el.InnerText()
	if err != nil {
		return StatusResult{Reason: err.Error()}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown"
	}
	return StatusResult{LoggedIn: true, ProfileName: name}
}

// UpdateHeadline updates the profile headline.
func (c *Client) UpdateHeadline(text string) error {
	return c.withBrowser(func() error {
		if err := c.ensureLoggedIn(); err != nil {
			return err
		}

		if _, err := c.page.WaitForSelector(headlineEditBtn, playwright.PageWaitForSelectorOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(10_000),
		}); err != nil {
			c.takeDebugScreenshot()
			return newSelectorError("Could not find headline edit button — Naukri UI may have changed")
		}
		if err := c.page.Click(headlineEditBtn); err != nil {
			return fmt.Errorf("clicking headline edit button: %w", err)
		}

		if _, err := c.page.WaitForSelector(headlineTextarea, playwright.PageWaitForSelectorOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(8_000),
		}); err != nil {
			c.takeDebugScreenshot()
			return newSelectorError("Headline input field did not appear")
		}
		if err := c.page.Fill(headlineTextarea, text); err != nil {
			return fmt.Errorf("filling headline textarea: %w", err)
		}

		return c.clickSave(headlineSaveBtn)
	})
}

// UpdateBio updates the profile summary / bio.
func (c *Client) UpdateBio(text string) error {
	return c.withBrowser(func() error {
		if err := c.ensureLoggedIn(); err != nil {
			return err
		}
		if err := c.openBioEditor(); err != nil {
			return err
		}
		if err := c.page.Fill(bioTextarea, text); err != nil {
			return fmt.Errorf("filling bio textarea: %w", err)
		}

		return c.clickSave(bioSaveBtn)
	})
}

// TouchBio makes a trivial, idempotent-safe edit to the bio (toggling a
// trailing full stop) so Naukri registers the profile as recently updated,
// without requiring a manual edit via Telegram.
func (c *Client) TouchBio() error {
	return c.withBrowser(func() error {
		if err := c.ensureLoggedIn(); err != nil {
			return err
		}
		if err := c.openBioEditor(); err != nil {
			return err
		}

		current, err := c.page.InputValue(bioTextarea)
		if err != nil {
			return fmt.Errorf("reading current bio text: %w", err)
		}
		if err := c.page.Fill(bioTextarea, toggleTrailingFullStop(current)); err != nil {
			return fmt.Errorf("filling bio textarea: %w", err)
		}

		return c.clickSave(bioSaveBtn)
	})
}

// openBioEditor scrolls the bio section into view, clicks its edit button,
// and waits for the textarea to appear. Assumes a browser is already running.
func (c *Client) openBioEditor() error {
	// Best-effort scroll so the edit button is in view; failure is not fatal.
	_ = c.page.Locator(bioSection).ScrollIntoViewIfNeeded()

	if _, err := c.page.WaitForSelector(bioEditBtn, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		// #lazyProfileSummary only mounts once scrolled into view; on
		// constrained ARM hardware (e.g. Raspberry Pi under xvfb-run) that can
		// take longer than the 10s once used here, especially on longer
		// profile pages where the widget sits further down. 25s gives that
		// headroom without slowing down the common case, since the wait
		// returns as soon as the element appears.
		Timeout: playwright.Float(25_000),
	}); err != nil {
		c.takeDebugScreenshot()
		return newSelectorError("Could not find bio edit button — Naukri UI may have changed")
	}
	if err := c.page.Click(bioEditBtn); err != nil {
		return fmt.Errorf("clicking bio edit button: %w", err)
	}

	if _, err := c.page.WaitForSelector(bioTextarea, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(8_000),
	}); err != nil {
		c.takeDebugScreenshot()
		return newSelectorError("Bio input field did not appear")
	}
	return nil
}

// AddSkill adds a skill to the skills section. Returns false if already present.
func (c *Client) AddSkill(skill string) (bool, error) {
	var added bool
	err := c.withBrowser(func() error {
		if err := c.ensureLoggedIn(); err != nil {
			return err
		}
		if err := c.openSkillsEditor(); err != nil {
			return err
		}

		existingCount, err := c.page.Locator(skillsChip(skill)).Count()
		if err != nil {
			return fmt.Errorf("counting existing skill chip: %w", err)
		}
		if existingCount > 0 {
			return nil // already exists
		}

		if err := c.page.Click(skillsInput); err != nil {
			return fmt.Errorf("clicking skills input: %w", err)
		}
		if err := c.page.Type(skillsInput, skill, playwright.PageTypeOptions{Delay: playwright.Float(80)}); err != nil {
			return fmt.Errorf("typing skill: %w", err)
		}

		dropdownSel := skillsDropdownItem(skill)
		if _, err := c.page.WaitForSelector(dropdownSel, playwright.PageWaitForSelectorOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5_000),
		}); err != nil {
			// No autocomplete match — try pressing Enter for free-text entry.
			if err := c.page.Keyboard().Press("Enter"); err != nil {
				return fmt.Errorf("pressing Enter for free-text skill entry: %w", err)
			}
			time.Sleep(300 * time.Millisecond)

			count, err := c.page.Locator(skillsChip(skill)).Count()
			if err != nil {
				return fmt.Errorf("checking skill chip after free-text entry: %w", err)
			}
			if count == 0 {
				return newSelectorError(
					"Skill '%s' not recognized by Naukri autocomplete. Try a slightly different name.", skill,
				)
			}
		} else if err := c.page.Click(dropdownSel); err != nil {
			return fmt.Errorf("clicking autocomplete suggestion: %w", err)
		}

		if err := c.clickSave(skillsSaveBtn); err != nil {
			return err
		}
		added = true
		return nil
	})
	return added, err
}

// RemoveSkill removes a skill from the skills section. Returns false if not found.
func (c *Client) RemoveSkill(skill string) (bool, error) {
	var removed bool
	err := c.withBrowser(func() error {
		if err := c.ensureLoggedIn(); err != nil {
			return err
		}
		if err := c.openSkillsEditor(); err != nil {
			return err
		}

		existingCount, err := c.page.Locator(skillsChip(skill)).Count()
		if err != nil {
			return fmt.Errorf("counting existing skill chip: %w", err)
		}
		if existingCount == 0 {
			return nil // not present
		}

		deleteSel := skillsChipDelete(skill)
		if _, err := c.page.WaitForSelector(deleteSel, playwright.PageWaitForSelectorOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5_000),
		}); err != nil {
			c.takeDebugScreenshot()
			return newSelectorError("Could not find delete button for skill '%s'", skill)
		}
		if err := c.page.Click(deleteSel); err != nil {
			return fmt.Errorf("clicking skill delete button: %w", err)
		}
		time.Sleep(300 * time.Millisecond)

		if err := c.clickSave(skillsSaveBtn); err != nil {
			return err
		}
		removed = true
		return nil
	})
	return removed, err
}

// ensureLoggedIn navigates to the profile page, re-logging in if the
// session has expired. Assumes a browser is already running.
func (c *Client) ensureLoggedIn() error {
	if _, err := c.page.Goto(profileURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(20_000),
	}); err != nil {
		return fmt.Errorf("navigating to profile page: %w", err)
	}

	if strings.Contains(c.page.URL(), "nlogin") {
		if c.email == "" || c.password == "" {
			return newSessionExpiredError("Session expired and no credentials available for re-login")
		}
		log.Println("Session expired; re-logging in with credentials")
		if err := c.loginLocked(); err != nil {
			return err
		}
		if _, err := c.page.Goto(profileURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   playwright.Float(20_000),
		}); err != nil {
			return fmt.Errorf("navigating to profile page after re-login: %w", err)
		}
	}

	c.closePopups()
	return nil
}

func (c *Client) openSkillsEditor() error {
	if _, err := c.page.WaitForSelector(skillsEditBtn, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10_000),
	}); err != nil {
		c.takeDebugScreenshot()
		return newSelectorError("Could not find skills edit button — Naukri UI may have changed")
	}
	if err := c.page.Click(skillsEditBtn); err != nil {
		return fmt.Errorf("clicking skills edit button: %w", err)
	}
	if _, err := c.page.WaitForSelector(skillsInput, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(8_000),
	}); err != nil {
		c.takeDebugScreenshot()
		return newSelectorError("Skills input field did not appear")
	}
	return nil
}

func (c *Client) clickSave(saveSelector string) error {
	if _, err := c.page.WaitForSelector(saveSelector, playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(8_000),
	}); err != nil {
		c.takeDebugScreenshot()
		return newSelectorError("Save button did not appear or click failed")
	}
	if err := c.page.Click(saveSelector); err != nil {
		c.takeDebugScreenshot()
		return newSelectorError("Save button did not appear or click failed")
	}
	// No better completion signal is available; wait for the modal to close.
	c.page.WaitForTimeout(1500)
	return nil
}

func (c *Client) closePopups() {
	for _, sel := range popupCloseSelectors {
		el, err := c.page.WaitForSelector(sel, playwright.PageWaitForSelectorOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(2_000),
		})
		if err != nil || el == nil {
			continue
		}
		if err := el.Click(); err == nil {
			c.page.WaitForTimeout(300)
		}
	}
}

func (c *Client) persistSession() error {
	if _, err := c.context.StorageState(playwright.BrowserContextStorageStateOptions{
		Path: playwright.String(c.authStatePath()),
	}); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}
	log.Println("Saved session to auth_state.json")
	return nil
}

func (c *Client) takeDebugScreenshot() {
	if _, err := c.page.Screenshot(playwright.PageScreenshotOptions{
		Path:     playwright.String(c.debugScreenshotPath()),
		FullPage: playwright.Bool(true),
	}); err != nil {
		log.Printf("failed to save debug screenshot: %v", err)
		return
	}
	log.Println("Debug screenshot saved to debug_last_error.png")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
