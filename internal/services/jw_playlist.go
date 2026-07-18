package services

// JWPlaylistFeed is the JSON feed format consumed by JW Player's playlist URL.
type JWPlaylistFeed struct {
	Playlist []JWPlaylistItem `json:"playlist"`
}

// JWPlaylistItem is a single media item in the JW playlist feed.
type JWPlaylistItem struct {
	Title   string             `json:"title"`
	MediaID string             `json:"mediaid,omitempty"`
	Image   string             `json:"image,omitempty"`
	Sources []JWPlaylistSource `json:"sources"`
	Tracks  []JWPlaylistTrack  `json:"tracks,omitempty"`
}

// JWPlaylistSource is a media source entry.
type JWPlaylistSource struct {
	File string `json:"file"`
	Type string `json:"type,omitempty"`
}

// JWPlaylistTrack is a text track (thumbnails, captions, chapters).
type JWPlaylistTrack struct {
	File string `json:"file"`
	Kind string `json:"kind"`
}

// BuildJWPlaylistFeed builds the JW Player playlist JSON for a single video.
func BuildJWPlaylistFeed(title, slug, posterURL, playlistM3U8, spriteVttURL string) JWPlaylistFeed {
	item := JWPlaylistItem{
		Title:   title,
		MediaID: slug,
		Image:   posterURL,
		Sources: []JWPlaylistSource{{
			File: playlistM3U8,
			Type: "application/vnd.apple.mpegurl",
		}},
	}

	if spriteVttURL != "" {
		item.Tracks = []JWPlaylistTrack{{
			File: spriteVttURL,
			Kind: "thumbnails",
		}}
	}

	return JWPlaylistFeed{Playlist: []JWPlaylistItem{item}}
}
