package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"outless/internal/service"
	"outless/internal/domain"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/crypto/bcrypt"
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
		Token string `json:"token"`
	}
}

type firstAdminStatusOutput struct {
	Body struct {
		CanRegister bool `json:"can_register"`
	}
}

type registerFirstAdminInput struct {
	Body struct {
		Username string `json:"username" maxLength:"64"`
		Password string `json:"password" maxLength:"128"`
	}
}

type registerFirstAdminOutput struct {
	Body struct {
		Token string `json:"token"`
	}
}

// Register wires auth endpoints into Huma API.
func (h *AuthHandler) Register(api huma.API) {
	huma.Get(api, "/v1/auth/register_first_admin", h.firstAdminStatus)
	huma.Post(api, "/v1/auth/register_first_admin", h.registerFirstAdmin)
	huma.Post(api, "/v1/auth/login", h.login)
}

func (h *AuthHandler) firstAdminStatus(ctx context.Context, _ *struct{}) (*firstAdminStatusOutput, error) {
	count, err := h.adminRepo.Count(ctx)
	if err != nil {
		h.logger.Error("failed to count admins", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to check bootstrap status")
	}

	out := &firstAdminStatusOutput{}
	out.Body.CanRegister = count == 0

	return out, nil
}

func (h *AuthHandler) registerFirstAdmin(ctx context.Context, input *registerFirstAdminInput) (*registerFirstAdminOutput, error) {
	username := input.Body.Username
	password := input.Body.Password

	if username == "" || password == "" {
		return nil, huma.Error400BadRequest("username and password are required")
	}

	// bcrypt cost 12 provides stronger security than DefaultCost (typically 10)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		h.logger.Error("failed to hash first admin password", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to register first admin")
	}

	admin := domain.Admin{
		ID:           newAdminID(),
		Username:     username,
		PasswordHash: string(passwordHash),
	}

	if err := h.adminRepo.Create(ctx, admin); err != nil {
		if errors.Is(err, domain.ErrAdminAlreadyExists) {
			return nil, huma.Error409Conflict("first admin is already registered")
		}
		h.logger.Error("failed to register first admin", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to register first admin")
	}

	token, err := h.jwtService.GenerateToken(admin.Username)
	if err != nil {
		h.logger.Error("failed to generate token after first admin registration", slog.String("username", username), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to generate token")
	}

	h.logger.Info("first admin registered", slog.String("username", username))
	out := &registerFirstAdminOutput{}
	out.Body.Token = token

	return out, nil
}

func (h *AuthHandler) login(ctx context.Context, input *loginInput) (*loginOutput, error) {
	username := input.Body.Username
	password := input.Body.Password

	if username == "" || password == "" {
		return nil, huma.Error400BadRequest("username and password are required")
	}

	admin, err := h.adminRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
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

	return out, nil
}

func newAdminID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("admin_%d", time.Now().UTC().UnixNano())
	}

	return hex.EncodeToString(bytes)
}
