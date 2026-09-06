//go:build sync

package sync

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/regclient/regclient/types/manifest"
	"golang.org/x/sync/singleflight"

	zerr "zotregistry.dev/zot/v2/errors"
	"zotregistry.dev/zot/v2/pkg/common"
	"zotregistry.dev/zot/v2/pkg/log"
)

type request struct {
	repo      string
	reference string
	// used for background retries, at most one background retry per service
	serviceID    int
	isBackground bool
}

/*
BaseOnDemand tracks on-demand image/referrer sync requests.

Concurrent SyncImage/SyncReferrers calls for the same key are deduplicated with
singleflight (one upstream sync, shared result). requestStore tracks in-flight
background retry goroutines so at most one retry runs per service/key.
*/
type BaseOnDemand struct {
	services []Service
	// background retry dedup: map[request]struct{}
	requestStore   *sync.Map
	imageFlight    singleflight.Group
	referrerFlight singleflight.Group
	streamManager  StreamManager
	log            log.Logger
}

func NewOnDemand(log log.Logger) *BaseOnDemand {
	return &BaseOnDemand{log: log, requestStore: &sync.Map{}}
}

func (onDemand *BaseOnDemand) Add(service Service) {
	onDemand.services = append(onDemand.services, service)
}

// SetStreamManager wires the stream manager shared by every streaming-enabled service into this
// on-demand handler. Left nil when no registry config enables streaming.
func (onDemand *BaseOnDemand) SetStreamManager(sm StreamManager) {
	onDemand.streamManager = sm
}

func (onDemand *BaseOnDemand) StreamManager() StreamManager {
	return onDemand.streamManager
}

// IsStreamingEnabledForRepo returns true if any on-demand service streams blobs for repo.
//
// Note: this only gates whether the caller attempts to stream at all - it does not guarantee the
// service that actually ends up serving repo (the first one in onDemand.services whose
// FetchManifest/SyncImage succeeds, chosen independently by FetchManifestForStream and syncImage)
// is one of the streaming-enabled ones. That match only holds if streaming-enabled services are
// listed so they win the eligibility race for their own repos; a config with multiple registries
// matching the same repo, only some of which stream, is a known gap in this v1.
func (onDemand *BaseOnDemand) IsStreamingEnabledForRepo(repo string) bool {
	for _, service := range onDemand.services {
		if service.IsStreamingForRepo(repo) {
			return true
		}
	}

	return false
}

// FetchManifestForStream fetches repo:reference's manifest directly from upstream and registers
// it (and its blobs) with the stream manager, then kicks off the real sync into local storage in
// the background and returns the manifest immediately - the caller can start serving/streaming
// it to a client without waiting for that background sync to finish.
//
// If repo:reference is already staged for streaming (e.g. a second client requesting the same
// image while the first client's background sync is still running), the cached manifest is
// returned directly and no second background sync is started. In the remaining race where two
// callers both pass that check before either stages the manifest, both still call the exported
// SyncImage below - which is deduplicated via imageFlight - so at most one real sync ever runs.
func (onDemand *BaseOnDemand) FetchManifestForStream(ctx context.Context, repo, reference string,
) (manifest.Manifest, error) {
	if cached, ok := onDemand.streamManager.StreamingImageManifest(repo, reference); ok {
		onDemand.log.Debug().Str("repo", repo).Str("reference", reference).
			Msg("streaming manifest already present in cache")

		return cached.referenceManifest, nil
	}

	var resultManifest manifest.Manifest

	var subManifests []manifest.Manifest

	var lastErr error

	for _, service := range onDemand.services {
		onDemand.log.Debug().Str("repo", repo).Str("reference", reference).Msg("attempting to fetch manifest")

		fetchedManifest, subs, err := service.FetchManifest(ctx, repo, reference)
		if err != nil {
			lastErr = err

			continue
		}

		resultManifest, subManifests = fetchedManifest, subs

		break
	}

	if resultManifest == nil {
		// Surface the last service's error (e.g. ErrSyncImageNotSigned, ErrSyncImageFilteredOut)
		// instead of always reporting ErrBlobNotFound, so a policy rejection is visible to the
		// caller rather than looking like a plain 404.
		if lastErr != nil {
			return nil, lastErr
		}

		return nil, zerr.ErrBlobNotFound
	}

	if err := onDemand.streamManager.StoreImageForStreaming(repo, reference,
		NewStreamableManifest(resultManifest, subManifests)); err != nil {
		return nil, err
	}

	onDemand.log.Debug().Str("repo", repo).Str("reference", reference).Msg("syncing image in the background")

	go func() {
		syncCtx := context.WithoutCancel(ctx)
		if err := onDemand.SyncImage(syncCtx, repo, reference); err != nil {
			onDemand.log.Err(err).Str("repository", repo).Str("reference", reference).
				Msg("background sync after streaming failed")
		}
	}()

	return resultManifest, nil
}

