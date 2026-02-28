package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/synapseai/platform/pkg/jwt"
	"github.com/synapseai/platform/services/auth/internal/domain"
	"github.com/synapseai/platform/services/auth/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo   *repository.UserRepository
	jwtManager *jwt.Manager
}

func NewAuthService(userRepo *repository.UserRepository, jwtManager *jwt.Manager) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
	}
}

func (s *AuthService) Register(req *domain.RegisterRequest) (*domain.AuthResponse, error) {
	// Validate input
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return nil, fmt.Errorf("all fields are required")
	}

	// Check if email already exists
	exists, err := s.userRepo.EmailExists(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("email already registered")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &domain.User{
		ID:           uuid.New().String(),
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Name:         req.Name,
		Role:         "user",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.userRepo.Create(user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate tokens
	accessToken, err := s.jwtManager.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &domain.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: domain.UserResponse{
			ID:    user.ID,
			Email: user.Email,
			Name:  user.Name,
			Role:  user.Role,
		},
	}, nil
}

func (s *AuthService) Login(req *domain.LoginRequest) (*domain.AuthResponse, error) {
	// Validate input
	if req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	// Find user
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate tokens
	accessToken, err := s.jwtManager.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &domain.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: domain.UserResponse{
			ID:    user.ID,
			Email: user.Email,
			Name:  user.Name,
			Role:  user.Role,
		},
	}, nil
}

func (s *AuthService) ValidateToken(token string) (*jwt.Claims, error) {
	return s.jwtManager.ValidateToken(token)
}

func (s *AuthService) GetUserByID(userID string) (*domain.User, error) {
	return s.userRepo.FindByID(userID)
}
