package search //nolint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
<<<<<<< HEAD
	v1 "github.com/google/go-containerregistry/pkg/v1"
	godigest "github.com/opencontainers/go-digest"
=======
>>>>>>> e3cb60b (boltdb query logic)
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rs/zerolog"
	. "github.com/smartystreets/goconvey/convey"
	"zotregistry.io/zot/pkg/extensions/monitoring"
	"zotregistry.io/zot/pkg/extensions/search/gql_generated"
	"zotregistry.io/zot/pkg/log"
	localCtx "zotregistry.io/zot/pkg/requestcontext"
	"zotregistry.io/zot/pkg/storage"
	"zotregistry.io/zot/pkg/storage/repodb"
	"zotregistry.io/zot/pkg/test/mocks"
)

var ErrTestError = errors.New("TestError")

func TestGlobalSearch(t *testing.T) {
	Convey("globalSearch", t, func() {
		const query = "repo1"
		Convey("RepoDB SearchRepos error", func() {
			mockSearchDB := mocks.RepoDBMock{
				SearchReposFn: func(ctx context.Context, searchText string, requestedPage repodb.PageInput,
				) ([]repodb.RepoMetadata, map[string]repodb.ManifestMetadata, error) {
					return make([]repodb.RepoMetadata, 0), make(map[string]repodb.ManifestMetadata), ErrTestError
				},
			}
<<<<<<< HEAD
			mockCve := mocks.CveInfoMock{}

			globalSearch([]string{"repo1"}, "name", "tag", mockOlum, mockCve, log.NewLogger("debug", ""))
		})

		Convey("GetImageTagsWithTimestamp fail", func() {
			mockOlum := mocks.OciLayoutUtilsMock{
				GetImageTagsWithTimestampFn: func(repo string) ([]common.TagInfo, error) {
					return []common.TagInfo{}, ErrTestError
				},
			}
			mockCve := mocks.CveInfoMock{}

			globalSearch([]string{"repo1"}, "name", "tag", mockOlum, mockCve, log.NewLogger("debug", ""))
		})

		Convey("GetImageManifests fail", func() {
			mockOlum := mocks.OciLayoutUtilsMock{
				GetImageManifestsFn: func(name string) ([]ispec.Descriptor, error) {
					return []ispec.Descriptor{}, ErrTestError
				},
			}
			mockCve := mocks.CveInfoMock{}

			globalSearch([]string{"repo1"}, "name", "tag", mockOlum, mockCve, log.NewLogger("debug", ""))
		})

		Convey("Manifests given, bad image blob manifest", func() {
			mockOlum := mocks.OciLayoutUtilsMock{
				GetImageManifestsFn: func(name string) ([]ispec.Descriptor, error) {
					return []ispec.Descriptor{
=======
			responseContext := graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter,
				graphql.DefaultRecover)
			repos, images, layers, err := globalSearch(responseContext, query, mockSearchDB, &gql_generated.PageInput{},
				log.NewLogger("debug", ""))
			So(err, ShouldNotBeNil)
			So(images, ShouldBeEmpty)
			So(layers, ShouldBeEmpty)
			So(repos, ShouldBeEmpty)
		})

		Convey("RepoDB SearchRepo is successful", func() {
			mockSearchDB := mocks.RepoDBMock{
				SearchReposFn: func(ctx context.Context, searchText string, requestedPage repodb.PageInput,
				) ([]repodb.RepoMetadata, map[string]repodb.ManifestMetadata, error) {
					repos := []repodb.RepoMetadata{
>>>>>>> e3cb60b (boltdb query logic)
						{
							Name: "repo1",
							Tags: map[string]string{
								"1.0.1": "digestTag1.0.1",
								"1.0.2": "digestTag1.0.2",
							},
							Signatures:  []string{"testSignature"},
							Stars:       100,
							Description: "Descriptions repo1",
							LogoPath:    "test/logoPath",
						},
					}

					createTime := time.Now()
					configBlob1, err := json.Marshal(ispec.Image{
						Config: ispec.ImageConfig{
							Labels: map[string]string{
								ispec.AnnotationVendor: "TestVendor1",
							},
						},
<<<<<<< HEAD
					}, nil
				},
				GetImageBlobManifestFn: func(imageDir string, digest godigest.Digest) (v1.Manifest, error) {
					return v1.Manifest{}, ErrTestError
				},
			}
			mockCve := mocks.CveInfoMock{}

			globalSearch([]string{"repo1"}, "name", "tag", mockOlum, mockCve, log.NewLogger("debug", ""))
		})

		Convey("Manifests given, no manifest tag", func() {
			mockOlum := mocks.OciLayoutUtilsMock{
				GetImageManifestsFn: func(name string) ([]ispec.Descriptor, error) {
					return []ispec.Descriptor{
						{
							Digest: "digest",
							Size:   -1,
						},
					}, nil
				},
			}
			mockCve := mocks.CveInfoMock{}

			globalSearch([]string{"repo1"}, "test", "tag", mockOlum, mockCve, log.NewLogger("debug", ""))
		})

		Convey("Global search success, no tag", func() {
			mockOlum := mocks.OciLayoutUtilsMock{
				GetRepoLastUpdatedFn: func(repo string) (common.TagInfo, error) {
					return common.TagInfo{
						Digest: "sha256:855b1556a45637abf05c63407437f6f305b4627c4361fb965a78e5731999c0c7",
					}, nil
				},
				GetImageManifestsFn: func(name string) ([]ispec.Descriptor, error) {
					return []ispec.Descriptor{
						{
							Digest: "sha256:855b1556a45637abf05c63407437f6f305b4627c4361fb965a78e5731999c0c7",
							Size:   -1,
							Annotations: map[string]string{
								ispec.AnnotationRefName: "this is a bad format",
=======
						Created: &createTime,
					})
					So(err, ShouldBeNil)

					configBlob2, err := json.Marshal(ispec.Image{
						Config: ispec.ImageConfig{
							Labels: map[string]string{
								ispec.AnnotationVendor: "TestVendor2",
>>>>>>> e3cb60b (boltdb query logic)
							},
						},
					})
					So(err, ShouldBeNil)

					manifestBlob, err := json.Marshal(ispec.Manifest{})
					So(err, ShouldBeNil)

					manifestMetas := map[string]repodb.ManifestMetadata{
						"digestTag1.0.1": {
							ManifestBlob:  manifestBlob,
							ConfigBlob:    configBlob1,
							DownloadCount: 100,
							Signatures:    make(map[string][]string),
							Dependencies:  make([]string, 0),
							Dependants:    make([]string, 0),
							BlobsSize:     0,
							BlobCount:     0,
						},
						"digestTag1.0.2": {
							ManifestBlob:  manifestBlob,
							ConfigBlob:    configBlob2,
							DownloadCount: 100,
							Signatures:    make(map[string][]string),
							Dependencies:  make([]string, 0),
							Dependants:    make([]string, 0),
							BlobsSize:     0,
							BlobCount:     0,
						},
					}

					return repos, manifestMetas, nil
				},
			}
<<<<<<< HEAD
			mockCve := mocks.CveInfoMock{}
			globalSearch([]string{"repo1/name"}, "name", "tag", mockOlum, mockCve, log.NewLogger("debug", ""))
=======

			const query = "repo1"
			limit := 1
			ofset := 0
			sortCriteria := gql_generated.SortCriteriaAlphabeticAsc
			pageInput := gql_generated.PageInput{
				Limit:  &limit,
				Offset: &ofset,
				SortBy: &sortCriteria,
			}

			responseContext := graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter,
				graphql.DefaultRecover)
			repos, images, layers, err := globalSearch(responseContext, query, mockSearchDB, &pageInput,
				log.NewLogger("debug", ""))
			So(err, ShouldBeNil)
			So(images, ShouldBeEmpty)
			So(layers, ShouldBeEmpty)
			So(repos, ShouldNotBeEmpty)
			So(len(repos[0].Vendors), ShouldEqual, 2)
>>>>>>> e3cb60b (boltdb query logic)
		})

		Convey("RepoDB SearchRepo Bad manifest refferenced", func() {
			mockSearchDB := mocks.RepoDBMock{
				SearchReposFn: func(ctx context.Context, searchText string, requestedPage repodb.PageInput,
				) ([]repodb.RepoMetadata, map[string]repodb.ManifestMetadata, error) {
					repos := []repodb.RepoMetadata{
						{
							Name: "repo1",
							Tags: map[string]string{
								"1.0.1": "digestTag1.0.1",
							},
							Signatures:  []string{"testSignature"},
							Stars:       100,
							Description: "Descriptions repo1",
							LogoPath:    "test/logoPath",
						},
					}

					configBlob, err := json.Marshal(ispec.Image{})
					So(err, ShouldBeNil)

					manifestMetas := map[string]repodb.ManifestMetadata{
						"digestTag1.0.1": {
							ManifestBlob:  []byte("bad manifest blob"),
							ConfigBlob:    configBlob,
							DownloadCount: 100,
							Signatures:    make(map[string][]string),
							Dependencies:  make([]string, 0),
							Dependants:    make([]string, 0),
							BlobsSize:     0,
							BlobCount:     0,
						},
					}

					return repos, manifestMetas, nil
				},
			}
<<<<<<< HEAD
			mockCve := mocks.CveInfoMock{}
			globalSearch([]string{"repo1/name"}, "name", "tag", mockOlum, mockCve, log.NewLogger("debug", ""))
		})

		Convey("Tag given, no layer match", func() {
			mockOlum := mocks.OciLayoutUtilsMock{
				GetExpandedRepoInfoFn: func(name string) (common.RepoInfo, error) {
					return common.RepoInfo{
						ImageSummaries: []common.ImageSummary{
							{
								Tag: "latest",
								Layers: []common.Layer{
									{
										Size:   "100",
										Digest: "sha256:855b1556a45637abf05c63407437f6f305b4627c4361fb965a78e5731999c0c7",
									},
								},
							},
						},
					}, nil
				},
				GetImageManifestSizeFn: func(repo string, manifestDigest godigest.Digest) int64 {
					return 100
				},
				GetImageConfigSizeFn: func(repo string, manifestDigest godigest.Digest) int64 {
					return 100
				},
				GetImageTagsWithTimestampFn: func(repo string) ([]common.TagInfo, error) {
					return []common.TagInfo{
=======

			query := "repo1"
			limit := 1
			ofset := 0
			sortCriteria := gql_generated.SortCriteriaAlphabeticAsc
			pageInput := gql_generated.PageInput{
				Limit:  &limit,
				Offset: &ofset,
				SortBy: &sortCriteria,
			}

			responseContext := graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter,
				graphql.DefaultRecover)

			repos, images, layers, err := globalSearch(responseContext, query, mockSearchDB, &pageInput,
				log.NewLogger("debug", ""))
			So(err, ShouldBeNil)
			So(images, ShouldBeEmpty)
			So(layers, ShouldBeEmpty)
			So(repos, ShouldNotBeEmpty)

			query = "repo1:1.0.1"

			responseContext = graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter,
				graphql.DefaultRecover)
			repos, images, layers, err = globalSearch(responseContext, query, mockSearchDB, &pageInput,
				log.NewLogger("debug", ""))
			So(err, ShouldBeNil)
			So(images, ShouldBeEmpty)
			So(layers, ShouldBeEmpty)
			So(repos, ShouldBeEmpty)
		})

		Convey("RepoDB SearchRepo good manifest refferenced and bad config blob", func() {
			mockSearchDB := mocks.RepoDBMock{
				SearchReposFn: func(ctx context.Context, searchText string, requestedPage repodb.PageInput,
				) ([]repodb.RepoMetadata, map[string]repodb.ManifestMetadata, error) {
					repos := []repodb.RepoMetadata{
>>>>>>> e3cb60b (boltdb query logic)
						{
							Name: "repo1",
							Tags: map[string]string{
								"1.0.1": "digestTag1.0.1",
							},
							Signatures:  []string{"testSignature"},
							Stars:       100,
							Description: "Descriptions repo1",
							LogoPath:    "test/logoPath",
						},
					}

					manifestBlob, err := json.Marshal(ispec.Manifest{})
					So(err, ShouldBeNil)

					manifestMetas := map[string]repodb.ManifestMetadata{
						"digestTag1.0.1": {
							ManifestBlob:  manifestBlob,
							ConfigBlob:    []byte("bad config blob"),
							DownloadCount: 100,
							Signatures:    make(map[string][]string),
							Dependencies:  make([]string, 0),
							Dependants:    make([]string, 0),
							BlobsSize:     0,
							BlobCount:     0,
						},
					}

					return repos, manifestMetas, nil
				},
			}
<<<<<<< HEAD
			mockCve := mocks.CveInfoMock{}
			globalSearch([]string{"repo1"}, "name", "tag", mockOlum, mockCve, log.NewLogger("debug", ""))
		})
	})
}

func TestRepoListWithNewestImage(t *testing.T) {
	Convey("repoListWithNewestImage", t, func() {
		Convey("GetImageManifests fail", func() {
			mockOlum := mocks.OciLayoutUtilsMock{
				GetImageManifestsFn: func(image string) ([]ispec.Descriptor, error) {
					return []ispec.Descriptor{}, ErrTestError
				},
			}

			ctx := graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.Recover)
			mockCve := mocks.CveInfoMock{}
			_, err := repoListWithNewestImage(ctx, []string{"repo1"}, mockOlum, mockCve, log.NewLogger("debug", ""))
			So(err, ShouldBeNil)

			errs := graphql.GetErrors(ctx)
			So(errs, ShouldNotBeEmpty)
		})

		Convey("GetImageBlobManifest fail", func() {
			mockOlum := mocks.OciLayoutUtilsMock{
				GetImageBlobManifestFn: func(imageDir string, digest godigest.Digest) (v1.Manifest, error) {
					return v1.Manifest{}, ErrTestError
				},
				GetImageManifestsFn: func(image string) ([]ispec.Descriptor, error) {
					return []ispec.Descriptor{
						{
							MediaType: "application/vnd.oci.image.layer.v1.tar",
							Size:      int64(0),
						},
					}, nil
				},
			}

			ctx := graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.Recover)
			mockCve := mocks.CveInfoMock{}
			_, err := repoListWithNewestImage(ctx, []string{"repo1"}, mockOlum, mockCve, log.NewLogger("debug", ""))
			So(err, ShouldBeNil)

			errs := graphql.GetErrors(ctx)
			So(errs, ShouldNotBeEmpty)
		})

		Convey("GetImageConfigInfo fail", func() {
			mockOlum := mocks.OciLayoutUtilsMock{
				GetImageManifestsFn: func(image string) ([]ispec.Descriptor, error) {
					return []ispec.Descriptor{
						{
							MediaType: "application/vnd.oci.image.layer.v1.tar",
							Size:      int64(0),
						},
					}, nil
				},
				GetImageConfigInfoFn: func(repo string, manifestDigest godigest.Digest) (ispec.Image, error) {
					return ispec.Image{
						Author: "test",
					}, ErrTestError
				},
			}

			ctx := graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.Recover)
			mockCve := mocks.CveInfoMock{}
			_, err := repoListWithNewestImage(ctx, []string{"repo1"}, mockOlum, mockCve, log.NewLogger("debug", ""))
			So(err, ShouldBeNil)

			errs := graphql.GetErrors(ctx)
			So(errs, ShouldNotBeEmpty)
=======

			query := "repo1"
			limit := 1
			ofset := 0
			sortCriteria := gql_generated.SortCriteriaAlphabeticAsc
			pageInput := gql_generated.PageInput{
				Limit:  &limit,
				Offset: &ofset,
				SortBy: &sortCriteria,
			}

			responseContext := graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter,
				graphql.DefaultRecover)
			repos, images, layers, err := globalSearch(responseContext, query, mockSearchDB, &pageInput,
				log.NewLogger("debug", ""))
			So(err, ShouldBeNil)
			So(images, ShouldBeEmpty)
			So(layers, ShouldBeEmpty)
			So(repos, ShouldNotBeEmpty)

			query = "repo1:1.0.1"
			responseContext = graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter,
				graphql.DefaultRecover)
			repos, images, layers, err = globalSearch(responseContext, query, mockSearchDB, &pageInput,
				log.NewLogger("debug", ""))
			So(err, ShouldBeNil)
			So(images, ShouldBeEmpty)
			So(layers, ShouldBeEmpty)
			So(repos, ShouldBeEmpty)
		})

		Convey("RepoDB SearchTags gives error", func() {
			mockSearchDB := mocks.RepoDBMock{
				SearchTagsFn: func(ctx context.Context, searchText string, requestedPage repodb.PageInput,
				) ([]repodb.RepoMetadata, map[string]repodb.ManifestMetadata, error) {
					return make([]repodb.RepoMetadata, 0), make(map[string]repodb.ManifestMetadata), ErrTestError
				},
			}
			const query = "repo1:1.0.1"

			responseContext := graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter,
				graphql.DefaultRecover)
			repos, images, layers, err := globalSearch(responseContext, query, mockSearchDB, &gql_generated.PageInput{},
				log.NewLogger("debug", ""))
			So(err, ShouldNotBeNil)
			So(images, ShouldBeEmpty)
			So(layers, ShouldBeEmpty)
			So(repos, ShouldBeEmpty)
		})

		Convey("RepoDB SearchTags is successful", func() {
			mockSearchDB := mocks.RepoDBMock{
				SearchTagsFn: func(ctx context.Context, searchText string, requestedPage repodb.PageInput,
				) ([]repodb.RepoMetadata, map[string]repodb.ManifestMetadata, error) {
					repos := []repodb.RepoMetadata{
						{
							Name: "repo1",
							Tags: map[string]string{
								"1.0.1": "digestTag1.0.1",
							},
							Signatures:  []string{"testSignature"},
							Stars:       100,
							Description: "Descriptions repo1",
							LogoPath:    "test/logoPath",
						},
					}

					configBlob1, err := json.Marshal(ispec.Image{
						Config: ispec.ImageConfig{
							Labels: map[string]string{
								ispec.AnnotationVendor: "TestVendor1",
							},
						},
					})
					So(err, ShouldBeNil)

					configBlob2, err := json.Marshal(ispec.Image{
						Config: ispec.ImageConfig{
							Labels: map[string]string{
								ispec.AnnotationVendor: "TestVendor2",
							},
						},
					})
					So(err, ShouldBeNil)

					manifestBlob, err := json.Marshal(ispec.Manifest{})
					So(err, ShouldBeNil)

					manifestMetas := map[string]repodb.ManifestMetadata{
						"digestTag1.0.1": {
							ManifestBlob:  manifestBlob,
							ConfigBlob:    configBlob1,
							DownloadCount: 100,
							Signatures:    make(map[string][]string),
							Dependencies:  make([]string, 0),
							Dependants:    make([]string, 0),
							BlobsSize:     0,
							BlobCount:     0,
						},
						"digestTag1.0.2": {
							ManifestBlob:  manifestBlob,
							ConfigBlob:    configBlob2,
							DownloadCount: 100,
							Signatures:    make(map[string][]string),
							Dependencies:  make([]string, 0),
							Dependants:    make([]string, 0),
							BlobsSize:     0,
							BlobCount:     0,
						},
					}

					return repos, manifestMetas, nil
				},
			}

			const query = "repo1:1.0.1"
			limit := 1
			ofset := 0
			sortCriteria := gql_generated.SortCriteriaAlphabeticAsc
			pageInput := gql_generated.PageInput{
				Limit:  &limit,
				Offset: &ofset,
				SortBy: &sortCriteria,
			}

			responseContext := graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter,
				graphql.DefaultRecover)
			repos, images, layers, err := globalSearch(responseContext, query, mockSearchDB, &pageInput,
				log.NewLogger("debug", ""))
			So(err, ShouldBeNil)
			So(images, ShouldNotBeEmpty)
			So(layers, ShouldBeEmpty)
			So(repos, ShouldBeEmpty)
>>>>>>> e3cb60b (boltdb query logic)
		})
	})
}

