// Command install-driver downloads only the Playwright driver, skipping the
// browser download. Used on platforms with no prebuilt Playwright Chromium
// build (e.g. 32-bit ARM / Raspberry Pi), where a system-installed browser
// (see CHROMIUM_PATH in internal/naukri/client.go) is used instead.
package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/mxschmitt/playwright-go"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	if err := playwright.Install(&playwright.RunOptions{SkipInstallBrowsers: true}); err != nil {
		log.Fatalf("could not install playwright driver: %v", err)
	}
}
