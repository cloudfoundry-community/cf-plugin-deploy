build:
	go build .

cf:
	cf uninstall-plugin deploy || true
	yes | cf install-plugin cf-plugin-*
