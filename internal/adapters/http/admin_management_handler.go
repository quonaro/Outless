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

func (h *AdminManagementHandler) Register(api huma.API) {
	huma.Get(api, "/v1/admins", h.ListAdmins)
	huma.Put(api, "/v1/admins/{id}", h.UpdateAdmin)
	huma.Delete(api, "/v1/admins/{id}", h.DeleteAdmin)
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
