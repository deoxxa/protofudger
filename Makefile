all: protofudger

protofudger: decode.go main_native.go
	go build

protofudger.js: decode.go main_js.go
	gopherjs build -m -o protofudger.js

clean:
	rm -f protofudger
	rm -f protofudger.js protofudger.js.map

.PHONY: all clean
