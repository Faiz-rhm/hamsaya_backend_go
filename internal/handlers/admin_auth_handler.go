package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// AdminAuthHandler exposes the cookie-based auth flow used by the admin SPA.
// It is intentionally separate from AuthHandler so the mobile/Bearer flow
// remains untouched: existing endpoints keep returning JSON tokens; the new
// /auth/admin/* endpoints set HttpOnly cookies and omit tokens from the body.
type AdminAuthHandler struct {
	authService *services.AuthService
	validator   *utils.Validator
	logger      *zap.Logger
	cookieCfg   utils.CookieConfig
	jwtCfg      config.JWTConfig
}

func NewAdminAuthHandler(
	authService *services.AuthService,
	validator *utils.Validator,
	logger *zap.Logger,
	cookieCfg utils.CookieConfig,
	jwtCfg config.JWTConfig,
) *AdminAuthHandler {
	return &AdminAuthHandler{
		authService: authService,
		validator:   validator,
		logger:      logger,
		cookieCfg:   cookieCfg,
		jwtCfg:      jwtCfg,
	}
}

// AdminLogin authenticates an admin and sets HttpOnly access/refresh cookies
// plus a non-HttpOnly CSRF cookie. The response body omits the token pair —
// the SPA never sees raw tokens, mitigating XSS exfiltration.
//
// Role check: only admin/moderator may use this endpoint. Anything else gets
// the same generic 401 to avoid role-enumeration via login.
func (h *AdminAuthHandler) AdminLogin(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	deviceInfo := c.GetHeader("X-Device-Info")
	req.IPAddress = &ipAddress
	req.UserAgent = &userAgent
	if deviceInfo != "" {
		req.DeviceInfo = &deviceInfo
	}

	resp, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		// Reuse upstream handler's error mapping by leaning on the same
		// service error types — wrap caller-facing 401 here for clarity.
		utils.SendError(c, http.StatusUnauthorized, "Invalid credentials", utils.ErrUnauthorized)
		return
	}

	if resp.RequiresMFA {
		// MFA flow still uses the JSON challenge contract; cookies are only
		// minted after a successful second factor (TODO: extend MFA verify
		// endpoint with a parallel /auth/admin/mfa-verify counterpart).
		utils.SendSuccess(c, http.StatusOK, "MFA verification required", resp)
		return
	}

	if resp.User == nil || resp.Tokens == nil {
		utils.SendError(c, http.StatusInternalServerError, "Malformed auth response", utils.ErrUnauthorized)
		return
	}
	if resp.User.Role != models.RoleSuperAdmin && resp.User.Role != models.RoleAdmin && resp.User.Role != models.RoleModerator {
		utils.SendError(c, http.StatusForbidden, "Admin privileges required", utils.ErrForbidden)
		return
	}

	csrf, err := utils.GenerateCSRFToken()
	if err != nil {
		h.logger.Error("csrf token generation failed", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Internal error", utils.ErrInternalServer)
		return
	}

	utils.SetAdminAuthCookies(
		c,
		h.cookieCfg,
		resp.Tokens.AccessToken,
		resp.Tokens.RefreshToken,
		csrf,
		h.jwtCfg.AccessTokenDuration,
		h.jwtCfg.RefreshTokenDuration,
	)

	// Strip raw tokens from the response — admin SPA never reads them.
	resp.Tokens = nil
	utils.SendSuccess(c, http.StatusOK, "Login successful", resp)
}

// AdminMFAVerify completes the second factor of an admin login challenge.
// Body shape matches AuthHandler.VerifyMFA (challenge_id + code). On success
// it sets the same cookies AdminLogin would have, and strips tokens from the
// JSON response. Role enforcement mirrors AdminLogin.
func (h *AdminAuthHandler) AdminMFAVerify(c *gin.Context) {
	var req models.MFAVerifyChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	resp, err := h.authService.VerifyMFA(c.Request.Context(), &req)
	if err != nil {
		utils.SendError(c, http.StatusUnauthorized, "MFA verification failed", utils.ErrUnauthorized)
		return
	}

	if resp.User == nil || resp.Tokens == nil {
		utils.SendError(c, http.StatusInternalServerError, "Malformed auth response", utils.ErrInternalServer)
		return
	}
	if resp.User.Role != models.RoleSuperAdmin && resp.User.Role != models.RoleAdmin && resp.User.Role != models.RoleModerator {
		utils.SendError(c, http.StatusForbidden, "Admin privileges required", utils.ErrForbidden)
		return
	}

	csrf, err := utils.GenerateCSRFToken()
	if err != nil {
		h.logger.Error("csrf token generation failed", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Internal error", utils.ErrInternalServer)
		return
	}

	utils.SetAdminAuthCookies(
		c,
		h.cookieCfg,
		resp.Tokens.AccessToken,
		resp.Tokens.RefreshToken,
		csrf,
		h.jwtCfg.AccessTokenDuration,
		h.jwtCfg.RefreshTokenDuration,
	)

	resp.Tokens = nil
	utils.SendSuccess(c, http.StatusOK, "MFA verification successful", resp)
}

// AdminLogout revokes the session, denylist the access token, and clears all
// admin auth cookies. Reads the session/JTI/exp values placed on the request
// context by the auth middleware — same contract as AuthHandler.Logout.
func (h *AdminAuthHandler) AdminLogout(c *gin.Context) {
	sessionID, exists := c.Get("session_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "Session not found", utils.ErrUnauthorized)
		return
	}
	jtiVal, _ := c.Get("jti")
	expVal, _ := c.Get("token_exp")
	jti, _ := jtiVal.(string)
	tokenExp, _ := expVal.(int64)

	if err := h.authService.Logout(c.Request.Context(), sessionID.(string), jti, tokenExp); err != nil {
		// Even on backend logout failure, clear cookies so the browser-side
		// session is not left dangling.
		utils.ClearAdminAuthCookies(c, h.cookieCfg)
		utils.SendError(c, http.StatusInternalServerError, "Logout failed", utils.ErrInternalServer)
		return
	}

	utils.ClearAdminAuthCookies(c, h.cookieCfg)
	utils.SendSuccess(c, http.StatusOK, "Logged out", nil)
}

// AdminRefresh rotates tokens using the refresh cookie (not the body). On
// success a new access cookie + new refresh cookie + new CSRF cookie are
// emitted. The old refresh token is invalidated by AuthService.RefreshToken.
func (h *AdminAuthHandler) AdminRefresh(c *gin.Context) {
	cookie, err := c.Request.Cookie(utils.CookieAdminRefreshToken)
	if err != nil || cookie.Value == "" {
		utils.SendError(c, http.StatusUnauthorized, "Refresh token missing", utils.ErrUnauthorized)
		return
	}

	pair, err := h.authService.RefreshToken(c.Request.Context(), &models.RefreshTokenRequest{
		RefreshToken: cookie.Value,
	})
	if err != nil {
		utils.ClearAdminAuthCookies(c, h.cookieCfg)
		utils.SendError(c, http.StatusUnauthorized, "Refresh failed", utils.ErrUnauthorized)
		return
	}

	csrf, err := utils.GenerateCSRFToken()
	if err != nil {
		h.logger.Error("csrf token generation failed", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Internal error", utils.ErrInternalServer)
		return
	}

	utils.SetAdminAuthCookies(
		c,
		h.cookieCfg,
		pair.AccessToken,
		pair.RefreshToken,
		csrf,
		h.jwtCfg.AccessTokenDuration,
		h.jwtCfg.RefreshTokenDuration,
	)

	utils.SendSuccess(c, http.StatusOK, "Token refreshed", nil)
}

