package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm/clause"
)

// Log request view
//
// A relay request can write more than one log row: a consume row on success and
// one error row per failed attempt. The request view collapses those rows into
// one entry keyed by request_id so retries stop looking like unrelated traffic.

type logRequestKey struct {
	RequestId     string `gorm:"column:request_id"`
	LastCreatedAt int64  `gorm:"column:last_created_at"`
}

// GetAllLogRequests returns one log per request_id, newest first. The returned
// log carries the full retry policy of its request and merged usage totals.
func GetAllLogRequests(filter LogFilter, startIdx int, num int) ([]*Log, int64, error) {
	return getLogRequests(filter, 0, startIdx, num)
}

// GetUserLogRequests is GetAllLogRequests restricted to one user's logs.
func GetUserLogRequests(userId int, filter LogFilter, startIdx int, num int) ([]*Log, int64, error) {
	return getLogRequests(filter, userId, startIdx, num)
}

func getLogRequests(filter LogFilter, userId int, startIdx int, num int) ([]*Log, int64, error) {
	if num <= 0 {
		num = 20
	}
	countQuery, err := filter.apply(LOG_DB.Table("logs"))
	if err != nil {
		return nil, 0, err
	}
	if userId > 0 {
		countQuery = countQuery.Where("logs.user_id = ?", userId)
	}
	countQuery = countQuery.Where("logs.request_id <> ''")

	var total int64
	if err := countQuery.Select("COUNT(DISTINCT logs.request_id)").Row().Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Log{}, 0, nil
	}

	keyQuery, err := filter.apply(LOG_DB.Table("logs"))
	if err != nil {
		return nil, 0, err
	}
	if userId > 0 {
		keyQuery = keyQuery.Where("logs.user_id = ?", userId)
	}
	var keys []logRequestKey
	err = keyQuery.
		Select("logs.request_id AS request_id, MAX(logs.created_at) AS last_created_at").
		Where("logs.request_id <> ''").
		Group("logs.request_id").
		Order("MAX(logs.created_at) DESC").
		Limit(num).
		Offset(startIdx).
		Scan(&keys).Error
	if err != nil {
		return nil, 0, err
	}
	if len(keys) == 0 {
		return []*Log{}, total, nil
	}

	requestIds := make([]string, 0, len(keys))
	for _, key := range keys {
		requestIds = append(requestIds, key.RequestId)
	}
	rowQuery, err := filter.apply(LOG_DB.Table("logs"))
	if err != nil {
		return nil, 0, err
	}
	if userId > 0 {
		rowQuery = rowQuery.Where("logs.user_id = ?", userId)
	}
	var rows []*Log
	if err := rowQuery.Where("logs.request_id IN ?", requestIds).Order("logs.created_at ASC, logs.id ASC").Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	byRequest := make(map[string][]*Log, len(keys))
	for _, row := range rows {
		byRequest[row.RequestId] = append(byRequest[row.RequestId], row)
	}
	merged := make([]*Log, 0, len(keys))
	for _, key := range keys {
		requestRows := byRequest[key.RequestId]
		if len(requestRows) == 0 {
			continue
		}
		merged = append(merged, mergeLogRequestRows(key.RequestId, requestRows))
	}
	if err := attachLogChannelNames(merged); err != nil {
		return merged, total, err
	}
	return merged, total, nil
}

