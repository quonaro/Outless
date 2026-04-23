package httpadapter

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

type ListAdminsOutput struct {
	Body []AdminItem `json:"admins"`
}

type UpdateAdminInput struct {
	ID   string `path:"id" required:"true"`
	Body struct {
		Password string `json:"password" required:"true" maxLength:"128"`
	}
}

type DeleteAdminInput struct {
	ID string `path:"id" required:"true"`
}

type AdminItem struct {
	ID       string `json:"id"`
	Username string `json:"username"`
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
	huma.Get(api, "/v1/admins", h.ListAdmins)
	huma.Put(api, "/v1/admins/{id}", h.UpdateAdmin)
	huma.Delete(api, "/v1/admins/{id}", h.DeleteAdmin)
	huma.Post(api, "/v1/admins/change-password", h.ChangePassword)
}

func (h *AdminManagementHandler) ListAdmins(ctx context.Context, _ *struct{}) (*ListAdminsOutput, error) {
	admins, err := h.adminRepo.List(ctx)
	if err != nil {
		h.logger.Error("failed to list admins", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to list admins")
	}

	response := make([]AdminItem, 0, len(admins))

	for _, a := range admins {
		response = append(response, AdminItem{
			ID:       a.ID,
			Username: a.Username,
		})
	}

	out := &ListAdminsOutput{}
	out.Body = response

	return out, nil
}

func (h *AdminManagementHandler) UpdateAdmin(ctx context.Context, input *UpdateAdminInput) (*struct{}, error) {
	if input.Body.Password == "" {
		return nil, huma.Error400BadRequest("password is required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to hash password")
	}

	admin := domain.Admin{
		ID:           input.ID,
		PasswordHash: string(hash),
	}

	if err := h.adminRepo.Update(ctx, admin); err != nil {
		if errors.Is(err, domain.ErrNodeNotFound) {
			return nil, huma.Error404NotFound("admin not found")
		}
		h.logger.Error("failed to update admin", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to update admin")
	}

	return nil, nil
}

func (h *AdminManagementHandler) DeleteAdmin(ctx context.Context, input *DeleteAdminInput) (*struct{}, error) {
	if err := h.adminRepo.Delete(ctx, input.ID); err != nil {
		h.logger.Error("failed to delete admin", slog.String("id", input.ID), slog.String("error", err.Error()))
		return nil, huma.Error500InternalServerError("failed to delete admin")
	}

	return nil, nil
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

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.Body.NewPassword), bcrypt.DefaultCost)
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
