package main

import (
	"fmt"
	"log"
	"os"

	"mpg/server"
)

func main() {
	// Создаем и запускаем сервер
	srv := server.NewServer(":8080")

	fmt.Println("Game server started on :8080")
	fmt.Println("Visit http://localhost:8080 in your browser")

	if err := srv.Start(); err != nil {
		log.Fatal("Error starting server:", err)
		os.Exit(1)
	}
}
