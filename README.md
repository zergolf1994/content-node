# Content Node

HTTP service เสิร์ฟ content ทั้งหมดของ [VdoHide](https://vdohide.xyz) — HLS playlist, รูป (poster/sprite/resize), stream ไฟล์, ซับไตเติล, VAST โฆษณา และ player feeds (`/playlist/*.json`, `/advert/*.json`)

> แทนที่ `server-content` เดิมทั้งตัว — **ไม่ใช่ worker** เป็น service ที่ต้องออนไลน์ตลอด (player ดึงทุกอย่างผ่านตัวนี้) รันคู่กับ player-node บนเครื่องเดียวกันได้ (คนละ port คนละ systemd unit)

## Routes

### Playlist / Stream
1. **`/{fileSlug}/playlist.m3u8`** — master playlist
   — lookup `files` ด้วย slug → รวม `medias` (type video, resolution original/1080/720/480/360)
   — ดึง `#EXT-X-STREAM-INF` จริงจาก storage ถ้าดึงไม่ได้ใช้ค่า estimate — มี resolution มาตรฐานแล้วซ่อน `original`
2. **`/{mediaSlug}/video.m3u8`** — video segment playlist: ดึงจาก storage แล้ว rewrite URL segment เป็น `https://{publicUrl}/...` (round-robin หลาย domain)
3. **`/{mediaSlug}/audio.m3u8`** — audio-only segment playlist สำหรับ separated media layout
4. **`/{fileSlug}.{ext}`** — proxy stream ไฟล์จาก storage รองรับ Range / seek — file เป็น video → เสิร์ฟ thumbnail แทน, เป็นรูป + มี `?w=&h=&fit=&q=` → resize on-the-fly, `.vtt` = ซับไตเติล ผ่านเส้นนี้เช่นกัน

### Images
5. **`/thumb/{slug}/{sec}.jpg`** — poster เฟรมที่วินาทีนั้น (proxy nginx-vod thumb ของ storage); ใช้ `poster.jpg` เพื่อเลือกเวลากึ่งกลางวิดีโอ หรือวินาที `0` เมื่อไม่มี duration
6. **`/{slug}/sprite/sprite.vtt`** + **`/{slug}/sprite/{n}.jpg`** — thumbnail track ตอน hover seek bar
   — 404 ของ path รูปตอบเป็น PNG placeholder 200x200 (ไม่ใช่ XML)

### Ads
7. **`/vast/{slug}.xml`** — VAST ของ custom domain (เช็ค enable + status active) / **`/vast/hobby.xml`** — VAST default จาก setting `advert_hobby`

### Player feeds
8. **`/playlist/{fileSlug}.json`** — JW Player playlist feed (title / poster / playlist.m3u8 / sprite track) — ย้ายมาจาก player-node
9. **`/advert/{adSlug}.json`** — unified advert feed (script + image + video) — `hobby` หรือ slug ของ custom domain — ย้ายมาจาก player-node

### Misc
10. **`/health`** — status + uptime

## Sync Scheduler

ตอน start + ทุก 1 นาที sync จาก MongoDB → `conf/*.json` + in-memory cache:
`settings` → `conf/setting.json`, `custom_domains` → `conf/domains.json`, `workspaces` → `conf/spaces.json`, `ads` (active) → `conf/ads.json`
— VAST / advert feed / playlist feed อ่านจาก cache พวกนี้ ไม่ query DB ต่อ request

## Redis Response Cache (optional)

ตั้ง `REDIS_URL` = เปิด cache กัน DB โดน hit ซ้ำๆ — **TTL 300s ทุก key**:

| Route | Redis key | เก็บอะไร |
|---|---|---|
| `/{slug}/playlist.m3u8` | `playlist_master:{slug}` | response (เล็ก ~0.4KB) |
| `/{slug}/video.m3u8` | `playlist_video_v4:{slug}` | **เฉพาะ video lookup** `{playbackBaseUrl, publicDomains, playlistName}` — storage ใช้ `index.m3u8` รองรับทั้ง muxed/video-only |
| `/{slug}/audio.m3u8` | `playlist_audio_v2:{slug}` | **เฉพาะ audio lookup** `{playbackBaseUrl, publicDomains, playlistName}` — body สร้างสดทุกครั้ง |
| `/playlist/{slug}.json` | `playlist_json:{slug}` | response (เล็ก ~0.5KB) |
| `/advert/{slug}.json` | `advert:{slug}` | response (เล็ก ~0.3KB) |

- key ตาม slug ล้วนๆ ไม่แยกโดเมน (content-node ไม่ตรวจโดเมน response เหมือนกันทุก host)
- response cache เก็บเฉพาะ 200 + body ≤ 512KB / hit จะมี header `X-Cache: HIT`
- ไม่ตั้ง `REDIS_URL` หรือ Redis ล่ม = ข้าม cache ทำงานแบบเดิม (fail-open)
- stream/รูป ไม่ผ่าน Redis — ปล่อยให้ CDN/nginx จัดการ

## Requirements

- **MongoDB** (vdohide platform database) — ตั้งผ่าน `DATABASE_URL`
- storage-node/nginx-vod รันอยู่หน้า storage; local resolve จาก `storages.local` ส่วน S3 resolve playback ผ่าน `storages.publicUrl`

---

## Installation (Linux Server)

```bash
curl -fsSL https://raw.githubusercontent.com/zergolf1994/content-node/main/install.sh | sudo -E bash -s -- \
    --database-url "mongodb+srv://user:pass@cluster.mongodb.net/platform"
```

| Option | Default | คำอธิบาย |
|---|---|---|
| `--database-url` | `""` | MongoDB connection string (`DATABASE_URL`) |
| `--port` | `8082` | HTTP port (player-node ใช้ 8081) |
| `--uninstall` | — | ถอนการติดตั้ง |

```bash
journalctl -u content-node -f          # ดู logs
systemctl restart content-node         # restart
curl http://localhost:8082/health      # health check
```

## Configuration (.env)

```env
DATABASE_URL=mongodb+srv://user:pass@cluster.mongodb.net/platform
PORT=8082
DOMAIN_STATIC=
LOG_PATH=logs/content-node.log

# Optional — Redis response cache (ไม่ตั้ง = ไม่ใช้, Redis ล่ม = ข้าม cache ไม่พังเว็บ)
REDIS_URL=redis://localhost:6379/0
```

## Development

```bash
go run ./cmd     # ต้องมี .env (DATABASE_URL)
build.bat        # Windows exe + copy .env → .build/
```

## Release

```bash
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions build + release อัตโนมัติ: `linux` (amd64), `linux-arm64`

---

## Collections Used

| Collection | การใช้งาน |
|---|---|
| `files` | lookup ด้วย slug (playlist, stream) |
| `medias` | resolution ของ file + segment playlist + thumbnail |
| `storages` | local host หรือ S3 publicUrl สำหรับ fetch m3u8/thumb/sprite และ rewrite segment |
| `custom_domains` | VAST ราย domain + advert feed + domain/space check (sync → cache) |
| `workspaces` | plan ของ space (hobby/pro) สำหรับ resolve ad slug (sync → cache) |
| `ads` | ตัวโฆษณา (video/image/script) ที่ผูกกับ space (sync → cache) |
| `settings` | `advert_hobby`, `player_maintenance`, `domain_*` (sync → `conf/setting.json`) |
| `video_process` | สถานะ processing ตอน playlist feed ยังไม่พร้อม |

> ⚠ **Index เป็นของฝั่ง vdohide-service (mongoose)** — repo นี้ไม่สร้าง index เอง
> ⚠ enum ใน `internal/core/enums/` ต้อง match กับ `vdohide-service/src/core/enums/`
