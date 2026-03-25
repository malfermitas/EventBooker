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
	"eventbooker/internal/logging"
	"eventbooker/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	logger                 *logging.EventBookerLogger
	notifier               NotificationSender
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
	logger *logging.EventBookerLogger,
	notifier NotificationSender,
	txManager repository.TxManager,
	userRepository repository.UserRepository,
	refreshTokenRepository repository.RefreshTokenRepository,
	config config.AuthConfig,
) AuthService {
	return &authService{
		logger:                 logger,
		notifier:               notifier,
		txManager:              txManager,
		userRepository:         userRepository,
		refreshTokenRepository: refreshTokenRepository,
		config:                 config,
	}
}

func (s *authService) Register(ctx context.Context, input RegisterInput) (*model.User, error) {
	requestLogger := s.logger.Ctx(ctx)
	email := strings.TrimSpace(strings.ToLower(input.Email))
	name := strings.TrimSpace(input.Name)
	if !isValidEmail(email) || name == "" || len(input.Password) < 8 {
		requestLogger.Warnw("user registration rejected due to invalid input", "email", email)
		return nil, ErrInvalidInput
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		requestLogger.Errorw("failed to hash user password", "email", email, "error", err)
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
			requestLogger.Warnw("user registration rejected because email already exists", "email", email)
			return nil, ErrEmailAlreadyExists
		}
		requestLogger.Errorw("failed to create user", "email", email, "error", err)
		return nil, err
	}

	requestLogger.Infow("user registered", "user_id", user.ID, "email", user.Email)
	s.notifyWelcome(ctx, user)
	return user, nil
}

func (s *authService) Login(ctx context.Context, input LoginInput) (*LoginResult, string, error) {
	requestLogger := s.logger.Ctx(ctx)
	user, err := s.authenticateUser(ctx, input.Email, input.Password)
	if err != nil {
		requestLogger.Warnw("user login failed", "email", strings.TrimSpace(strings.ToLower(input.Email)), "error", err)
		return nil, "", err
	}

	result, refreshToken, err := s.issueSession(ctx, user, input.UserAgent, input.IPAddress, nil)
	if err != nil {
		requestLogger.Errorw("failed to issue login session", "user_id", user.ID, "error", err)
		return nil, "", err
	}

	requestLogger.Infow("user logged in", "user_id", user.ID)
	return result, refreshToken, nil
}

func (s *authService) Refresh(ctx context.Context, input RefreshInput) (*RefreshResult, string, error) {
	requestLogger := s.logger.Ctx(ctx)
	if strings.TrimSpace(input.RefreshToken) == "" {
		requestLogger.Warn("refresh rejected because token is empty")
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
		requestLogger.Warnw("session refresh failed", "error", err)
		return nil, "", err
	}

	requestLogger.Info("session refreshed")
	return result, rawRefreshToken, nil
}

func (s *authService) Logout(ctx context.Context, input LogoutInput) error {
	requestLogger := s.logger.Ctx(ctx)
	if strings.TrimSpace(input.RefreshToken) == "" {
		requestLogger.Debug("logout skipped because refresh token is empty")
		return nil
	}

	token, err := s.refreshTokenRepository.GetByTokenHash(ctx, hashRefreshToken(input.RefreshToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			requestLogger.Debug("logout skipped because session was not found")
			return nil
		}
		requestLogger.Errorw("failed to load refresh token for logout", "error", err)
		return err
	}

	if token.RevokedAt != nil {
		requestLogger.Debugw("logout skipped because session is already revoked", "token_id", token.ID)
		return nil
	}

	if err := s.refreshTokenRepository.RevokeByID(ctx, token.ID, time.Now().UTC()); err != nil {
		requestLogger.Errorw("failed to revoke refresh token", "token_id", token.ID, "error", err)
		return err
	}

	requestLogger.Infow("user logged out", "user_id", token.UserID)
	return nil
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
	requestLogger := s.logger.Ctx(ctx)
	if userID <= 0 {
		requestLogger.Warnw("get user rejected due to invalid user id", "user_id", userID)
		return nil, ErrInvalidInput
	}

	user, err := s.userRepository.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			requestLogger.Warnw("user not found", "user_id", userID)
			return nil, ErrUserNotFound
		}
		requestLogger.Errorw("failed to load user", "user_id", userID, "error", err)
		return nil, err
	}

	return user, nil
}

func (s *authService) authenticateUser(ctx context.Context, email, password string) (*model.User, error) {
	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	requestLogger := s.logger.Ctx(ctx)
	user, err := s.userRepository.GetByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			requestLogger.Warnw("authentication failed because user was not found", "email", normalizedEmail)
			return nil, ErrInvalidCredentials
		}
		requestLogger.Errorw("failed to load user for authentication", "email", normalizedEmail, "error", err)
		return nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		requestLogger.Warnw("authentication failed due to invalid password", "user_id", user.ID)
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
	requestLogger := s.logger.Ctx(ctx)
	accessToken, expiresIn, err := s.generateAccessToken(user)
	if err != nil {
		requestLogger.Errorw("failed to generate access token", "user_id", user.ID, "error", err)
		return nil, "", err
	}

	rawRefreshToken, err := generateRefreshToken()
	if err != nil {
		requestLogger.Errorw("failed to generate refresh token", "user_id", user.ID, "error", err)
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
		requestLogger.Errorw("failed to persist refresh token", "user_id", user.ID, "error", err)
		return nil, "", err
	}

	if replacedToken != nil {
		if err := s.refreshTokenRepository.RevokeAndReplace(ctx, replacedToken.ID, time.Now().UTC(), refreshToken.ID); err != nil {
			requestLogger.Errorw("failed to rotate refresh token", "old_token_id", replacedToken.ID, "new_token_id", refreshToken.ID, "error", err)
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

func (s *authService) notifyWelcome(ctx context.Context, user *model.User) {
	if s.notifier == nil {
		return
	}

	message := fmt.Sprintf(
		"Welcome to EventBooker, %s! Your account has been created successfully. To link Telegram notifications, send /start %d to the DelayedNotifier bot.",
		user.Name,
		user.ID,
	)

	if err := s.notifier.ScheduleEmail(ctx, user.Email, message, time.Now().UTC()); err != nil {
		s.logger.Ctx(ctx).Warnw("failed to schedule welcome notification", "user_id", user.ID, "error", err)
	}
}
