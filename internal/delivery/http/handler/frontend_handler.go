package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wb-go/wbf/ginext"
)

type frontendHandler struct {
	telegramBotUsername string
}

func NewFrontendHandler(telegramBotUsername string) FrontendHandler {
	return frontendHandler{telegramBotUsername: telegramBotUsername}
}

func (h frontendHandler) Index(ctx *ginext.Context) {
	ctx.HTML(http.StatusOK, "app/index.html", gin.H{
		"TelegramBotUsername": h.telegramBotUsername,
	})
}
