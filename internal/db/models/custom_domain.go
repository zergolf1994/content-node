package models

import (
	"time"

	"github.com/zergolf1994/goose"
)

// PlayerConfig holds video player configuration for a custom domain.
type PlayerConfig struct {
	Skin            string  `bson:"skin" json:"skin"`
	LogoImageURL    *string `bson:"logoImageUrl,omitempty" json:"logoImageUrl,omitempty"`
	LogoWebsiteURL  *string `bson:"logoWebsiteUrl,omitempty" json:"logoWebsiteUrl,omitempty"`
	LogoPosition    *string `bson:"logoPosition,omitempty" json:"logoPosition,omitempty"`
	PosterURL       *string `bson:"posterUrl,omitempty" json:"posterUrl,omitempty"`
	BaseColor       string  `bson:"baseColor" json:"baseColor"`
	DisplayTitle    bool    `bson:"displayTitle" json:"displayTitle"`
	AutoPlay        bool    `bson:"autoPlay" json:"autoPlay"`
	MuteSound       bool    `bson:"muteSound" json:"muteSound"`
	RepeatVideo     bool    `bson:"repeatVideo" json:"repeatVideo"`
	ContinuePlay    bool    `bson:"continuePlay" json:"continuePlay"`
	ContinuePlayArk bool    `bson:"continuePlayArk" json:"continuePlayArk"`
	Sharing         bool    `bson:"sharing" json:"sharing"`
	Captions        bool    `bson:"captions" json:"captions"`
	PlaybackRate    bool    `bson:"playbackRate" json:"playbackRate"`
	Keyboard        bool    `bson:"keyboard" json:"keyboard"`
	Download        bool    `bson:"download" json:"download"`
	PIP             bool    `bson:"pip" json:"pip"`
	ShowPreviewTime bool    `bson:"showPreviewTime" json:"showPreviewTime"`
	FastForward     bool    `bson:"fastForward" json:"fastForward"`
	Rewind          bool    `bson:"rewind" json:"rewind"`
	SeekStep        int     `bson:"seekStep" json:"seekStep"`
}

// AdContent holds a single embedded advert entry (video, image, or script).
type AdContent struct {
	ID          string   `bson:"_id,omitempty" json:"_id,omitempty"`
	Enabled     bool     `bson:"enabled" json:"enabled"`
	Name        string   `bson:"name" json:"name"`
	MP4URL      *string  `bson:"mp4Url,omitempty" json:"mp4Url,omitempty"`
	SkipSeconds *int     `bson:"skipSeconds,omitempty" json:"skipSeconds,omitempty"`
	ImageURL    *string  `bson:"imageUrl,omitempty" json:"imageUrl,omitempty"`
	ShowOn      []string `bson:"showOn,omitempty" json:"showOn,omitempty"`
	WebsiteURL  *string  `bson:"websiteUrl,omitempty" json:"websiteUrl,omitempty"`
	Script      *string  `bson:"script,omitempty" json:"script,omitempty"`
}

// DomainAdvertCategory groups enabled flag with a list of adverts.
type DomainAdvertCategory struct {
	Enabled bool        `bson:"enabled" json:"enabled"`
	List    []AdContent `bson:"list" json:"list"`
}

// DomainAdverts holds embedded video, image, and script adverts on a domain.
type DomainAdverts struct {
	Video  DomainAdvertCategory `bson:"video" json:"video"`
	Image  DomainAdvertCategory `bson:"image" json:"image"`
	Script DomainAdvertCategory `bson:"script" json:"script"`
}

// DomainAds holds legacy advertisement configuration (Ad document ID lists).
type DomainAds struct {
	Video  []string `bson:"video" json:"video"`
	Image  []string `bson:"image" json:"image"`
	Script []string `bson:"script" json:"script"`
}

// DomainDNS holds DNS configuration for domain verification.
type DomainDNS struct {
	RecordType        string     `bson:"recordType" json:"recordType"`
	Value             string     `bson:"value" json:"value"`
	TTL               int        `bson:"ttl" json:"ttl"`
	VerificationToken string     `bson:"verificationToken" json:"verificationToken"`
	RetryCount        int        `bson:"retryCount" json:"retryCount"`
	LastVerified      *time.Time `bson:"lastVerified,omitempty" json:"lastVerified,omitempty"`
	Reason            *string    `bson:"reason,omitempty" json:"reason,omitempty"`
}

// CustomDomain represents a custom domain with player/ad config.
// Collection: "custom_domains" | _id: String (UUID)
type CustomDomain struct {
	ID        string         `bson:"_id" json:"id" goose:"required,default:uuid"`
	Enable    bool           `bson:"enable" json:"enable"`
	Name      string         `bson:"name" json:"name" goose:"required,unique"`
	Status    string         `bson:"status" json:"status" goose:"default:pending"` // pending, active, failed, expired
	CreatorID *string        `bson:"creatorId,omitempty" json:"creatorId,omitempty" goose:"ref:user,index"`
	SpaceID   *string        `bson:"spaceId,omitempty" json:"spaceId,omitempty" goose:"ref:workspaces,index"`
	Slug      string         `bson:"slug" json:"slug" goose:"unique,default:random(11),index"`
	DNS       *DomainDNS     `bson:"dns,omitempty" json:"dns,omitempty"`
	Player    *PlayerConfig  `bson:"player,omitempty" json:"player,omitempty"`
	Adverts   *DomainAdverts `bson:"adverts,omitempty" json:"adverts,omitempty"`
	Ads       *DomainAds     `bson:"ads,omitempty" json:"ads,omitempty"`
	CreatedAt time.Time      `bson:"createdAt" json:"createdAt" goose:"default:now"`
	UpdatedAt time.Time      `bson:"updatedAt" json:"updatedAt" goose:"default:now"`
}

// CustomDomainModel is the goose model for the "custom_domains" collection.
var CustomDomainModel = goose.NewModel[CustomDomain]("custom_domains")
