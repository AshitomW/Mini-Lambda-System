package service

import (
	"context"
	"io"

	"AshitomW/mini-lambda/internal/runner"
)

// ImageService encapsulates operations on Docker container images.
type ImageService struct {
	runner runner.ContainerRunner
}

// NewImageService initializes an ImageService instance.
func NewImageService(runner runner.ContainerRunner) *ImageService {
	return &ImageService{runner: runner}
}

// UploadImage streams a tar archive to the underlying container engine.
func (s *ImageService) UploadImage(ctx context.Context, reader io.Reader) error {
	return s.runner.LoadImage(ctx, reader)
}

// ListImages returns the tags of all locally cached images.
func (s *ImageService) ListImages(ctx context.Context) ([]string, error) {
	return s.runner.ListImages(ctx)
}
