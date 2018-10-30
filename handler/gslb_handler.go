package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type GSLBService interface {
	AddGSLB(gslbName, addr string) error
	DelGSLB(gslbName, addr string) error
	GSLBMonitorInfo(addr string) ([]byte, error)
}

type gslbHandler struct {
	s GSLBService
}

func InitGSLBHandler(s GSLBService, router gin.IRouter) {
	h := &gslbHandler{s: s}

	r := router.Group("/gslbs")
	r.PUT("/add/:xauth/:gslbname/:addr", h.Add)
	r.PUT("/del/:xauth/:gslbname/:addr", h.Del)
	r.GET("/info/:addr/monitored", h.GSLBMonitorInfo)
}

func (h *gslbHandler) Add(ctx *gin.Context) {
	gslbName := ctx.Param("gslbname")
	if len(gslbName) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "gslb name invalid")
		return
	}

	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if err := h.s.AddGSLB(gslbName, addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *gslbHandler) Del(ctx *gin.Context) {
	gslbName := ctx.Param("gslbname")
	if len(gslbName) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "gslb name invalid")
		return
	}

	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if err := h.s.DelGSLB(gslbName, addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *gslbHandler) GSLBMonitorInfo(ctx *gin.Context) {
	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if data, err := h.s.GSLBMonitorInfo(addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.Writer.Header().Set("Content-Type", "text/html")
		ctx.Writer.Write(data)
	}
}
