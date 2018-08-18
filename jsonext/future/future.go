package main

import (
	".."
	"fmt"
	"os"
)

func main() {
	// Create a v1 and v2 struct for Person. In the v1 case we don't
	// know about Address so it appears as an extension
	personv1 := struct {
		Name   string
		Extras map[string]interface{} `json:",exts"`
	}{}

	// But in the v2 case Address is now a well-known property.
	personv2 := struct {
		Name    string
		Address string
		Extras  map[string]interface{} `json:",exts"`
	}{}

	// The "Person" JSON that we'll parse into both versions of the struct
	json := []byte(`{
		"Name": "john",
		"Address": "123 main street"
	}`)

	// Parse our JSON into both the v1 and v2 structs
	jsonext.Unmarshal(json, &personv1)
	jsonext.Unmarshal(json, &personv2)

	// Now, since we want to find Address whether its well-defined or
	// an extension, use `StructGet` for both v1 and v2 structs.
	// This allows us to future-proof our code - if we want.
	address1, _ := jsonext.StructGet(personv1, "Address")
	address2, _ := jsonext.StructGet(personv2, "Address")

	fmt.Printf("Address1: %s\n", address1)
	fmt.Printf("Address2: %s\n", address2)

	if address1 != address2 {
		fmt.Printf("Something went really wrong!\n")
		os.Exit(1)
	}
}
