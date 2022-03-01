package main

import (
	"log"
	"net/http"
	"os"
)

var h = &HS{}

func init() {
	//example "redis://mypasswordhere@10.10.10.50:6379"
	h.InitRedisAndRedisSearch(os.Getenv("REDIS_RAW_URL"), "idx:blog")
	h.CreateAndInitIndexDoc()
}

func main() {
	http.HandleFunc("/s", h.Search)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
