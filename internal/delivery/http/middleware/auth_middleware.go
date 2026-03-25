package middleware

import (
	"net/http"
	"strings"

	"eventbooker/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthMiddleware interface {
	RequireAuth() gin.HandlerFunc
}

type authMiddleware struct {
	authService service.AuthService
}

func NewAuthMiddleware(authService service.AuthService) AuthMiddleware {
	return &authMiddleware{authService: authService}
}

func (m *authMiddleware) RequireAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		headerValue := strings.TrimSpace(ctx.GetHeader("Authorization"))
		if !strings.HasPrefix(headerValue, "Bearer ") {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": service.ErrUnauthorized.Error()})
			return
		}

		claims, err := m.authService.ParseAccessToken(strings.TrimSpace(strings.TrimPrefix(headerValue, "Bearer ")))
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		ctx.Set(CurrentUserIDKey, claims.UserID)
		ctx.Set(CurrentUserEmailKey, claims.Email)
		ctx.Set(CurrentUserRoleKey, string(claims.Role))
		ctx.Next()
	}
}
