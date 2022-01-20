package main

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	mrand "math/rand"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	godigest "github.com/opencontainers/go-digest"
	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/resty.v1"
	"zotregistry.io/zot/pkg/test"
)

const (
	KiB                  = 1 * 1024
	MiB                  = 1 * KiB * 1024
	GiB                  = 1 * MiB * 1024
	maxSize              = 1 * GiB // 1GiB
	defaultDirPerms      = 0o700
	defaultFilePerms     = 0o600
	defaultSchemaVersion = 2
	smallBlob            = 1 * MiB
	mediumBlob           = 10 * MiB
	largeBlob            = 100 * MiB
	cicdFmt              = "ci-cd"
	pbty33               = 0.33
)

//nolint:gochecknoglobals // used only in this test
var blobHash map[string]godigest.Digest = map[string]godigest.Digest{}

func setup(workingDir string) {
	_ = os.MkdirAll(workingDir, defaultDirPerms)

	const multiplier = 10

	const rndPageSize = 4 * KiB

	for size := 1 * MiB; size < maxSize; size *= multiplier {
		fname := path.Join(workingDir, fmt.Sprintf("%d.blob", size))

		fhandle, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultFilePerms)
		if err != nil {
			log.Fatal(err)
		}

		err = fhandle.Truncate(int64(size))
		if err != nil {
			log.Fatal(err)
		}

		_, err = fhandle.Seek(0, 0)
		if err != nil {
			log.Fatal(err)
		}

		// write a random first page so every test run has different blob content
		rnd := make([]byte, rndPageSize)
		if _, err := crand.Read(rnd); err != nil {
			log.Fatal(err)
		}

		if _, err := fhandle.Write(rnd); err != nil {
			log.Fatal(err)
		}

		if _, err := fhandle.Seek(0, 0); err != nil {
			log.Fatal(err)
		}

		fhandle.Close() // should flush the write

		// pre-compute the SHA256
		fhandle, err = os.OpenFile(fname, os.O_RDONLY, defaultFilePerms)
		if err != nil {
			log.Fatal(err)
		}

		defer fhandle.Close()

		digest, err := godigest.FromReader(fhandle)
		if err != nil {
			log.Fatal(err) //nolint:gocritic // file closed on exit
		}

		blobHash[fname] = digest
	}
}

func teardown(workingDir string) {
	_ = os.RemoveAll(workingDir)
}

// statistics handling.

type Durations []time.Duration

func (a Durations) Len() int           { return len(a) }
func (a Durations) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Durations) Less(i, j int) bool { return a[i] < a[j] }

type statsSummary struct {
	latencies       []time.Duration
	name            string
	min, max, total time.Duration
	rps             float32
	statusHist      map[string]int
	errors          int
}

func newStatsSummary(name string) statsSummary {
	summary := statsSummary{
		name:       name,
		min:        -1,
		max:        -1,
		statusHist: make(map[string]int),
	}

	return summary
}

type statsRecord struct {
	latency    time.Duration
	statusCode int
	isConnFail bool
	isErr      bool
}

func updateStats(summary *statsSummary, record statsRecord) {
	if record.isConnFail || record.isErr {
		summary.errors++
	}

	if summary.min < 0 || record.latency < summary.min {
		summary.min = record.latency
	}

	if summary.max < 0 || record.latency > summary.max {
		summary.max = record.latency
	}

	// 2xx
	if record.statusCode >= http.StatusOK &&
		record.statusCode <= http.StatusAccepted {
		summary.statusHist["2xx"]++
	}

	// 3xx
	if record.statusCode >= http.StatusMultipleChoices &&
		record.statusCode <= http.StatusPermanentRedirect {
		summary.statusHist["3xx"]++
	}

	// 4xx
	if record.statusCode >= http.StatusBadRequest &&
		record.statusCode <= http.StatusUnavailableForLegalReasons {
		summary.statusHist["4xx"]++
	}

	// 5xx
	if record.statusCode >= http.StatusInternalServerError &&
		record.statusCode <= http.StatusNetworkAuthenticationRequired {
		summary.statusHist["5xx"]++
	}

	summary.latencies = append(summary.latencies, record.latency)
}

