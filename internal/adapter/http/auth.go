package http

import (
	"context"
	"errors"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"outless/internal/domain"
	"outless/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

// AuthHandler handles admin authentication endpoints.
type AuthHandler struct {
	adminRepo  domain.AdminRepository
	jwtService *service.JWTService
	logger     *slog.Logger
}

// NewAuthHandler constructs an auth handler.
func NewAuthHandler(adminRepo domain.AdminRepository, jwtService *service.JWTService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		adminRepo:  adminRepo,
		jwtService: jwtService,
		logger:     logger,
	}
}

type loginInput struct {
	Body struct {
		Username string `json:"username" maxLength:"64"`
		Password string `json:"password" maxLength:"128"`
	}
}

type loginOutput struct {
	Body struct {
		Token    string `json:"token"`
		Username string `json:"username"`
	}
}

// Register wires auth endpoints into Huma API.
func (h *AuthHandler) Register(api huma.API) {
	huma.Post(api, "/v1/auth/login", h.login)
}

func (h *AuthHandler) login(ctx context.Context, input *loginInput) (*loginOutput, error) {
	username := input.Body.Username
	password := input.Body.Password

	if username == "" || password == "" {
		return nil, huma.Error400BadRequest("username and password are required")
	}

	admin, err := h.adminRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, domain.ErrAdminNotFound) {
			h.logger.Warn("login attempt with unknown username", slog.String("username", username))
			return nil, huma.Error401Unauthorized("invalid credentials")
		}
		h.logger.Error("failed to find admin", slog.String("username", username), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("authentication failed")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		h.logger.Warn("login attempt with invalid password", slog.String("username", username))
		return nil, huma.Error401Unauthorized("invalid credentials")
	}

	token, err := h.jwtService.GenerateToken(admin.Username)
	if err != nil {
		h.logger.Error("failed to generate token", slog.String("username", username), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to generate token")
	}

	h.logger.Info("admin logged in", slog.String("username", username))
	out := &loginOutput{}
	out.Body.Token = token
	out.Body.Username = admin.Username

	return out, nil
}
