package plugin

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	GetWorkspacePlugins(ctx context.Context, workspaceID uuid.UUID) ([]WorkspacePlugin, error)
	GetWorkspacePlugin(ctx context.Context, workspaceID uuid.UUID, pluginID string) (*WorkspacePlugin, error)
	SaveWorkspacePlugin(ctx context.Context, wp *WorkspacePlugin) error
}

type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) GetWorkspacePlugins(ctx context.Context, workspaceID uuid.UUID) ([]WorkspacePlugin, error) {
	var plugins []WorkspacePlugin
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&plugins).Error
	return plugins, err
}

func (r *gormRepository) GetWorkspacePlugin(ctx context.Context, workspaceID uuid.UUID, pluginID string) (*WorkspacePlugin, error) {
	var wp WorkspacePlugin
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND plugin_id = ?", workspaceID, pluginID).First(&wp).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &wp, nil
}

func (r *gormRepository) SaveWorkspacePlugin(ctx context.Context, wp *WorkspacePlugin) error {
	return r.db.WithContext(ctx).Save(wp).Error
}
