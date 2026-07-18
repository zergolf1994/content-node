package models

import (
	"time"

	"github.com/zergolf1994/goose"
)

// WorkspaceMetadata holds metadata embedded in a Workspace.
type WorkspaceMetadata struct {
	BillingID *string    `bson:"billingId,omitempty" json:"billingId,omitempty" goose:"ref:user"`
	DeletedAt *time.Time `bson:"deletedAt,omitempty" json:"deletedAt,omitempty"`
	DeletedBy *string    `bson:"deletedBy,omitempty" json:"deletedBy,omitempty" goose:"ref:user"`
}

// WorkspaceCapacity holds storage capacity stats embedded in a Workspace.
type WorkspaceCapacity struct {
	Total       interface{} `bson:"total" json:"total"`
	Used        interface{} `bson:"used" json:"used"`
	Free        interface{} `bson:"free" json:"free"`
	Percentage  float64     `bson:"percentage" json:"percentage"`
	LastUpdated *time.Time  `bson:"lastUpdated,omitempty" json:"lastUpdated,omitempty"`
}

// WorkspacePlan holds the subscription plan for a workspace.
type WorkspacePlan struct {
	PlanType     string      `bson:"planType" json:"planType" goose:"default:hobby"`
	StorageLimit interface{} `bson:"storageLimit,omitempty" json:"storageLimit,omitempty"`
	PriceTotal   *float64    `bson:"priceTotal,omitempty" json:"priceTotal,omitempty"`
	AdsEnabled   bool        `bson:"adsEnabled" json:"adsEnabled"`
	ExpiresAt    *time.Time  `bson:"expiresAt,omitempty" json:"expiresAt,omitempty"`
	DowngradeAt  *time.Time  `bson:"downgradeAt,omitempty" json:"downgradeAt,omitempty"`
}

// WorkspaceSettings holds settings embedded in a Workspace.
type WorkspaceSettings struct {
	RequestToJoin bool `bson:"requestToJoin" json:"requestToJoin"`
}

// Workspace represents a workspace record.
// Collection: "workspaces" | _id: String (UUID)
type Workspace struct {
	ID        string             `bson:"_id" json:"id" goose:"required,default:uuid"`
	CreatorID string             `bson:"creatorId" json:"creatorId" goose:"ref:user,index,required"`
	Status    string             `bson:"status" json:"status" goose:"default:pending"`
	Name      string             `bson:"name" json:"name" goose:"required"`
	Slug      string             `bson:"slug" json:"slug" goose:"unique,default:random(11)"`
	Image     *string            `bson:"image,omitempty" json:"image,omitempty"`
	Metadata  *WorkspaceMetadata `bson:"metadata,omitempty" json:"metadata,omitempty"`
	Capacity  *WorkspaceCapacity `bson:"capacity,omitempty" json:"capacity,omitempty"`
	Plan      *WorkspacePlan     `bson:"plan,omitempty" json:"plan,omitempty"`
	Settings  *WorkspaceSettings `bson:"settings,omitempty" json:"settings,omitempty"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt" goose:"default:now"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt" goose:"default:now"`
}

// WorkspaceModel is the goose model for the "workspaces" collection.
var WorkspaceModel = goose.NewModel[Workspace]("workspaces")
