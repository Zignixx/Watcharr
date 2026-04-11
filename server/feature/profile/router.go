package profile

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sbondCo/Watcharr/feature/auth/authmiddleware"
	"github.com/sbondCo/Watcharr/router"
)

type Router struct {
	br      *router.BaseRouter
	service *Service
}

func NewRouter(br *router.BaseRouter, service *Service) *Router {
	return &Router{
		br,
		service,
	}
}

func (r *Router) AddRoutes() {
	// Public route
	r.br.Router.GET("/profile/:userId/:username", r.GetPublicProfile)

	profile := r.br.Router.Group("/profile").Use(authmiddleware.AuthRequired(nil, r.br.Cfg))

	// Get user profile details
	profile.GET("", r.GetProfile)
}

// Get user profile details
func (r *Router) GetProfile(c *gin.Context) {
	userId := c.MustGet("userId").(uint)
	response, err := r.service.getProfile(userId)
	if err != nil {
		c.JSON(http.StatusForbidden, router.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// Get public user profile details
func (r *Router) GetPublicProfile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid user id"})
		return
	}
	response, err := r.service.getPublicProfile(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, router.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}
