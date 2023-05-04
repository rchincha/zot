package sync

import (
	"io"
	"log"
	"net/http"
	"os"

	godigest "github.com/opencontainers/go-digest"
	"gopkg.in/fsnotify.v1"
)

type Syncer interface {
	AddClient(*godigest.Digest, http.Response)
}

type downstreamClient struct {
	response http.Response
	offset   int
}

type upstreamSource struct {
	url        string
	maxSize    int
	syncedBlob string
}

type syncer struct {
	clients  []downstreamClients
	upstream upstreamSource
	digest   *godigest.Digest
}

func NewSyncer(url string) *sr {
	return &syncer{upstream: upstreamSource{url: url}, clients: []downstreamClients{}}
}

func (sr *syncer) Run() {
	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// FIXME: how do we end this loop? all clients died or file is fully copied over?
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			log.Println("event:", event)
			if event.Has(fsnotify.Write) {
				for i, client := range sr.downstreamClients {
					fp, err := os.Open(sr.upstreamSource.syncedBlob)
					if err != nil {
						// FIXME: maybe the blob moved to final location?
					}

					// FIXME: should we cache this file handle for later use
					// and also automatic ref-counting, if so then this Seek is
					// not needed

					_, err = fp.Seek(client.offset, io.SeekStart)
					if err != nil {
						// FIXME: handle this
					}

					// copy to http response, which may have timed out
					copied, err := io.CopyN(client.response, fp, 32*1024*1024)
					if err != nil {
						// FIXME: handle this, if the client has disconnected remove from the list
					}

					client.offset += copied

					// FIXME: are we done?
					if client.offset == sr.upstreamSource.maxSize-1 {
						client.WriteStatus()
						client.Close()
						sr.downstreamClients = sr.downstreamClients[:i] + sr.downstreamClients[i+1:]
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}

	err = watcher.Add(sr.upstream.syncedBlob)
	if err != nil {
		log.Fatal(err)
	}

	// Block main goroutine forever.
	<-make(chan struct{})
}

// client code
func ClientMain(request *http.Request, response http.Response) {
	// Found:
	err := findBlob(digest)
	if err != nil {
		sr := NewSyncer(someRegistryUrl, digest)
		sr.AddClient(response)
	}
}