func TestUserAvailableRepos(t *testing.T) {
	Convey("Type assertion fails", t, func() {
		var invalid struct{}

		log := log.Logger{Logger: zerolog.New(os.Stdout)}
		dir := t.TempDir()
		metrics := monitoring.NewMetricsServer(false, log)
		defaultStore := storage.NewImageStore(dir, false, 0, false, false, log, metrics, nil)

		repoList, err := defaultStore.GetRepositories()
		So(err, ShouldBeNil)

		ctx := context.TODO()
		key := localCtx.GetContextKey()
		ctx = context.WithValue(ctx, key, invalid)

		repos, err := userAvailableRepos(ctx, repoList)
		So(err, ShouldNotBeNil)
		So(repos, ShouldBeEmpty)
	})
}

func TestMatching(t *testing.T) {
	pine := "pine"

	Convey("Perfect Matching", t, func() {
		query := "alpine"
		score := calculateImageMatchingScore("alpine", strings.Index("alpine", query))
		So(score, ShouldEqual, 0)
	})

	Convey("Partial Matching", t, func() {
		query := pine
		score := calculateImageMatchingScore("alpine", strings.Index("alpine", query))
		So(score, ShouldEqual, 2)
	})

	Convey("Complex Partial Matching", t, func() {
		query := pine
		score := calculateImageMatchingScore("repo/test/alpine", strings.Index("alpine", query))
		So(score, ShouldEqual, 2)

		query = pine
		score = calculateImageMatchingScore("repo/alpine/test", strings.Index("alpine", query))
		So(score, ShouldEqual, 2)

		query = pine
		score = calculateImageMatchingScore("alpine/repo/test", strings.Index("alpine", query))
		So(score, ShouldEqual, 2)
	})
}
