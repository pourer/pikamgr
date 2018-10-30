package handler

import (
	"net/http"

	"github.com/pourer/pikamgr/protocol"

	"github.com/gin-gonic/gin"
)

type AggService interface {
	Overview() (*protocol.Overview, error)
	Topom() (*protocol.Topom, error)
	Stats() (*protocol.Stats, error)
}

type aggHandler struct {
	s AggService
}

func InitAggHandler(s AggService, router, apiRouter gin.IRouter) {
	h := &aggHandler{s: s}

	r := router.Group("/topom")
	r.GET("", h.Overview)
	r.GET("/model", h.Topom)
	r.GET("/stats", h.Stats)

	apiRouter.GET("/stats/:xauth", h.Stats)
}

func (h *aggHandler) Overview(ctx *gin.Context) {
	if data, err := h.s.Overview(); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.IndentedJSON(http.StatusOK, data)
	}
}

func (h *aggHandler) Topom(ctx *gin.Context) {
	if data, err := h.s.Overview(); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.IndentedJSON(http.StatusOK, data)
	}
}

func (h *aggHandler) Stats(ctx *gin.Context) {
	if data, err := h.s.Stats(); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.IndentedJSON(http.StatusOK, data)
	}
}
