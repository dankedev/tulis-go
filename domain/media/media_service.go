package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrMediaNotFound = errors.New("media not found")
)

type MediaService interface {
	SaveFile(ctx context.Context, workspaceID uuid.UUID, filename string, fileData []byte, mimeType string, size int64, altText, caption string) (*Media, error)
	GetMediaByID(ctx context.Context, id uuid.UUID) (*Media, error)
	DeleteMedia(ctx context.Context, id uuid.UUID) error
	ListMedia(ctx context.Context, workspaceID uuid.UUID, page, perPage int) ([]Media, int64, error)
}

type mediaService struct {
	repo MediaRepository
}

func NewMediaService(repo MediaRepository) MediaService {
	return &mediaService{repo: repo}
}

func (s *mediaService) SaveFile(ctx context.Context, workspaceID uuid.UUID, filename string, fileData []byte, mimeType string, size int64, altText, caption string) (*Media, error) {
	// Create upload dir if not exists
	uploadDir := "uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, err
	}

	uniqueID := uuid.New()
	cleanFilename := strings.ReplaceAll(filename, " ", "_")
	savedFilename := uniqueID.String() + "_" + cleanFilename
	filePath := filepath.Join(uploadDir, savedFilename)

	// Save original file
	err := os.WriteFile(filePath, fileData, 0644)
	if err != nil {
		return nil, err
	}

	// Image thumbnail generation
	lowerName := strings.ToLower(filename)
	isImage := strings.HasPrefix(mimeType, "image/") ||
		strings.HasSuffix(lowerName, ".png") ||
		strings.HasSuffix(lowerName, ".jpg") ||
		strings.HasSuffix(lowerName, ".jpeg") ||
		strings.HasSuffix(lowerName, ".gif")

	if isImage {
		reader := bytes.NewReader(fileData)
		img, format, err := image.Decode(reader)
		if err == nil {
			// aspect ratio preserving thumbnail resize
			thumbImg := createThumbnail(img)
			thumbPath := filepath.Join(uploadDir, "thumb_"+savedFilename)

			out, err := os.Create(thumbPath)
			if err == nil {
				defer out.Close()
				if format == "jpeg" || format == "jpg" {
					_ = jpeg.Encode(out, thumbImg, nil)
				} else if format == "png" {
					_ = png.Encode(out, thumbImg)
				} else if format == "gif" {
					_ = gif.Encode(out, thumbImg, nil)
				}
			}
		}
	}

	m := &Media{
		ID:          uniqueID,
		WorkspaceID: workspaceID,
		Filename:    cleanFilename,
		Path:        "/uploads/" + savedFilename,
		MimeType:    mimeType,
		Size:        size,
		AltText:     altText,
		Caption:     caption,
	}

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}

	return m, nil
}

func (s *mediaService) GetMediaByID(ctx context.Context, id uuid.UUID) (*Media, error) {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	return m, nil
}

func (s *mediaService) DeleteMedia(ctx context.Context, id uuid.UUID) error {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrMediaNotFound
	}

	// Delete local files
	localPath := strings.TrimPrefix(m.Path, "/")
	_ = os.Remove(localPath)

	// Delete thumbnail if it exists
	thumbPath := filepath.Join("uploads", "thumb_"+filepath.Base(localPath))
	_ = os.Remove(thumbPath)

	return s.repo.Delete(ctx, id)
}

func (s *mediaService) ListMedia(ctx context.Context, workspaceID uuid.UUID, page, perPage int) ([]Media, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	offset := (page - 1) * perPage
	return s.repo.List(ctx, workspaceID, perPage, offset)
}

// Pure standard library nearest-neighbor resizer helper
func scaleImage(src image.Image, w, h int) image.Image {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcX := (x * srcW) / w
			srcY := (y * srcH) / h
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func createThumbnail(src image.Image) image.Image {
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	if srcW <= 300 {
		return src
	}
	dstW := 300
	dstH := (srcH * 300) / srcW
	return scaleImage(src, dstW, dstH)
}
