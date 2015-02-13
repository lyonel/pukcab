OS:=$(shell uname -s)
ARCH:=$(shell uname -m)
VERSION:=$(shell git describe)

export GOPATH=${PWD}

.PHONY: pukcab clean update release

pukcab:
	go build -o $@
	strip $@

release: pukcab README.md
	zip pukcab-${VERSION}-${OS}-${ARCH}.zip $^
	-git push -q git@github.com:/lyonel/pukcab.git
	-git push -q --tags git@github.com:/lyonel/pukcab.git

clean:
	go clean

update:
	go get -u github.com/mattn/go-sqlite3
	go get -u github.com/BurntSushi/toml
	go get -u github.com/antage/mntent
