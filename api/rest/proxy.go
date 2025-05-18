package rest

import (
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

func Proxy(c *gin.Context) {
	rawProxiedUrl := c.Param("url")[1:]
	proxiedUrl, err := url.Parse(rawProxiedUrl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req, _ := http.NewRequest("GET", proxiedUrl.String(), nil)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	for k, v := range resp.Header {
		for _, vv := range v {
			c.Header(k, vv)
		}
	}

	c.Data(statusCode, contentType, response)
}
