package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

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
	  "xxx": { "yyy": 1 , "zzz": "zoom"}
	}`

	v := struct {
		F1     string `json:"f1"`
		F2     string `json:"f2"`
		F3     int    `json:"F3"`
		Nested struct {
			N1      string                 `json:"n1"`
			N2      string                 `json:"n2"`
			NN      int                    `json:"nn"`
			EExtras map[string]interface{} `json:",exts"`
			// Eextras map[string]interface{} `json:",exts"`
		} `json:"nested"`
		Extras map[string]interface{} `json:",exts"`
		// Eextras map[string]interface{} `json:",exts"`
	}{}

	if err := jsonext.Unmarshal([]byte(myJson), &v); err != nil {
		fmt.Printf("Err: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Original:\n%v\n\n", myJson)

	b, err := json.MarshalIndent(v, "", "  ")
	fmt.Printf("Parsed(%s):\n%v\n", err, string(b))

	b1, err := jsonext.Marshal(v)
	fmt.Printf("\nAs Ext No Indent(%s):\n%v\n", err, string(b1))

	b2, err := jsonext.MarshalIndent(v, "", "  ")
	fmt.Printf("\nAs Ext(%s):\n%v\n", err, string(b2))

	// Now verify the results
	originMap := map[string]interface{}{}
	afterMap1 := map[string]interface{}{}
	afterMap2 := map[string]interface{}{}

	json.Unmarshal([]byte(myJson), &originMap)
	json.Unmarshal([]byte(b1), &afterMap1)
	json.Unmarshal([]byte(b2), &afterMap2)

	rc := 0

	if !reflect.DeepEqual(originMap, afterMap1) {
		fmt.Printf("Original and No Indent do not match!\n")
		fmt.Printf("old: %#v\n", originMap)
		fmt.Printf("new: %#v\n", afterMap1)
		rc = 1
	} else {
		fmt.Printf("Test1: PASS\n")
	}

	if !reflect.DeepEqual(originMap, afterMap2) {
		fmt.Printf("Original and Indent do not match!\n")
		rc = 1
	} else {
		fmt.Printf("Test2: PASS\n")
	}

	// Test 3
	t3json := `{ "f1": "value1" }`
	t3v1 := struct {
		Extras map[string]interface{} `json:",exts"`
	}{}
	t3v2 := struct {
		F1     string                 `json:"f1"`
		Extras map[string]interface{} `json:",exts"`
	}{}

	jsonext.Unmarshal([]byte(t3json), &t3v1)
	jsonext.Unmarshal([]byte(t3json), &t3v2)

	r1, e1 := jsonext.StructGet(t3v1, "f1")
	r2, e2 := jsonext.StructGet(t3v1, "f1")

	if r1 == nil || r1 != r2 {
		fmt.Printf("Values don't match: r1(%v, e1) r2(%v,e2)\n", r1, e1, r2, e2)
		rc = 1
	} else {
		fmt.Printf("Test3: PASS\n")
	}

	os.Exit(rc)
}
