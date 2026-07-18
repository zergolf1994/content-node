package cache

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// ─── Optional Redis response cache ───────────────────────────
// มี REDIS_URL = เปิดใช้ / ไม่มี = ปิด (ทุกฟังก์ชันเป็น no-op)
// Redis ล่มกลางทาง = fail-open: cache miss เฉยๆ แล้วไปยิง DB ตามปกติ
// ห้ามให้ cache เป็นเหตุให้หน้าเว็บพัง

var client *redis.Client

// Init connects to Redis from a URL (redis://[:pass@]host:port/db).
// Empty url = cache disabled. Connection failure = disabled + warning
// (the service must still run without Redis).
func Init(url string) {
	if url == "" {
		log.Println("📦 Redis cache disabled (no REDIS_URL)")
		return
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("⚠️ REDIS_URL invalid — cache disabled: %v", err)
		return
	}
	c := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		log.Printf("⚠️ Redis unreachable — cache disabled: %v", err)
		return
	}
	client = c
	log.Printf("📦 Redis cache enabled (%s)", opt.Addr)
}

func Enabled() bool { return client != nil }

// ─── Lookup cache (ค่าเล็กๆ ที่ resolve จาก DB) ───────────────
// ใช้กับ route ที่ body ใหญ่ (เช่น video.m3u8 หลาย KB) — เก็บเฉพาะผล
// lookup ไว้ข้าม DB ส่วนตัว response สร้างสดทุกครั้ง (CF cache ปลายทางแล้ว)

// GetJSON reads key into v. Returns false on miss/disabled/error.
func GetJSON(key string, v interface{}) bool {
	if client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	raw, err := client.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(raw, v) == nil
}

// SetJSON stores v under key with the standard TTL (fire-and-forget).
func SetJSON(key string, v interface{}) {
	if client == nil {
		return
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Set(ctx, key, raw, TTL).Err(); err != nil {
		log.Printf("⚠️ Redis set failed: %v", err)
	}
}

// ─── Response cache ──────────────────────────────────────────

// entry — response ที่เก็บลง Redis (body เป็น []byte, JSON encode = base64)
type entry struct {
	Status      int    `json:"s"`
	ContentType string `json:"ct"`
	Body        []byte `json:"b"`
}

const maxCacheBody = 512 * 1024 // เกินนี้ไม่ cache (กันของใหญ่หลงเข้ามา)

// recorder จับ response ของ handler จริงไว้ก่อนส่งออก
type recorder struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (rec *recorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *recorder) Write(b []byte) (int, error) {
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	if len(rec.body)+len(b) <= maxCacheBody {
		rec.body = append(rec.body, b...)
	} else {
		rec.body = nil // ใหญ่เกิน — เลิกเก็บ
	}
	return rec.ResponseWriter.Write(b)
}

// TTL — อายุ cache ทุก route (300s ตามที่ตกลง)
const TTL = 300 * time.Second

// Serve — cache ชั้น response: hit = ตอบจาก Redis ไม่แตะ DB เลย
// key ตาม slug ล้วนๆ (เช่น "playlist_master/{slug}") — content-node ไม่ตรวจ
// โดเมนแล้ว response เหมือนกันทุก host ไม่ต้องแยก cache ตามโดเมน
func Serve(w http.ResponseWriter, r *http.Request, key string, next http.HandlerFunc) {
	if client == nil || r.Method != http.MethodGet {
		next(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	raw, err := client.Get(ctx, key).Bytes()
	cancel()
	if err == nil {
		var e entry
		if json.Unmarshal(raw, &e) == nil && e.Status == http.StatusOK {
			if e.ContentType != "" {
				w.Header().Set("Content-Type", e.ContentType)
			}
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("X-Cache", "HIT")
			w.Write(e.Body)
			return
		}
	}

	rec := &recorder{ResponseWriter: w}
	next(rec, r)

	// เก็บเฉพาะ 200 ที่ body ไม่ใหญ่เกิน — ตอบ client ไปแล้ว เก็บแบบ fire-and-forget
	if rec.status == http.StatusOK && len(rec.body) > 0 {
		e := entry{
			Status:      rec.status,
			ContentType: rec.Header().Get("Content-Type"),
			Body:        rec.body,
		}
		if raw, err := json.Marshal(e); err == nil {
			sCtx, sCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer sCancel()
			if err := client.Set(sCtx, key, raw, TTL).Err(); err != nil {
				log.Printf("⚠️ Redis set failed: %v", err)
			}
		}
	}
}
