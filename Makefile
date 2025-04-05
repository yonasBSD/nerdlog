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

test:
	# The tests run rather slow so we use "-v -p 1" so that we get the unbuffered
	# output.
	go test ./... -count 1 -v -p 1
