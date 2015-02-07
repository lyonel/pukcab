export GOPATH=${PWD}

.PHONY: pukcab clean update

pukcab:
	go build -o $@
	strip $@

clean:
	go clean

update:
	go get -u github.com/mattn/go-sqlite3
	go get -u github.com/BurntSushi/toml
	go get -u github.com/antage/mntent
