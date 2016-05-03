GO_LDFLAGS := -ldflags="-X main.Version=$(VERSION)"

build:
	go build $(GO_LDFLAGS) .

cf:
	cf uninstall-plugin deploy || true
	yes | cf install-plugin cf-plugin-*


it: build cf
