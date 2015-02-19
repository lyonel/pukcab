OS:=$(shell uname -s)
ARCH:=$(shell uname -m)
VERSION:=$(shell git describe --tags --long | cut -d - -f 1,2 | tr - .)

export GOPATH=${PWD}

.PHONY: pukcab clean update release

pukcab:
	go build -o $@
	strip $@

release: pukcab README.md
	zip pukcab-${VERSION}-${OS}-${ARCH}.zip $^
	git archive -o pukcab-${VERSION}.tar.gz --prefix pukcab-${VERSION}/ HEAD

rpm: pukcab-${VERSION}-${OS}-${ARCH}.zip
	rpmbuild -bb -D "%_rpmdir RPM" -D "%_sourcedir ${PWD}" -D "%_builddir ${PWD}/RPM/BUILD" -D "%_buildrootdir ${PWD}/RPM/BUILDROOT" -D "%VERSION "${VERSION} pukcab.spec

github:
	-git push -q git@github.com:/lyonel/pukcab.git
	-git push -q --tags git@github.com:/lyonel/pukcab.git

clean:
	go clean

update:
	go get -u github.com/mattn/go-sqlite3
	go get -u github.com/BurntSushi/toml
	go get -u github.com/antage/mntent
