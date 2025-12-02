package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response represents a generic API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data"`
	Meta    Pagination  `json:"meta"`
}

// Pagination holds pagination metadata
// Matches frontend PaginationMeta structure
type Pagination struct {
	CurrentPage  int                    `json:"currentPage"`
	ItemsPerPage int                    `json:"itemsPerPage"`
	TotalItems   int64                  `json:"totalItems"`
	TotalPages   int                    `json:"totalPages"`
	Filters      map[string]interface{} `json:"filters,omitempty"`
	Sorts        map[string]interface{} `json:"sorts,omitempty"`
}

// SendSuccess sends a successful response
func SendSuccess(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// SendError sends an error response
func SendError(c *gin.Context, statusCode int, message string, err error) {
	response := Response{
		Success: false,
		Message: message,
	}

	if err != nil {
		response.Error = err.Error()
		// Log the error
		GetLogger().Errorw("API Error",
			"status", statusCode,
			"message", message,
			"error", err.Error(),
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
		)
	}

	c.JSON(statusCode, response)
}

// SendAppError sends an application error response
func SendAppError(c *gin.Context, appErr *AppError) {
	SendError(c, appErr.Code, appErr.Message, appErr.Err)
}

// SendPaginated sends a paginated response
// Optional: pass filters and sorts maps if you want to include them in response
func SendPaginated(c *gin.Context, data interface{}, page, limit int, totalCount int64) {
	SendPaginatedWithFilters(c, data, page, limit, totalCount, nil, nil)
}

// SendPaginatedWithFilters sends a paginated response with filters and sorts
func SendPaginatedWithFilters(c *gin.Context, data interface{}, page, limit int, totalCount int64, filters map[string]interface{}, sorts map[string]interface{}) {
	totalPages := int(totalCount) / limit
	if int(totalCount)%limit != 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Success: true,
		Data:    data,
		Meta: Pagination{
			CurrentPage:  page,
			ItemsPerPage: limit,
			TotalItems:   totalCount,
			TotalPages:   totalPages,
			Filters:      filters,
			Sorts:        sorts,
		},
	})
}

// SendCreated sends a 201 Created response
func SendCreated(c *gin.Context, message string, data interface{}) {
	SendSuccess(c, http.StatusCreated, message, data)
}

// SendNoContent sends a 204 No Content response
func SendNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// SendBadRequest sends a 400 Bad Request response
func SendBadRequest(c *gin.Context, message string, err error) {
	SendError(c, http.StatusBadRequest, message, err)
}

// SendUnauthorized sends a 401 Unauthorized response
func SendUnauthorized(c *gin.Context, message string, err error) {
	SendError(c, http.StatusUnauthorized, message, err)
}

// SendForbidden sends a 403 Forbidden response
func SendForbidden(c *gin.Context, message string, err error) {
	SendError(c, http.StatusForbidden, message, err)
}

// SendNotFound sends a 404 Not Found response
func SendNotFound(c *gin.Context, message string, err error) {
	SendError(c, http.StatusNotFound, message, err)
}

// SendConflict sends a 409 Conflict response
func SendConflict(c *gin.Context, message string, err error) {
	SendError(c, http.StatusConflict, message, err)
}

// SendInternalServerError sends a 500 Internal Server Error response
func SendInternalServerError(c *gin.Context, message string, err error) {
	SendError(c, http.StatusInternalServerError, message, err)
}