// mergeLogRequestRows builds the request-view log. It keeps the row with the
// most complete retry policy as the display carrier, then merges billing and
// usage totals from every row of the request.
func mergeLogRequestRows(requestId string, rows []*Log) *Log {
	policies := make([]logRequestPolicy, len(rows))
	carrierIndex := 0
	carrierPolicy := 0
	for i, row := range rows {
		policies[i] = parseLogRequestPolicy(row.Other)
		policyLen := len(policies[i].AdminInfo.RequestPolicy)
		if policyLen > carrierPolicy ||
			(policyLen == carrierPolicy && row.Type == LogTypeConsume && rows[carrierIndex].Type != LogTypeConsume) {
			carrierIndex = i
			carrierPolicy = policyLen
		}
	}
	carrier := rows[carrierIndex]

	merged := *carrier
	merged.RequestId = requestId
	merged.Id = 0
	merged.CreatedAt = 0
	merged.Quota = 0
	merged.PromptTokens = 0
	merged.CompletionTokens = 0
	merged.UseTime = 0
	merged.IsStream = false
	hasConsume := false
	hasError := false
	errorRows := 0
	for _, row := range rows {
		merged.Id = max(merged.Id, row.Id)
		merged.CreatedAt = max(merged.CreatedAt, row.CreatedAt)
		merged.Quota += row.Quota
		merged.PromptTokens += row.PromptTokens
		merged.CompletionTokens += row.CompletionTokens
		merged.UseTime = max(merged.UseTime, row.UseTime)
		merged.IsStream = merged.IsStream || row.IsStream
		switch row.Type {
		case LogTypeConsume:
			hasConsume = true
		case LogTypeError:
			hasError = true
			errorRows++
		}
	}
	if hasConsume {
		merged.Type = LogTypeConsume
	} else if hasError {
		merged.Type = LogTypeError
	}

	channels := logRequestChannelIDs(policies[carrierIndex])
	if len(channels) > 0 {
		merged.ChannelId = channels[len(channels)-1]
	}
	attempts := logRequestAttempts(policies[carrierIndex])
	if len(rows) > 1 || attempts > 1 {
		merged.Other = withMergedRequestInfo(merged.Other, len(rows), errorRows, attempts)
	}
	return &merged
}

func withMergedRequestInfo(other string, logCount int, errorRows int, attempts int) string {
	if other == "" {
		other = "{}"
	}
	var values map[string]any
	if err := common.UnmarshalJsonStr(other, &values); err != nil {
		return other
	}
	values["request_log_count"] = logCount
	values["request_error_count"] = errorRows
	values["request_attempts"] = attempts
	adminInfo, _ := values[logOtherAdminInfoKey].(map[string]any)
	if adminInfo == nil {
		adminInfo = make(map[string]any)
	}
	adminInfo["request_merged"] = true
	adminInfo["request_log_count"] = logCount
	adminInfo["request_error_count"] = errorRows
	adminInfo["request_attempts"] = attempts
	values[logOtherAdminInfoKey] = adminInfo
	encoded, err := common.Marshal(values)
	if err != nil {
		return other
	}
	return string(encoded)
}

type logRequestPolicy struct {
	StatusCode           int    `json:"status_code"`
	BillingSource        string `json:"billing_source"`
	SubscriptionConsumed *int   `json:"subscription_consumed"`
	AdminInfo            struct {
		RequestPolicy []struct {
			Attempt int `json:"attempt"`
		} `json:"request_policy"`
		UseChannel      []any `json:"use_channel"`
		RequestLogCount int   `json:"request_log_count"`
	} `json:"admin_info"`
}

func parseLogRequestPolicy(other string) logRequestPolicy {
	var policy logRequestPolicy
	if other == "" {
		return policy
	}
	_ = common.UnmarshalJsonStr(other, &policy)
	return policy
}

func logRequestChannelIDs(policy logRequestPolicy) []int {
	channels := make([]int, 0, len(policy.AdminInfo.UseChannel))
	for _, raw := range policy.AdminInfo.UseChannel {
		switch value := raw.(type) {
		case float64:
			channels = append(channels, int(value))
		case string:
			if channel, err := strconv.Atoi(value); err == nil {
				channels = append(channels, channel)
			}
		}
	}
	return channels
}

func logRequestAttempts(policy logRequestPolicy) int {
	attempts := 0
	for _, event := range policy.AdminInfo.RequestPolicy {
		attempts = max(attempts, event.Attempt)
	}
	attempts = max(attempts, len(logRequestChannelIDs(policy)))
	if attempts == 0 {
		attempts = 1
	}
	return attempts
}

func logRequestLogCount(policy logRequestPolicy) int {
	return max(1, policy.AdminInfo.RequestLogCount)
}

const (
	logExpandRowLimit     = 500
	logExpandRequestLimit = 200
)

