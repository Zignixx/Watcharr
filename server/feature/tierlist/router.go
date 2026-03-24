package tierlist

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sbondCo/Watcharr/feature/auth/authmiddleware"
	"github.com/sbondCo/Watcharr/router"
)

type Router struct {
	br *router.BaseRouter
	s  *Service
}

func NewRouter(br *router.BaseRouter, service *Service) *Router {
	return &Router{br: br, s: service}
}

func (r *Router) AddRoutes() {
	// Public route (no auth required)
	tlPublic := r.br.Router.Group("/tierlist")
	tlPublic.GET("/:id/:username", r.GetPublicTierlist)

	tl := r.br.Router.Group("/tierlist").Use(authmiddleware.AuthRequired(nil, r.br.Cfg))
	tl.GET("", r.GetTierlist)
	tl.POST("/tier", r.CreateTier)
	tl.PUT("/tier/:id", r.UpdateTier)
	tl.DELETE("/tier/:id", r.DeleteTier)
	tl.PUT("/reorder", r.ReorderTiers)
	tl.PUT("/items", r.UpdateTierItems)
	tl.POST("/sync-ratings", r.SyncRatings)
	tl.POST("/defaults", r.CreateDefaultTiers)
	tl.GET("/untiered", r.GetUntieredWatched)
	tl.GET("/presets", r.GetPresets)
	tl.POST("/preset", r.CreatePreset)
	tl.DELETE("/preset/:id", r.DeletePreset)
}

func (r *Router) GetTierlist(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	tiers, err := r.s.GetTiers(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to get tierlist"})
		return
	}
	c.JSON(http.StatusOK, tiers)
}

func (r *Router) CreateTier(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	var req CreateTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid request"})
		return
	}
	tier, err := r.s.CreateTier(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to create tier"})
		return
	}
	c.JSON(http.StatusOK, tier)
}

func (r *Router) UpdateTier(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	tierID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid tier id"})
		return
	}
	var req UpdateTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid request"})
		return
	}
	if err := r.s.UpdateTier(userID, uint(tierID), req); err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to update tier"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (r *Router) DeleteTier(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	tierID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid tier id"})
		return
	}
	if err := r.s.DeleteTier(userID, uint(tierID)); err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to delete tier"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (r *Router) ReorderTiers(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	var req ReorderTiersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid request"})
		return
	}
	if err := r.s.ReorderTiers(userID, req.TierIDs); err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to reorder tiers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (r *Router) UpdateTierItems(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	var req UpdateTierItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid request"})
		return
	}
	if err := r.s.UpdateTierItems(userID, req.Items); err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to update tier items"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (r *Router) SyncRatings(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	if err := r.s.SyncRatings(userID); err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to sync ratings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (r *Router) CreateDefaultTiers(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	tiers, err := r.s.CreateDefaultTiers(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to create default tiers"})
		return
	}
	c.JSON(http.StatusOK, tiers)
}

func (r *Router) GetUntieredWatched(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	watched, err := r.s.GetUntieredWatched(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to get untiered watched"})
		return
	}
	c.JSON(http.StatusOK, watched)
}

func (r *Router) GetPresets(c *gin.Context) {
	presets, err := r.s.GetAllPresets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to get presets"})
		return
	}
	c.JSON(http.StatusOK, presets)
}

func (r *Router) CreatePreset(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	var req CreatePresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid request"})
		return
	}
	preset, err := r.s.CreatePreset(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to create preset"})
		return
	}
	c.JSON(http.StatusOK, preset)
}

func (r *Router) DeletePreset(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	presetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid preset id"})
		return
	}
	if err := r.s.DeletePreset(userID, uint(presetID)); err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to delete preset"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (r *Router) GetPublicTierlist(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid user id"})
		return
	}
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "username required"})
		return
	}
	tiers, err := r.s.GetPublicTiers(uint(userID), username)
	if err != nil {
		c.JSON(http.StatusForbidden, router.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, tiers)
}
