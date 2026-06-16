package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/utils"
)

// AppVersionHandler serves the in-app update gate. The mobile app calls this on
// launch and compares its own build number against the returned thresholds.
// Backend-driven (not Play/iTunes scraping) so it works in Afghanistan where
// Google is blocked.
type AppVersionHandler struct {
	cfg config.AppVersionConfig
}

// NewAppVersionHandler builds the handler from the loaded config.
func NewAppVersionHandler(cfg config.AppVersionConfig) *AppVersionHandler {
	return &AppVersionHandler{cfg: cfg}
}

// AppVersionResponse is what the mobile gate consumes.
type AppVersionResponse struct {
	Platform    string `json:"platform"`
	MinBuild    int    `json:"min_build"`    // below this → force update (block)
	LatestBuild int    `json:"latest_build"` // below this → soft prompt
	StoreURL    string `json:"store_url"`
}

// GetAppVersion handles GET /api/v1/app/version?platform=ios|android
//
// Public (no auth) — the gate runs before login. Unknown/missing platform
// defaults to android. Returns zeros when unconfigured, which the app treats as
// "no update required" (gate is opt-in via env).
func (h *AppVersionHandler) GetAppVersion(c *gin.Context) {
	platform := c.Query("platform")

	resp := AppVersionResponse{Platform: platform}
	switch platform {
	case "ios":
		resp.MinBuild = h.cfg.MinBuildIOS
		resp.LatestBuild = h.cfg.LatestBuildIOS
		resp.StoreURL = h.cfg.StoreURLIOS
	default:
		resp.Platform = "android"
		resp.MinBuild = h.cfg.MinBuildAndroid
		resp.LatestBuild = h.cfg.LatestBuildAndroid
		resp.StoreURL = h.cfg.StoreURLAndroid
	}

	utils.SendSuccess(c, http.StatusOK, "App version info", resp)
}
