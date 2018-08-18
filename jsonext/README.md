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

To access a property that might be defined in your struct or within an
extension (meaning, be forwards and backward compatible), use:
```
	StructGet(structValue, key)
```
For example:
```
	address, err := jsonext.StructGet( person, "address" )
```
will find `address` whether it ends up being defined as a sibling to
`Name` or ends up being parsed into `Extras`.

See: [`future/future.go`](future/future.go) for a full example of how to use it.
