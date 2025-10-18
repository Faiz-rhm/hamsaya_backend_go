package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSendSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SendSuccess(c, http.StatusOK, "Success", gin.H{"id": 123})

	assert.Equal(t, http.StatusOK, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "Success", response.Message)
}

func TestSendError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Initialize a test logger (ensure it's initialized before use)
	if Logger == nil {
		if err := InitLogger("error"); err != nil {
			t.Fatalf("Failed to initialize logger: %v", err)
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Set request info for logging
	c.Request = httptest.NewRequest("GET", "/test", nil)

	SendError(c, http.StatusBadRequest, "Bad Request", ErrBadRequest)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Equal(t, "Bad Request", response.Message)
}

func TestSendPaginated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := []string{"item1", "item2", "item3"}
	SendPaginated(c, data, 1, 10, 25)

	assert.Equal(t, http.StatusOK, w.Code)

	var response PaginatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Meta.Page)
	assert.Equal(t, 10, response.Meta.Limit)
	assert.Equal(t, 3, response.Meta.TotalPages)
	assert.Equal(t, int64(25), response.Meta.TotalCount)
}