// GetLogsByRequestIds returns raw log rows for the requested request ids.
// A non-zero userId restricts the result to that owner. The boolean reports
// whether the row limit truncated the result.
func GetLogsByRequestIds(requestIds []string, userId int) ([]*Log, bool, error) {
	ids := make([]string, 0, min(len(requestIds), logExpandRequestLimit))
	seen := make(map[string]struct{}, len(requestIds))
	for _, requestId := range requestIds {
		requestId = strings.TrimSpace(requestId)
		if requestId == "" || strings.HasPrefix(requestId, "legacy:") {
			continue
		}
		if _, ok := seen[requestId]; ok {
			continue
		}
		seen[requestId] = struct{}{}
		ids = append(ids, requestId)
		if len(ids) >= logExpandRequestLimit {
			break
		}
	}
	if len(ids) == 0 {
		return []*Log{}, false, nil
	}

	tx := LOG_DB.Table("logs").Where("logs.request_id IN ?", ids)
	if userId > 0 {
		tx = tx.Where("logs.user_id = ?", userId)
	}
	var rows []*Log
	if err := tx.Order("logs.created_at ASC, logs.id ASC").Limit(logExpandRowLimit + 1).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	truncated := len(rows) > logExpandRowLimit
	if truncated {
		rows = rows[:logExpandRowLimit]
	}
	if err := attachLogChannelNames(rows); err != nil {
		return nil, false, err
	}
	return rows, truncated, nil
}

// Session view
//
// A session collapses consecutive requests from the same user, token and model
// into one row when they fall into the same five-minute bucket. It is an
// opt-in browsing view: request view remains the precise, per-request source.

const (
	logSessionWindowSeconds = 300
	logSessionRequestLimit  = 100
)

type logSessionKey struct {
	UserId        int    `gorm:"column:user_id"`
	TokenId       int    `gorm:"column:token_id"`
	ModelName     string `gorm:"column:model_name"`
	Bucket        int64  `gorm:"column:session_bucket"`
	LastCreatedAt int64  `gorm:"column:last_created_at"`
}

// GetAllLogSessions returns one log per consecutive user/token/model session.
func GetAllLogSessions(filter LogFilter, startIdx int, num int) ([]*Log, int64, error) {
	return getLogSessions(filter, 0, startIdx, num)
}

// GetUserLogSessions is GetAllLogSessions restricted to one user's logs.
func GetUserLogSessions(userId int, filter LogFilter, startIdx int, num int) ([]*Log, int64, error) {
	return getLogSessions(filter, userId, startIdx, num)
}

