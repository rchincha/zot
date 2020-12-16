package cache

import "github.com/anuvu/zot/pkg/log"

type upstreamCache struct {
	client     cache.Client
	downloadCh chan string
	log        log.Logger
}

type UpstreamCache interface {
	Put(string) error
}

func NewUpstreamCache(log log.Logger) UpstreamCache {

	c := &upstreamCache{}
	c.downloadCh = make(chan string, 1024)
	return c
}

func (c *upstreamCache) Put(image string) error {
	c.downloadCh <- image
	return nil
}

func (c *upstreamCache) downloader() {
	for {
		select {
		case img := <-c.downloadCh:
		}
	}
}
