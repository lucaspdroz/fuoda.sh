APP := fuoda.sh
PKG := ./cmd/fuoda.sh

.PHONY: build install dev clean

build:
	CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o $(APP) $(PKG)

install: build
	mkdir -p $(HOME)/.local/bin
	install -m 0755 $(APP) $(HOME)/.local/bin/$(APP)

dev:
	go build -o $(APP) $(PKG)
	mkdir -p $(HOME)/.local/bin
	cp $(APP) $(HOME)/.local/bin/

clean:
	rm -f $(APP)
