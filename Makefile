OS:=$(shell uname -s)
ARCH?=$(shell uname -m)
VERSION:=$(shell git describe --tags --long | cut -d - -f 1,2 | tr - .)

export GOPATH=${PWD}

.PHONY: pukcab clean update release

.SUFFIXES: .md .pdf .html

pukcab:
	go build -o $@
	strip $@

pukcab.exe:
	CC=i686-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=386 go build -tags windows,!linux,!freebsd,!darwin -o $@

release: pukcab-${VERSION}-${OS}-${ARCH}.zip pukcab-${VERSION}.tar.gz

i686 i386 386:
	$(MAKE) CGO_ENABLED=1 GOARCH=386 ARCH=i686 rpm


pukcab-${VERSION}-${OS}-${ARCH}.zip: pukcab README.md
	zip $@ $^

pukcab-${VERSION}.tar.gz:
	git archive -o $@ --prefix pukcab-${VERSION}/ HEAD

rpm: pukcab-${VERSION}-${OS}-${ARCH}.zip
	rpmbuild -bb --target=${ARCH} -D "%arch ${ARCH}" -D "%_rpmdir RPM" -D "%_sourcedir ${PWD}" -D "%_builddir ${PWD}/RPM/BUILD" -D "%_buildrootdir ${PWD}/RPM/BUILDROOT" -D "%VERSION "${VERSION} pukcab.spec

github:
	-git push -q git@github.com:/lyonel/pukcab.git
	-git push -q --tags git@github.com:/lyonel/pukcab.git

clean:
	go clean

update:
	git submodule update --init --recursive
	cd src ; go install github.com/*/*

.md.html: md.css
	pandoc -t html5 --self-contained --css md.css -o $@ $<

.md.pdf:
	pandoc -V version="${VERSION}" --smart --chapters --toc --toc-depth=2 -o $@ $<

