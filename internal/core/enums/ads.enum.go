package enums

// ─── Ads Type / Status ───────────────────────────────────────────────
// ค่าตรงกับที่ใช้ใน collection "ads" (admin ฝั่ง vdohide เป็นคนเขียน)

const (
	AdsTypeVideo  = "video"
	AdsTypeImage  = "image"
	AdsTypeScript = "script"

	AdsStatusActive = "active"
)

// ─── Setting Keys ────────────────────────────────────────────────────

const (
	// advert_hobby = {vdo, image, javascript} — รายการ Ad IDs สำหรับ hobby plan
	SettingAdvertHobby = "advert_hobby"
)
