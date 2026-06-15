package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"tala/internal/domain"
)

const MaxImageUploadBytes int64 = 10 * 1024 * 1024

type UploadedImageFile struct {
	File        *os.File
	ContentType string
	Size        int64
}

func UploadDirForDBPath(dbPath string) string {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" || dbPath == ":memory:" {
		return filepath.Join(".tala", "uploads", "images")
	}
	if strings.HasPrefix(dbPath, "file:") {
		return filepath.Join(".tala", "uploads", "images")
	}
	return filepath.Join(filepath.Dir(dbPath), "uploads", "images")
}

func (s *Service) UploadImage(ctx context.Context, username, originalName string, src io.Reader) (domain.UploadedImage, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return domain.UploadedImage{}, domain.NewError(domain.CodeMissingUsername, "Username is required for this operation.", "username")
	}
	if strings.TrimSpace(s.uploadDir) == "" {
		return domain.UploadedImage{}, domain.NewError(domain.CodeInternal, "Image upload storage is not configured.", "image")
	}
	if src == nil {
		return domain.UploadedImage{}, domain.NewError(domain.CodeValidationError, "Image file is required.", "image")
	}
	data, err := io.ReadAll(io.LimitReader(src, MaxImageUploadBytes+1))
	if err != nil {
		return domain.UploadedImage{}, err
	}
	if len(data) == 0 {
		return domain.UploadedImage{}, domain.NewError(domain.CodeValidationError, "Image file is required.", "image")
	}
	if int64(len(data)) > MaxImageUploadBytes {
		return domain.UploadedImage{}, domain.NewError(domain.CodeValidationError, "Image file is too large.", "image")
	}
	contentType, ext, ok := sniffImage(data)
	if !ok {
		return domain.UploadedImage{}, domain.NewError(domain.CodeValidationError, "Unsupported image type.", "image")
	}
	if err := os.MkdirAll(s.uploadDir, 0o755); err != nil {
		return domain.UploadedImage{}, err
	}
	filename := "image_" + uuid.NewString() + ext
	path := filepath.Join(s.uploadDir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return domain.UploadedImage{}, err
	}
	uploaded := domain.UploadedImage{
		URL:         "/uploads/images/" + filename,
		Filename:    filename,
		ContentType: contentType,
		Size:        int64(len(data)),
	}
	uploaded.Markdown = imageMarkdown(uploaded.URL, originalName)
	return uploaded, nil
}

func (s *Service) UploadImageFile(ctx context.Context, username, path, altText string) (domain.UploadedImage, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return domain.UploadedImage{}, domain.NewError(domain.CodeValidationError, "Image path is required.", "path")
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.UploadedImage{}, domain.NewError(domain.CodeNotFound, "Image file not found.", "path")
		}
		return domain.UploadedImage{}, err
	}
	defer file.Close()
	name := strings.TrimSpace(altText)
	if name == "" {
		name = filepath.Base(path)
	}
	return s.UploadImage(ctx, username, name, file)
}

func (s *Service) OpenUploadedImage(filename string) (UploadedImageFile, error) {
	filename = strings.TrimSpace(filename)
	if strings.TrimSpace(s.uploadDir) == "" {
		return UploadedImageFile{}, domain.NewError(domain.CodeNotFound, "Image not found.", "filename")
	}
	if filename == "" || filename != filepath.Base(filename) || strings.Contains(filename, "/") || strings.Contains(filename, `\`) {
		return UploadedImageFile{}, domain.NewError(domain.CodeNotFound, "Image not found.", "filename")
	}
	path := filepath.Join(s.uploadDir, filename)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return UploadedImageFile{}, domain.NewError(domain.CodeNotFound, "Image not found.", "filename")
		}
		return UploadedImageFile{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return UploadedImageFile{}, err
	}
	if stat.IsDir() {
		_ = file.Close()
		return UploadedImageFile{}, domain.NewError(domain.CodeNotFound, "Image not found.", "filename")
	}
	header := make([]byte, 512)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		_ = file.Close()
		return UploadedImageFile{}, err
	}
	if _, err := file.Seek(0, 0); err != nil {
		_ = file.Close()
		return UploadedImageFile{}, err
	}
	contentType, _, ok := sniffImage(header[:n])
	if !ok {
		contentType = "application/octet-stream"
	}
	return UploadedImageFile{File: file, ContentType: contentType, Size: stat.Size()}, nil
}

func sniffImage(data []byte) (contentType, ext string, ok bool) {
	if len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp", ".webp", true
	}
	contentType = http.DetectContentType(data)
	switch contentType {
	case "image/png":
		return contentType, ".png", true
	case "image/jpeg":
		return contentType, ".jpg", true
	case "image/gif":
		return contentType, ".gif", true
	default:
		return "", "", false
	}
}

func imageMarkdown(url, altText string) string {
	altText = strings.TrimSpace(altText)
	if altText == "" {
		altText = "uploaded image"
	}
	altText = strings.NewReplacer("[", "(", "]", ")", "\n", " ", "\r", " ").Replace(altText)
	return fmt.Sprintf("![%s](%s)", altText, url)
}
