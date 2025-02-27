package main

import (
	"fmt"
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Welcome to Monopoly Backend!")
}

func main() {
	http.HandleFunc("/", handler)

	port := "8080"
	fmt.Println("Server running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
