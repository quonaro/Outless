package http

import (
	"context"
	"errors"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"outless/internal/domain"

	"github.com/danielgtaylor/huma/v2"
)

type AdminManagementHandler struct {
	adminRepo domain.AdminRepository
	logger    *slog.Logger
}

func NewAdminManagementHandler(adminRepo domain.AdminRepository, logger *slog.Logger) *AdminManagementHandler {
	return &AdminManagementHandler{
		adminRepo: adminRepo,
		logger:    logger,
	}
}

// ChangePasswordInput is the body for POST /v1/admins/change-password.
type ChangePasswordInput struct {
	Body struct {
		CurrentLogin    string `json:"current_login" required:"true" maxLength:"64"`
		CurrentPassword string `json:"current_password" required:"true" maxLength:"128"`
		NewLogin        string `json:"new_login" maxLength:"64"`
		NewPassword     string `json:"new_password" required:"true" maxLength:"128"`
	}
}

func (h *AdminManagementHandler) Register(api huma.API) {
	// Single-admin contract: only password/login rotation is exposed.
	huma.Post(api, "/v1/admins/change-password", h.ChangePassword)
}

// ChangePassword verifies current credentials and updates login and/or password.
func (h *AdminManagementHandler) ChangePassword(ctx context.Context, input *ChangePasswordInput) (*struct{}, error) {
	if input.Body.CurrentLogin == "" || input.Body.CurrentPassword == "" || input.Body.NewPassword == "" {
		return nil, huma.Error400BadRequest("current_login, current_password and new_password are required")
	}

	admin, err := h.adminRepo.FindByUsername(ctx, input.Body.CurrentLogin)
	if err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			return nil, huma.Error401Unauthorized("invalid credentials")
		}
		h.logger.Error("failed to lookup admin", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to change password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(input.Body.CurrentPassword)); err != nil {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}

	// bcrypt cost 12 provides stronger security than DefaultCost (typically 10)
	newHash, err := bcrypt.GenerateFromPassword([]byte(input.Body.NewPassword), 12)
	if err != nil {
		h.logger.Error("failed to hash new password", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to change password")
	}

	updated := domain.Admin{
		ID:           admin.ID,
		Username:     input.Body.NewLogin,
		PasswordHash: string(newHash),
	}

	if err := h.adminRepo.Update(ctx, updated); err != nil {
		h.logger.Error("failed to update admin credentials", slog.String("id", admin.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to change password")
	}

	h.logger.Info("admin credentials changed", slog.String("id", admin.ID))
	return nil, nil
}
