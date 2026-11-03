package controller

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"golang.org/x/net/html"
)

const (
	mbdpayAlipayPath = "/release/alipay/pay"
	mbdpayWxH5Path   = "/release/wx/prepay"
	mbdpaySearchPath = "/release/main/search_order"
)

// var so tests can point the gateway at an httptest server.
var mbdpayBaseURL = "https://newapi.mbd.pub"

func mbdpaySign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for name, value := range params {
		if value != "" {
			keys = append(keys, name)
		}
	}
	slices.Sort(keys)

	var builder strings.Builder
	for i, name := range keys {
		if i > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(name)
		builder.WriteByte('=')
		builder.WriteString(params[name])
	}
	builder.WriteString("&key=")
	builder.WriteString(key)

	sum := md5.Sum([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

func mbdpayPost(ctx context.Context, path string, params map[string]any) (map[string]any, error) {
	payload := make(map[string]any, len(params)+2)
	signParams := make(map[string]string, len(params)+2)
	for name, value := range params {
		payload[name] = value
		if text := mbdpayStringValue(value); text != "" {
			signParams[name] = text
		}
	}
	payload["app_id"] = setting.MbdpayAppId
	signParams["app_id"] = setting.MbdpayAppId
	payload["sign"] = mbdpaySign(signParams, setting.MbdpayAppKey)

	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, mbdpayBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	client := service.GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var result map[string]any
	decodeErr := common.Unmarshal(responseBody, &result)
	if response.StatusCode != http.StatusOK {
		if decodeErr == nil {
			if message := mbdpayStringValue(result["error"]); message != "" {
				return nil, fmt.Errorf("http status %d: %s", response.StatusCode, message)
			}
		}
		return nil, fmt.Errorf("http status %d: %s", response.StatusCode, string(responseBody))
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("invalid response: %w", decodeErr)
	}
	if message := mbdpayStringValue(result["error"]); message != "" {
		return nil, errors.New(message)
	}
	return result, nil
}

func mbdpayStringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func parseMbdpayAlipayForm(raw string) (string, map[string]string, error) {
	document, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return "", nil, err
	}

	var form *html.Node
	var findForm func(*html.Node)
	findForm = func(node *html.Node) {
		if form != nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == "form" {
			form = node
			return
		}
		for child := node.FirstChild; child != nil && form == nil; child = child.NextSibling {
			findForm(child)
		}
	}
	findForm(document)
	if form == nil {
		return "", nil, errors.New("response does not contain a payment form")
	}

	action := ""
	for _, attribute := range form.Attr {
		if attribute.Key == "action" {
			action = attribute.Val
			break
		}
	}
	if action == "" {
		return "", nil, errors.New("payment form has no action")
	}

	params := make(map[string]string)
	var collectInputs func(*html.Node)
	collectInputs = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "input" {
			name := ""
			value := ""
			for _, attribute := range node.Attr {
				switch attribute.Key {
				case "name":
					name = attribute.Val
				case "value":
					value = attribute.Val
				}
			}
			if name != "" {
				params[name] = value
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			collectInputs(child)
		}
	}
	collectInputs(form)
	if len(params) == 0 {
		return "", nil, errors.New("payment form has no parameters")
	}
	return action, params, nil
}

func mbdpayReturnUrl() (string, error) {
	returnUrl := setting.MbdpayReturnUrl
	if returnUrl == "" {
		returnUrl = paymentReturnPath("/wallet?show_history=true")
	}
	if !strings.HasPrefix(returnUrl, "http://") && !strings.HasPrefix(returnUrl, "https://") {
		return "", errors.New("server address is not configured")
	}
	return returnUrl, nil
}

func mbdpayDescription(amount int64) string {
	systemName := strings.TrimSpace(common.SystemName)
	if systemName == "" {
		systemName = "New API"
	}
	return fmt.Sprintf("%s recharge %d", systemName, amount)
}

// RequestMbdpayPay creates a MbdPay order and returns either a ready-to-submit
// Alipay form or a WeChat H5 payment URL.
func RequestMbdpayPay(c *gin.Context) {
	if !isMbdpayTopUpEnabled() {
		paymentError(c, common.NewMessage("MbdPay payment is not enabled"))
		return
	}

	var req EpayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		paymentError(c, common.NewMessage("Invalid parameters"))
		return
	}
	if req.PaymentMethod != model.PaymentMethodMbdpayAlipay && req.PaymentMethod != model.PaymentMethodMbdpayWxpay {
		paymentError(c, common.NewMessage("Unsupported payment method"))
		return
	}
	minTopup := int64(setting.MbdpayMinTopUp)
	if req.Amount < minTopup {
		paymentError(c, common.NewMessage("Top-up amount cannot be less than {{min}}", map[string]any{"min": minTopup}))
		return
	}
	id := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, id, req.Amount) {
		return
	}

	group, err := model.GetUserGroup(id, true)
	if err != nil {
		paymentError(c, common.NewMessage("Failed to get the user group"))
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	amountTotal := int64(math.Round(payMoney * 100))
	if amountTotal < 1 {
		paymentError(c, common.NewMessage("Payment amount is too low"))
		return
	}

	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(req.Amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}
	tradeNo := fmt.Sprintf("MBD%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   req.PaymentMethod,
		PaymentProvider: model.PaymentProviderMbdpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("Mbdpay failed to create top-up order user_id=%d trade_no=%s payment_method=%s amount=%d error=%q", id, tradeNo, req.PaymentMethod, req.Amount, err.Error()))
		paymentError(c, common.NewMessage("Failed to create order"))
		return
	}

	commonParams := map[string]any{
		"description":  mbdpayDescription(req.Amount),
		"amount_total": amountTotal,
		"out_trade_no": tradeNo,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	switch req.PaymentMethod {
	case model.PaymentMethodMbdpayAlipay:
		returnUrl, err := mbdpayReturnUrl()
		if err != nil {
			logger.LogError(c.Request.Context(), common.LogText("Mbdpay return URL is not configured user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
			topUp.Status = common.TopUpStatusFailed
			_ = topUp.Update()
			paymentError(c, common.NewMessage("Payment configuration error"))
			return
		}
		params := make(map[string]any, len(commonParams)+2)
		for name, value := range commonParams {
			params[name] = value
		}
		params["url"] = returnUrl
		params["callback_url"] = returnUrl
		result, err := mbdpayPost(ctx, mbdpayAlipayPath, params)
		if err != nil {
			logger.LogError(c.Request.Context(), common.LogText("Mbdpay failed to create Alipay order user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
			topUp.Status = common.TopUpStatusFailed
			_ = topUp.Update()
			paymentError(c, common.NewMessage("Failed to start payment"))
			return
		}
		action, formParams, err := parseMbdpayAlipayForm(mbdpayStringValue(result["body"]))
		if err != nil {
			logger.LogError(c.Request.Context(), common.LogText("Mbdpay Alipay response is invalid user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
			topUp.Status = common.TopUpStatusFailed
			_ = topUp.Update()
			paymentError(c, common.NewMessage("Failed to start payment"))
			return
		}
		logger.LogInfo(c.Request.Context(), common.LogText("Mbdpay Alipay order created user_id=%d trade_no=%s amount=%d money=%.2f", id, tradeNo, req.Amount, payMoney))
		c.JSON(http.StatusOK, gin.H{"message": "success", "data": formParams, "url": action})
	case model.PaymentMethodMbdpayWxpay:
		params := make(map[string]any, len(commonParams)+1)
		for name, value := range commonParams {
			params[name] = value
		}
		params["channel"] = "h5"
		result, err := mbdpayPost(ctx, mbdpayWxH5Path, params)
		if err != nil {
			logger.LogError(c.Request.Context(), common.LogText("Mbdpay failed to create WeChat H5 order user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
			topUp.Status = common.TopUpStatusFailed
			_ = topUp.Update()
			paymentError(c, common.NewMessage("Failed to start payment"))
			return
		}
		h5Url := mbdpayStringValue(result["h5_url"])
		if h5Url == "" {
			logger.LogError(c.Request.Context(), common.LogText("Mbdpay WeChat H5 response has no h5_url user_id=%d trade_no=%s", id, tradeNo))
			topUp.Status = common.TopUpStatusFailed
			_ = topUp.Update()
			paymentError(c, common.NewMessage("Failed to start payment"))
			return
		}
		logger.LogInfo(c.Request.Context(), common.LogText("Mbdpay WeChat H5 order created user_id=%d trade_no=%s amount=%d money=%.2f", id, tradeNo, req.Amount, payMoney))
		c.JSON(http.StatusOK, gin.H{
			"message": "success",
			"data": gin.H{
				"payment_url": h5Url,
				"order_id":    tradeNo,
			},
		})
	}
}

type mbdpayWebhookPayload struct {
	Type string `json:"type"`
	Data struct {
		OutTradeNo string `json:"out_trade_no"`
		Amount     int64  `json:"amount"`
		Payway     int    `json:"payway"`
		ChargeId   string `json:"charge_id"`
	} `json:"data"`
}

// MbdpayWebhook handles MbdPay's server-to-server callback. The callback has no
// signature, so settlement only happens after the order is confirmed as paid
// through the signed search_order API.
func MbdpayWebhook(c *gin.Context) {
	if !isMbdpayWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), common.LogText("Mbdpay webhook rejected reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.String(http.StatusForbidden, "fail")
		return
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("Mbdpay webhook failed to read request body path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.String(http.StatusBadRequest, "fail")
		return
	}
	logger.LogInfo(c.Request.Context(), common.LogText("Mbdpay webhook received request path=%q client_ip=%s body=%q", c.Request.RequestURI, c.ClientIP(), string(body)))

	var payload mbdpayWebhookPayload
	if err := common.Unmarshal(body, &payload); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("Mbdpay webhook failed to parse path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.String(http.StatusBadRequest, "fail")
		return
	}

	if payload.Type == "complaint" {
		logger.LogWarn(c.Request.Context(), common.LogText("Mbdpay order complaint received trade_no=%s client_ip=%s", payload.Data.OutTradeNo, c.ClientIP()))
		c.String(http.StatusOK, "success")
		return
	}
	if payload.Type != "charge_succeeded" {
		logger.LogInfo(c.Request.Context(), common.LogText("Mbdpay webhook ignored event type=%s client_ip=%s", payload.Type, c.ClientIP()))
		c.String(http.StatusOK, "success")
		return
	}

	tradeNo := strings.TrimSpace(payload.Data.OutTradeNo)
	if tradeNo == "" {
		logger.LogWarn(c.Request.Context(), common.LogText("Mbdpay webhook missing out_trade_no client_ip=%s", c.ClientIP()))
		c.String(http.StatusBadRequest, "fail")
		return
	}

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderMbdpay {
		// The callback carries no signature. Unknown order numbers must not
		// trigger an outbound verification request.
		logger.LogWarn(c.Request.Context(), common.LogText("Mbdpay callback order not found trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		c.String(http.StatusOK, "success")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	order, err := mbdpayPost(ctx, mbdpaySearchPath, map[string]any{"out_trade_no": tradeNo})
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("Mbdpay webhook failed to verify order trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		c.String(http.StatusInternalServerError, "fail")
		return
	}
	state := mbdpayStringValue(order["state"])
	if state != "1" && state != "2" {
		logger.LogWarn(c.Request.Context(), common.LogText("Mbdpay webhook order is not paid trade_no=%s state=%s client_ip=%s", tradeNo, state, c.ClientIP()))
		c.String(http.StatusInternalServerError, "fail")
		return
	}
	queryAmount := mbdpayStringValue(order["amount"])
	expectedAmount := strconv.FormatInt(int64(math.Round(topUp.Money*100)), 10)
	if queryAmount == "" || queryAmount != expectedAmount {
		logger.LogError(c.Request.Context(), common.LogText("Mbdpay webhook amount mismatch trade_no=%s query_amount=%s expected_amount=%s client_ip=%s", tradeNo, queryAmount, expectedAmount, c.ClientIP()))
		c.String(http.StatusBadRequest, "fail")
		return
	}
	if payload.Data.Amount > 0 && queryAmount != strconv.FormatInt(payload.Data.Amount, 10) {
		logger.LogError(c.Request.Context(), common.LogText("Mbdpay webhook callback amount mismatch trade_no=%s query_amount=%s callback_amount=%d client_ip=%s", tradeNo, queryAmount, payload.Data.Amount, c.ClientIP()))
		c.String(http.StatusBadRequest, "fail")
		return
	}

	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	alreadyDone, err := model.RechargeMbdpay(tradeNo, c.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, model.ErrTopUpNotFound):
			logger.LogError(c.Request.Context(), common.LogText("Mbdpay callback order not found trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		case errors.Is(err, model.ErrPaymentMethodMismatch):
			logger.LogWarn(c.Request.Context(), common.LogText("Mbdpay order payment gateway mismatch trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		case errors.Is(err, model.ErrTopUpStatusInvalid):
			logger.LogWarn(c.Request.Context(), common.LogText("Mbdpay order status invalid trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		default:
			logger.LogError(c.Request.Context(), common.LogText("Mbdpay top-up processing failed trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		}
		c.String(http.StatusInternalServerError, "fail")
		return
	}
	if alreadyDone {
		logger.LogInfo(c.Request.Context(), common.LogText("Mbdpay duplicate callback ignored trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
	} else {
		logger.LogInfo(c.Request.Context(), common.LogText("Mbdpay top-up succeeded trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
	}
	c.String(http.StatusOK, "success")
}
