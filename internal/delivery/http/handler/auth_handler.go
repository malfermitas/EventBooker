package handler

import (
	"eventbooker/internal/config"
	"eventbooker/internal/service"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wb-go/wbf/ginext"
)

type authHandler struct {
	authService service.AuthService
	config      config.AuthConfig
}

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func NewAuthHandler(authService service.AuthService, config config.AuthConfig) AuthHandler {
	return authHandler{authService: authService, config: config}
}

func (h authHandler) Register(ctx *ginext.Context) {
	var req registerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authService.Register(ctx.Request.Context(), service.RegisterInput{
		Email:    req.Email,
		Name:     req.Name,
		Password: req.Password,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, userResponseFromModel(user))
}

func (h authHandler) Login(ctx *ginext.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, refreshToken, err := h.authService.Login(ctx.Request.Context(), service.LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		UserAgent: ctx.Request.UserAgent(),
		IPAddress: clientIP(ctx),
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	setRefreshCookie(ctx, h.config, refreshToken)
	ctx.JSON(http.StatusOK, authResponseFromResult(result.Tokens, result.User))
}

func (h authHandler) Refresh(ctx *ginext.Context) {
	refreshToken, err := ctx.Cookie(h.config.RefreshCookieName)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": service.ErrUnauthorized.Error()})
		return
	}

	result, newRefreshToken, err := h.authService.Refresh(ctx.Request.Context(), service.RefreshInput{
		RefreshToken: refreshToken,
		UserAgent:    ctx.Request.UserAgent(),
		IPAddress:    clientIP(ctx),
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	setRefreshCookie(ctx, h.config, newRefreshToken)
	ctx.JSON(http.StatusOK, authResponseFromResult(result.Tokens, result.User))
}

func (h authHandler) Logout(ctx *ginext.Context) {
	refreshToken, _ := ctx.Cookie(h.config.RefreshCookieName)
	if err := h.authService.Logout(ctx.Request.Context(), service.LogoutInput{RefreshToken: refreshToken}); err != nil {
		writeServiceError(ctx, err)
		return
	}

	clearRefreshCookie(ctx, h.config)
	ctx.Status(http.StatusNoContent)
}

func (h authHandler) Me(ctx *ginext.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": service.ErrUnauthorized.Error()})
		return
	}

	user, err := h.authService.GetUser(ctx.Request.Context(), userID)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, userResponseFromModel(user))
}

func setRefreshCookie(ctx *ginext.Context, cfg config.AuthConfig, token string) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(
		cfg.RefreshCookieName,
		token,
		int(cfg.RefreshTTL().Seconds()),
		"/auth",
		"",
		cfg.RefreshCookieSecure,
		true,
	)
}

func clearRefreshCookie(ctx *ginext.Context, cfg config.AuthConfig) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(cfg.RefreshCookieName, "", -1, "/auth", "", cfg.RefreshCookieSecure, true)
}

func clientIP(ctx *ginext.Context) string {
	ip := strings.TrimSpace(ctx.ClientIP())
	if ip == "<nil>" {
		return ""
	}

	return ip
}
