package models

import (
	"path"
	"strings"
)

// ─── Media helpers ───────────────────────────────────────────

// EffectiveFileID returns ClonedFrom if set, otherwise FileID.
// Cloned media shares the original file, so paths must resolve to the source.
func (m *Media) EffectiveFileID() string {
	if m.ClonedFrom != nil && *m.ClonedFrom != "" {
		return *m.ClonedFrom
	}
	if m.FileID != nil {
		return *m.FileID
	}
	return ""
}

// ObjectPath returns the persisted object key, with a legacy fallback for
// cloned media created before path became mandatory.
func (m *Media) ObjectPath() string {
	if m.Path != nil && strings.TrimSpace(*m.Path) != "" {
		return strings.TrimLeft(strings.TrimSpace(*m.Path), "/")
	}
	fileName := ""
	if m.FileName != nil {
		fileName = strings.TrimSpace(*m.FileName)
	}
	if m.EffectiveFileID() == "" || fileName == "" {
		return ""
	}
	return path.Join(m.EffectiveFileID(), fileName)
}

// GetFilePath returns the expected file path on storage.
// Structure: {storagePath}/{fileId}/{file_name}
func (m *Media) GetFilePath(storagePath string) string {
	fileName := ""
	if m.FileName != nil {
		fileName = *m.FileName
	}
	return storagePath + "/" + m.EffectiveFileID() + "/" + fileName
}

// ─── File helpers ────────────────────────────────────────────

// IsTrashed checks if the file has been trashed.
func (f *File) IsTrashed() bool {
	return f.Metadata != nil && f.Metadata.TrashedAt != nil
}

// IsDeleted checks if the file has been soft-deleted.
func (f *File) IsDeleted() bool {
	return f.Metadata != nil && f.Metadata.DeletedAt != nil
}
