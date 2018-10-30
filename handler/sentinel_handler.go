package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type SentinelService interface {
	AddSentinel(addr string) error
	DelSentinel(addr string, force bool) error
	ResyncSentinels() error
	SentinelInfo(addr string) ([]byte, error)
	SentinelMonitoredInfo(addr string) (interface{}, error)
}

type sentinelHandler struct {
	s SentinelService
}

func InitSentinelHandler(s SentinelService, router gin.IRouter) {
	h := &sentinelHandler{s: s}

	r := router.Group("/sentinels")
	r.PUT("/add/:xauth/:addr", h.Add)
	r.PUT("/del/:xauth/:addr/:force", h.Del)
	r.PUT("/resync-all/:xauth", h.ResyncAll)
	r.GET("/info/:addr", h.SentinelInfo)
	r.GET("/info/:addr/monitored", h.SentinelMonitoredInfo)
}

func (h *sentinelHandler) Add(ctx *gin.Context) {
	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if err := h.s.AddSentinel(addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *sentinelHandler) Del(ctx *gin.Context) {
	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	force, err := strconv.Atoi(ctx.Param("force"))
	if err != nil {
		ctx.IndentedJSON(http.StatusBadRequest, "invalid force")
		return
	}

	if err := h.s.DelSentinel(addr, force != 0); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *sentinelHandler) ResyncAll(ctx *gin.Context) {
	if err := h.s.ResyncSentinels(); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *sentinelHandler) SentinelInfo(ctx *gin.Context) {
	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if data, err := h.s.SentinelInfo(addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.Writer.Header().Set("Content-Type", "text/html")
		ctx.Writer.Write([]byte(fmt.Sprintf(TextPreHtml, data)))
	}
}

func (h *sentinelHandler) SentinelMonitoredInfo(ctx *gin.Context) {
	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if data, err := h.s.SentinelMonitoredInfo(addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.IndentedJSON(http.StatusOK, data)
	}
}