// ShouldCheckUpstreamManifest reports whether the manifest for repo:reference has to be
// validated against upstream. Only a service that completed a successful check records a
// timestamp, so a single service reporting that the interval has not elapsed means this
// reference was verified recently and can be served from local storage.
func (onDemand *BaseOnDemand) ShouldCheckUpstreamManifest(repo, reference string) bool {
	for _, service := range onDemand.services {
		if !service.ShouldCheckUpstream(repo, reference) {
			return false
		}
	}

	return true
}

func onDemandKey(repo, reference string) string {
	return repo + "\x00" + reference
}

func (onDemand *BaseOnDemand) SyncImage(ctx context.Context, repo, reference string) error {
	key := onDemandKey(repo, reference)

	// leader is set only in the closure that actually runs; waiters never execute it.
	leader := false

	_, err, shared := onDemand.imageFlight.Do(key, func() (any, error) {
		leader = true

		return nil, onDemand.syncImage(ctx, repo, reference)
	})

	// singleflight sets shared for every participant when dups > 0, including the leader.
	if shared && !leader {
		onDemand.log.Info().Str("repo", repo).Str("reference", reference).
			Msg("image already demanded, on-demand sync result was shared")
	}

	return err
}

func (onDemand *BaseOnDemand) SyncReferrers(ctx context.Context, repo string,
	subjectDigestStr string, referenceTypes []string,
) error {
	key := onDemandKey(repo, subjectDigestStr)

	// leader is set only in the closure that actually runs; waiters never execute it.
	leader := false

	_, err, shared := onDemand.referrerFlight.Do(key, func() (any, error) {
		leader = true

		return nil, onDemand.syncReferrers(ctx, repo, subjectDigestStr, referenceTypes)
	})

	// singleflight sets shared for every participant when dups > 0, including the leader.
	if shared && !leader {
		onDemand.log.Info().Str("repo", repo).Str("reference", subjectDigestStr).
			Msg("referrers for image already demanded, on-demand sync result was shared")
	}

	return err
}

