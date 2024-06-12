package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/njuptlzf/image-reloader/model"
	"github.com/njuptlzf/image-reloader/service"
)

// 目前先这样全局变量
var watcherService *service.WatcherService

func InitWatcherService() *service.WatcherService {
	watcherService = service.NewWatcherService()
	go watcherService.StartWatcher()
	return watcherService
}

func WebhookHandler(c *gin.Context) {
	var event *model.PushEvent
	err := c.BindJSON(&event)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 将事件发送到更新处理器
	go func() {
		watcherService.UpdateHandlerChan <- *event
	}()
	<-watcherService.UpdateHandlerDone
	c.JSON(http.StatusOK, gin.H{"status": "event received"})
}
