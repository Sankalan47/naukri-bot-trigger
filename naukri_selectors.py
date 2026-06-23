# Naukri.com DOM selectors — update this file when Naukri ships a new frontend.
# Validate all selectors with: playwright codegen https://www.naukri.com/mnjuser/profile

# --- Login page (https://www.naukri.com/nlogin/login) ---
LOGIN_EMAIL_FIELD    = "#usernameField"
LOGIN_PASSWORD_FIELD = "#passwordField"
LOGIN_SUBMIT_BTN     = "//button[@type='submit' and contains(normalize-space(),'Login')]"
LOGIN_SUCCESS_URL    = "mnjuser"  # URL fragment present when logged in

# --- Post-login overlays / popups ---
POPUP_CLOSE_SELECTORS = [
    "//span[contains(@class,'crossIcon') or contains(@class,'cross-icon')]",
    "//button[contains(@class,'crossIcon') or contains(@class,'cross-icon')]",
    "//button[normalize-space()='Got it']",
    "//button[normalize-space()='Maybe later']",
    "//button[normalize-space()='SKIP AND CONTINUE']",
    "//div[contains(@class,'naukri-modal')]//button[contains(@class,'close')]",
]

# --- Profile page ---
PROFILE_URL = "https://www.naukri.com/mnjuser/profile"
PROFILE_NAME = "//div[contains(@class,'name')] | //h1[contains(@class,'name')]"

# --- Headline section ---
HEADLINE_EDIT_BTN = "//div[contains(@class,'resumeHeadlineEdit')]//span[contains(@class,'edit') or @title='Edit']"
HEADLINE_TEXTAREA = "#resumeHeadlineTxt"
HEADLINE_SAVE_BTN = "//div[contains(@class,'resumeHeadlineEdit')]//button[@type='submit']"

# --- Profile summary / bio section ---
BIO_SECTION       = ".widgetCont"
BIO_EDIT_BTN      = "//span[contains(@class,'widgetTitle') and contains(normalize-space(),'rofile summary')]/following-sibling::span[contains(@class,'edit') and contains(@class,'icon')]"
BIO_TEXTAREA      = "#profileSummaryTxt"
BIO_SAVE_BTN      = "//button[normalize-space()='Save']"

# --- Skills section ---
SKILLS_EDIT_BTN      = "//div[contains(@class,'keySkillArea')]//span[contains(@class,'edit') or @title='Edit']"
SKILLS_INPUT         = "//div[contains(@class,'keySkillArea')]//input[@type='text']"
SKILLS_SAVE_BTN      = "//div[contains(@class,'keySkillArea')]//button[@type='submit']"


def skills_dropdown_item(skill: str) -> str:
    """XPath for an autocomplete suggestion matching the given skill name."""
    return f"//ul[contains(@class,'suggesstedSkill') or contains(@class,'autocomplete')]//li[normalize-space()='{skill}']"


def skills_chip(skill: str) -> str:
    """XPath for an existing skill chip/tag in the skills editor."""
    return (
        f"//div[contains(@class,'keySkillArea')]"
        f"//span[contains(@class,'chip') or contains(@class,'tag') or contains(@class,'skill')]"
        f"[normalize-space()='{skill}']"
    )


def skills_chip_delete(skill: str) -> str:
    """XPath for the delete button on a specific skill chip."""
    return (
        f"//div[contains(@class,'keySkillArea')]"
        f"//*[normalize-space()='{skill}']"
        f"/following-sibling::*[contains(@class,'delete') or contains(@class,'remove') or contains(@class,'cross')]"
        f" | "
        f"//div[contains(@class,'keySkillArea')]"
        f"//*[normalize-space()='{skill}']"
        f"/..//*[contains(@class,'delete') or contains(@class,'remove') or @aria-label='remove' or @title='Remove']"
    )
