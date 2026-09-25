package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "served from a boxer sandbox, as %s\n", r.Host)
	})
	// 0.0.0.0, not localhost: the forwarded port arrives on the guest's interface.
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
