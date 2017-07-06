all: jsonMod

jsonMod: jsonMod.go
	go build -tags netgo -installsuffix netgo -o jsonMod jsonMod.go

clean:
	rm -rf jsonMod
