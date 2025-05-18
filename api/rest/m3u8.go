package rest

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"

	"github.com/etherlabsio/go-m3u8/m3u8"
	"github.com/gin-gonic/gin"
)

func M3U8(c *gin.Context) {
	rawUrl := c.Param("url")[1:]
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid URL.",
		})
		return
	}

	req, _ := http.NewRequest("GET", parsedUrl.String(), nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Failed to fetch URL.",
		})
		return
	}
	defer resp.Body.Close()

	playlist, err := m3u8.Read(resp.Body)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Failed to parse M3U8 playlist.",
		})
		return
	}

	for _, item := range playlist.Items {
		switch item.(type) {
		case *m3u8.KeyItem:
			key := item.(*m3u8.KeyItem)
			proxyURI := fmt.Sprintf("/proxied/%s", url.QueryEscape(*key.Encryptable.URI))
			key.Encryptable.URI = &proxyURI
		case *m3u8.PlaylistItem:
			playlistItem := item.(*m3u8.PlaylistItem)
			proxyURI := fmt.Sprintf("/proxied/%s", url.QueryEscape(playlistItem.URI))
			playlistItem.URI = proxyURI
		case *m3u8.SegmentItem:
			segmentItem := item.(*m3u8.SegmentItem)
			proxyURI := fmt.Sprintf("/proxied/%s", url.QueryEscape(segmentItem.Segment))
			segmentItem.Segment = proxyURI
		}
	}

	var buffer bytes.Buffer
	buffer.WriteString(playlist.String())

	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", buffer.Bytes())
}
