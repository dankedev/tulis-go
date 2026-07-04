package workspace

type CreateWorkspaceReq struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	Plan string `json:"plan"`
}

type UpdateWorkspaceReq struct {
	Name     string                 `json:"name"`
	Slug     string                 `json:"slug"`
	Settings map[string]interface{} `json:"settings"`
}

type AddMemberReq struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type UpdateMemberRoleReq struct {
	Role string `json:"role"`
}