type cicdTestSummary struct {
	Name  string      `json:"name"`
	Unit  string      `json:"unit"`
	Value interface{} `json:"value"`
	Range string      `json:"range,omitempty"`
}

//nolint:gochecknoglobals // used only in this test
var cicdSummary = []cicdTestSummary{}

func printStats(requests int, summary *statsSummary, outFmt string) {
	log.Printf("============\n")
	log.Printf("Test name:\t%s", summary.name)
	log.Printf("Time taken for tests:\t%v", summary.total)
	log.Printf("Complete requests:\t%v", requests-summary.errors)
	log.Printf("Failed requests:\t%v", summary.errors)
	log.Printf("Requests per second:\t%v", summary.rps)
	log.Printf("\n")

	for k, v := range summary.statusHist {
		log.Printf("%s responses:\t%v", k, v)
	}

	log.Printf("\n")
	log.Printf("min: %v", summary.min)
	log.Printf("max: %v", summary.max)
	log.Printf("%s:\t%v", "p50", summary.latencies[requests/2])
	log.Printf("%s:\t%v", "p75", summary.latencies[requests*3/4])
	log.Printf("%s:\t%v", "p90", summary.latencies[requests*9/10])
	log.Printf("%s:\t%v", "p99", summary.latencies[requests*99/100])
	log.Printf("\n")

	// ci/cd
	if outFmt == cicdFmt {
		cicdSummary = append(cicdSummary,
			cicdTestSummary{
				Name:  summary.name,
				Unit:  "requests per sec",
				Value: summary.rps,
				Range: "3",
			},
		)
	}
}

// nolint:gosec
func flipTestSize(probability float64) int {
	switch toss := mrand.Float64(); {
	case toss < probability:
		return smallBlob
	case toss < 2*probability:
		return mediumBlob
	default:
		return largeBlob
	}
}

// test suites/funcs.

type testFunc func(workdir, url, auth, repo string, requests int, config testConfig, statsCh chan statsRecord) error

func GetCatalog(workdir, url, auth, repo string, requests int, config testConfig, statsCh chan statsRecord) error {
	client := resty.New()

	if auth != "" {
		creds := strings.Split(auth, ":")
		client.SetBasicAuth(creds[0], creds[1])
	}

	for count := 0; count < requests; count++ {
		func() {
			start := time.Now()

			var isConnFail, isErr bool

			var statusCode int

			var latency time.Duration

			defer func() {
				// send a stats record
				statsCh <- statsRecord{
					latency:    latency,
					statusCode: statusCode,
					isConnFail: isConnFail,
					isErr:      isErr,
				}
			}()

			// send request and get response
			resp, err := client.R().Get(url + "/v2/_catalog")

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusOK {
				isErr = true

				return
			}
		}()
	}

	return nil
}

