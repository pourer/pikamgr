package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TemplateFileService interface {
	ViewTemplateFile(fileName string) ([]byte, error)
}

type tfHandler struct {
	s TemplateFileService
}

func InitTFHandler(s TemplateFileService, router gin.IRouter) {
	h := &tfHandler{s: s}

	r := router.Group("/tf")
	r.GET("/info/:filename", h.Info)
}

func (h *tfHandler) Info(ctx *gin.Context) {
	fileName := ctx.Param("filename")
	if len(fileName) == 0 {
		ctx.IndentedJSON(http.StatusBadRequest, "template file name invalid")
		return
	}

	if data, err := h.s.ViewTemplateFile(fileName); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.Writer.Header().Set("Content-Type", "text/html")
		ctx.Writer.Write([]byte(fmt.Sprintf(TextPreHtml, data)))
	}
}
