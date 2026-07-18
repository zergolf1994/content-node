package enums

// ─── Domain Status ───────────────────────────────────────────────────
// ต้อง match กับ vdohide-service/src/core/enums/domain.enum.ts (DomainStatus)

const (
	DomainStatusPending = "pending"
	DomainStatusActive  = "active"
	DomainStatusFailed  = "failed"
	DomainStatusExpired = "expired"
)

// ─── Ads Image ShowOn ────────────────────────────────────────────────
// ต้อง match กับ vdohide-service/src/core/enums/domain.enum.ts (AdsImageShowOn)

const (
	AdsImageShowOnReady = "ready"
	AdsImageShowOnEnd   = "end"
	AdsImageShowOnPause = "pause"
)