func PushMonolithStreamed(workdir, url, auth, trepo string, requests int,
	config testConfig, statsCh chan statsRecord) error {
	client := resty.New()

	if auth != "" {
		creds := strings.Split(auth, ":")
		client.SetBasicAuth(creds[0], creds[1])
	}

	manifestHash := make(map[string]string)
	configHash := make(map[string]string)
	layerHash := make(map[string][]ispec.Descriptor)

	for count := 0; count < requests; count++ {
		func() {
			start := time.Now()

			var isConnFail, isErr bool

			var statusCode int

			var latency time.Duration

			defer func() {
				// send a stats record
				statsCh <- statsRecord{
					latency:    latency,
					statusCode: statusCode,
					isConnFail: isConnFail,
					isErr:      isErr,
				}
			}()

			ruid, err := uuid.NewUUID()
			if err != nil {
				log.Fatal(err)
			}

			var repo string

			if trepo != "" {
				repo = trepo + "/" + ruid.String()
			} else {
				repo = ruid.String()
			}

			// create a new upload
			resp, err := resty.R().
				Post(fmt.Sprintf("%s/v2/%s/blobs/uploads/", url, repo))

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusAccepted {
				isErr = true

				return
			}

			loc := test.Location(url, resp)

			size := config.size
			blob := path.Join(workdir, fmt.Sprintf("%d.blob", size))

			fhandle, err := os.OpenFile(blob, os.O_RDONLY, defaultFilePerms)
			if err != nil {
				isConnFail = true

				return
			}

			defer fhandle.Close()

			// stream the entire blob
			digest := blobHash[blob]

			resp, err = client.R().
				SetContentLength(true).
				SetHeader("Content-Length", fmt.Sprintf("%d", size)).
				SetHeader("Content-Type", "application/octet-stream").
				SetQueryParam("digest", digest.String()).
				SetBody(fhandle).
				Put(loc)

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusCreated {
				isErr = true

				return
			}

			// upload image config blob
			resp, err = resty.R().
				Post(fmt.Sprintf("%s/v2/%s/blobs/uploads/", url, repo))

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusAccepted {
				isErr = true

				return
			}

			loc = test.Location(url, resp)
			cblob, cdigest := test.GetRandomImageConfig()
			resp, err = client.R().
				SetContentLength(true).
				SetHeader("Content-Length", fmt.Sprintf("%d", len(cblob))).
				SetHeader("Content-Type", "application/octet-stream").
				SetQueryParam("digest", cdigest.String()).
				SetBody(cblob).
				Put(loc)

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusCreated {
				isErr = true

				return
			}

			// create a manifest
			manifest := ispec.Manifest{
				Versioned: imeta.Versioned{
					SchemaVersion: defaultSchemaVersion,
				},
				Config: ispec.Descriptor{
					MediaType: "application/vnd.oci.image.config.v1+json",
					Digest:    cdigest,
					Size:      int64(len(cblob)),
				},
				Layers: []ispec.Descriptor{
					{
						MediaType: "application/vnd.oci.image.layer.v1.tar",
						Digest:    digest,
						Size:      int64(size),
					},
				},
			}

			content, err := json.MarshalIndent(&manifest, "", "\t")
			if err != nil {
				log.Fatal(err)
			}

			manifestTag := fmt.Sprintf("tag%d", count)

			resp, err = resty.R().
				SetContentLength(true).
				SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
				SetBody(content).
				Put(fmt.Sprintf("%s/v2/%s/manifests/%s", url, repo, manifestTag))

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusCreated {
				isErr = true

				return
			}

			manifestHash[repo] = manifestTag
			configHash[manifestTag] = cdigest.String()
			layerHash[digest.String()] = manifest.Layers
		}()
	}

	err := deleteUploadedFiles(manifestHash, configHash, layerHash, url, client)
	if err != nil {
		return err
	}

	return nil
}

