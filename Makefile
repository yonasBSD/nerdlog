all: clean nerdlog

.PHONY: nerdlog
nerdlog:
	go build -o bin/nerdlog ./cmd/nerdlog-tui

.PHONY: clean
clean:
	rm -rf bin

.PHONY: install
install:
	sudo ln -sf $(PWD)/bin/nerdlog /usr/local/bin/nerdlog;
