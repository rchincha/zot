//go:build search

package search_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/resty.v1"

	"zotregistry.dev/zot/v2/pkg/api"
	"zotregistry.dev/zot/v2/pkg/api/config"
	extconf "zotregistry.dev/zot/v2/pkg/extensions/config"
	. "zotregistry.dev/zot/v2/pkg/test/common"
	. "zotregistry.dev/zot/v2/pkg/test/image-utils"
)

// A read policy or adminPolicy whose CEL condition evaluates false must hide the
// repository from search exactly as it is hidden from direct OCI reads.
func TestSearchHonorsReadPolicyConditions(t *testing.T) {
	Convey("Search hides repos whose read policy condition is false", t, func() {
		const (
			owner, ownerPass = "owner", "owner123"
			alice, alicePass = "alice", "alice123"
			carol, carolPass = "carol", "carol123"
			dave, davePass   = "dave", "dave123"
			privateRepo      = "private/app"
			publicRepo       = "public/app"
			artifactType     = "application/vnd.test.provenance"
		)

		htpasswdPath := MakeHtpasswdFileFromString(t,
			GetBcryptCredString(owner, ownerPass)+"\n"+GetBcryptCredString(alice, alicePass)+"\n"+
				GetBcryptCredString(carol, carolPass)+"\n"+GetBcryptCredString(dave, davePass))

		conf := config.New()
		conf.HTTP.Port = "0"
		conf.HTTP.Auth.HTPasswd = config.AuthHTPasswd{Path: htpasswdPath}
		conf.Storage.RootDirectory = t.TempDir()

		enable := true
		conf.Extensions = &extconf.ExtensionConfig{Search: &extconf.SearchConfig{Enable: &enable}}

		ownerPolicy := config.Policy{
			Users:   []string{owner},
			Actions: []string{"read", "create", "update", "delete"},
		}
		conf.HTTP.AccessControl = &config.AccessControlConfig{
			// carol and dave are admins with no repo policy: only the admin condition decides.
			// It is false for carol (the IP never matches) and true for dave.
			AdminPolicy: config.Policy{
				Users:   []string{carol, dave},
				Actions: []string{"read"},
				Conditions: []config.Condition{{
					Expression: `req.user.username == "dave" || req.client.ip == "127.0.0.9"`,
					Message:    "admin access only from the admin host",
				}},
			},
			Repositories: config.Repositories{
				"private/**": config.PolicyGroup{Policies: []config.Policy{ownerPolicy, {
					Users:      []string{alice},
					Actions:    []string{"read"},
					Conditions: []config.Condition{{Expression: `false`, Message: "alice access expired"}},
				}}},
				"public/**": config.PolicyGroup{Policies: []config.Policy{ownerPolicy, {
					Users:      []string{alice},
					Actions:    []string{"read"},
					Conditions: []config.Condition{{Expression: `true`}},
				}}},
			},
		}

		ctlr := api.NewController(conf)
		cm := NewControllerManager(ctlr)
		baseURL := cm.StartAndWait()

		defer cm.StopServer()

		image := CreateImageWith().RandomLayers(1, 10).DefaultConfig().
			Annotations(map[string]string{ispec.AnnotationTitle: "conditional-title"}).Build()
		digest := image.ManifestDescriptor.Digest.String()

		for _, repo := range []string{privateRepo, publicRepo} {
			So(UploadImageWithBasicAuth(image, baseURL, repo, "latest", owner, ownerPass), ShouldBeNil)

			referrer := CreateImageWith().RandomLayers(1, 10).EmptyConfig().
				ArtifactType(artifactType).Subject(image.DescriptorRef()).Build()
			So(UploadImageWithBasicAuth(referrer, baseURL, repo, referrer.DigestStr(), owner, ownerPass),
				ShouldBeNil)
		}

		aliceClient := resty.New().SetBasicAuth(alice, alicePass)
		carolClient := resty.New().SetBasicAuth(carol, carolPass)
		daveClient := resty.New().SetBasicAuth(dave, davePass)

		// Direct OCI reads, which search must agree with.
		for _, check := range []struct {
			client *resty.Client
			repo   string
			status int
		}{
			{aliceClient, privateRepo, http.StatusForbidden},
			{aliceClient, publicRepo, http.StatusOK},
			{carolClient, privateRepo, http.StatusForbidden},
			{daveClient, privateRepo, http.StatusOK},
		} {
			resp, err := check.client.R().Get(baseURL + "/v2/" + check.repo + "/manifests/latest")
			So(err, ShouldBeNil)
			So(resp.StatusCode(), ShouldEqual, check.status)
		}

		queries := map[string]func(repo string) string{
			"Image": func(repo string) string {
				return `{Image(image:"` + repo + `:latest"){RepoName Tag Digest Title}}`
			},
			"Referrers": func(repo string) string {
				return `{Referrers(repo:"` + repo + `",digest:"` + digest + `"){ArtifactType Digest}}`
			},
			"ImageList": func(repo string) string {
				return `{ImageList(repo:"` + repo + `"){Results{RepoName Tag Digest}}}`
			},
			"ExpandedRepoInfo": func(repo string) string {
				return `{ExpandedRepoInfo(repo:"` + repo + `"){Summary{Name} Images{RepoName Digest}}}`
			},
			"GlobalSearch": func(repo string) string {
				return `{GlobalSearch(query:"` + repo + `"){Repos{Name} Images{RepoName Digest}}}`
			},
			"GlobalSearchFiltered": func(repo string) string {
				return `{GlobalSearch(query:"` + repo + `",filter:{Os:["linux"]}){Repos{Name} Images{RepoName Digest}}}`
			},
			"ImageListForDigest": func(string) string {
				return `{ImageListForDigest(id:"` + digest + `"){Results{RepoName Tag Digest}}}`
			},
			"RepoListWithNewestImage": func(string) string {
				return `{RepoListWithNewestImage{Results{Name NewestImage{Digest}}}}`
			},
			"DerivedImageList": func(repo string) string {
				return `{DerivedImageList(image:"` + repo + `:latest"){Results{RepoName Digest}}}`
			},
			"BaseImageList": func(repo string) string {
				return `{BaseImageList(image:"` + repo + `:latest"){Results{RepoName Digest}}}`
			},
		}

		search := func(client *resty.Client, query string) string {
			resp, err := client.R().Get(baseURL + graphqlQueryPrefix + "?query=" + url.QueryEscape(query))
			So(err, ShouldBeNil)
			So(resp.StatusCode(), ShouldEqual, http.StatusOK)

			// Errors are fine (e.g. "not found"), leaked data is not.
			var body struct {
				Data json.RawMessage `json:"data"`
			}
			So(json.Unmarshal(resp.Body(), &body), ShouldBeNil)

			return string(body.Data)
		}

		for name, query := range queries {
			Convey(name, func() {
				// Referrers results carry no repo name; look for the referrer itself.
				leakMarker := func(repo string) string {
					if name == "Referrers" {
						return artifactType
					}

					return repo
				}

				// Denied by a false repo policy condition, and by a false adminPolicy condition.
				So(search(carolClient, query(privateRepo)), ShouldNotContainSubstring, leakMarker(privateRepo))
				So(search(aliceClient, query(privateRepo)), ShouldNotContainSubstring, leakMarker(privateRepo))

				// Controls: the same query does return data when the condition is true, so the
				// assertions above are not passing because the query is broken.
				switch name {
				case "DerivedImageList", "BaseImageList":
					// no related images exist; nothing to compare against
				default:
					So(search(aliceClient, query(publicRepo)), ShouldContainSubstring, leakMarker(publicRepo))
					So(search(daveClient, query(privateRepo)), ShouldContainSubstring, leakMarker(privateRepo))
				}
			})
		}
	})
}
