package middleware

import (
	"net/http"
	"strings"

	"eventbooker/internal/logging"
	"eventbooker/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthMiddleware interface {
	RequireAuth() gin.HandlerFunc
}

type authMiddleware struct {
	logger      *logging.EventBookerLogger
	authService service.AuthService
}

func NewAuthMiddleware(logger *logging.EventBookerLogger, authService service.AuthService) AuthMiddleware {
	return &authMiddleware{logger: logger, authService: authService}
}

func (m *authMiddleware) RequireAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestLogger := m.logger.Ctx(ctx.Request.Context())
		headerValue := strings.TrimSpace(ctx.GetHeader("Authorization"))
		if !strings.HasPrefix(headerValue, "Bearer ") {
			requestLogger.Warn("authorization header is missing bearer token")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": service.ErrUnauthorized.Error()})
			return
		}

		claims, err := m.authService.ParseAccessToken(strings.TrimSpace(strings.TrimPrefix(headerValue, "Bearer ")))
		if err != nil {
			requestLogger.Warnw("access token validation failed", "error", err)
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		ctx.Set(CurrentUserIDKey, claims.UserID)
		ctx.Set(CurrentUserEmailKey, claims.Email)
		ctx.Set(CurrentUserRoleKey, string(claims.Role))
		ctx.Next()
	}
}
