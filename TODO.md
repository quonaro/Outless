# Outless TODO

## Planned but not yet implemented features

### 1. Audit Log

- **Table:** `audit_logs` (`id`, `admin_id`, `action`, `resource_type`, `resource_id`, `old_value`, `new_value`, `ip`, `created_at`)
- **Endpoints:** `GET /v1/audit-logs` (paginated, filterable by admin/resource)
- **Integration:** Middleware or decorator around all mutating HTTP handlers (POST/PUT/DELETE) to capture who changed what, when, and from which IP.
- **Use case:** Traceability for admin actions — who deleted a node, who deactivated a token, etc.

### 2. RBAC (Role-Based Access Control)

- **Tables:** `roles` (`id`, `name`), `admin_roles` (`admin_id`, `role_id`)
- **Roles:** `superadmin` (full access), `manager` (can manage nodes/tokens/groups, no settings), `readonly` (view only)
- **JWT Claims:** Include `role` field in JWT claims.
- **Middleware:** Check `role` claim against operation permissions before executing handler.
- **Endpoints:** CRUD for roles, assign/unassign roles to admins.
- **Use case:** Delegated management without giving full admin access.
