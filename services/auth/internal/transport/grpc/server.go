package grpc

import (
	"context"

	pb "github.com/synapseai/platform/proto/auth"
	"github.com/synapseai/platform/services/auth/internal/service"
)

type Server struct {
	pb.UnimplementedAuthServiceServer
	authService *service.AuthService
}

func NewServer(authService *service.AuthService) *Server {
	return &Server{authService: authService}
}

func (s *Server) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	claims, err := s.authService.ValidateToken(req.Token)
	if err != nil {
		return &pb.ValidateTokenResponse{
			Valid: false,
			Error: err.Error(),
		}, nil
	}

	return &pb.ValidateTokenResponse{
		Valid:  true,
		UserId: claims.UserID,
		Email:  claims.Email,
		Role:   claims.Role,
	}, nil
}

func (s *Server) GetUserByID(ctx context.Context, req *pb.GetUserByIDRequest) (*pb.GetUserByIDResponse, error) {
	user, err := s.authService.GetUserByID(req.UserId)
	if err != nil {
		return nil, err
	}

	return &pb.GetUserByIDResponse{
		UserId:    user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (s *Server) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	// Validate the refresh token and generate new tokens
	claims, err := s.authService.ValidateToken(req.RefreshToken)
	if err != nil {
		return &pb.RefreshTokenResponse{
			Error: "invalid refresh token",
		}, nil
	}

	// Generate new access token
	newAccessToken, err := s.authService.ValidateToken(claims.UserID)
	if err != nil {
		return &pb.RefreshTokenResponse{
			Error: "failed to generate new token",
		}, nil
	}

	_ = newAccessToken // Use the token appropriately

	return &pb.RefreshTokenResponse{
		AccessToken:  req.RefreshToken, // In real implementation, generate new token
		RefreshToken: req.RefreshToken,
	}, nil
}
