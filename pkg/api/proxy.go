package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/dchest/siphash"
	"github.com/gorilla/mux"
)

func ClusterProxy(ctrlr *Controller) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			config := ctrlr.Config
			logger := ctrlr.Log

			// if no cluster or single-node cluster, handle locally
			if config.Cluster == nil || len(config.Cluster.Members) == 1 {
				next.ServeHTTP(response, request)
				return
			}

			vars := mux.Vars(request)
			name, ok := vars["name"]

			if !ok || name == "" {
				response.WriteHeader(http.StatusNotFound)
				return
			}

			h := siphash.New([]byte(config.Cluster.HashKey))
			h.Write([]byte(name))
			sum64 := h.Sum64()

			targetMember := config.Cluster.Members[sum64%uint64(len(config.Cluster.Members))]

			// from the member list and our DNS/IP address, figure out if this request should be handled locally
			localMember := fmt.Sprintf("%s:%s", config.HTTP.Address, config.HTTP.Port)
			if targetMember == localMember {
				logger.Debug().Msg("Target cluster member is the local member. Handling request locally.")
				next.ServeHTTP(response, request)
				return
			}
			logger.Debug().Msg(fmt.Sprintf("Target member is %s. Proxying the request", targetMember))

			proxyQueryScheme := "http"
			if config.HTTP.TLS != nil {
				proxyQueryScheme = "https"
			}

			proxyHttpRequest(response, request, targetMember, proxyQueryScheme)
		})
	}
}

func proxyHttpRequest(w http.ResponseWriter, req *http.Request, targetMember string, requestScheme string) {
	cloneUrl := *req.URL
	cloneUrl.Scheme = requestScheme
	cloneUrl.Host = targetMember

	clonedBody := copyRequestBody(req)
	fwdRequest, err := http.NewRequest(req.Method, cloneUrl.String(), clonedBody)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	copyHeader(fwdRequest.Header, req.Header)

	resp, err := http.DefaultTransport.RoundTrip(fwdRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func copyRequestBody(src *http.Request) io.ReadCloser {
	var b bytes.Buffer
	_, _ = b.ReadFrom(src.Body)
	src.Body = io.NopCloser(&b)
	return io.NopCloser(bytes.NewReader(b.Bytes()))
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
