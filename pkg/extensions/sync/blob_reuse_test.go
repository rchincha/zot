//go:build sync

package sync //nolint:testpackage // white-box test for unexported preseedLocalBlobs/preseedBlob

import (
	"context"
	"io"
	"os"
	"path"
	"testing"

	godigest "github.com/opencontainers/go-digest"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/types/manifest"
	"github.com/regclient/regclient/types/ref"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zotregistry.dev/zot/v2/pkg/log"
	"zotregistry.dev/zot/v2/pkg/storage"
)

// newTestBaseService builds a minimal *BaseService with just enough wired up (storeController,
// logger) to call preseedLocalBlobs directly, without a real upstream registry.
func newTestBaseService(t *testing.T, storeCtrl storage.StoreController) *BaseService {
	t.Helper()

	return &BaseService{storeController: storeCtrl, log: log.NewTestLogger()}
}

func TestPreseedLocalBlobs(t *testing.T) {
	t.Parallel()

	root, storeCtrl := newTestStore(t)
	writeOCISingleManifest(t, storeCtrl, root, "repo-a", predictTestTag)

	concrete, ok := storeCtrl.(storage.StoreController)
	require.True(t, ok)

	service := newTestBaseService(t, concrete)

	regClient := regclient.New()
	srcRef := mustOCIDirRef(t, repoPath(root, "repo-a"), predictTestTag)

	man, err := regClient.ManifestGet(context.Background(), srcRef)
	require.NoError(t, err)

	defer regClient.Close(context.Background(), man.GetRef())

	imager, ok := man.(manifest.Imager)
	require.True(t, ok)

	configDesc, err := imager.GetConfig()
	require.NoError(t, err)

	layers, err := imager.GetLayers()
	require.NoError(t, err)

	presentDigests := []godigest.Digest{configDesc.Digest}
	for _, layer := range layers {
		presentDigests = append(presentDigests, layer.Digest)
	}

	// A digest the local store does not have must be skipped without error, not fail the batch.
	missingDigest := godigest.FromString("this blob was never synced")
	blobDigests := append(append([]godigest.Digest{}, presentDigests...), missingDigest)

	localImageRef := mustOCIDirRef(t, path.Join(t.TempDir(), "repo-a"), predictTestTag)

	seeded := service.preseedLocalBlobs(context.Background(), "repo-a", localImageRef, blobDigests)
	assert.Equal(t, len(presentDigests), seeded, "every locally-present digest must be seeded, the missing one skipped")

	imageStore := concrete.GetImageStore("repo-a")

	for _, digest := range presentDigests {
		destPath := path.Join(localImageRef.Path, "blobs", digest.Algorithm().String(), digest.Encoded())

		written, err := os.ReadFile(destPath)
		require.NoError(t, err, "digest %s must have been written to the temp OCI layout", digest)

		srcReader, _, err := imageStore.GetBlob("repo-a", digest, "")
		require.NoError(t, err)

		expected, err := io.ReadAll(srcReader)
		require.NoError(t, err)
		require.NoError(t, srcReader.Close())

		assert.Equal(t, expected, written, "seeded content for %s must match the real local blob", digest)
	}

	missingPath := path.Join(localImageRef.Path, "blobs", missingDigest.Algorithm().String(), missingDigest.Encoded())
	_, err = os.Stat(missingPath)
	assert.True(t, os.IsNotExist(err), "a digest absent from local storage must not be written")

	// Re-seeding onto a destination that already has the file must be a safe no-op.
	seededAgain := service.preseedLocalBlobs(context.Background(), "repo-a", localImageRef, blobDigests)
	assert.Equal(t, len(presentDigests), seededAgain)
}

func TestPreseedLocalBlobsNoOpCases(t *testing.T) {
	t.Parallel()

	_, storeCtrl := newTestStore(t)
	concrete, ok := storeCtrl.(storage.StoreController)
	require.True(t, ok)

	service := newTestBaseService(t, concrete)

	t.Run("empty blob digest list is a no-op", func(t *testing.T) {
		t.Parallel()

		localImageRef := mustOCIDirRef(t, path.Join(t.TempDir(), "repo"), predictTestTag)
		assert.Equal(t, 0, service.preseedLocalBlobs(context.Background(), "repo", localImageRef, nil))
	})

	t.Run("a ref with no path is a no-op", func(t *testing.T) {
		t.Parallel()

		digests := []godigest.Digest{godigest.FromString("x")}
		assert.Equal(t, 0, service.preseedLocalBlobs(context.Background(), "repo", ref.Ref{}, digests))
	})
}