func PushChunkStreamed(workdir, url, auth, trepo string, requests int,
	config testConfig, statsCh chan statsRecord) error {
	client := resty.New()

	if auth != "" {
		creds := strings.Split(auth, ":")
		client.SetBasicAuth(creds[0], creds[1])
	}

	manifestHash := make(map[string]string)
	configHash := make(map[string]string)
	layerHash := make(map[string][]ispec.Descriptor)

	for count := 0; count < requests; count++ {
		func() {
			start := time.Now()

			var isConnFail, isErr bool

			var statusCode int

			var latency time.Duration

			defer func() {
				// send a stats record
				statsCh <- statsRecord{
					latency:    latency,
					statusCode: statusCode,
					isConnFail: isConnFail,
					isErr:      isErr,
				}
			}()

			ruid, err := uuid.NewUUID()
			if err != nil {
				log.Fatal(err)
			}

			var repo string

			if trepo != "" {
				repo = trepo + "/" + ruid.String()
			} else {
				repo = ruid.String()
			}

			// create a new upload
			resp, err := resty.R().
				Post(fmt.Sprintf("%s/v2/%s/blobs/uploads/", url, repo))

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusAccepted {
				isErr = true

				return
			}

			loc := test.Location(url, resp)

			size := config.size
			blob := path.Join(workdir, fmt.Sprintf("%d.blob", size))

			fhandle, err := os.OpenFile(blob, os.O_RDONLY, defaultFilePerms)
			if err != nil {
				isConnFail = true

				return
			}

			defer fhandle.Close()

			digest := blobHash[blob]

			// upload blob
			resp, err = client.R().
				SetContentLength(true).
				SetHeader("Content-Type", "application/octet-stream").
				SetBody(fhandle).
				Patch(loc)

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			loc = test.Location(url, resp)

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusAccepted {
				isErr = true

				return
			}

			// finish upload
			resp, err = client.R().
				SetContentLength(true).
				SetHeader("Content-Length", fmt.Sprintf("%d", size)).
				SetHeader("Content-Type", "application/octet-stream").
				SetQueryParam("digest", digest.String()).
				Put(loc)

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusCreated {
				isErr = true

				return
			}

			// upload image config blob
			resp, err = resty.R().
				Post(fmt.Sprintf("%s/v2/%s/blobs/uploads/", url, repo))

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusAccepted {
				isErr = true

				return
			}

			loc = test.Location(url, resp)
			cblob, cdigest := test.GetRandomImageConfig()
			resp, err = client.R().
				SetContentLength(true).
				SetHeader("Content-Type", "application/octet-stream").
				SetBody(fhandle).
				Patch(loc)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusAccepted {
				isErr = true

				return
			}

			// upload blob
			resp, err = client.R().
				SetContentLength(true).
				SetHeader("Content-Type", "application/octet-stream").
				SetBody(cblob).
				Patch(loc)

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			loc = test.Location(url, resp)

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusAccepted {
				isErr = true

				return
			}

			// finish upload
			resp, err = client.R().
				SetContentLength(true).
				SetHeader("Content-Length", fmt.Sprintf("%d", len(cblob))).
				SetHeader("Content-Type", "application/octet-stream").
				SetQueryParam("digest", cdigest.String()).
				Put(loc)

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusCreated {
				isErr = true

				return
			}

			// create a manifest
			manifest := ispec.Manifest{
				Versioned: imeta.Versioned{
					SchemaVersion: defaultSchemaVersion,
				},
				Config: ispec.Descriptor{
					MediaType: "application/vnd.oci.image.config.v1+json",
					Digest:    cdigest,
					Size:      int64(len(cblob)),
				},
				Layers: []ispec.Descriptor{
					{
						MediaType: "application/vnd.oci.image.layer.v1.tar",
						Digest:    digest,
						Size:      int64(size),
					},
				},
			}

			content, err := json.Marshal(manifest)
			if err != nil {
				log.Fatal(err)
			}

			manifestTag := fmt.Sprintf("tag%d", count)

			// finish upload
			resp, err = resty.R().
				SetContentLength(true).
				SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
				SetBody(content).
				Put(fmt.Sprintf("%s/v2/%s/manifests/%s", url, repo, manifestTag))

			latency = time.Since(start)

			if err != nil {
				isConnFail = true

				return
			}

			// request specific check
			statusCode = resp.StatusCode()
			if statusCode != http.StatusCreated {
				isErr = true

				return
			}

			manifestHash[repo] = manifestTag
			configHash[manifestTag] = cdigest.String()
			layerHash[digest.String()] = manifest.Layers
		}()
	}

	err := deleteUploadedFiles(manifestHash, configHash, layerHash, url, client)
	if err != nil {
		return err
	}

	return nil
}

