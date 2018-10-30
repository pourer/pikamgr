package handler

import (
	"github.com/pourer/pikamgr/utils/log"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func RecordSourceHandler(ctx *gin.Context) {
	path := ctx.Request.URL.Path

	var remoteAddr = ctx.Request.RemoteAddr
	var headerAddr string
	for _, key := range []string{"X-Real-IP", "X-Forwarded-For"} {
		if val := ctx.Request.Header.Get(key); val != "" {
			headerAddr = val
			break
		}
	}

	if ctx.Request.Method == "GET" {
		log.Debugf("API call %s from %s [%s]", path, remoteAddr, headerAddr)
	} else {
		log.Infof("API call %s from %s [%s]", path, remoteAddr, headerAddr)
	}

	ctx.Next()
}

var GzipHandler = gzip.Gzip(gzip.DefaultCompression)

func validPort(port int) bool {
	if port < 10000 || port > 59999 {
		return false
	}
	return true
}

var TextPreHtml = `
<html>
<head>
  <meta charset="UTF-8">
</head>
<body>
	<pre>
%s
	</pre>
</body>
</html>
`