func getLogSessions(filter LogFilter, userId int, startIdx int, num int) ([]*Log, int64, error) {
	if num <= 0 {
		num = 20
	}
	bucketExpr := logSessionBucketExpr()
	groupColumns := []clause.Column{
		{Name: "logs.user_id"},
		{Name: "logs.token_id"},
		{Name: "logs.model_name"},
		{Name: bucketExpr, Raw: true},
	}

	countQuery, err := filter.apply(LOG_DB.Table("logs"))
	if err != nil {
		return nil, 0, err
	}
	if userId > 0 {
		countQuery = countQuery.Where("logs.user_id = ?", userId)
	}
	countQuery = countQuery.Select("1").Clauses(clause.GroupBy{Columns: groupColumns})
	var total int64
	if err := LOG_DB.Table("(?) AS session_groups", countQuery).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Log{}, 0, nil
	}

	keyQuery, err := filter.apply(LOG_DB.Table("logs"))
	if err != nil {
		return nil, 0, err
	}
	if userId > 0 {
		keyQuery = keyQuery.Where("logs.user_id = ?", userId)
	}
	var keys []logSessionKey
	selectColumns := "logs.user_id AS user_id, logs.token_id AS token_id, logs.model_name AS model_name, " +
		bucketExpr + " AS session_bucket, MAX(logs.created_at) AS last_created_at"
	err = keyQuery.
		Select(selectColumns).
		Clauses(clause.GroupBy{Columns: groupColumns}).
		Order("MAX(logs.created_at) DESC").
		Limit(num).
		Offset(startIdx).
		Scan(&keys).Error
	if err != nil {
		return nil, 0, err
	}
	if len(keys) == 0 {
		return []*Log{}, total, nil
	}

	conditions := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)*4)
	for _, key := range keys {
		conditions = append(conditions, "(logs.user_id = ? AND logs.token_id = ? AND logs.model_name = ? AND "+bucketExpr+" = ?)")
		args = append(args, key.UserId, key.TokenId, key.ModelName, key.Bucket)
	}
	rowQuery, err := filter.apply(LOG_DB.Table("logs"))
	if err != nil {
		return nil, 0, err
	}
	if userId > 0 {
		rowQuery = rowQuery.Where("logs.user_id = ?", userId)
	}
	var rows []*Log
	if err := rowQuery.Where(strings.Join(conditions, " OR "), args...).Order("logs.created_at ASC, logs.id ASC").Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	bySession := make(map[string][]*Log, len(keys))
	for _, row := range rows {
		bySession[logSessionRowKey(row)] = append(bySession[logSessionRowKey(row)], row)
	}
	sessions := make([]*Log, 0, len(keys))
	for _, key := range keys {
		sessionKey := fmt.Sprintf("%d|%d|%s|%d", key.UserId, key.TokenId, key.ModelName, key.Bucket)
		sessionRows := bySession[sessionKey]
		if len(sessionRows) == 0 {
			continue
		}
		requests := make([]*Log, 0, len(sessionRows))
		byRequest := make(map[string][]*Log)
		for _, row := range sessionRows {
			requestId := row.RequestId
			if requestId == "" {
				requestId = fmt.Sprintf("legacy:%d", row.Id)
			}
			byRequest[requestId] = append(byRequest[requestId], row)
		}
		for requestId, requestRows := range byRequest {
			requests = append(requests, mergeLogRequestRows(requestId, requestRows))
		}
		sort.Slice(requests, func(i, j int) bool {
			if requests[i].CreatedAt == requests[j].CreatedAt {
				return requests[i].Id < requests[j].Id
			}
			return requests[i].CreatedAt < requests[j].CreatedAt
		})
		sessions = append(sessions, mergeLogSessionRows(requests))
	}
	if err := attachLogChannelNames(sessions); err != nil {
		return sessions, total, err
	}
	return sessions, total, nil
}

func logSessionBucketExpr() string {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return fmt.Sprintf("intDiv(logs.created_at, %d)", logSessionWindowSeconds)
	}
	return fmt.Sprintf("FLOOR(logs.created_at / %d)", logSessionWindowSeconds)
}

func logSessionRowKey(row *Log) string {
	return fmt.Sprintf("%d|%d|%s|%d", row.UserId, row.TokenId, row.ModelName, row.CreatedAt/logSessionWindowSeconds)
}

func mergeLogSessionRows(requests []*Log) *Log {
	carrier := requests[len(requests)-1]
	merged := *carrier
	merged.Id = 0
	merged.CreatedAt = 0
	merged.Quota = 0
	merged.PromptTokens = 0
	merged.CompletionTokens = 0
	merged.UseTime = 0
	merged.IsStream = false
	errorRequests := 0
	logCount := 0
	summaries := make([]map[string]any, 0, min(len(requests), logSessionRequestLimit))
	for index, request := range requests {
		policy := parseLogRequestPolicy(request.Other)
		merged.Id = max(merged.Id, request.Id)
		merged.CreatedAt = max(merged.CreatedAt, request.CreatedAt)
		if request.Type == LogTypeConsume && policy.BillingSource == "subscription" && policy.SubscriptionConsumed != nil {
			merged.Quota += *policy.SubscriptionConsumed
		} else if request.Type == LogTypeRefund {
			merged.Quota -= request.Quota
		} else {
			merged.Quota += request.Quota
		}
		merged.PromptTokens += request.PromptTokens
		merged.CompletionTokens += request.CompletionTokens
		merged.UseTime += request.UseTime
		merged.IsStream = merged.IsStream || request.IsStream
		logCount += logRequestLogCount(policy)
		if request.Type == LogTypeError {
			errorRequests++
		}
		if index < logSessionRequestLimit {
			summaries = append(summaries, map[string]any{
				"request_id":  request.RequestId,
				"created_at":  request.CreatedAt,
				"type":        request.Type,
				"status_code": policy.StatusCode,
				"quota":       request.Quota,
				"attempts":    logRequestAttempts(policy),
				"error":       request.Content,
			})
		}
	}
	merged.Type = carrier.Type
	for _, request := range requests {
		if request.Type == LogTypeConsume {
			merged.Type = LogTypeConsume
			break
		}
	}
	if merged.Type == LogTypeRefund {
		merged.Quota = -merged.Quota
	}
	if len(requests) > 1 {
		merged.Other = withSessionInfo(carrier.Other, summaries, requests, errorRequests, logCount)
	}
	return &merged
}

