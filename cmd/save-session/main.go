// Command save-session is the dev-only headed test tool, replacing
// test_local.py. It launches a visible, slowed-down browser and either
// captures a manual login into cookies.json (--save-cookies) or exercises
// the login/status/write-action flows directly against naukri.com.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/sankalan47/naukri-bot/internal/config"
	"github.com/sankalan47/naukri-bot/internal/naukri"
)

func main() {
	saveCookies := flag.Bool("save-cookies", false, "log in manually in a visible browser and save cookies.json")
	updateBio := flag.String("update-bio", "", "call UpdateBio with this text and exit")
	updateHeadline := flag.String("update-headline", "", "call UpdateHeadline with this text and exit")
	addSkill := flag.String("add-skill", "", "call AddSkill with this skill and exit")
	removeSkill := flag.String("remove-skill", "", "call RemoveSkill with this skill and exit")
	touchBio := flag.Bool("touch-bio", false, "call TouchBio (toggle trailing full stop) and exit")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	client := naukri.NewClient(cfg.NaukriEmail, cfg.NaukriPassword)
	if err := client.StartWithOptions(naukri.HeadedStartOptions()); err != nil {
		log.Fatalf("starting naukri client: %v", err)
	}

	if *saveCookies {
		runSaveCookies(client)
	} else {
		runDefault(client, *updateBio, *updateHeadline, *addSkill, *removeSkill, *touchBio)
	}

	waitForEnter("Press Enter to close browser...")
	if err := client.Stop(); err != nil {
		log.Printf("error stopping client: %v", err)
	}
}

func runSaveCookies(client *naukri.Client) {
	if _, err := client.Page().Goto("https://www.naukri.com/nlogin/login"); err != nil {
		log.Fatalf("navigating to login page: %v", err)
	}
	waitForEnter("Log in to Naukri in the browser, then press Enter here...")

	cookies, err := client.Context().Cookies()
	if err != nil {
		log.Fatalf("reading cookies: %v", err)
	}
	raw, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		log.Fatalf("marshaling cookies: %v", err)
	}
	if err := os.WriteFile("cookies.json", raw, 0o600); err != nil {
		log.Fatalf("writing cookies.json: %v", err)
	}
	fmt.Println("Saved cookies.json")
}

func runDefault(client *naukri.Client, updateBio, updateHeadline, addSkill, removeSkill string, touchBio bool) {
	fmt.Println("Logging in...")
	if err := client.Login(); err != nil {
		log.Fatalf("login failed: %v", err)
	}
	fmt.Println("Login OK")

	status := client.CheckStatus()
	fmt.Printf("Status: %+v\n", status)
	fmt.Println("Current URL:", client.Page().URL())

	if updateBio != "" {
		if err := client.UpdateBio(updateBio); err != nil {
			log.Fatalf("update bio failed: %v", err)
		}
		fmt.Println("Bio updated")
	}
	if updateHeadline != "" {
		if err := client.UpdateHeadline(updateHeadline); err != nil {
			log.Fatalf("update headline failed: %v", err)
		}
		fmt.Println("Headline updated")
	}
	if addSkill != "" {
		added, err := client.AddSkill(addSkill)
		if err != nil {
			log.Fatalf("add skill failed: %v", err)
		}
		fmt.Println("Added:", added)
	}
	if removeSkill != "" {
		removed, err := client.RemoveSkill(removeSkill)
		if err != nil {
			log.Fatalf("remove skill failed: %v", err)
		}
		fmt.Println("Removed:", removed)
	}
	if touchBio {
		if err := client.TouchBio(); err != nil {
			log.Fatalf("touch bio failed: %v", err)
		}
		fmt.Println("Bio touched (trailing full stop toggled)")
	}
}

func waitForEnter(prompt string) {
	fmt.Println(prompt)
	bufio.NewReader(os.Stdin).ReadString('\n')
}
