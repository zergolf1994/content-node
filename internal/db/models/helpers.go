package models

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
