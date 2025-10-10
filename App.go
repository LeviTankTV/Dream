package main

import (
	"fmt"
	"log"
	"os"

	"mpg/server"
)

func main() {
	srv := server.NewServer(":8080")
	defer srv.Close()

	fmt.Println("Game server started on :8080")
	fmt.Println("API endpoints available at http://localhost:8080")

	if err := srv.Start(); err != nil {
		log.Fatal("Error starting server:", err)
		os.Exit(1)
	}
}
