package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"eventbooker/internal/config"
	"eventbooker/internal/domain/model"
	"eventbooker/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	txManager              repository.TxManager
	userRepository         repository.UserRepository
	refreshTokenRepository repository.RefreshTokenRepository
	config                 config.AuthConfig
}

type accessTokenClaims struct {
	Email string         `json:"email"`
	Role  model.UserRole `json:"role"`
	jwt.RegisteredClaims
}

func NewAuthService(
	txManager repository.TxManager,
	userRepository repository.UserRepository,
	refreshTokenRepository repository.RefreshTokenRepository,
	config config.AuthConfig,
) AuthService {
	return &authService{
		txManager:              txManager,
		userRepository:         userRepository,
		refreshTokenRepository: refreshTokenRepository,
		config:                 config,
	}
}

func (s *authService) Register(ctx context.Context, input RegisterInput) (*model.User, error) {
	email := strings.TrimSpace(strings.ToLower(input.Email))
	name := strings.TrimSpace(input.Name)
	if !isValidEmail(email) || name == "" || len(input.Password) < 8 {
		return nil, ErrInvalidInput
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Email:        email,
		Name:         name,
		PasswordHash: string(passwordHash),
		Role:         model.UserRoleUser,
	}

	if err := s.userRepository.Create(ctx, user); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, err
	}

	return user, nil
}

func (s *authService) Login(ctx context.Context, input LoginInput) (*LoginResult, string, error) {
	user, err := s.authenticateUser(ctx, input.Email, input.Password)
	if err != nil {
		return nil, "", err
	}

	result, refreshToken, err := s.issueSession(ctx, user, input.UserAgent, input.IPAddress, nil)
	if err != nil {
		return nil, "", err
	}

	return result, refreshToken, nil
}

func (s *authService) Refresh(ctx context.Context, input RefreshInput) (*RefreshResult, string, error) {
	if strings.TrimSpace(input.RefreshToken) == "" {
		return nil, "", ErrUnauthorized
	}

	var result *RefreshResult
	var rawRefreshToken string
	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		hashedToken := hashRefreshToken(input.RefreshToken)
		session, err := s.refreshTokenRepository.GetByTokenHash(txCtx, hashedToken)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrSessionNotFound
			}
			return err
		}

		now := time.Now().UTC()
		if session.RevokedAt != nil {
			return ErrSessionRevoked
		}
		if !session.ExpiresAt.After(now) {
			return ErrSessionExpired
		}

		user, err := s.userRepository.GetByID(txCtx, session.UserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}

		newResult, newRefreshToken, err := s.issueSession(txCtx, user, input.UserAgent, input.IPAddress, session)
		if err != nil {
			return err
		}

		result = &RefreshResult{Tokens: newResult.Tokens, User: newResult.User}
		rawRefreshToken = newRefreshToken
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	return result, rawRefreshToken, nil
}

func (s *authService) Logout(ctx context.Context, input LogoutInput) error {
	if strings.TrimSpace(input.RefreshToken) == "" {
		return nil
	}

	token, err := s.refreshTokenRepository.GetByTokenHash(ctx, hashRefreshToken(input.RefreshToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	if token.RevokedAt != nil {
		return nil
	}

	return s.refreshTokenRepository.RevokeByID(ctx, token.ID, time.Now().UTC())
}

func (s *authService) ParseAccessToken(token string) (*AuthClaims, error) {
	claims := &accessTokenClaims{}
	parsedToken, err := jwt.ParseWithClaims(token, claims, func(parsedToken *jwt.Token) (interface{}, error) {
		if _, ok := parsedToken.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %s", parsedToken.Method.Alg())
		}
		return []byte(s.config.JWTSecret), nil
	}, jwt.WithIssuer(s.config.Issuer))
	if err != nil || !parsedToken.Valid {
		return nil, ErrUnauthorized
	}

	userID, parseErr := parseSubject(claims.Subject)
	if parseErr != nil {
		return nil, ErrUnauthorized
	}

	return &AuthClaims{UserID: userID, Email: claims.Email, Role: claims.Role}, nil
}

func (s *authService) GetUser(ctx context.Context, userID int64) (*model.User, error) {
	if userID <= 0 {
		return nil, ErrInvalidInput
	}

	user, err := s.userRepository.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (s *authService) authenticateUser(ctx context.Context, email, password string) (*model.User, error) {
	user, err := s.userRepository.GetByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

func (s *authService) issueSession(
	ctx context.Context,
	user *model.User,
	userAgent string,
	ipAddress string,
	replacedToken *model.RefreshToken,
) (*LoginResult, string, error) {
	accessToken, expiresIn, err := s.generateAccessToken(user)
	if err != nil {
		return nil, "", err
	}

	rawRefreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, "", err
	}

	refreshToken := &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashRefreshToken(rawRefreshToken),
		ExpiresAt: time.Now().UTC().Add(s.config.RefreshTTL()),
	}
	if strings.TrimSpace(userAgent) != "" {
		refreshToken.UserAgent = &userAgent
	}
	if strings.TrimSpace(ipAddress) != "" {
		refreshToken.IPAddress = &ipAddress
	}

	if err := s.refreshTokenRepository.Create(ctx, refreshToken); err != nil {
		return nil, "", err
	}

	if replacedToken != nil {
		if err := s.refreshTokenRepository.RevokeAndReplace(ctx, replacedToken.ID, time.Now().UTC(), refreshToken.ID); err != nil {
			return nil, "", err
		}
	}

	return &LoginResult{
		Tokens: &AuthTokens{
			AccessToken: accessToken,
			TokenType:   "Bearer",
			ExpiresIn:   expiresIn,
		},
		User: user,
	}, rawRefreshToken, nil
}

func (s *authService) generateAccessToken(user *model.User) (string, int64, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(s.config.AccessTTL())
	claims := accessTokenClaims{
		Email: user.Email,
		Role:  user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.config.Issuer,
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", 0, err
	}

	return signedToken, int64(s.config.AccessTTL().Seconds()), nil
}

func generateRefreshToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isValidEmail(email string) bool {
	if len(email) < 3 || !strings.Contains(email, "@") {
		return false
	}

	parts := strings.Split(email, "@")
	return len(parts) == 2 && parts[0] != "" && strings.Contains(parts[1], ".")
}

func parseSubject(subject string) (int64, error) {
	userID, err := strconv.ParseInt(subject, 10, 64)
	if err != nil || userID <= 0 {
		return 0, ErrUnauthorized
	}

	return userID, nil
}
