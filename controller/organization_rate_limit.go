package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetRateLimits returns paginated rate limit rules for an org
func GetRateLimits(c *gin.Context) {
	var query dto.RateLimitListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListRateLimitsService(&query, pageInfo)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    pageInfo,
	})
}

// CreateRateLimit creates a new rate limit rule
func CreateRateLimit(c *gin.Context) {
	var req dto.CreateRateLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	resp, err := service.CreateRateLimitService(&req)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

// GetRateLimit returns a single rate limit rule
func GetRateLimit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	resp, err := service.GetRateLimitService(id)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

// UpdateRateLimit updates an existing rate limit rule
func UpdateRateLimit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	var req dto.UpdateRateLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	resp, err := service.UpdateRateLimitService(id, &req)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

// DeleteRateLimit deletes a rate limit rule
func DeleteRateLimit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	if err := service.DeleteRateLimitService(id); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// GetUserRateLimit returns the effective rate limit for a user
func GetUserRateLimit(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	modelName := c.Query("model_name")
	modelId := 0
	if modelIdStr := c.Query("model_id"); modelIdStr != "" {
		if id, err := strconv.Atoi(modelIdStr); err == nil {
			modelId = id
		}
	}

	resp, err := service.GetUserEffectiveRateLimit(userId, modelName, modelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}

// ClearRateLimitCache clears all rate limit cache (for testing)
func ClearRateLimitCache(c *gin.Context) {
	service.ClearAllRateLimitCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
