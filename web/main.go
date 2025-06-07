package main

import (
	"flag"
	"log"
	"os"

	"github.com/df07/go-progressive-raytracer/web/server"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "Port to serve on")
	flag.Parse()

	// Create and start web server
	webServer := server.NewServer(*port)

	log.Printf("Progressive Raytracer Web Server")
	log.Printf("Visit http://localhost:%d to start rendering", *port)

	if err := webServer.Start(); err != nil {
		log.Printf("Error starting server: %v", err)
		os.Exit(1)
	}
}
