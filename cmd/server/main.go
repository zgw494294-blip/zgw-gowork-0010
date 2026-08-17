package main

import (
	"flag"
	"log"
	"net/http"

	"volunteer-scheduler/internal/httpapi"
	"volunteer-scheduler/internal/service"
	"volunteer-scheduler/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	flag.Parse()

	st := store.NewStore()
	svc := service.NewService(st)
	handler := httpapi.NewHandler(svc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	log.Printf("Server started at %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
