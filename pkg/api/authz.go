package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/anuvu/zot/pkg/log"
	"github.com/gorilla/mux"
)

type key int

const (
	CREATE           = "create"
	READ             = "read"
	UPDATE           = "update"
	DELETE           = "delete"
	contextKeyID key = 0
)

// AccessController authorizes users to act on resources.
type AccessController struct {
	Config *AccessControl
	Log    log.Logger
}

// AccessControllerContext context passed down to http.Handlers.
type AccessControllerContext struct {
	userAllowedRepos []string
	isAdmin          bool
}

func NewAccessController(config *Config) *AccessController {
	return &AccessController{
		Config: config.HTTP.AccessControl,
		Log:    log.NewLogger(config.Log.Level, config.Log.Output),
	}
}

// getReadRepos get repositories from config file that the user has READ perms.
func (a *AccessController) getReadRepos(username string) []string {
	var repos []string

	for r, pg := range a.Config.Repositories {
		for _, p := range pg.Policies {
			if (contains(p.Users, username) && contains(p.Actions, READ)) ||
				contains(pg.DefaultPolicy, READ) {
				repos = append(repos, r)
			}
		}
	}

	return repos
}

// can verify if a user can do action on repository.
func (a *AccessController) can(username, action, repository string) bool {
	can := false
	// check repo based policy
	for r, pg := range a.Config.Repositories {
		if repository == r {
			can = isPermitted(username, action, pg)
		}
	}

	//check admins based policy
	if !can {
		if a.isAdmin(username) {
			if contains(a.Config.AdminPolicy.Actions, action) || contains(a.Config.DefaultAdminPolicy, action) {
				can = true
			}
		}
	}

	return can
}

// isAdmin returns if user is .
func (a *AccessController) isAdmin(username string) bool {
	return contains(a.Config.AdminPolicy.Users, username)
}

// isPermitted returns true if username can do action on a repository policy.
func isPermitted(username, action string, pg PolicyGroup) bool {
	var result bool
	// check repo/system based policies
	for _, p := range pg.Policies {
		if contains(p.Users, username) && contains(p.Actions, action) {
			result = true
			break
		}
	}

	// check defaultPolicy
	if !result {
		if contains(pg.DefaultPolicy, action) {
			result = true
		}
	}

	return result
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}

	return false
}

func AuthzHandler(c *Controller) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			resource := vars["name"]
			reference, ok := vars["reference"]

			ac := NewAccessController(c.Config)
			username := getUsername(r)

			// build acl context for this user and pass it down
			userAllowedRepos := ac.getReadRepos(username)
			acContext := AccessControllerContext{userAllowedRepos: userAllowedRepos}

			if ac.isAdmin(username) {
				acContext.isAdmin = true
			} else {
				acContext.isAdmin = false
			}
			ctx := context.WithValue(r.Context(), contextKeyID, acContext)
			if r.RequestURI == "/v2/_catalog" || r.RequestURI == "/v2/" {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			var action string
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				action = READ
			}

			if r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodPost {
				// assume user wants to create
				action = CREATE
				if ok {
					is := c.StoreController.GetImageStore(resource)
					tags, err := is.GetImageTags(resource)
					// if repo exists and request's tag doesn't exist yet then action is UPDATE
					if err == nil && contains(tags, reference) && reference != "latest" {
						action = UPDATE
					}
				}
			}

			if r.Method == http.MethodDelete {
				action = DELETE
			}

			can := ac.can(username, action, resource)
			if !can {
				authzFail(w, c.Config.HTTP.Realm, c.Config.HTTP.Auth.FailDelay)
			} else {
				next.ServeHTTP(w, r.WithContext(ctx))
			}
		})
	}
}

func getUsername(r *http.Request) string {
	// this should work because it worked in auth middleware
	basicAuth := r.Header.Get("Authorization")
	s := strings.SplitN(basicAuth, " ", 2)
	b, _ := base64.StdEncoding.DecodeString(s[1])
	pair := strings.SplitN(string(b), ":", 2)

	return pair[0]
}

func isBearerAuthEnabled(config *Config) bool {
	if config.HTTP.Auth.Bearer != nil &&
		config.HTTP.Auth.Bearer.Cert != "" &&
		config.HTTP.Auth.Bearer.Realm != "" &&
		config.HTTP.Auth.Bearer.Service != "" {
		return true
	}

	return false
}

func authzFail(w http.ResponseWriter, realm string, delay int) {
	time.Sleep(time.Duration(delay) * time.Second)
	w.Header().Set("WWW-Authenticate", realm)
	w.Header().Set("Content-Type", "application/json")
	WriteJSON(w, http.StatusForbidden, NewErrorList(NewError(DENIED)))
}
