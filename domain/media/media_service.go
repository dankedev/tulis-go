package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"time"

	"github.com/dankedev/tulis-go/storage"
	"github.com/google/uuid"
)

var (
	ErrMediaNotFound = errors.New("media not found")
)

type MediaService interface {
	SaveFile(ctx context.Context, workspaceID uuid.UUID, filename string, fileData []byte, mimeType string, size int64, altText, caption string) (*Media, error)
	GetMediaByID(ctx context.Context, id uuid.UUID) (*Media, error)
	UpdateMedia(ctx context.Context, id uuid.UUID, altText, caption string) (*Media, error)
	DeleteMedia(ctx context.Context, id uuid.UUID) error
	ListMedia(ctx context.Context, workspaceID uuid.UUID, page, perPage int, search string) ([]Media, int64, error)
}

type mediaService struct {
	repo    MediaRepository
	storage storage.Storage
}

func NewMediaService(repo MediaRepository, storage storage.Storage) MediaService {
	return &mediaService{repo: repo, storage: storage}
}

func (s *mediaService) SaveFile(ctx context.Context, workspaceID uuid.UUID, filename string, fileData []byte, mimeType string, size int64, altText, caption string) (*Media, error) {
	uniqueID := uuid.New()
	cleanFilename := strings.ReplaceAll(filename, " ", "_")
	key := s.storage.GenerateKey(workspaceID.String(), cleanFilename, time.Now())

	if err := s.storage.Upload(ctx, key, fileData, mimeType); err != nil {
		return nil, err
	}

	isImage := strings.HasPrefix(mimeType, "image/") ||
		strings.HasSuffix(strings.ToLower(filename), ".png") ||
		strings.HasSuffix(strings.ToLower(filename), ".jpg") ||
		strings.HasSuffix(strings.ToLower(filename), ".jpeg") ||
		strings.HasSuffix(strings.ToLower(filename), ".gif")

	if isImage {
		reader := bytes.NewReader(fileData)
		img, format, err := image.Decode(reader)
		if err == nil {
			thumbImg := createThumbnail(img)
			thumbKey := "thumb_" + key

			out := new(bytes.Buffer)
			if format == "jpeg" || format == "jpg" {
				_ = jpeg.Encode(out, thumbImg, nil)
			} else if format == "png" {
				_ = png.Encode(out, thumbImg)
			} else if format == "gif" {
				_ = gif.Encode(out, thumbImg, nil)
			}

			_ = s.storage.Upload(ctx, thumbKey, out.Bytes(), mimeType)
		}
	}

	m := &Media{
		ID:          uniqueID,
		WorkspaceID: workspaceID,
		Filename:    cleanFilename,
		Path:        s.storage.GetURL(key),
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

func (s *mediaService) UpdateMedia(ctx context.Context, id uuid.UUID, altText, caption string) (*Media, error) {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	m.AltText = altText
	m.Caption = caption
	if err := s.repo.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *mediaService) DeleteMedia(ctx context.Context, id uuid.UUID) error {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrMediaNotFound
	}

	key := s.storage.ExtractKey(m.Path)
	if key != "" {
		_ = s.storage.Delete(ctx, key)
		thumbKey := "thumb_" + key
		_ = s.storage.Delete(ctx, thumbKey)
	}

	return s.repo.Delete(ctx, id)
}

func (s *mediaService) ListMedia(ctx context.Context, workspaceID uuid.UUID, page, perPage int, search string) ([]Media, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	offset := (page - 1) * perPage
	return s.repo.List(ctx, workspaceID, perPage, offset, search)
}

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
