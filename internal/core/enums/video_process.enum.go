package enums

// ─── Video Process Statuses ──────────────────────────────────────────
// Must match VideoProcessStatus in vdohide-service (video-process.enum.ts).

const (
	VideoProcessStatusPending    = "pending"
	VideoProcessStatusProcessing = "processing"
	VideoProcessStatusCompleted  = "completed"
	VideoProcessStatusFailed     = "failed"
	VideoProcessStatusCancelled  = "cancelled"
)
