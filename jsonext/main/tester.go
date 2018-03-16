package main

import (
	"encoding/json"
	"fmt"
	"os"

	".."
)

func main() {
	myJson := `
	{ "f1": "value1",
	  "f2": "value2",
	  "F3": 42,
	  "yyy": "xxx",
	  "nested": {
		"n1": "nn1",
		"n2": "nn2",
		"nn": 99,
		"ee": "more"
	  },
	  "xxx": { "yyy": 1 }
	}`

	v := struct {
		F1     string `json:"f1"`
		F2     string
		F3     int
		Nested struct {
			N1      string
			N2      string
			NN      int
			EExtras map[string]interface{} `json:",exts"`
			// Eextras map[string]interface{} `json:",exts"`
		}
		Extras map[string]interface{} `json:",exts"`
		// Eextras map[string]interface{} `json:",exts"`
	}{}

	if err := jsonext.Unmarshal([]byte(myJson), &v); err != nil {
		fmt.Printf("Err: %v\n", err)
		os.Exit(1)
	}

	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Printf("Original:\n%v\n\nParsed:\n%v\n", myJson, string(b))
}
