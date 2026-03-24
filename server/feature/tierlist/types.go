package tierlist

// Request types for tierlist API

type CreateTierRequest struct {
	Name      string `json:"name" binding:"required"`
	Color     string `json:"color" binding:"required"`
	TextColor string `json:"textColor" binding:"required"`
}

type UpdateTierRequest struct {
	Name      string `json:"name" binding:"required"`
	Color     string `json:"color" binding:"required"`
	TextColor string `json:"textColor" binding:"required"`
}

type ReorderTiersRequest struct {
	TierIDs []uint `json:"tierIds" binding:"required"`
}

type UpdateTierItemsRequest struct {
	// Map of tierID -> ordered list of watchedIDs
	Items map[uint][]uint `json:"items" binding:"required"`
}

type CreatePresetRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Tiers       []PresetTierEntry `json:"tiers" binding:"required,min=1"`
}

type PresetTierEntry struct {
	Name      string `json:"name" binding:"required"`
	Color     string `json:"color" binding:"required"`
	TextColor string `json:"textColor" binding:"required"`
}
