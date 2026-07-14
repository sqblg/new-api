package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type refundRequestBody struct {
	Amount         int64  `json:"amount"`
	TopUpId        int    `json:"topup_id"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

func RequestRefund(c *gin.Context) {
	var body refundRequestBody
	if err := common.DecodeJson(c.Request.Body, &body); err != nil || body.Amount <= 0 || body.TopUpId <= 0 || strings.TrimSpace(body.IdempotencyKey) == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	userId := c.GetInt("id")
	if existing := model.GetRefundRequestByIdempotency(userId, strings.TrimSpace(body.IdempotencyKey)); existing != nil {
		if existing.Amount != body.Amount || existing.TopUpId != body.TopUpId {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "幂等键已用于其他退款参数"})
			return
		}
		common.ApiSuccess(c, existing)
		return
	}
	request := &model.RefundRequest{UserId: userId, TopUpId: body.TopUpId, Amount: body.Amount, Reason: strings.TrimSpace(body.Reason), IdempotencyKey: strings.TrimSpace(body.IdempotencyKey), Status: model.RefundStatusPending}
	if err := request.Insert(); err != nil {
		if existing := model.GetRefundRequestByIdempotency(userId, strings.TrimSpace(body.IdempotencyKey)); existing != nil {
			common.ApiSuccess(c, existing)
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, request)
}

func GetRefundRequests(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("size"))
	requests, total, err := model.ListRefundRequests(offset, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"items": requests, "total": total, "start": offset, "size": limit}})
}

func ApproveRefund(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	request := model.GetRefundRequest(id)
	if request == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "退款申请不存在"})
		return
	}
	if err := request.Approve(c.GetInt("id")); err != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
		return
	}
	common.ApiSuccess(c, model.GetRefundRequest(id))
}
