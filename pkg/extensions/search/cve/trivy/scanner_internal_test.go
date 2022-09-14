package trivy

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"zotregistry.io/zot/pkg/api/config"
	extconf "zotregistry.io/zot/pkg/extensions/config"
	"zotregistry.io/zot/pkg/extensions/monitoring"
	"zotregistry.io/zot/pkg/extensions/search/common"
	"zotregistry.io/zot/pkg/log"
	"zotregistry.io/zot/pkg/storage"
)

func TestMultipleStoragePath(t *testing.T) {
	Convey("Test multiple storage path", t, func() {
		// Create temporary directory
		firstRootDir := t.TempDir()
		secondRootDir := t.TempDir()
		thirdRootDir := t.TempDir()

		log := log.NewLogger("debug", "")
		metrics := monitoring.NewMetricsServer(false, log)

		conf := config.New()
		conf.Extensions = &extconf.ExtensionConfig{}
		conf.Extensions.Lint = &extconf.LintConfig{}

		// Create ImageStore
		firstStore := storage.NewImageStore(firstRootDir, false, storage.DefaultGCDelay, false, false, log, metrics, nil)

		secondStore := storage.NewImageStore(secondRootDir, false, storage.DefaultGCDelay, false, false, log, metrics, nil)

		thirdStore := storage.NewImageStore(thirdRootDir, false, storage.DefaultGCDelay, false, false, log, metrics, nil)

		storeController := storage.StoreController{}

		storeController.DefaultStore = firstStore

		subStore := make(map[string]storage.ImageStore)

		subStore["/a"] = secondStore
		subStore["/b"] = thirdStore

		storeController.SubStore = subStore

		layoutUtils := common.NewBaseOciLayoutUtils(storeController, log)

		scanner := NewScanner(storeController, layoutUtils, log)

		So(scanner.storeController.DefaultStore, ShouldNotBeNil)
		So(scanner.storeController.SubStore, ShouldNotBeNil)
	})
}
