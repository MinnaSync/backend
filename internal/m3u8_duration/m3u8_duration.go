package m3u8_duration

import (
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/etherlabsio/go-m3u8/m3u8"
)

func cleanURL(u string) string {
	parsedUrl, _ := url.Parse(u)
	parsedUrl.Path = path.Dir(parsedUrl.Path)

	return parsedUrl.String()
}

func FetchM3u8Duration(url string) (float64, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	playlist, err := m3u8.Read(resp.Body)
	if err != nil {
		return 0, err
	}

	if playlist.IsMaster() {
		for _, item := range playlist.Items {
			switch item.(type) {
			case *m3u8.PlaylistItem:
				item := item.(*m3u8.PlaylistItem)

				uri := fmt.Sprintf("%s/%s", cleanURL(url), item.URI)
				return FetchM3u8Duration(uri)
			}
		}
	}

	return playlist.Duration(), nil
}