//nolint: gocyclo
func Pull(workdir, url, auth, trepo string, requests int,
	config testConfig, statsCh chan statsRecord) error {
	client := resty.New()

	if auth != "" {
		creds := strings.Split(auth, ":")
		client.SetBasicAuth(creds[0], creds[1])
	}

	var manifestHash, manifestHashSizeS, manifestHashSizeM, manifestHashSizeL map[string]string

	var configHash, configHashSizeS, configHashSizeM, configHashSizeL map[string]string

	var layerHash, layerHashSizeS, layerHashSizeM, layerHashSizeL map[string][]ispec.Descriptor

	var err error

	if config.mixedSize {
		// Push small blob
		manifestHashSizeS, configHashSizeS, layerHashSizeS, err = push(workdir, url, trepo, smallBlob, client)
		if err != nil {
			return err
		}

		// Push medium blob
		manifestHashSizeM, configHashSizeM, layerHashSizeM, err = push(workdir, url, trepo, mediumBlob, client)
		if err != nil {
			return err
		}

		// Push large blob
		manifestHashSizeL, configHashSizeL, layerHashSizeL, err = push(workdir, url, trepo, largeBlob, client)
		if err != nil {
			return err
		}
	} else {
		// Push blob given size
		manifestHash, configHash, layerHash, err = push(workdir, url, trepo, config.size, client)
		if err != nil {
			return err
		}
	}

	// download image
	for count := 0; count < requests; count++ {
		func() {
			start := time.Now()

			var isConnFail, isErr bool

			var statusCode int

			var latency time.Duration

			defer func() {
				// send a stats record
				statsCh <- statsRecord{
					latency:    latency,
					statusCode: statusCode,
					isConnFail: isConnFail,
					isErr:      isErr,
				}
			}()

			if config.mixedSize {
				size := flipTestSize(config.probability)

				switch size {
				case smallBlob:
					manifestHash = manifestHashSizeS
					configHash = configHashSizeS
				case mediumBlob:
					manifestHash = manifestHashSizeM
					configHash = configHashSizeM
				case largeBlob:
					manifestHash = manifestHashSizeL
					configHash = configHashSizeL
				}
			}

			for repo, manifestTag := range manifestHash {
				manifestLoc := fmt.Sprintf("%s/v2/%s/manifests/%s", url, repo, manifestTag)

				// check manifest
				resp, err := client.R().Head(manifestLoc)

				latency = time.Since(start)

				if err != nil {
					isConnFail = true

					return
				}

				// request specific check
				statusCode = resp.StatusCode()
				if statusCode != http.StatusOK {
					isErr = true

					return
				}

				// send request and get the manifest
				resp, err = client.R().Get(manifestLoc)

				latency = time.Since(start)

				if err != nil {
					isConnFail = true

					return
				}

				// request specific check
				statusCode = resp.StatusCode()
				if statusCode != http.StatusOK {
					isErr = true

					return
				}

				manifestBody := resp.Body()

				// file copy simulation
				_, err = io.Copy(ioutil.Discard, bytes.NewReader(manifestBody))

				latency = time.Since(start)

				if err != nil {
					log.Fatal(err)
				}

				var pulledManifest ispec.Manifest

				err = json.Unmarshal(manifestBody, &pulledManifest)
				if err != nil {
					log.Fatal(err)
				}

				// check config
				configLoc := fmt.Sprintf("%s/v2/%s/blobs/%s", url, repo, configHash[manifestTag])
				resp, err = client.R().Head(configLoc)

				latency = time.Since(start)

				if err != nil {
					isConnFail = true

					return
				}

				// request specific check
				statusCode = resp.StatusCode()
				if statusCode != http.StatusOK {
					isErr = true

					return
				}

				// send request and get the config
				resp, err = client.R().Get(configLoc)

				latency = time.Since(start)

				if err != nil {
					isConnFail = true

					return
				}

				// request specific check
				statusCode = resp.StatusCode()
				if statusCode != http.StatusOK {
					isErr = true

					return
				}

				configBody := resp.Body()

				// file copy simulation
				_, err = io.Copy(ioutil.Discard, bytes.NewReader(configBody))

				latency = time.Since(start)

				if err != nil {
					log.Fatal(err)
				}

				// download blobs
				for _, layer := range pulledManifest.Layers {
					blobDigest := layer.Digest
					blobLoc := fmt.Sprintf("%s/v2/%s/blobs/%s", url, repo, blobDigest)

					// check blob
					resp, err := client.R().Head(blobLoc)

					latency = time.Since(start)

					if err != nil {
						isConnFail = true

						return
					}

					// request specific check
					statusCode = resp.StatusCode()
					if statusCode != http.StatusOK {
						isErr = true

						return
					}

					// send request and get response the blob
					resp, err = client.R().Get(blobLoc)

					latency = time.Since(start)

					if err != nil {
						isConnFail = true

						return
					}

					// request specific check
					statusCode = resp.StatusCode()
					if statusCode != http.StatusOK {
						isErr = true

						return
					}

					blobBody := resp.Body()

					// file copy simulation
					_, err = io.Copy(ioutil.Discard, bytes.NewReader(blobBody))
					if err != nil {
						log.Fatal(err)
					}
				}
			}
		}()
	}

	// Clean up
	if config.mixedSize {
		err = deleteUploadedFiles(manifestHashSizeS, configHashSizeS, layerHashSizeS, url, client)
		if err != nil {
			return err
		}

		err = deleteUploadedFiles(manifestHashSizeM, configHashSizeM, layerHashSizeM, url, client)
		if err != nil {
			return err
		}

		err = deleteUploadedFiles(manifestHashSizeL, configHashSizeL, layerHashSizeL, url, client)
		if err != nil {
			return err
		}
	} else {
		err = deleteUploadedFiles(manifestHash, configHash, layerHash, url, client)
		if err != nil {
			return err
		}
	}

	return nil
}

