package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// operatingMaxRangeSeconds 限制单次查询跨度，避免无界时间范围把聚合查询拖慢。
const operatingMaxRangeSeconds int64 = 366 * 24 * 60 * 60

// parseOperatingRange 校验 [start_timestamp, end_timestamp) 区间。边界由调用方按
// 北京时间切好，服务端只保证区间合法且跨度有界。
func parseOperatingRange(c *gin.Context) (int64, int64, bool) {
	startAt, err := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	if err != nil || startAt <= 0 {
		common.ApiErrorT(c, "Invalid start timestamp")
		return 0, 0, false
	}
	endAt, err := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if err != nil || endAt <= startAt {
		common.ApiErrorT(c, "Invalid end timestamp")
		return 0, 0, false
	}
	if endAt-startAt > operatingMaxRangeSeconds {
		common.ApiErrorT(c, "Time range is too large")
		return 0, 0, false
	}
	return startAt, endAt, true
}

func GetOperatingSummary(c *gin.Context) {
	startAt, endAt, ok := parseOperatingRange(c)
	if !ok {
		return
	}
	summary, err := model.GetOperatingSummary(startAt, endAt)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetOperatingExpenses(c *gin.Context) {
	startAt, endAt, ok := parseOperatingRange(c)
	if !ok {
		return
	}
	expenses, err := model.GetOperatingExpenses(startAt, endAt)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, expenses)
}

type operatingExpenseRequest struct {
	Category    string `json:"category"`
	AmountCents int64  `json:"amount_cents"`
	OccurredAt  int64  `json:"occurred_at"`
	Note        string `json:"note"`
}

func CreateOperatingExpense(c *gin.Context) {
	var req operatingExpenseRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	expense, err := model.CreateOperatingExpense(model.OperatingExpenseInput{
		Category:    req.Category,
		AmountCents: req.AmountCents,
		OccurredAt:  req.OccurredAt,
		Note:        req.Note,
		OperatorId:  c.GetInt("id"),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, expense)
}

func DeleteOperatingExpense(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if err := model.DeleteOperatingExpense(id); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// GetOperatingFlow 返回期间内的收入、支出与发放流水，按发生时间倒序分页。
func GetOperatingFlow(c *gin.Context) {
	startAt, endAt, ok := parseOperatingRange(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	flow, err := model.GetOperatingFlow(startAt, endAt, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, flow)
}
