package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Service struct {
	repo   interface{}
	logger *logrus.Logger
}

func NewService(repo interface{}) *Service {
	return &Service{repo: repo, logger: logrus.New()}
}

type JWTService interface {
	GenerateToken(userID string) (string, error)
	ValidateToken(tokenString string) (*jwt.Token, error)
	GetUserIDFromToken(tokenString string) (uuid.UUID, error)
}

type jwtService struct {
	secret     string
	expiryTime time.Duration
}

func NewJWTService(secret string, expiryTime time.Duration) JWTService {
	return &jwtService{
		secret:     secret,
		expiryTime: expiryTime,
	}
}

func (s *jwtService) GenerateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(s.expiryTime).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

func (s *jwtService) ValidateToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(s.secret), nil
	})
}

func (s *jwtService) GetUserIDFromToken(tokenString string) (uuid.UUID, error) {
	token, err := s.ValidateToken(tokenString)
	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, jwt.ErrTokenInvalidClaims
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return uuid.Nil, jwt.ErrTokenInvalidClaims
	}

	return uuid.Parse(userIDStr)
}
