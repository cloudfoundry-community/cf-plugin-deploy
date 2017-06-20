GO_LDFLAGS := -ldflags="-X main.Version=$(VERSION)"

build:
	go build $(GO_LDFLAGS) .

cf:
	cf uninstall-plugin deploy || true
	yes | cf install-plugin cf-plugin-*

bootstrap:
	spruce merge --prune meta examples/bootstrap.yml > examples/.bootstrap.yml
	cf deploy < examples/.bootstrap.yml
	rm examples/.bootstrap.yml

cups:
	spruce merge --prune meta examples/cups.yml > examples/.cups.yml
	cf deploy < examples/.cups.yml
	rm examples/.cups.yml

it: build cf
