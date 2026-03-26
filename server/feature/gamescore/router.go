package gamescore

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/feature/auth/authmiddleware"
	"github.com/sbondCo/Watcharr/router"
)

type Router struct {
	br      *router.BaseRouter
	service *Service
}

func NewRouter(br *router.BaseRouter, service *Service) *Router {
	return &Router{br, service}
}

func (r *Router) AddRoutes() {
	gs := r.br.Router.Group("/gamescore").Use(authmiddleware.AuthRequired(nil, r.br.Cfg))
	gs.POST("", r.SaveScore)
	gs.GET("", r.GetScores)
}

func (r *Router) SaveScore(c *gin.Context) {
	userId := c.MustGet("userId").(uint)
	var req domain.GameScoreSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: err.Error()})
		return
	}
	if err := r.service.SaveScore(userId, req); err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to save score"})
		return
	}
	c.Status(http.StatusOK)
}

func (r *Router) GetScores(c *gin.Context) {
	userId := c.MustGet("userId").(uint)
	scores, err := r.service.GetScores(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "failed to get scores"})
		return
	}
	c.JSON(http.StatusOK, scores)
}