// test driver.

type testConfig struct {
	name  string
	tfunc testFunc
	// test-specific params
	size        int
	probability float64
	mixedSize   bool
}

var testSuite = []testConfig{ // nolint:gochecknoglobals // used only in this test
	{
		name:  "Get Catalog",
		tfunc: GetCatalog,
	},
	{
		name:  "Push Monolith 1MB",
		tfunc: PushMonolithStreamed,
		size:  smallBlob,
	},
	{
		name:  "Push Monolith 10MB",
		tfunc: PushMonolithStreamed,
		size:  mediumBlob,
	},
	{
		name:  "Push Monolith 100MB",
		tfunc: PushMonolithStreamed,
		size:  largeBlob,
	},
	{
		name:  "Push Chunk Streamed 1MB",
		tfunc: PushChunkStreamed,
		size:  smallBlob,
	},
	{
		name:  "Push Chunk Streamed 10MB",
		tfunc: PushChunkStreamed,
		size:  mediumBlob,
	},
	{
		name:  "Push Chunk Streamed 100MB",
		tfunc: PushChunkStreamed,
		size:  largeBlob,
	},
	{
		name:  "Pull 1MB",
		tfunc: Pull,
		size:  smallBlob,
	},
	{
		name:  "Pull 10MB",
		tfunc: Pull,
		size:  mediumBlob,
	},
	{
		name:  "Pull 100MB",
		tfunc: Pull,
		size:  largeBlob,
	},
	{
		name:        "Pull Mixed 33% 1MB, 33% 10MB, 33% 100MB",
		tfunc:       Pull,
		probability: pbty33,
		mixedSize:   true,
	},
}

func Perf(workdir, url, auth, repo string, concurrency int, requests int, outFmt string) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	// logging
	log.SetFlags(0)
	log.SetOutput(tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent))

	// initialize test data
	setup(workdir)
	defer teardown(workdir)

	// common header
	log.Printf("Registry URL:\t%s", url)
	log.Printf("\n")
	log.Printf("Concurrency Level:\t%v", concurrency)
	log.Printf("Total requests:\t%v", requests)
	log.Printf("Working dir:\t%v", workdir)
	log.Printf("\n")

	for _, tconfig := range testSuite {
		statsCh := make(chan statsRecord, requests)

		var wg sync.WaitGroup

		summary := newStatsSummary(tconfig.name)

		start := time.Now()

		for c := 0; c < concurrency; c++ {
			// parallelize with clients
			wg.Add(1)

			go func() {
				defer wg.Done()

				_ = tconfig.tfunc(workdir, url, auth, repo, requests/concurrency, tconfig, statsCh)
			}()
		}
		wg.Wait()

		summary.total = time.Since(start)
		summary.rps = float32(requests) / float32(summary.total.Seconds())

		for count := 0; count < requests; count++ {
			record := <-statsCh
			updateStats(&summary, record)
		}

		sort.Sort(Durations(summary.latencies))

		printStats(requests, &summary, outFmt)
	}

	if outFmt == cicdFmt {
		jsonOut, err := json.Marshal(cicdSummary)
		if err != nil {
			log.Fatal(err) //nolint:gocritic // file closed on exit
		}

		if err := ioutil.WriteFile(fmt.Sprintf("%s.json", outFmt), jsonOut, defaultFilePerms); err != nil {
			log.Fatal(err)
		}
	}
}
