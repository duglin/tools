all: build test

build: tester

tester: main/tester.go
	go build -o tester main/tester.go

test: tester
	./tester

clean:
	rm -f tester
