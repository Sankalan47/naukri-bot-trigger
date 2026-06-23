import asyncio
import json
import logging
import os
from pathlib import Path

from playwright.async_api import (
    BrowserContext,
    Page,
    async_playwright,
    TimeoutError as PWTimeout,
)

import naukri_selectors as S

logger = logging.getLogger(__name__)

AUTH_STATE_FILE = Path("auth_state.json")
COOKIES_FILE = Path("cookies.json")
DEBUG_SCREENSHOT = Path("debug_last_error.png")

NAUKRI_LOGIN_URL = "https://www.naukri.com/nlogin/login"


# --- Custom exceptions ---

class NaukriError(Exception):
    pass

class NaukriAuthError(NaukriError):
    pass

class NaukriSessionExpiredError(NaukriError):
    pass

class NaukriSelectorError(NaukriError):
    pass

class NaukriTimeoutError(NaukriError):
    pass


# --- Main class ---

class NaukriBot:
    """Async Playwright wrapper for all Naukri.com profile interactions."""

    def __init__(self):
        self._playwright = None
        self._browser = None
        self._context: BrowserContext | None = None
        self._page: Page | None = None

    async def start(self) -> None:
        """Launch browser and load session. Called once at bot startup."""
        self._playwright = await async_playwright().start()
        self._browser = await self._playwright.chromium.launch(
            headless=True,
            args=["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],
        )

        user_agent = (
            "Mozilla/5.0 (X11; Linux x86_64) "
            "AppleWebKit/537.36 (KHTML, like Gecko) "
            "Chrome/120.0.0.0 Safari/537.36"
        )
        viewport = {"width": 1280, "height": 800}

        if AUTH_STATE_FILE.exists():
            logger.info("Loading session from auth_state.json")
            self._context = await self._browser.new_context(
                storage_state=str(AUTH_STATE_FILE),
                viewport=viewport,
                user_agent=user_agent,
            )
        elif COOKIES_FILE.exists():
            logger.info("Loading session from cookies.json")
            self._context = await self._browser.new_context(
                viewport=viewport, user_agent=user_agent
            )
            raw_cookies = json.loads(COOKIES_FILE.read_text())
            await self._context.add_cookies(raw_cookies)
        else:
            logger.info("No saved session found — will login with credentials")
            self._context = await self._browser.new_context(
                viewport=viewport, user_agent=user_agent
            )

        self._page = await self._context.new_page()

        email = os.environ.get("NAUKRI_EMAIL")
        password = os.environ.get("NAUKRI_PASSWORD")
        if not AUTH_STATE_FILE.exists() and not COOKIES_FILE.exists():
            if email and password:
                await self.login()
            else:
                logger.warning(
                    "No session file and no credentials in .env — "
                    "bot will fail on first profile action."
                )

    async def stop(self) -> None:
        """Gracefully close browser. Called on bot shutdown."""
        if self._browser:
            await self._browser.close()
        if self._playwright:
            await self._playwright.stop()

    async def login(self) -> bool:
        """Log in with email + password. Saves auth_state.json on success."""
        email = os.environ.get("NAUKRI_EMAIL", "")
        password = os.environ.get("NAUKRI_PASSWORD", "")
        if not email or not password:
            raise NaukriAuthError("NAUKRI_EMAIL or NAUKRI_PASSWORD not set in .env")

        logger.info("Logging in to Naukri with credentials")
        try:
            await self._page.goto(NAUKRI_LOGIN_URL, wait_until="domcontentloaded", timeout=20_000)
            await self._page.wait_for_selector(S.LOGIN_EMAIL_FIELD, state="visible", timeout=10_000)
        except PWTimeout:
            raise NaukriTimeoutError("Timed out loading Naukri login page")

        await self._page.fill(S.LOGIN_EMAIL_FIELD, email)
        await self._page.fill(S.LOGIN_PASSWORD_FIELD, password)

        try:
            await self._page.click(S.LOGIN_SUBMIT_BTN)
            await self._page.wait_for_url(f"**/{S.LOGIN_SUCCESS_URL}/**", timeout=20_000)
        except PWTimeout:
            await self._take_debug_screenshot()
            raise NaukriAuthError("Login failed — check NAUKRI_EMAIL and NAUKRI_PASSWORD in .env")

        await self._close_popups()
        await self._save_session()
        logger.info("Login successful; session saved to auth_state.json")
        return True

    async def check_status(self) -> dict:
        """Return session health as a dict with 'logged_in' and either 'profile_name' or 'reason'."""
        try:
            await self._page.goto(S.PROFILE_URL, wait_until="domcontentloaded", timeout=20_000)
            if "nlogin" in self._page.url:
                return {"logged_in": False, "reason": "redirected to login page"}

            name_el = await self._page.wait_for_selector(S.PROFILE_NAME, timeout=10_000)
            name = (await name_el.inner_text()).strip()
            return {"logged_in": True, "profile_name": name or "unknown"}
        except PWTimeout:
            return {"logged_in": False, "reason": "timed out loading profile page"}
        except Exception as e:
            return {"logged_in": False, "reason": str(e)}

    async def update_headline(self, text: str) -> bool:
        """Update the profile headline."""
        await self._ensure_logged_in()
        try:
            await self._page.wait_for_selector(S.HEADLINE_EDIT_BTN, state="visible", timeout=10_000)
        except PWTimeout:
            await self._take_debug_screenshot()
            raise NaukriSelectorError("Could not find headline edit button — Naukri UI may have changed")

        await self._page.click(S.HEADLINE_EDIT_BTN)

        try:
            await self._page.wait_for_selector(S.HEADLINE_TEXTAREA, state="visible", timeout=8_000)
        except PWTimeout:
            await self._take_debug_screenshot()
            raise NaukriSelectorError("Headline input field did not appear")

        await self._page.fill(S.HEADLINE_TEXTAREA, text)
        await self._click_save(S.HEADLINE_SAVE_BTN)
        return True

    async def update_bio(self, text: str) -> bool:
        """Update the profile summary / bio."""
        await self._ensure_logged_in()

        try:
            await self._page.evaluate(
                "el => el && el.scrollIntoView()",
                await self._page.query_selector(S.BIO_SECTION),
            )
        except Exception:
            pass

        try:
            await self._page.wait_for_selector(S.BIO_EDIT_BTN, state="visible", timeout=10_000)
        except PWTimeout:
            await self._take_debug_screenshot()
            raise NaukriSelectorError("Could not find bio edit button — Naukri UI may have changed")

        await self._page.click(S.BIO_EDIT_BTN)

        try:
            await self._page.wait_for_selector(S.BIO_TEXTAREA, state="visible", timeout=8_000)
        except PWTimeout:
            await self._take_debug_screenshot()
            raise NaukriSelectorError("Bio input field did not appear")

        await self._page.fill(S.BIO_TEXTAREA, text)
        await self._click_save(S.BIO_SAVE_BTN)
        return True

    async def add_skill(self, skill: str) -> bool:
        """Add a skill to the skills section. Returns False if already present."""
        await self._ensure_logged_in()
        await self._open_skills_editor()

        existing = self._page.locator(S.skills_chip(skill))
        if await existing.count() > 0:
            return False  # already exists

        await self._page.click(S.SKILLS_INPUT)
        await self._page.type(S.SKILLS_INPUT, skill, delay=80)

        dropdown_sel = S.skills_dropdown_item(skill)
        try:
            await self._page.wait_for_selector(dropdown_sel, state="visible", timeout=5_000)
            await self._page.click(dropdown_sel)
        except PWTimeout:
            # No autocomplete match — try pressing Enter for free-text entry
            await self._page.keyboard.press("Enter")
            await asyncio.sleep(0.3)
            # Verify chip appeared; if not, skill was rejected
            if await self._page.locator(S.skills_chip(skill)).count() == 0:
                raise NaukriSelectorError(
                    f"Skill '{skill}' not recognized by Naukri autocomplete. "
                    "Try a slightly different name."
                )

        await self._click_save(S.SKILLS_SAVE_BTN)
        return True

    async def remove_skill(self, skill: str) -> bool:
        """Remove a skill from the skills section. Returns False if not found."""
        await self._ensure_logged_in()
        await self._open_skills_editor()

        existing = self._page.locator(S.skills_chip(skill))
        if await existing.count() == 0:
            return False  # skill not present

        delete_sel = S.skills_chip_delete(skill)
        try:
            await self._page.wait_for_selector(delete_sel, state="visible", timeout=5_000)
            await self._page.click(delete_sel)
            await asyncio.sleep(0.3)
        except PWTimeout:
            await self._take_debug_screenshot()
            raise NaukriSelectorError(
                f"Could not find delete button for skill '{skill}'"
            )

        await self._click_save(S.SKILLS_SAVE_BTN)
        return True

    # --- Internal helpers ---

    async def _ensure_logged_in(self) -> None:
        """Navigate to profile page; re-login if session has expired."""
        await self._page.goto(S.PROFILE_URL, wait_until="domcontentloaded", timeout=20_000)
        if "nlogin" in self._page.url:
            email = os.environ.get("NAUKRI_EMAIL")
            password = os.environ.get("NAUKRI_PASSWORD")
            if email and password:
                logger.info("Session expired; re-logging in with credentials")
                await self.login()
                await self._page.goto(S.PROFILE_URL, wait_until="domcontentloaded", timeout=20_000)
            else:
                raise NaukriSessionExpiredError(
                    "Session expired and no credentials available for re-login"
                )
        await self._close_popups()

    async def _open_skills_editor(self) -> None:
        try:
            await self._page.wait_for_selector(S.SKILLS_EDIT_BTN, state="visible", timeout=10_000)
        except PWTimeout:
            await self._take_debug_screenshot()
            raise NaukriSelectorError("Could not find skills edit button — Naukri UI may have changed")
        await self._page.click(S.SKILLS_EDIT_BTN)
        try:
            await self._page.wait_for_selector(S.SKILLS_INPUT, state="visible", timeout=8_000)
        except PWTimeout:
            await self._take_debug_screenshot()
            raise NaukriSelectorError("Skills input field did not appear")

    async def _click_save(self, save_selector: str) -> None:
        try:
            await self._page.wait_for_selector(save_selector, state="visible", timeout=8_000)
            await self._page.click(save_selector)
            # Wait for the modal to close or a success indicator
            await self._page.wait_for_timeout(1500)
        except PWTimeout:
            await self._take_debug_screenshot()
            raise NaukriSelectorError("Save button did not appear or click failed")

    async def _close_popups(self) -> None:
        for sel in S.POPUP_CLOSE_SELECTORS:
            try:
                btn = await self._page.wait_for_selector(sel, state="visible", timeout=2_000)
                if btn:
                    await btn.click()
                    await self._page.wait_for_timeout(300)
            except PWTimeout:
                pass

    async def _save_session(self) -> None:
        await self._context.storage_state(path=str(AUTH_STATE_FILE))
        logger.info("Saved session to auth_state.json")

    async def _take_debug_screenshot(self) -> None:
        try:
            await self._page.screenshot(path=str(DEBUG_SCREENSHOT))
            logger.info("Debug screenshot saved to %s", DEBUG_SCREENSHOT)
        except Exception:
            pass
