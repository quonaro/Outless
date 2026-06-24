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
	adminRepo   domain.AdminRepository
	jwtService  *service.JWTService
	totpService *service.TOTPService
	logger      *slog.Logger
}

// NewAuthHandler constructs an auth handler.
func NewAuthHandler(
	adminRepo domain.AdminRepository,
	jwtService *service.JWTService,
	totpService *service.TOTPService,
	logger *slog.Logger,
) *AuthHandler {
	return &AuthHandler{
		adminRepo:   adminRepo,
		jwtService:  jwtService,
		totpService: totpService,
		logger:      logger,
	}
}

type loginInput struct {
	Body struct {
		Username string `json:"username" maxLength:"64"`
		Password string `json:"password" maxLength:"128"`
		TOTPCode string `json:"totp_code,omitempty" maxLength:"6"`
	}
}

type loginOutput struct {
	Body struct {
		Token        string `json:"token,omitempty"`
		Username     string `json:"username,omitempty"`
		TOTPRequired bool   `json:"totp_required"`
	}
}

// Register wires auth endpoints into Huma API.
func (h *AuthHandler) Register(api huma.API) {
	huma.Post(api, "/v1/auth/login", h.login)
	huma.Post(api, "/v1/auth/totp/setup", h.totpSetup)
	huma.Post(api, "/v1/auth/totp/verify", h.totpVerifySetup)
	huma.Post(api, "/v1/auth/totp/disable", h.totpDisable)
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

	if admin.TOTPEnabled {
		if input.Body.TOTPCode == "" {
			out := &loginOutput{}
			out.Body.TOTPRequired = true
			out.Body.Username = admin.Username
			return out, nil
		}
		if !h.totpService.ValidateCode(admin.TOTPSecret, input.Body.TOTPCode) {
			h.logger.Warn("login attempt with invalid totp code", slog.String("username", username))
			return nil, huma.Error401Unauthorized("invalid credentials")
		}
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
	out.Body.TOTPRequired = false

	return out, nil
}

type totpSetupOutput struct {
	Body struct {
		Secret   string `json:"secret"`
		URI      string `json:"uri"`
		QRBase64 string `json:"qr_base64"`
	}
}

func (h *AuthHandler) totpSetup(ctx context.Context, _ *struct{}) (*totpSetupOutput, error) {
	claims := GetClaims(ctx)
	if claims == nil {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	admin, err := h.adminRepo.FindByUsername(ctx, claims.Username)
	if err != nil {
		return nil, huma.Error404NotFound("admin not found")
	}

	secret, uri, err := h.totpService.GenerateKey("Outless", admin.Username)
	if err != nil {
		h.logger.Error("failed to generate totp key", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to generate totp key")
	}

	qrBase64, err := h.totpService.GenerateQRCodePNG(uri)
	if err != nil {
		h.logger.Error("failed to generate qr code", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to generate qr code")
	}

	admin.TOTPSecret = secret
	if err := h.adminRepo.Update(ctx, admin); err != nil {
		h.logger.Error("failed to save totp secret", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to save totp secret")
	}

	out := &totpSetupOutput{}
	out.Body.Secret = secret
	out.Body.URI = uri
	out.Body.QRBase64 = qrBase64
	return out, nil
}

type totpVerifyInput struct {
	Body struct {
		Code string `json:"code" maxLength:"6"`
	}
}

func (h *AuthHandler) totpVerifySetup(ctx context.Context, input *totpVerifyInput) (*struct{}, error) {
	claims := GetClaims(ctx)
	if claims == nil {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	admin, err := h.adminRepo.FindByUsername(ctx, claims.Username)
	if err != nil {
		return nil, huma.Error404NotFound("admin not found")
	}

	if admin.TOTPSecret == "" {
		return nil, huma.Error400BadRequest("totp not set up")
	}

	if !h.totpService.ValidateCode(admin.TOTPSecret, input.Body.Code) {
		return nil, huma.Error400BadRequest("invalid totp code")
	}

	admin.TOTPEnabled = true
	if err := h.adminRepo.Update(ctx, admin); err != nil {
		h.logger.Error("failed to enable totp", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to enable totp")
	}

	h.logger.Info("totp enabled", slog.String("username", admin.Username))
	return nil, nil
}

type totpDisableInput struct {
	Body struct {
		Code     string `json:"code" maxLength:"6"`
		Password string `json:"password" maxLength:"128"`
	}
}

func (h *AuthHandler) totpDisable(ctx context.Context, input *totpDisableInput) (*struct{}, error) {
	claims := GetClaims(ctx)
	if claims == nil {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	admin, err := h.adminRepo.FindByUsername(ctx, claims.Username)
	if err != nil {
		return nil, huma.Error404NotFound("admin not found")
	}

	if !admin.TOTPEnabled {
		return nil, huma.Error400BadRequest("totp not enabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(input.Body.Password)); err != nil {
		return nil, huma.Error401Unauthorized("invalid password")
	}

	if !h.totpService.ValidateCode(admin.TOTPSecret, input.Body.Code) {
		return nil, huma.Error400BadRequest("invalid totp code")
	}

	admin.TOTPEnabled = false
	admin.TOTPSecret = ""
	if err := h.adminRepo.Update(ctx, admin); err != nil {
		h.logger.Error("failed to disable totp", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to disable totp")
	}

	h.logger.Info("totp disabled", slog.String("username", admin.Username))
	return nil, nil
}
