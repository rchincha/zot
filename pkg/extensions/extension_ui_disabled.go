//go:build !search || !ui
// +build !search !ui

package extensions

import (
	"github.com/gorilla/mux"

	"zotregistry.io/zot/pkg/api/config"
	"zotregistry.io/zot/pkg/log"
)

func SetupUIRoutes(conf *config.Config, router *mux.Router,
	log log.Logger,
) {
	log.Warn().Msg("skipping setting up ui routes because given zot binary doesn't include this feature," +
		"please build a binary that does so")
}
