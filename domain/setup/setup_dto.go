package setup

type SetupStatusResponse struct {
	IsSetupCompleted bool `json:"is_setup_completed"`
}

type SetupRequest struct {
	WorkspaceName string `json:"workspace_name" validate:"required"`
	AdminName     string `json:"admin_name" validate:"required"`
	AdminEmail    string `json:"admin_email" validate:"required,email"`
	AdminPassword string `json:"admin_password" validate:"required,min=6"`
}
