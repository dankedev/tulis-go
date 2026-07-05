package importer

import (
	"context"

	"github.com/dankedev/kontent/domain/media"
	"github.com/dankedev/kontent/domain/post"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ImporterRepository interface {
	CreateImportLog(ctx context.Context, log *ImportLog) error
	GetImportLog(ctx context.Context, id uuid.UUID) (*ImportLog, error)
	ListImportLogs(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]ImportLog, int64, error)
	UpdateImportLog(ctx context.Context, log *ImportLog) error
}

type importerRepository struct {
	db *gorm.DB
}

func NewImporterRepository(db *gorm.DB) ImporterRepository {
	return &importerRepository{db: db}
}

func (r *importerRepository) CreateImportLog(ctx context.Context, log *ImportLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *importerRepository) GetImportLog(ctx context.Context, id uuid.UUID) (*ImportLog, error) {
	var log ImportLog
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *importerRepository) ListImportLogs(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]ImportLog, int64, error) {
	var logs []ImportLog
	var total int64

	query := r.db.WithContext(ctx).Model(&ImportLog{}).Where("workspace_id = ?", workspaceID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *importerRepository) UpdateImportLog(ctx context.Context, log *ImportLog) error {
	return r.db.WithContext(ctx).Save(log).Error
}

type postRepoHelper interface {
	Create(ctx context.Context, post *post.Post) error
	FindByID(ctx context.Context, id uuid.UUID) (*post.Post, error)
	FindBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*post.Post, error)
	AssignTaxonomies(ctx context.Context, postID uuid.UUID, taxonomyIDs []uuid.UUID) error
	CreateTaxonomy(ctx context.Context, taxonomy *post.Taxonomy) error
	FindTaxonomyBySlug(ctx context.Context, workspaceID uuid.UUID, slug string, taxType string) (*post.Taxonomy, error)
	CreateRevision(ctx context.Context, revision *post.PostRevision) error
}

type mediaRepoHelper interface {
	Create(ctx context.Context, m *media.Media) error
}

func NewImporterRepositoryWithHelpers(db *gorm.DB, postRepo postRepoHelper, mediaRepo mediaRepoHelper) ImporterRepository {
	return &importerRepository{db: db}
}
