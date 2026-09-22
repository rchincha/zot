package common_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"zotregistry.dev/zot/v2/pkg/api/config"
	"zotregistry.dev/zot/v2/pkg/common"
	reqCtx "zotregistry.dev/zot/v2/pkg/requestcontext"
)

func TestAuthzOnlyAdminsMiddleware(t *testing.T) {
	Convey("AuthzOnlyAdminsMiddleware", t, func() {
		nextCalled := false
		next := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			nextCalled = true
			response.WriteHeader(http.StatusOK)
		})

		Convey("allows requests when authentication is disabled", func() {
			conf := config.New()
			request := httptest.NewRequest(http.MethodGet, "/admin", nil)
			response := httptest.NewRecorder()

			common.AuthzOnlyAdminsMiddleware(conf)(next).ServeHTTP(response, request)

			So(response.Code, ShouldEqual, http.StatusOK)
			So(nextCalled, ShouldBeTrue)
		})

		Convey("denies a non-admin principal with Bearer authentication", func() {
			conf := config.New()
			conf.HTTP.Auth.Bearer = &config.BearerConfig{
				Cert:    "/path/to/cert.pem",
				Realm:   "test-realm",
				Service: "test-service",
			}
			request := httptest.NewRequest(http.MethodGet, "/admin", nil)
			userAc := reqCtx.NewUserAccessControl()
			userAc.SetUsername("non-admin")
			userAc.SetIsAdmin(false)
			userAc.SaveOnRequest(request)
			response := httptest.NewRecorder()

			common.AuthzOnlyAdminsMiddleware(conf)(next).ServeHTTP(response, request)

			So(response.Code, ShouldEqual, http.StatusForbidden)
			So(nextCalled, ShouldBeFalse)
		})

		Convey("allows an admin principal with Bearer authentication", func() {
			conf := config.New()
			conf.HTTP.Auth.Bearer = &config.BearerConfig{
				Cert:    "/path/to/cert.pem",
				Realm:   "test-realm",
				Service: "test-service",
			}
			request := httptest.NewRequest(http.MethodGet, "/admin", nil)
			userAc := reqCtx.NewUserAccessControl()
			userAc.SetUsername("admin")
			userAc.SetIsAdmin(true)
			userAc.SaveOnRequest(request)
			response := httptest.NewRecorder()

			common.AuthzOnlyAdminsMiddleware(conf)(next).ServeHTTP(response, request)

			So(response.Code, ShouldEqual, http.StatusOK)
			So(nextCalled, ShouldBeTrue)
		})

		Convey("denies a missing principal with mTLS authentication", func() {
			conf := config.New()
			conf.HTTP.TLS = &config.TLSConfig{Cert: "server.cert", Key: "server.key", CACert: "ca.crt"}
			request := httptest.NewRequest(http.MethodGet, "/admin", nil)
			response := httptest.NewRecorder()

			common.AuthzOnlyAdminsMiddleware(conf)(next).ServeHTTP(response, request)

			So(response.Code, ShouldEqual, http.StatusUnauthorized)
			So(nextCalled, ShouldBeFalse)
		})

		Convey("denies a malformed access-control context", func() {
			conf := config.New()
			conf.HTTP.TLS = &config.TLSConfig{Cert: "server.cert", Key: "server.key", CACert: "ca.crt"}
			request := httptest.NewRequest(http.MethodGet, "/admin", nil)
			request = request.WithContext(context.WithValue(request.Context(), reqCtx.GetContextKey(), "invalid"))
			response := httptest.NewRecorder()

			common.AuthzOnlyAdminsMiddleware(conf)(next).ServeHTTP(response, request)

			So(response.Code, ShouldEqual, http.StatusUnauthorized)
			So(nextCalled, ShouldBeFalse)
		})
	})
}
