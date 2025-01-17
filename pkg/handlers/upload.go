package handlers

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kots/pkg/kotsutil"
	"github.com/replicatedhq/kots/pkg/logger"
	"github.com/replicatedhq/kots/pkg/preflight"
	"github.com/replicatedhq/kots/pkg/render"
	"github.com/replicatedhq/kots/pkg/store"
	"github.com/replicatedhq/kots/pkg/version"
)

type UploadExistingAppRequest struct {
	Slug           string `json:"slug"`
	VersionLabel   string `json:"versionLabel,omitempty"`
	UpdateCursor   string `json:"updateCursor,omitempty"`
	Deploy         bool   `json:"deploy"`
	SkipPreflights bool   `json:"skipPreflights"`
}

type UploadResponse struct {
	Slug string `json:"slug"`
}

// UploadExistingApp can be used to upload a multipart form file to the existing app
// This is used in the KOTS CLI when calling kots upload ...
// NOTE: this uses special kots token authorization
func (h *Handler) UploadExistingApp(w http.ResponseWriter, r *http.Request) {
	if err := requireValidKOTSToken(w, r); err != nil {
		logger.Error(err)
		return
	}

	metadata := r.FormValue("metadata")
	uploadExistingAppRequest := UploadExistingAppRequest{}
	if err := json.NewDecoder(strings.NewReader(metadata)).Decode(&uploadExistingAppRequest); err != nil {
		logger.Error(err)
		w.WriteHeader(400)
		return
	}

	archive, _, err := r.FormFile("file")
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	tmpFile, err := ioutil.TempFile("", "kotsadm")
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}
	_, err = io.Copy(tmpFile, archive)
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}
	defer os.RemoveAll(tmpFile.Name())

	archiveDir, err := version.ExtractArchiveToTempDirectory(tmpFile.Name())
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}
	defer os.RemoveAll(archiveDir)

	// encrypt any plain text values
	kotsKinds, err := kotsutil.LoadKotsKindsFromPath(archiveDir)
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	if kotsKinds.ConfigValues != nil {
		if err := kotsKinds.EncryptConfigValues(); err != nil {
			logger.Error(err)
			w.WriteHeader(500)
			return
		}
		updated, err := kotsKinds.Marshal("kots.io", "v1beta1", "ConfigValues")
		if err != nil {
			logger.Error(err)
			w.WriteHeader(500)
			return
		}

		if err := ioutil.WriteFile(filepath.Join(archiveDir, "upstream", "userdata", "config.yaml"), []byte(updated), 0644); err != nil {
			logger.Error(err)
			w.WriteHeader(500)
			return
		}
	}

	a, err := store.GetStore().GetAppFromSlug(uploadExistingAppRequest.Slug)
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	registrySettings, err := store.GetStore().GetRegistryDetailsForApp(a.ID)
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}
	app, err := store.GetStore().GetApp(a.ID)
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}
	downstreams, err := store.GetStore().ListDownstreamsForApp(a.ID)
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	err = render.RenderDir(archiveDir, app, downstreams, registrySettings)
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	newSequence, err := store.GetStore().CreateAppVersion(a.ID, &a.CurrentSequence, archiveDir, "KOTS Upload", false, &version.DownstreamGitOps{})
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	if !uploadExistingAppRequest.SkipPreflights {
		if err := preflight.Run(a.ID, a.Slug, newSequence, a.IsAirgap, archiveDir); err != nil {
			logger.Error(err)
			w.WriteHeader(500)
			return
		}
	}

	if uploadExistingAppRequest.Deploy {
		if err := version.DeployVersion(a.ID, newSequence); err != nil {
			logger.Error(errors.Wrap(err, "failed to deploy latest version"))
			w.WriteHeader(500)
			return
		}
	}

	uploadResponse := UploadResponse{
		Slug: a.Slug,
	}

	JSON(w, 200, uploadResponse)
}
