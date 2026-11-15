package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTopUpQuotaValidation(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	oldPrice := operation_setting.Price
	common.QuotaPerUnit = 500000
	operation_setting.Price = 1
	t.Cleanup(func() {
		common.QuotaPerUnit = oldQuotaPerUnit
		operation_setting.Price = oldPrice
	})

	testCases := []struct {
		name      string
		amount    string
		wantQuota int
		wantErr   bool
	}{
		{
			name:      "currency amount below limit",
			amount:    "4294",
			wantQuota: 2_147_000_000,
		},
		{
			name:      "currency amount above limit",
			amount:    "4295",
			wantQuota: 2_147_500_000,
		},
		{
			name:      "fractional payment amount",
			amount:    "9.9",
			wantQuota: 4_950_000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			amount, err := decimal.NewFromString(tc.amount)
			require.NoError(t, err)
			quota, err := getTopUpQuota(amount)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantQuota, quota)
		})
	}
}

func TestTopUpQuotaConvertsPaymentAmountByPrice(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	oldPrice := operation_setting.Price
	oldTopupGroupRatio := common.TopupGroupRatio2JSONString()
	oldOptions := operation_setting.GetPaymentSetting().AmountOptions
	oldDiscounts := operation_setting.GetPaymentSetting().AmountDiscount
	common.QuotaPerUnit = 500000
	operation_setting.Price = 7
	operation_setting.GetPaymentSetting().AmountOptions = []float64{9.9, 19.9}
	operation_setting.GetPaymentSetting().AmountDiscount = map[string]float64{"9.9": 0.9}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1}`))
	t.Cleanup(func() {
		common.QuotaPerUnit = oldQuotaPerUnit
		operation_setting.Price = oldPrice
		operation_setting.GetPaymentSetting().AmountOptions = oldOptions
		operation_setting.GetPaymentSetting().AmountDiscount = oldDiscounts
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(oldTopupGroupRatio))
	})

	amount := decimal.RequireFromString("9.9")
	quota, err := getTopUpQuota(amount)
	require.NoError(t, err)
	assert.Equal(t, 707143, quota)
	assert.True(t, decimal.RequireFromString("9.9").Div(decimal.NewFromInt(7)).Equal(getCreditAmount(amount)))
	assert.Equal(t, 8.91, getPayMoney(amount, "default"))
	assert.Equal(t, 19.9, getPayMoney(decimal.RequireFromString("19.9"), "default"))
}

func TestValidateTopUpQuotaReturnsMaximumAmount(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	oldPrice := operation_setting.Price
	common.QuotaPerUnit = 500000
	operation_setting.Price = 1
	t.Cleanup(func() {
		common.QuotaPerUnit = oldQuotaPerUnit
		operation_setting.Price = oldPrice
	})

	maxAmount := decimal.NewFromInt(common.MaxWalletQuota).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Floor()

	_, err := validateTopUpQuota(maxAmount)
	require.NoError(t, err)
	_, err = validateTopUpQuota(maxAmount.Add(decimal.NewFromInt(1)))
	require.EqualError(t, err, fmt.Sprintf("Top-up amount cannot exceed %s", maxAmount.String()))
}

func TestRequestAmountRejectsTopUpThatCannotBeSettled(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	oldPrice := operation_setting.Price
	common.QuotaPerUnit = 500000
	operation_setting.Price = 1
	t.Cleanup(func() {
		common.QuotaPerUnit = oldQuotaPerUnit
		operation_setting.Price = oldPrice
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	maxAmount := decimal.NewFromInt(common.MaxWalletQuota).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Floor()
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/amount",
		strings.NewReader(fmt.Sprintf(`{"amount":%s}`, maxAmount.Add(decimal.NewFromInt(1)).String())),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestAmount(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	maxAmountText := maxAmount.String()
	assert.JSONEq(t, fmt.Sprintf(`{"message":"error","data":"Top-up amount cannot exceed %s","message_key":"Top-up amount cannot exceed {{max}}","message_params":{"max":"%s"}}`, maxAmountText, maxAmountText), recorder.Body.String())
}

func TestEpayAmountRequestBindsFractionalAmount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/pay",
		strings.NewReader(`{"amount":9.9,"payment_method":"alipay"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	var req EpayRequest
	require.NoError(t, ctx.ShouldBindJSON(&req))
	assert.True(t, decimal.RequireFromString("9.9").Equal(req.Amount))
	assert.Equal(t, "alipay", req.PaymentMethod)
}

func TestRequestAmountRejectsTopUpThatWouldOverflowWallet(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	oldPrice := operation_setting.Price
	oldDB := model.DB
	common.QuotaPerUnit = 500000
	operation_setting.Price = 1

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	t.Cleanup(func() {
		common.QuotaPerUnit = oldQuotaPerUnit
		operation_setting.Price = oldPrice
		model.DB = oldDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	require.NoError(t, model.DB.Create(&model.User{
		Id:       42,
		Username: "topup_capacity_user",
		Quota:    common.MaxWalletQuota - 100_000,
		Status:   common.UserStatusEnabled,
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 42)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/amount",
		strings.NewReader(`{"amount":1}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	RequestAmount(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"message":"error","data":"Top-up quota limit exceeded","message_key":"Top-up quota limit exceeded"}`, recorder.Body.String())
}

func TestValidateCreditedQuotaRejectsOverflow(t *testing.T) {
	_, err := validateCreditedQuota(decimal.NewFromInt(int64(common.MaxWalletQuota / 2)))
	require.NoError(t, err)
	_, err = validateCreditedQuota(decimal.Zero)
	require.EqualError(t, err, "Top-up quota must be greater than 0")
	_, err = validateCreditedQuota(decimal.NewFromInt(common.MaxWalletQuota + 1))
	require.EqualError(
		t,
		err,
		"Top-up quota exceeds the range the system can represent",
	)
}

func TestStripeCreditedQuotaIncludesGroupRatio(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	oldTopupGroupRatio := common.TopupGroupRatio2JSONString()
	common.QuotaPerUnit = 500000
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"vip":2}`))
	t.Cleanup(func() {
		common.QuotaPerUnit = oldQuotaPerUnit
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(oldTopupGroupRatio))
	})

	_, err := validateCreditedQuota(getStripeCreditedQuota(2147, "vip"))
	require.NoError(t, err)
	_, err = validateCreditedQuota(getStripeCreditedQuota(2148, "vip"))
	require.NoError(t, err)
	_, err = validateCreditedQuota(getStripeCreditedQuota(int64(common.MaxWalletQuota), "vip"))
	require.Error(t, err)

	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"free":0}`))
	assert.True(t, decimal.NewFromInt(500000).Equal(getStripeCreditedQuota(1, "free")))
}
