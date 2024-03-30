package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
)

func main() {
	http.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("Hello, world!"))
		w.WriteHeader(200)
	})

	port, err := strconv.ParseInt(os.Getenv("PORT"), 10, 64)
	if err != nil {
		panic(err)
	}

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("listening at %s", addr)
	http.ListenAndServe(addr, nil)
}
