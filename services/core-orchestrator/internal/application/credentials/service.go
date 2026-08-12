package credentials

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"core-orchestrator/internal/domain"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrUnauthorized = errors.New("unauthorized")

type AuthService struct {
	repository *mysqlInfra.AuthRepository
	jwtSecret  []byte
	jwtIssuer  string
	jwtTTL     time.Duration
}

type AccessClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func NewAuthService(repository *mysqlInfra.AuthRepository, jwtSecret, jwtIssuer string, jwtTTLMinutes int) *AuthService {
	return &AuthService{
		repository: repository,
		jwtSecret:  []byte(jwtSecret),
		jwtIssuer:  jwtIssuer,
		jwtTTL:     time.Duration(jwtTTLMinutes) * time.Minute,
	}
}

func (s *AuthService) Login(username, password, userAgent, clientIP string) (*domain.LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	credentials, err := s.repository.FindUserCredentialsByUsername(username)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrAuthUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !credentials.IsActive {
		return nil, ErrUnauthorized
	}

	err = bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(s.jwtTTL)
	jti, err := generateJTI()
	if err != nil {
		return nil, fmt.Errorf("error generating token id: %w", err)
	}

	claims := AccessClaims{
		Username: credentials.Username,
		Role:     credentials.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.jwtIssuer,
			Subject:   fmt.Sprintf("%d", credentials.ID),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("error signing jwt: %w", err)
	}

	err = s.repository.StoreToken(jti, credentials.ID, tokenString, issuedAt, expiresAt, userAgent, clientIP)
	if err != nil {
		return nil, err
	}

	return &domain.LoginResult{
		AccessToken: tokenString,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		User: domain.AuthUser{
			ID:       credentials.ID,
			Username: credentials.Username,
			Role:     credentials.Role,
			IsActive: credentials.IsActive,
		},
	}, nil
}

func (s *AuthService) ValidateToken(tokenString string) (*domain.AuthUser, error) {
	if strings.TrimSpace(tokenString) == "" {
		return nil, ErrUnauthorized
	}

	claims := &AccessClaims{}
	parsedToken, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	}, jwt.WithIssuer(s.jwtIssuer), jwt.WithLeeway(5*time.Second))
	if err != nil || !parsedToken.Valid {
		return nil, ErrUnauthorized
	}

	if claims.ID == "" {
		return nil, ErrUnauthorized
	}

	user, err := s.repository.FindActiveUserByToken(claims.ID, tokenString)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrAuthUserNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	return user, nil
}

func generateJTI() (string, error) {
	raw := make([]byte, 16)
	_, err := rand.Read(raw)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}
