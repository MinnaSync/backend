package rest

import "github.com/gin-gonic/gin"

func Register(r *gin.Engine) {
	r.GET("/proxied/*url", Proxy)
	r.GET("/m3u8/*url", M3U8)
}