func (onDemand *BaseOnDemand) syncReferrers(ctx context.Context, repo, subjectDigestStr string,
	referenceTypes []string,
) error {
	var err error

	for serviceID, service := range onDemand.services {
		timeout := service.GetSyncTimeout()

		onDemand.log.Debug().
			Str("repo", repo).
			Str("reference", subjectDigestStr).
			Int("serviceID", serviceID).
			Dur("timeout", timeout).
			Msg("starting on-demand referrer sync")

		// Create a detached context with timeout to ensure sync completes even if HTTP client disconnects.
		// This prevents Kubernetes timeout/retries from aborting in-progress referrer downloads.
		syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		err = service.SyncReferrers(syncCtx, repo, subjectDigestStr, referenceTypes)

		cancel()

		if err != nil {
			if errors.Is(err, zerr.ErrManifestNotFound) ||
				errors.Is(err, zerr.ErrSyncImageFilteredOut) ||
				errors.Is(err, zerr.ErrSyncImageNotSigned) ||
				errors.Is(err, zerr.ErrRepoNotFound) ||
				// some public registries may return 401 for not found.
				errors.Is(err, zerr.ErrUnauthorizedAccess) {
				continue
			}

			req := request{
				repo:         repo,
				reference:    subjectDigestStr,
				serviceID:    serviceID,
				isBackground: true,
			}

			// if there is already a background routine, skip
			if _, requested := onDemand.requestStore.LoadOrStore(req, struct{}{}); requested {
				continue
			}

			if service.CanRetryOnError() {
				retryErr := err

				// retry in background
				go func(service Service, serviceTimeout time.Duration) {
					// remove image after syncing
					defer func() {
						onDemand.requestStore.Delete(req)
						onDemand.log.Info().Str("repo", repo).Str("reference", subjectDigestStr).
							Msg("sync routine for image exited")
					}()

					onDemand.log.Info().Str("repo", repo).Str("reference", subjectDigestStr).Str("err", retryErr.Error()).
						Msg("sync routine: starting routine to copy image, because of error")

					// Use detached context with timeout for background retry
					retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), serviceTimeout)
					defer cancel()

					err := service.SyncReferrers(retryCtx, repo, subjectDigestStr, referenceTypes)
					if err != nil {
						onDemand.log.Error().Str("errorType", common.TypeOf(err)).Str("repo", repo).Str("reference", subjectDigestStr).
							Err(err).Msg("sync routine: starting routine to retry copy image due to error")
					}
				}(service, timeout)
			}
		} else {
			break
		}
	}

	return err
}

func (onDemand *BaseOnDemand) syncImage(ctx context.Context, repo, reference string) error {
	var err error

	for serviceID, service := range onDemand.services {
		timeout := service.GetSyncTimeout()

		onDemand.log.Debug().
			Str("repo", repo).
			Str("reference", reference).
			Int("serviceID", serviceID).
			Dur("timeout", timeout).
			Msg("starting on-demand image sync")

		// Create a detached context with timeout to ensure sync completes even if HTTP client disconnects.
		// This prevents Kubernetes timeout/retries from aborting in-progress image downloads.
		syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		err = service.SyncImage(syncCtx, repo, reference)

		cancel()

		if err != nil {
			if errors.Is(err, zerr.ErrManifestNotFound) ||
				errors.Is(err, zerr.ErrSyncImageFilteredOut) ||
				errors.Is(err, zerr.ErrSyncImageNotSigned) ||
				errors.Is(err, zerr.ErrRepoNotFound) ||
				// some public registries may return 401 for not found.
				errors.Is(err, zerr.ErrUnauthorizedAccess) {
				continue
			}

			req := request{
				repo:         repo,
				reference:    reference,
				serviceID:    serviceID,
				isBackground: true,
			}

			// if there is already a background routine, skip
			if _, requested := onDemand.requestStore.LoadOrStore(req, struct{}{}); requested {
				continue
			}

			if service.CanRetryOnError() {
				retryErr := err

				// retry in background
				go func(service Service, serviceTimeout time.Duration) {
					// remove image after syncing
					defer func() {
						onDemand.requestStore.Delete(req)
						onDemand.log.Info().Str("repo", repo).Str("reference", reference).
							Msg("sync routine for image exited")
					}()

					onDemand.log.Info().Str("repo", repo).Str("reference", reference).Str("err", retryErr.Error()).
						Msg("sync routine: starting routine to retry copy image due to error")

					// Use detached context with timeout for background retry
					retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), serviceTimeout)
					defer cancel()

					err := service.SyncImage(retryCtx, repo, reference)
					if err != nil {
						onDemand.log.Error().Str("errorType", common.TypeOf(err)).Str("repo", repo).Str("reference", reference).
							Err(err).Msg("sync routine: error while copying image")
					}
				}(service, timeout)
			}
		} else {
			break
		}
	}

	return err
}
