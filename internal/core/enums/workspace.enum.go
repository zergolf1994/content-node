package enums

// ─── Plan Types ──────────────────────────────────────────────────────
// Must match PlanType in vdohide-service (workspace.enum.ts).

const (
	PlanTypeHobby      = "hobby"
	PlanTypePro        = "pro"
	PlanTypeEnterprise = "enterprise"
)

// ─── Workspace Statuses ──────────────────────────────────────────────
// Must match WorkspaceStatus in vdohide-service (workspace.enum.ts).

const (
	WorkspaceStatusPending  = "pending"
	WorkspaceStatusActive   = "active"
	WorkspaceStatusInactive = "inactive"
	WorkspaceStatusDeleted  = "deleted"
)

// ─── Member Roles ────────────────────────────────────────────────────
// Must match MemberRole in vdohide-service (workspace.enum.ts).

const (
	MemberRoleOwner  = "owner"
	MemberRoleAdmin  = "admin"
	MemberRoleEditor = "editor"
	MemberRoleViewer = "viewer"
)
