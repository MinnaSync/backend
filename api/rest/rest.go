package rest

import "github.com/gin-gonic/gin"

func Register(r *gin.Engine) {
	r.GET("/proxied/*url", Proxy)
}
