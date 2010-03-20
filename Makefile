include $(GOROOT)/src/Make.$(GOARCH)

TARBALL=/tmp/nonewcontent.tar.gz

all: nonewcontent

test: nonewcontent
	./nonewcontent -debug=4

%.$O: %.go 
	${GC} $<

nonewcontent: nonewcontent.$O
	${LD} -o $@ nonewcontent.$O

dist: distclean
	(cd ..; tar czvf ${TARBALL} nonewcontent/*)

clean:
	rm -f nonewcontent *.$O *.a

distclean: clean
	rm -f *~ 