func withSessionInfo(other string, summaries []map[string]any, requests []*Log, errorCount int, logCount int) string {
	if other == "" {
		other = "{}"
	}
	var values map[string]any
	if err := common.UnmarshalJsonStr(other, &values); err != nil {
		return other
	}
	if values == nil {
		values = make(map[string]any)
	}
	sources := make(map[string]struct{})
	subscriptionConsumed, walletDeducted := 0, 0
	for _, request := range requests {
		if request.Type != LogTypeConsume && request.Type != LogTypeRefund {
			continue
		}
		billing := parseLogRequestPolicy(request.Other)
		sources[billing.BillingSource] = struct{}{}
		sign := 1
		if request.Type == LogTypeRefund {
			sign = -1
		}
		if billing.BillingSource == "subscription" {
			consumed := request.Quota
			if billing.SubscriptionConsumed != nil {
				consumed = *billing.SubscriptionConsumed
			}
			subscriptionConsumed += sign * consumed
		} else {
			walletDeducted += sign * request.Quota
		}
	}
	// Per-subscription balances and IDs belong to individual requests. Keep
	// only aggregate deductions on a session, including mixed funding sources.
	for key := range values {
		if strings.HasPrefix(key, "subscription_") {
			delete(values, key)
		}
	}
	delete(values, "billing_source")
	for source := range sources {
		if len(sources) > 1 {
			values["billing_source"] = "mixed"
			break
		}
		if source != "" {
			values["billing_source"] = source
		}
	}
	values["subscription_consumed"] = subscriptionConsumed
	values["wallet_quota_deducted"] = walletDeducted
	values["session_requests"] = summaries
	values["session_request_count"] = len(requests)
	values["session_error_count"] = errorCount
	values["session_log_count"] = logCount
	values["session_window_seconds"] = logSessionWindowSeconds
	encoded, err := common.Marshal(values)
	if err != nil {
		return other
	}
	return string(encoded)
}

// Log summary

const logSummaryRowLimit = 200000

type LogSummaryBucket struct {
	Timestamp   int64 `json:"t"`
	Requests    int64 `json:"requests"`
	Success     int64 `json:"success"`
	Failed      int64 `json:"failed"`
	Retried     int64 `json:"retried"`
	Quota       int64 `json:"quota"`
	Tokens      int64 `json:"tokens"`
	Failed4xx   int64 `json:"failed_4xx"`
	Failed5xx   int64 `json:"failed_5xx"`
	Interrupted int64 `json:"interrupted"`
	P95         int   `json:"p95"`
}

type LogSummary struct {
	Quota               int64              `json:"quota"`
	Rpm                 int                `json:"rpm"`
	Tpm                 int                `json:"tpm"`
	Requests            int64              `json:"requests"`
	SuccessRequests     int64              `json:"success_requests"`
	FailedRequests      int64              `json:"failed_requests"`
	Failed4xx           int64              `json:"failed_4xx"`
	Failed5xx           int64              `json:"failed_5xx"`
	InterruptedRequests int64              `json:"interrupted_requests"`
	StatusCodes         map[string]int64   `json:"status_codes"`
	RetriedRequests     int64              `json:"retried_requests"`
	ErrorRows           int64              `json:"error_rows"`
	ErrorRate           float64            `json:"error_rate"`
	PromptTokens        int64              `json:"prompt_tokens"`
	CompletionTokens    int64              `json:"completion_tokens"`
	Tokens              int64              `json:"tokens"`
	P50                 int                `json:"p50"`
	P95                 int                `json:"p95"`
	P99                 int                `json:"p99"`
	AvgUseTime          float64            `json:"avg_use_time"`
	Series              []LogSummaryBucket `json:"series"`
}

