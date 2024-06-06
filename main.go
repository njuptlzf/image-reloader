package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/njuptlzf/image-reloader/handler"
)

func main() {
	handler.InitWatcherService()

	r := gin.Default()
	r.POST("/webhook", handler.WebhookHandler)

	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
