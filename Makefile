TARBALL=/tmp/nonewcontent.tar.gz

all: nonewcontent

test: nonewcontent
	./nonewcontent 

%: %.go 
	go build $<

dist: distclean
	(cd ..; tar czvf ${TARBALL} nonewcontent/*)

clean:
	rm -f nonewcontent

distclean: clean
	rm -f *~ 