type logSummaryRow struct {
	Id               int    `gorm:"column:id"`
	RequestId        string `gorm:"column:request_id"`
	Type             int    `gorm:"column:type"`
	Quota            int    `gorm:"column:quota"`
	PromptTokens     int    `gorm:"column:prompt_tokens"`
	CompletionTokens int    `gorm:"column:completion_tokens"`
	UseTime          int    `gorm:"column:use_time"`
	CreatedAt        int64  `gorm:"column:created_at"`
	Other            string `gorm:"column:other"`
}

type logSummaryRequest struct {
	hasConsume       bool
	hasError         bool
	interrupted      bool
	lastErrorStatus  int
	lastErrorAt      int64
	rows             int64
	quota            int64
	promptTokens     int64
	completionTokens int64
	useTime          int
	lastCreatedAt    int64
}

// SummarizeLogs computes request-level totals, error rate, latency and a time
// series for relay rows matching the log filters. Oversized queries fail
// explicitly instead of returning partial totals as a complete summary.
func SummarizeLogs(filter LogFilter, userId int) (LogSummary, error) {
	var summary LogSummary
	query, err := filter.apply(LOG_DB.Table("logs"))
	if err != nil {
		return summary, err
	}
	if userId > 0 {
		query = query.Where("logs.user_id = ?", userId)
	}
	query = query.Where("logs.type IN ?", []int{LogTypeConsume, LogTypeError})
	rows, err := query.
		Select("logs.id, logs.request_id, logs.type, logs.quota, logs.prompt_tokens, logs.completion_tokens, logs.use_time, logs.created_at, logs.other").
		Limit(logSummaryRowLimit + 1).
		Rows()
	if err != nil {
		return summary, err
	}
	defer rows.Close()

	now := time.Now().Unix()
	requests := make(map[string]*logSummaryRequest)
	summary.StatusCodes = make(map[string]int64)
	minTs, maxTs := int64(0), int64(0)
	rowCount := 0
	for rows.Next() {
		rowCount++
		if rowCount > logSummaryRowLimit {
			return LogSummary{}, common.NewMessage("Too many logs to summarize. Narrow the time range or add filters.")
		}
		var row logSummaryRow
		if err := LOG_DB.ScanRows(rows, &row); err != nil {
			return summary, err
		}
		key := row.RequestId
		if key == "" {
			key = "legacy:" + strconv.Itoa(row.Id)
		}
		item := requests[key]
		if item == nil {
			item = &logSummaryRequest{}
			requests[key] = item
		}
		item.rows++
		item.quota += int64(row.Quota)
		item.promptTokens += int64(row.PromptTokens)
		item.completionTokens += int64(row.CompletionTokens)
		item.useTime = max(item.useTime, row.UseTime)
		item.lastCreatedAt = max(item.lastCreatedAt, row.CreatedAt)
		var summaryOther struct {
			StatusCode   int `json:"status_code"`
			StreamStatus struct {
				Status string `json:"status"`
			} `json:"stream_status"`
		}
		if row.Type == LogTypeConsume || row.Type == LogTypeError {
			_ = common.UnmarshalJsonStr(row.Other, &summaryOther)
		}
		switch row.Type {
		case LogTypeConsume:
			item.hasConsume = true
			status := summaryOther.StreamStatus.Status
			if status != "" && status != "ok" && status != "completed" {
				item.interrupted = true
			}
		case LogTypeError:
			item.hasError = true
			summary.ErrorRows++
			if row.CreatedAt >= item.lastErrorAt {
				item.lastErrorStatus = summaryOther.StatusCode
				item.lastErrorAt = row.CreatedAt
			}
		}
		if row.Type == LogTypeConsume && row.CreatedAt >= now-60 {
			summary.Rpm++
			summary.Tpm += row.PromptTokens + row.CompletionTokens
		}
		if minTs == 0 || row.CreatedAt < minTs {
			minTs = row.CreatedAt
		}
		if row.CreatedAt > maxTs {
			maxTs = row.CreatedAt
		}
	}
	if err := rows.Err(); err != nil {
		return summary, err
	}

	durations := make([]int, 0, len(requests))
	for _, item := range requests {
		summary.Requests++
		if item.hasConsume {
			summary.SuccessRequests++
			if item.interrupted {
				summary.InterruptedRequests++
			}
		} else if item.hasError {
			summary.FailedRequests++
			code := item.lastErrorStatus
			if code >= 400 && code < 500 {
				summary.Failed4xx++
			} else if code >= 500 && code < 600 {
				summary.Failed5xx++
			}
			if code > 0 {
				summary.StatusCodes[strconv.Itoa(code)]++
			}
		}
		if item.rows > 1 {
			summary.RetriedRequests++
		}
		summary.Quota += item.quota
		summary.PromptTokens += item.promptTokens
		summary.CompletionTokens += item.completionTokens
		durations = append(durations, item.useTime)
	}
	summary.Tokens = summary.PromptTokens + summary.CompletionTokens
	if summary.Requests > 0 {
		summary.ErrorRate = float64(summary.FailedRequests) / float64(summary.Requests) * 100
		totalUseTime := 0
		for _, duration := range durations {
			totalUseTime += duration
		}
		summary.AvgUseTime = float64(totalUseTime) / float64(len(durations))
	}
	sort.Ints(durations)
	summary.P50 = logPercentile(durations, 50)
	summary.P95 = logPercentile(durations, 95)
	summary.P99 = logPercentile(durations, 99)
	summary.Series = buildLogSummarySeries(filter, requests, minTs, maxTs)
	return summary, nil
}

