package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// BanMiddleware enforces IP and device bans on every request.
type BanMiddleware struct {
	adminRepo repositories.AdminRepository
	logger    *zap.Logger
}

// NewBanMiddleware creates a new BanMiddleware.
func NewBanMiddleware(adminRepo repositories.AdminRepository, logger *zap.Logger) *BanMiddleware {
	return &BanMiddleware{adminRepo: adminRepo, logger: logger}
}

// Enforce returns a Gin middleware that rejects banned IPs and device IDs.
// Device ID is read from the X-Device-ID request header.
func (m *BanMiddleware) Enforce() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ip := realIP(c)

		if ip != "" {
			banned, err := m.adminRepo.IsIPBanned(ctx, ip)
			if err != nil {
				m.logger.Warn("Ban check failed for IP", zap.String("ip", ip), zap.Error(err))
			} else if banned {
				m.logger.Info("Blocked banned IP", zap.String("ip", ip))
				utils.SendError(c, http.StatusForbidden, "Access denied", nil)
				c.Abort()
				return
			}
		}

		deviceID := strings.TrimSpace(c.GetHeader("X-Device-ID"))
		if deviceID != "" {
			banned, err := m.adminRepo.IsDeviceBanned(ctx, deviceID)
			if err != nil {
				m.logger.Warn("Ban check failed for device", zap.String("device_id", deviceID), zap.Error(err))
			} else if banned {
				m.logger.Info("Blocked banned device", zap.String("device_id", deviceID))
				utils.SendError(c, http.StatusForbidden, "Access denied", nil)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// realIP returns the client IP via gin's ClientIP, which honors
// X-Forwarded-For ONLY from the trusted-proxy set configured at boot
// (router.SetTrustedProxies). Previously this blindly took the first XFF
// entry, letting any client spoof their IP to dodge IP bans or frame another
// address — fixed by deferring to gin's trusted-proxy-aware resolution.
func realIP(c *gin.Context) string {
	return c.ClientIP()
}
