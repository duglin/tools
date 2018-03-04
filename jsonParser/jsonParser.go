package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"unicode"
)

func Unmarshal(jsonStr []byte, obj interface{}) error {
	objValue := reflect.ValueOf(obj)

	// If its a pointer, dereference it so we can check its real type
	if objValue.Type().Kind() == reflect.Ptr {
		objValue = objValue.Elem()
	}

	// If its not a struct then just do normal JSON parsing
	if objValue.Type().Kind() != reflect.Struct {
		return json.Unmarshal(jsonStr, obj)
	}

	rawMap := map[string]json.RawMessage{}
	knownFields := map[string]reflect.Value{}
	var extensions map[string]interface{} = nil

	// Look for the "extension" property
	for i := 0; i != objValue.NumField(); i++ {
		field := objValue.Type().Field(i)

		// Skip non-exported properties
		if unicode.IsLower(rune(field.Name[0])) {
			continue
		}

		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		// If not custom name, then just use the property name itself
		if jsonName == "" {
			jsonName = field.Name
		}

		// If they've defined an "extension" property, save it
		if strings.Contains(field.Tag.Get("json"), ",exts") {
			// Can't define two extension properties
			if extensions != nil {
				return fmt.Errorf("Duplicate extension property (%s) defined",
					objValue.Type().Field(i).Name)
			}

			// Create a new map to hold our extension
			newV := reflect.ValueOf(map[string]interface{}{})

			// Verify the extension property is of the correct type
			if !objValue.Field(i).Type().ConvertibleTo(newV.Type()) {
				return fmt.Errorf("JSON Extension field %q must be a %s not %s",
					objValue.Type().Field(i).Name,
					newV.Type().String(),
					objValue.Field(i).Type().String())
			}

			// Override any existing map with our new one
			objValue.Field(i).Set(newV)

			// Save a ref to our map so we can populate it later
			extensions, _ = newV.Interface().(map[string]interface{})
		}
		knownFields[strings.ToLower(jsonName)] = objValue.Field(i)
	}

	// Lazy parse the json
	if err := json.Unmarshal(jsonStr, &rawMap); err != nil {
		return err
	}

	// For each property in the json, put it in the right spot
	for key, val := range rawMap {
		if field, found := knownFields[strings.ToLower(key)]; found {
			// Found a normal property, so parse it
			err := Unmarshal(val, field.Addr().Interface())
			if err != nil {
				return err
			}
		} else if extensions != nil {
			// Unknown, save it in our extension property, if we have one
			var v interface{}
			if err := Unmarshal(val, &v); err != nil {
				return err
			}
			extensions[key] = v
		}
	}
	return nil
}

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

	if err := Unmarshal([]byte(myJson), &v); err != nil {
		fmt.Printf("Err: %v\n", err)
		os.Exit(1)
	}

	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Printf("Original:\n%v\n\nParsed:\n%v\n", myJson, string(b))
}