func logPercentile(sorted []int, percentile int) int {
	if len(sorted) == 0 {
		return 0
	}
	index := (percentile*len(sorted) + 99) / 100
	index = max(1, min(index, len(sorted)))
	return sorted[index-1]
}

func buildLogSummarySeries(filter LogFilter, requests map[string]*logSummaryRequest, minTs int64, maxTs int64) []LogSummaryBucket {
	start := filter.StartTimestamp
	if start == 0 {
		start = minTs
	}
	end := filter.EndTimestamp
	if end == 0 {
		end = maxTs
	}
	if start == 0 && end == 0 {
		return []LogSummaryBucket{}
	}
	if end <= start {
		end = start + 1
	}
	size := logSummaryBucketSeconds(end - start)
	for (end-start)/size > 400 {
		size *= 2
	}
	first := start / size * size
	buckets := make([]LogSummaryBucket, 0, (end-first)/size+1)
	bucketDurations := make([][]int, 0, (end-first)/size+1)
	index := make(map[int64]int, len(buckets))
	for timestamp := first; timestamp <= end; timestamp += size {
		index[timestamp] = len(buckets)
		buckets = append(buckets, LogSummaryBucket{Timestamp: timestamp})
		bucketDurations = append(bucketDurations, nil)
	}
	for _, request := range requests {
		bucketIndex, ok := index[request.lastCreatedAt/size*size]
		if !ok {
			continue
		}
		bucket := &buckets[bucketIndex]
		bucket.Requests++
		if request.hasConsume {
			bucket.Success++
			if request.interrupted {
				bucket.Interrupted++
			}
		} else if request.hasError {
			bucket.Failed++
			code := request.lastErrorStatus
			if code >= 400 && code < 500 {
				bucket.Failed4xx++
			} else if code >= 500 && code < 600 {
				bucket.Failed5xx++
			}
		}
		if request.rows > 1 {
			bucket.Retried++
		}
		bucket.Quota += request.quota
		bucket.Tokens += request.promptTokens + request.completionTokens
		bucketDurations[bucketIndex] = append(bucketDurations[bucketIndex], request.useTime)
	}
	for i := range buckets {
		sort.Ints(bucketDurations[i])
		buckets[i].P95 = logPercentile(bucketDurations[i], 95)
	}
	return buckets
}

func logSummaryBucketSeconds(rangeSeconds int64) int64 {
	switch {
	case rangeSeconds <= 3600:
		return 60
	case rangeSeconds <= 6*3600:
		return 300
	case rangeSeconds <= 24*3600:
		return 1800
	case rangeSeconds <= 3*86400:
		return 3600
	case rangeSeconds <= 7*86400:
		return 6 * 3600
	default:
		return 86400
	}
}
