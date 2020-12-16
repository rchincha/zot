package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	lpath "path"

	"github.com/anuvu/zap/errors"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rs/zerolog"
)

const baseURL = "/v2"

var NilManifest = ispec.Manifest{}

type Client interface {
	DownloadBlobs(string, string, string, string) (ispec.Manifest, error)
}

type client struct {
	httpClient  *http.Client
	bearerToken string
	log         zerolog.Logger
}

func NewClient(log zerolog.Logger) Client {
	c := &client{httpClient: &http.Client{Timeout: time.Second * 10}, log: log}
	return c
}

func (c *client) DownloadBlobs(srcRepo, domain, path, tag string) (ispec.Manifest, error) {
	var uri url.URL

	blobsDir := lpath.Join(srcRepo, "blobs", "sha256")
	os.MkdirAll(blobsDir, 0600)

	// retry atmost twice - the first time may fail because of failed authN
	for retries := 0; retries < 2; retries++ {
		if domain == "docker.io" {
			uri = url.URL{Scheme: "https", Host: "registry-1." + domain, Path: fmt.Sprintf("%s/%s/manifests/%s", baseURL, path, tag)}
		} else {
			uri = url.URL{Scheme: "https", Host: domain, Path: fmt.Sprintf("%s/%s/manifests/%s", baseURL, path, tag)}
		}
		c.log.Debug().Str("url", uri.String()).Msg("downloading container-image")

		req, err := http.NewRequest("GET", uri.String(), nil)
		req.Header.Add("Accept", "application/vnd.oci.image.manifest.v1+json")
		if c.bearerToken != "" {
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return NilManifest, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			authHdr := resp.Header["Www-Authenticate"]
			tokens := strings.Split(strings.ToLower(authHdr[0]), " ")
			switch tokens[0] {
			case "bearer":
				fields := strings.Split(tokens[1], ",")
				m := make(map[string]string)
				for _, e := range fields {
					parts := strings.Split(e, "=")
					key := parts[0]
					val := parts[1]
					m[key] = val[1 : len(val)-1]
				}
				c.getBearerToken(m["realm"], m["service"], m["scope"])
			case "basic":
				// FIXME: implement basic auth (will require additional cmdline params)
			default:
				return NilManifest, errors.ErrUnknownAuth
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			c.log.Error().Int("status", resp.StatusCode).Str("url", req.URL.String()).Msg("unexpected HTTP response")
			return NilManifest, errors.ErrBadHttpResponse
		}

		// FIXME: resp.Headers['Accept'] may not support oci, so will need
		// conversion => docker2oci()
		blob, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return NilManifest, err
		}
		defer resp.Body.Close()

		var manifest ispec.Manifest
		if err := json.Unmarshal(blob, &manifest); err != nil {
			return NilManifest, err
		}

		uri = url.URL{Scheme: "https", Host: domain, Path: fmt.Sprintf("%s/%s/blobs/%s", baseURL, path, manifest.Config.Digest.String())}
		req, err = http.NewRequest("GET", uri.String(), nil)
		req.Header.Add("Accept", manifest.Config.MediaType)

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return NilManifest, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.log.Error().Int("status", resp.StatusCode).Str("url", req.URL.String()).Msg("unexpected HTTP response")
			return NilManifest, errors.ErrBadHttpResponse
		}

		fh, err := os.Create(lpath.Join(blobsDir, manifest.Config.Digest.Encoded()))
		defer fh.Close()
		io.Copy(fh, resp.Body)

		for _, layer := range manifest.Layers {
			uri = url.URL{Scheme: "https", Host: domain, Path: fmt.Sprintf("%s/%s/blobs/%s", baseURL, path, layer.Digest.String())}
			req, err := http.NewRequest("GET", uri.String(), nil)
			req.Header.Add("Accept", layer.MediaType)

			resp, err = c.httpClient.Do(req)
			if err != nil {
				return NilManifest, err
			}
			defer resp.Body.Close()

			fh, err := os.Create(lpath.Join(blobsDir, layer.Digest.Encoded()))
			defer fh.Close()
			io.Copy(fh, resp.Body)
		}

		return manifest, nil
	}

	return NilManifest, errors.ErrUnknownAuth
}

type bearerToken struct {
	Token          string    `json:"token"`
	AccessToken    string    `json:"access_token"`
	ExpiresIn      int       `json:"expires_in"`
	IssuedAt       time.Time `json:"issued_at"`
	expirationTime time.Time
}

func (c *client) getBearerToken(realm, service, scope string) error {
	uri, err := url.Parse(realm)
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Add("service", service)
	params.Add("scope", scope)
	uri.RawQuery = params.Encode()
	resp, err := c.httpClient.Get(uri.String())
	if err != nil {
		return nil
	}
	blob, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var token bearerToken
	if err := json.Unmarshal(blob, &token); err != nil {
		return err
	}

	c.bearerToken = token.Token

	return nil
}
