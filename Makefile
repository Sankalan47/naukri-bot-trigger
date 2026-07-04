.PHONY: build build-linux-arm64 build-linux-arm install-browsers test run run-save-session clean

build:
	go build -o bin/naukri-bot ./cmd/naukri-bot
	go build -o bin/save-session ./cmd/save-session

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o bin/naukri-bot-linux-arm64 ./cmd/naukri-bot

# For 32-bit ARM targets (e.g. Raspberry Pi 3B) where Playwright has no
# prebuilt Node.js/Chromium. Run install-driver-linux-arm once on the target
# to fetch the driver, then rely on CHROMIUM_PATH/PLAYWRIGHT_NODEJS_PATH to
# use system-installed binaries instead of Playwright's own downloads.
build-linux-arm:
	GOOS=linux GOARCH=arm GOARM=7 go build -o bin/naukri-bot-linux-arm ./cmd/naukri-bot
	GOOS=linux GOARCH=arm GOARM=7 go build -o bin/install-driver-linux-arm ./cmd/install-driver

install-browsers:
	go run github.com/mxschmitt/playwright-go/cmd/playwright install --with-deps chromium

test:
	go test ./...

run: build
	./bin/naukri-bot

run-save-session: build
	./bin/save-session

clean:
	rm -rf bin
