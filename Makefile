all: build run

build:
	CGO_ENABLED=0 GOOS=darwin go build -ldflags '-w -s'  -o  google_dns_darwin
	CGO_ENABLED=0 GOOS=linux go build -ldflags '-w -s'  -o  google_dns
	# CGO_ENABLED=0 GOOS=windows go build -ldflags '-w -s' -o google_dns.exe

clean:
	rm   google_dns_darwin
run:
	./google_dns_darwin

.PHONY: all build
