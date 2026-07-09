package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// maxVerificationDocuments caps documents per verification request.
const maxVerificationDocuments = 5

// BusinessVerificationHandler exposes the owner submit/status endpoints and
// the admin review queue.
type BusinessVerificationHandler struct {
	verificationService *services.BusinessVerificationService
	storageService      *services.StorageService
	validator           *utils.Validator
	logger              *zap.Logger
}

// NewBusinessVerificationHandler constructs the handler.
func NewBusinessVerificationHandler(
	verificationService *services.BusinessVerificationService,
	storageService *services.StorageService,
	validator *utils.Validator,
	logger *zap.Logger,
) *BusinessVerificationHandler {
	return &BusinessVerificationHandler{
		verificationService: verificationService,
		storageService:      storageService,
		validator:           validator,
		logger:              logger,
	}
}

// SubmitVerification godoc
// @Summary Submit business verification request (owner only)
// @Description Multipart: 1-5 document images (field "documents") + optional license_no and note fields
// @Tags businesses
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Param documents formData file true "Document images (repeatable, max 5)"
// @Param license_no formData string false "Business license number"
// @Param note formData string false "Note for the reviewer"
// @Success 201 {object} utils.Response{data=models.BusinessVerificationRequest}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id}/verification [post]
func (h *BusinessVerificationHandler) SubmitVerification(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}
	businessID := c.Param("business_id")

	form, err := c.MultipartForm()
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid multipart form", err)
		return
	}
	files := form.File["documents"]
	if len(files) == 0 {
		utils.SendError(c, http.StatusBadRequest, "At least one document image is required", nil)
		return
	}
	if len(files) > maxVerificationDocuments {
		utils.SendError(c, http.StatusBadRequest, "Too many documents (max 5)", nil)
		return
	}

	documents := make([]models.Photo, 0, len(files))
	for _, header := range files {
		if !utils.EnforceUploadSize(c, header.Size, utils.MaxImageUploadBytes) {
			return
		}
		file, err := header.Open()
		if err != nil {
			utils.SendError(c, http.StatusBadRequest, "Failed to read document", err)
			return
		}
		photo, err := h.storageService.UploadImage(c.Request.Context(), file, header, services.ImageTypePost)
		_ = file.Close()
		if err != nil {
			h.handleError(c, err)
			return
		}
		documents = append(documents, *photo)
	}

	var licenseNo, note *string
	if v := c.PostForm("license_no"); v != "" {
		licenseNo = &v
	}
	if v := c.PostForm("note"); v != "" {
		note = &v
	}

	req, err := h.verificationService.Submit(c.Request.Context(), businessID, userID.(string), licenseNo, note, documents)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Verification request submitted", req)
}

// GetVerificationStatus godoc
// @Summary Get latest verification request for a business (owner only)
// @Tags businesses
// @Produce json
// @Security BearerAuth
// @Param business_id path string true "Business ID"
// @Success 200 {object} utils.Response{data=models.BusinessVerificationRequest}
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /businesses/{business_id}/verification [get]
func (h *BusinessVerificationHandler) GetVerificationStatus(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	req, err := h.verificationService.Status(c.Request.Context(), c.Param("business_id"), userID.(string))
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Verification status retrieved", req)
}

// ListVerifications godoc
// @Summary List business verification requests (admin)
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Filter: PENDING | APPROVED | REJECTED"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} utils.Response{data=[]models.BusinessVerificationListItem}
// @Failure 401 {object} utils.Response
// @Router /admin/business-verifications [get]
func (h *BusinessVerificationHandler) ListVerifications(c *gin.Context) {
	limit := 20
	offset := 0
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}
	var status *string
	if v := c.Query("status"); v == models.VerificationStatusPending ||
		v == models.VerificationStatusApproved || v == models.VerificationStatusRejected {
		status = &v
	}

	items, total, err := h.verificationService.List(c.Request.Context(), status, limit, offset)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Verification requests retrieved", map[string]interface{}{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// ReviewVerification godoc
// @Summary Approve or reject a verification request (admin)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request_id path string true "Request ID"
// @Param request body models.ReviewBusinessVerificationRequest true "action: approve | reject (+ reason)"
// @Success 200 {object} utils.Response{data=models.BusinessVerificationRequest}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /admin/business-verifications/{request_id} [patch]
func (h *BusinessVerificationHandler) ReviewVerification(c *gin.Context) {
	adminID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	var req models.ReviewBusinessVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	result, err := h.verificationService.Review(
		c.Request.Context(), c.Param("request_id"), adminID.(string), req.Action, req.Reason,
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Verification request reviewed", result)
}

func (h *BusinessVerificationHandler) handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}
	h.logger.Error("Unhandled error in business verification handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}
