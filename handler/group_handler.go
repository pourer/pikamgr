package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type GroupService interface {
	CreateGroup(groupName string, rPort, wPort int) error
	RemoveGroup(groupName string) error
	ResyncGroup(groupName string) error
	ResyncGroupAll() error
	AddGroupServer(groupName, addr string) error
	DelGroupServer(groupName, addr string) error
	GroupPromoteServer(groupName, addr string) error
	GroupForceFullSyncServer(groupName, addr string) error
	ServerInfo(addr string) ([]byte, error)
}

type groupHandler struct {
	s GroupService
}

func InitGroupHandler(s GroupService, router gin.IRouter) {
	h := &groupHandler{s: s}

	r := router.Group("/group")
	r.PUT("/create/:xauth/:gname/:rport/:wport", h.Create)
	r.PUT("/remove/:xauth/:gname", h.Remove)
	r.PUT("/resync/:xauth/:gname", h.Resync)
	r.PUT("/resync-all/:xauth", h.ResyncAll)
	r.PUT("/add/:xauth/:gname/:addr", h.AddServer)
	r.PUT("/del/:xauth/:gname/:addr", h.DelServer)
	r.PUT("/promote/:xauth/:gname/:addr", h.PromoteServer)
	r.PUT("/force-full-sync/:xauth/:gname/:addr", h.ForceFullSyncServer)
	r.GET("/info/:addr", h.ServerInfo)
}

func (h *groupHandler) Create(ctx *gin.Context) {
	groupName := ctx.Param("gname")
	if groupName == "" {
		ctx.IndentedJSON(http.StatusBadRequest, "group name invalid")
		return
	}

	readPort, err := strconv.Atoi(ctx.Param("rport"))
	if err != nil || !validPort(readPort) {
		ctx.IndentedJSON(http.StatusBadRequest, "proxy read port invalid")
		return
	}
	writePort, err := strconv.Atoi(ctx.Param("wport"))
	if err != nil || !validPort(writePort) {
		ctx.IndentedJSON(http.StatusBadRequest, "proxy write port invalid")
		return
	}

	if err := h.s.CreateGroup(groupName, readPort, writePort); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *groupHandler) Remove(ctx *gin.Context) {
	groupName := ctx.Param("gname")
	if groupName == "" {
		ctx.IndentedJSON(http.StatusBadRequest, "group name invalid")
		return
	}

	if err := h.s.RemoveGroup(groupName); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *groupHandler) Resync(ctx *gin.Context) {
	groupName := ctx.Param("gname")
	if groupName == "" {
		ctx.IndentedJSON(http.StatusBadRequest, "group name invalid")
		return
	}

	if err := h.s.ResyncGroup(groupName); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *groupHandler) ResyncAll(ctx *gin.Context) {
	if err := h.s.ResyncGroupAll(); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *groupHandler) AddServer(ctx *gin.Context) {
	groupName := ctx.Param("gname")
	if groupName == "" {
		ctx.IndentedJSON(http.StatusBadRequest, "group name invalid")
		return
	}

	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if err := h.s.AddGroupServer(groupName, addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *groupHandler) DelServer(ctx *gin.Context) {
	groupName := ctx.Param("gname")
	if groupName == "" {
		ctx.IndentedJSON(http.StatusBadRequest, "group name invalid")
		return
	}

	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if err := h.s.DelGroupServer(groupName, addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *groupHandler) PromoteServer(ctx *gin.Context) {
	groupName := ctx.Param("gname")
	if groupName == "" {
		ctx.IndentedJSON(http.StatusBadRequest, "group name invalid")
		return
	}

	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if err := h.s.GroupPromoteServer(groupName, addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *groupHandler) ForceFullSyncServer(ctx *gin.Context) {
	groupName := ctx.Param("gname")
	if groupName == "" {
		ctx.IndentedJSON(http.StatusBadRequest, "group name invalid")
		return
	}

	addr := ctx.Param("addr")
	if len(addr) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if err := h.s.GroupForceFullSyncServer(groupName, addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.IndentedJSON(http.StatusOK, nil)
}

func (h *groupHandler) ServerInfo(ctx *gin.Context) {
	addr := ctx.Param("addr")
	if addr == "" {
		ctx.IndentedJSON(http.StatusBadRequest, "missing addr")
		return
	}

	if data, err := h.s.ServerInfo(addr); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.Writer.Header().Set("Content-Type", "text/html")
		ctx.Writer.Write([]byte(fmt.Sprintf(TextPreHtml, data)))
	}
}
