APP := fuoda.sh
PKG := ./cmd/fuoda.sh
UNAME_S := $(shell uname -s)

.PHONY: build install dev clean

build:
	CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o $(APP) $(PKG)

install: build
ifeq ($(UNAME_S),Darwin)
	sudo mkdir -p /usr/local/bin
	sudo install -m 0755 $(APP) /usr/local/bin/$(APP)
	sudo codesign -s - -f /usr/local/bin/$(APP)
else
	mkdir -p $(HOME)/.local/bin
	install -m 0755 $(APP) $(HOME)/.local/bin/$(APP)
endif

dev:
	go build -o $(APP) $(PKG)
ifeq ($(UNAME_S),Darwin)
	sudo mkdir -p /usr/local/bin
	sudo cp $(APP) /usr/local/bin/
	sudo codesign -s - -f /usr/local/bin/$(APP)
else
	mkdir -p $(HOME)/.local/bin
	cp $(APP) $(HOME)/.local/bin/
endif

clean:
	rm -f $(APP)