// Package naukri wraps browser automation against naukri.com profile pages.
//
// Naukri.com DOM selectors — update this file when Naukri ships a new frontend.
// Validate all selectors with: playwright codegen https://www.naukri.com/mnjuser/profile
package naukri

import "fmt"

// Login page (https://www.naukri.com/nlogin/login)
const (
	loginEmailField    = "#usernameField"
	loginPasswordField = "#passwordField"
	loginSubmitBtn     = "//button[@type='submit' and contains(normalize-space(),'Login')]"
	loginSuccessURL    = "mnjuser" // URL fragment present when logged in
)

// Post-login overlays / popups.
var popupCloseSelectors = []string{
	"//span[contains(@class,'crossIcon') or contains(@class,'cross-icon')]",
	"//button[contains(@class,'crossIcon') or contains(@class,'cross-icon')]",
	"//button[normalize-space()='Got it']",
	"//button[normalize-space()='Maybe later']",
	"//button[normalize-space()='SKIP AND CONTINUE']",
	"//div[contains(@class,'naukri-modal')]//button[contains(@class,'close')]",
}

// Profile page.
const (
	profileURL  = "https://www.naukri.com/mnjuser/profile"
	profileName = "//div[contains(@class,'name')] | //h1[contains(@class,'name')]"
)

// Headline section.
const (
	headlineEditBtn  = "//div[contains(@class,'resumeHeadlineEdit')]//span[contains(@class,'edit') or @title='Edit']"
	headlineTextarea = "#resumeHeadlineTxt"
	headlineSaveBtn  = "//div[contains(@class,'resumeHeadlineEdit')]//button[@type='submit']"
)

// Profile summary / bio section. The widget lazy-loads (data-plugin="lazyload")
// only once scrolled into view, so bioSection must be a selector unique to
// this one widget — ".widgetCont" matches ~10 widgets on the page, and
// Locator.ScrollIntoViewIfNeeded() silently no-ops (strict-mode violation) on
// an ambiguous locator, so the widget (and bioEditBtn within it) never mounts.
const (
	bioSection  = "#lazyProfileSummary"
	bioEditBtn  = "//span[contains(@class,'widgetTitle') and contains(normalize-space(),'rofile summary')]/following-sibling::span[contains(@class,'edit') and contains(@class,'icon')]"
	bioTextarea = "#profileSummaryTxt"
	bioSaveBtn  = "//button[normalize-space()='Save']"
)

// Skills section.
const (
	skillsEditBtn = "//div[contains(@class,'keySkillArea')]//span[contains(@class,'edit') or @title='Edit']"
	skillsInput   = "//div[contains(@class,'keySkillArea')]//input[@type='text']"
	skillsSaveBtn = "//div[contains(@class,'keySkillArea')]//button[@type='submit']"
)

// skillsDropdownItem returns the XPath for an autocomplete suggestion matching the given skill name.
func skillsDropdownItem(skill string) string {
	return fmt.Sprintf("//ul[contains(@class,'suggesstedSkill') or contains(@class,'autocomplete')]//li[normalize-space()='%s']", skill)
}

// skillsChip returns the XPath for an existing skill chip/tag in the skills editor.
func skillsChip(skill string) string {
	return fmt.Sprintf(
		"//div[contains(@class,'keySkillArea')]"+
			"//span[contains(@class,'chip') or contains(@class,'tag') or contains(@class,'skill')]"+
			"[normalize-space()='%s']",
		skill,
	)
}

// skillsChipDelete returns the XPath for the delete button on a specific skill chip.
func skillsChipDelete(skill string) string {
	return fmt.Sprintf(
		"//div[contains(@class,'keySkillArea')]"+
			"//*[normalize-space()='%s']"+
			"/following-sibling::*[contains(@class,'delete') or contains(@class,'remove') or contains(@class,'cross')]"+
			" | "+
			"//div[contains(@class,'keySkillArea')]"+
			"//*[normalize-space()='%s']"+
			"/..//*[contains(@class,'delete') or contains(@class,'remove') or @aria-label='remove' or @title='Remove']",
		skill, skill,
	)
}
