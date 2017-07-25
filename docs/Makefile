# Compiles all of the docs in docs/build.
all: build/slate
	cd build/slate && \
	cp -r ${CURDIR}/source/* source/ && \
	cp -r ${CURDIR}/*md source/ && \
	bundle exec middleman build --clean && \
	cp -r build/* ${CURDIR}/build/ && \
	echo "docs successfully compiled to HTML. To view docs, run:" && \
	echo "  open build/index.html"

build/slate:
	mkdir -p build && \
	cd build && \
	git clone https://github.com/quilt/slate.git && \
	cd slate && \
	bundle install
