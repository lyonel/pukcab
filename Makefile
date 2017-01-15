OS:=$(shell uname -s)
ARCH?=$(shell uname -m)
VERSION:=$(shell git describe --tags --long | cut -d - -f 1,2 | tr - .)
SHORTVERSION:=$(shell git describe --tags --long | cut -d - -f 1 | tr - .)
PANDOC:=pandoc -V title="Pukcab ${SHORTVERSION}" -V date="`date +%F`" --smart --toc --toc-depth=2

export GOPATH=${PWD}

.PHONY: pukcab clean update release doc godoc dependencies

.SUFFIXES: .md .pdf .html

bin/pukcab: dependencies
	@cd src/pukcab ; ../../bin/govend
	go install pukcab

bin/convert: dependencies
	@cd src/convert ; ../../bin/govend
	go install convert

bin/govend:
	go get https://github.com/govend/govend

dependencies: bin/govend

update: bin/govend
	@cd src/pukcab ; ../../bin/govend -t -v -u
	@cd src/convert ; ../../bin/govend -t -v -u

pukcab.exe:
	CC=i686-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=386 go build -tags windows,!linux,!freebsd,!darwin -o $@

release: pukcab-${VERSION}-${OS}-${ARCH}.zip pukcab-${VERSION}.tar.gz
	scp pukcab-${VERSION}.tar.gz pukcab-${VERSION}-${OS}-${ARCH}.zip www.internal:/var/www/html/software/files

i686 i386 386:
	$(MAKE) CGO_ENABLED=1 GOARCH=386 ARCH=i686 rpm

arm5 armv5 armv5l:
	$(MAKE) GOARM=5 ARCH=armv5l rpm

arm7 armv7 armv7l:
	$(MAKE) GOARM=7 ARCH=armv7l rpm

arm: arm5 arm7

pukcab-${VERSION}-${OS}-${ARCH}.zip: pukcab MANUAL.html
	zip $@ $^

tgz: pukcab-${VERSION}.tar.gz

pukcab-${VERSION}.tar.gz:
	git archive -o $@ --prefix pukcab-${VERSION}/ HEAD

rpm: pukcab-${VERSION}-${OS}-${ARCH}.zip
	rpmbuild -bb --target=${ARCH} -D "%arch ${ARCH}" -D "%_rpmdir RPM" -D "%_sourcedir ${PWD}" -D "%_builddir ${PWD}/RPM/BUILD" -D "%_buildrootdir ${PWD}/RPM/BUILDROOT" -D "%VERSION "${VERSION} pukcab.spec

srpm: tgz pukcab.spec.in
	sed -e s/@@VERSION@@/${VERSION}/g pukcab.spec.in > pukcab.spec
	rpmbuild -bs -D "%_srcrpmdir ${PWD}" -D "%_sourcedir ${PWD}" pukcab.spec

github:
	-git push -q git@github.com:/lyonel/pukcab.git
	-git push -q --tags git@github.com:/lyonel/pukcab.git

clean:
	go clean

.md.html: md.css
	${PANDOC} -t html5 --self-contained --css md.css -o $@ $<

.md.pdf:
	${PANDOC} -o $@ $<

doc: MANUAL.html MANUAL.pdf

copr: srpm
	scp pukcab-${VERSION}-*.src.rpm www.internal:/var/www/html/software/files
	copr build --nowait lyonel/Pukcab pukcab-${VERSION}-*.src.rpm
	copr build --nowait lyonel/ezIX pukcab-${VERSION}-*.src.rpm

godoc:
	godoc -http :6060 &
	open http://localhost:6060/pkg
