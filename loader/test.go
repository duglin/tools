package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ioutil.ReadAll(r.Body)

		len := 0
		if t := r.URL.Query().Get("sleep"); t != "" {
			len, _ = strconv.Atoi(t)
		}

		fmt.Printf("ID: %s - sleep: %d\n", r.Header.Get("ID"), len)

		if len > 0 {
			time.Sleep(time.Duration(len) * time.Second)
		}
	})

	fmt.Printf("Listening on port 8080\n")
	http.ListenAndServe(":8080", nil)
}
