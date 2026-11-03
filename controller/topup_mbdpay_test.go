package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMbdpaySignMatchesDocumentedAlgorithm(t *testing.T) {
	params := map[string]string{
		"app_id":       "12345",
		"amount_total": "1",
		"out_trade_no": "123123123123",
		"description":  "",
	}
	// md5("amount_total=1&app_id=12345&out_trade_no=123123123123&key=xxxxxxxxx")
	assert.Equal(t, "0ef829e8cc7548090c1353e6bef70b58", mbdpaySign(params, "xxxxxxxxx"))
}

func TestParseMbdpayAlipayForm(t *testing.T) {
	body := `<form id='alipaysubmit' name='alipaysubmit' action='https://openapi.alipay.com/gateway.do?charset=UTF-8&amp;version=1.0' method='POST'>` +
		`<input type='hidden' name='method' value='alipay.trade.wap.pay'/>` +
		`<input type='hidden' name='biz_content' value='{"out_trade_no":"MBD1NO123"}'/>` +
		`<input type='submit' value='ok' style='display:none;'></form>` +
		`<script>document.forms['alipaysubmit'].submit();</script>`

	action, params, err := parseMbdpayAlipayForm(body)
	require.NoError(t, err)
	assert.Equal(t, "https://openapi.alipay.com/gateway.do?charset=UTF-8&version=1.0", action)
	assert.Equal(t, "alipay.trade.wap.pay", params["method"])
	assert.Equal(t, `{"out_trade_no":"MBD1NO123"}`, params["biz_content"])
}

func TestParseMbdpayAlipayFormRejectsInvalidBody(t *testing.T) {
	_, _, err := parseMbdpayAlipayForm("<html><body>no form</body></html>")
	require.Error(t, err)
}

func TestMbdpayWebhookRejectsCallbacksWithoutPaidOrder(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalEnabled := setting.MbdpayEnabled
	originalAppId := setting.MbdpayAppId
	originalAppKey := setting.MbdpayAppKey
	originalBaseURL := mbdpayBaseURL
	originalDB := model.DB
	t.Cleanup(func() {
		setting.MbdpayEnabled = originalEnabled
		setting.MbdpayAppId = originalAppId
		setting.MbdpayAppKey = originalAppKey
		mbdpayBaseURL = originalBaseURL
		model.DB = originalDB
	})

	setting.MbdpayEnabled = true
	setting.MbdpayAppId = "app_id"
	setting.MbdpayAppKey = "app_key"

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.TopUp{}))
	model.DB = db
	require.NoError(t, db.Create(&model.TopUp{
		UserId:          1,
		Amount:          1,
		Money:           1.00,
		TradeNo:         "MBD1NO123",
		PaymentMethod:   model.PaymentMethodMbdpayAlipay,
		PaymentProvider: model.PaymentProviderMbdpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}).Error)

	searchResponse := `{"error":"找不到该订单"}`
	searchCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, mbdpaySearchPath, r.URL.Path)
		searchCalls++
		_, _ = w.Write([]byte(searchResponse))
	}))
	t.Cleanup(server.Close)
	mbdpayBaseURL = server.URL

	router := gin.New()
	router.POST("/api/mbdpay/webhook", MbdpayWebhook)

	call := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/mbdpay/webhook", strings.NewReader(body))
		require.NotPanics(t, func() { router.ServeHTTP(recorder, request) })
		return recorder
	}

	webhook := func(tradeNo string, amount int) string {
		return `{"type":"charge_succeeded","data":{"out_trade_no":"` + tradeNo + `","amount":` + strconv.Itoa(amount) + `,"payway":2}}`
	}

	// Unknown order numbers never reach the outbound verification request.
	assert.Equal(t, http.StatusOK, call(webhook("MBDUNKNOWN", 100)).Code)
	assert.Equal(t, 0, searchCalls)

	searchResponse = `{"error":"找不到该订单"}`
	assert.Equal(t, http.StatusInternalServerError, call(webhook("MBD1NO123", 100)).Code)

	searchResponse = `{"state":"0","amount":"100"}`
	assert.Equal(t, http.StatusInternalServerError, call(webhook("MBD1NO123", 100)).Code)

	searchResponse = `{"state":"1","amount":"50"}`
	assert.Equal(t, http.StatusBadRequest, call(webhook("MBD1NO123", 100)).Code)

	searchResponse = `{"state":"1","amount":"100"}`
	assert.Equal(t, http.StatusBadRequest, call(webhook("MBD1NO123", 999)).Code)

	setting.MbdpayEnabled = false
	assert.Equal(t, http.StatusForbidden, call(webhook("MBD1NO123", 100)).Code)
}
