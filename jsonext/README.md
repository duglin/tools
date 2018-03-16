This GO-lang JSON Parser will support extensions - meaning unknown
properties in the JSON. If you include a property in your struct
with the `exts` json tag then all unknown JSON properties will be 
placed in there. For example, struct that looks like:
```
struct {
  Name string `json:"name"`
  Extras map[string]interface{} `json:",exts"`
}
```

Will result in this JSON:
```
{ "name": "john",
  "address": "123 main street" }
```
being parsed as:
```
struct {
  Name: "john",
  Extras: {
    "address": "123 main street",
  }
}
```
