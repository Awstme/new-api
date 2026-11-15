package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogRequestTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	previousDB, previousLogDB := DB, LOG_DB
	previousGroupCol := logGroupCol
	previousType := common.LogDatabaseType()
	DB, LOG_DB = db, db
	logGroupCol = "`group`"
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		logGroupCol = previousGroupCol
		common.SetLogDatabaseType(previousType)
		assert.NoError(t, sqlDB.Close())
	})

	require.NoError(t, db.AutoMigrate(&Log{}))
	require.NoError(t, db.Exec("CREATE TABLE channels (id integer primary key, name text)").Error)
	require.NoError(t, db.Exec("INSERT INTO channels (id, name) VALUES (12, 'ch12'), (16, 'ch16'), (17, 'ch17')").Error)
	return db
}

func requestPolicyOther(policy []map[string]any, channels []any) string {
	return common.MapToJsonStr(map[string]any{
		"admin_info": map[string]any{
			"request_policy": policy,
			"use_channel":    channels,
		},
	})
}

func TestLogRequestViewMergesRetries(t *testing.T) {
	db := setupLogRequestTestDB(t)

	firstFailure := requestPolicyOther([]map[string]any{
		{"attempt": 1, "channel_id": 17, "elapsed_ms": 3, "decision": map[string]any{"action": "attempt", "reason": "channel_selected", "source": "routing"}},
		{"attempt": 1, "channel_id": 17, "status": 401, "elapsed_ms": 900, "decision": map[string]any{"action": "retry", "reason": "retry_status_matched", "source": "global"}},
	}, []any{"17"})
	fullPolicy := requestPolicyOther([]map[string]any{
		{"attempt": 1, "channel_id": 17, "elapsed_ms": 3, "decision": map[string]any{"action": "attempt", "reason": "channel_selected", "source": "routing"}},
		{"attempt": 1, "channel_id": 17, "status": 401, "elapsed_ms": 900, "decision": map[string]any{"action": "retry", "reason": "retry_status_matched", "source": "global"}},
		{"attempt": 2, "channel_id": 16, "elapsed_ms": 905, "decision": map[string]any{"action": "attempt", "reason": "channel_selected", "source": "routing"}},
		{"attempt": 2, "channel_id": 16, "status": 200, "elapsed_ms": 1500, "decision": map[string]any{"action": "success", "reason": "request_completed", "source": "upstream"}},
	}, []any{"17", "16"})

	rows := []*Log{
		{RequestId: "req-1", UserId: 7, Username: "alice", TokenName: "prod", ModelName: "gpt-4o", Type: LogTypeError, CreatedAt: 100, ChannelId: 17, Content: "status_code=401, Bad key", Other: firstFailure},
		{RequestId: "req-1", UserId: 7, Username: "alice", TokenName: "prod", ModelName: "gpt-4o", Type: LogTypeError, CreatedAt: 101, ChannelId: 16, Content: "status_code=401, Bad key", Other: firstFailure},
		{RequestId: "req-1", UserId: 7, Username: "alice", TokenName: "prod", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 102, ChannelId: 16, Quota: 100, PromptTokens: 10, CompletionTokens: 5, UseTime: 9, IsStream: true, Other: fullPolicy},
	}
	require.NoError(t, db.Create(&rows).Error)

	logs, total, err := GetAllLogRequests(LogFilter{}, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	merged := logs[0]
	assert.Equal(t, LogTypeConsume, merged.Type)
	assert.Equal(t, "req-1", merged.RequestId)
	assert.Equal(t, 100, merged.Quota)
	assert.Equal(t, 10, merged.PromptTokens)
	assert.Equal(t, 5, merged.CompletionTokens)
	assert.Equal(t, 9, merged.UseTime)
	assert.True(t, merged.IsStream)
	assert.Equal(t, 16, merged.ChannelId)
	assert.Equal(t, "ch16", merged.ChannelName)
	assert.Equal(t, 3, merged.Id)
	assert.EqualValues(t, 102, merged.CreatedAt)

	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(merged.Other, &other))
	adminInfo, ok := other["admin_info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(3), adminInfo["request_log_count"])
	assert.Equal(t, float64(2), adminInfo["request_error_count"])
	assert.Equal(t, float64(2), adminInfo["request_attempts"])
	policy, ok := adminInfo["request_policy"].([]any)
	require.True(t, ok)
	assert.Len(t, policy, 4, "the merged row must carry the complete retry policy")
}

func countLogsWithFilter(t *testing.T, filter LogFilter) int64 {
	t.Helper()
	tx, err := filter.apply(LOG_DB.Table("logs"))
	require.NoError(t, err)
	var count int64
	require.NoError(t, tx.Count(&count).Error)
	return count
}

func countLogsWithQuery(t *testing.T, query string) int64 {
	t.Helper()
	return countLogsWithFilter(t, LogFilter{Query: query})
}

func TestLogQueryFilterMatchesTextColumns(t *testing.T) {
	db := setupLogRequestTestDB(t)
	rows := []*Log{
		{RequestId: "req-alice", Username: "alice", TokenName: "prod", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 100},
		{RequestId: "req-bob-403", Username: "bob", TokenName: "test", ModelName: "claude", Type: LogTypeError, CreatedAt: 101, Content: "status_code=403, quota exceeded"},
		{RequestId: "req-carol", Username: "carol", TokenName: "prod", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 102},
	}
	require.NoError(t, db.Create(&rows).Error)

	assert.EqualValues(t, 1, countLogsWithQuery(t, "alice"))
	assert.EqualValues(t, 1, countLogsWithQuery(t, "alice prod"))
	assert.EqualValues(t, 1, countLogsWithQuery(t, "quota 403"))
	assert.EqualValues(t, 2, countLogsWithQuery(t, "gpt"))
	assert.EqualValues(t, 1, countLogsWithQuery(t, "req-bob"))
	assert.EqualValues(t, 0, countLogsWithQuery(t, "no-such-value"))
	assert.Equal(t, "%100!%!_x!!%", logQueryPattern("100%_x!"))
}

func TestLogSmallFiltersAreFuzzy(t *testing.T) {
	db := setupLogRequestTestDB(t)
	rows := []*Log{
		{RequestId: "req-alice", Username: "alice", TokenName: "prod", ModelName: "gpt-4o", UpstreamRequestId: "up-1", Type: LogTypeConsume, CreatedAt: 100},
		{RequestId: "req-bob-403", Username: "bob", TokenName: "test", ModelName: "claude", UpstreamRequestId: "up-2", Type: LogTypeError, CreatedAt: 101},
		{RequestId: "req-carol", Username: "carol", TokenName: "prod", ModelName: "gpt-4o", UpstreamRequestId: "up-3", Type: LogTypeConsume, CreatedAt: 102},
	}
	require.NoError(t, db.Create(&rows).Error)

	assert.EqualValues(t, 2, countLogsWithFilter(t, LogFilter{ModelName: "gpt"}))
	assert.EqualValues(t, 1, countLogsWithFilter(t, LogFilter{Username: "ali"}))
	assert.EqualValues(t, 2, countLogsWithFilter(t, LogFilter{TokenName: "rod"}))
	assert.EqualValues(t, 1, countLogsWithFilter(t, LogFilter{RequestId: "bob"}))
	assert.EqualValues(t, 1, countLogsWithFilter(t, LogFilter{UpstreamRequestId: "up-2"}))
	assert.EqualValues(t, 0, countLogsWithFilter(t, LogFilter{ModelName: "missing"}))
}

func TestLogSessionViewMergesAdjacentRequests(t *testing.T) {
	db := setupLogRequestTestDB(t)
	rows := []*Log{
		{RequestId: "req-a", UserId: 7, TokenId: 1, Username: "alice", TokenName: "prod", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 100, ChannelId: 16, Quota: 10, PromptTokens: 10, CompletionTokens: 1, UseTime: 2},
		{RequestId: "req-b", UserId: 7, TokenId: 1, Username: "alice", TokenName: "prod", ModelName: "gpt-4o", Type: LogTypeError, CreatedAt: 150, ChannelId: 16, Content: "status_code=500, upstream failed", Other: common.MapToJsonStr(map[string]any{"status_code": 500})},
		{RequestId: "req-c", UserId: 7, TokenId: 1, Username: "alice", TokenName: "prod", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 200, ChannelId: 16, Quota: 20, PromptTokens: 20, CompletionTokens: 2, UseTime: 5},
		{RequestId: "req-d", UserId: 7, TokenId: 1, Username: "alice", TokenName: "prod", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 500, ChannelId: 16, Quota: 5, PromptTokens: 5, CompletionTokens: 1, UseTime: 1},
		{RequestId: "req-e", UserId: 7, TokenId: 1, Username: "alice", TokenName: "prod", ModelName: "claude", Type: LogTypeConsume, CreatedAt: 210, ChannelId: 12, Quota: 7, PromptTokens: 7, CompletionTokens: 1, UseTime: 1},
	}
	require.NoError(t, db.Create(&rows).Error)

	sessions, total, err := GetAllLogSessions(LogFilter{}, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, sessions, 3)

	var merged *Log
	for _, session := range sessions {
		if session.Other == "" {
			continue
		}
		var other map[string]any
		require.NoError(t, common.UnmarshalJsonStr(session.Other, &other))
		if count, _ := other["session_request_count"].(float64); count == 3 {
			merged = session
			break
		}
	}
	require.NotNil(t, merged, "the three adjacent gpt requests must form one session")
	assert.Equal(t, LogTypeConsume, merged.Type)
	assert.Equal(t, "req-c", merged.RequestId)
	assert.Equal(t, 30, merged.Quota)
	assert.Equal(t, 30, merged.PromptTokens)
	assert.Equal(t, 3, merged.CompletionTokens)
	assert.Equal(t, 7, merged.UseTime)
	assert.Equal(t, 16, merged.ChannelId)
	assert.Equal(t, "ch16", merged.ChannelName)

	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(merged.Other, &other))
	assert.EqualValues(t, 3, other["session_request_count"])
	assert.EqualValues(t, 1, other["session_error_count"])
	assert.EqualValues(t, 3, other["session_log_count"])
	assert.EqualValues(t, logSessionWindowSeconds, other["session_window_seconds"])
	requests, ok := other["session_requests"].([]any)
	require.True(t, ok)
	require.Len(t, requests, 3)
	first, ok := requests[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "req-a", first["request_id"])
	second, ok := requests[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "req-b", second["request_id"])
	assert.EqualValues(t, LogTypeError, second["type"])
	assert.EqualValues(t, 500, second["status_code"])
	assert.Contains(t, second["error"], "upstream failed")
}

func TestGetLogsByRequestIdsScopesUser(t *testing.T) {
	db := setupLogRequestTestDB(t)
	rows := []*Log{
		{RequestId: "req-a", UserId: 7, Username: "alice", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 100, ChannelId: 16},
		{RequestId: "req-b", UserId: 8, Username: "bob", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 101, ChannelId: 16},
		{RequestId: "", UserId: 7, Username: "alice", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 102, ChannelId: 16},
	}
	require.NoError(t, db.Create(&rows).Error)

	all, truncated, err := GetLogsByRequestIds([]string{"req-a", "req-b", "legacy:1", "req-a"}, 0)
	require.NoError(t, err)
	assert.False(t, truncated)
	require.Len(t, all, 2)

	self, _, err := GetLogsByRequestIds([]string{"req-a", "req-b"}, 7)
	require.NoError(t, err)
	require.Len(t, self, 1)
	assert.Equal(t, "req-a", self[0].RequestId)
}

func TestLogSummaryHealthBreakdown(t *testing.T) {
	db := setupLogRequestTestDB(t)
	other := func(status int, streamStatus string) string {
		values := map[string]any{}
		if status > 0 {
			values["status_code"] = status
		}
		if streamStatus != "" {
			values["stream_status"] = map[string]any{"status": streamStatus}
		}
		return common.MapToJsonStr(values)
	}
	rows := []*Log{
		{RequestId: "req-ok", Username: "alice", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 100, Quota: 10, PromptTokens: 100, CompletionTokens: 20, UseTime: 3, Other: other(0, "ok")},
		{RequestId: "req-4xx", Username: "bob", ModelName: "gpt-4o", Type: LogTypeError, CreatedAt: 101, Content: "status_code=403", Other: other(403, "")},
		{RequestId: "req-5xx", Username: "bob", ModelName: "gpt-4o", Type: LogTypeError, CreatedAt: 102, Content: "status_code=500", Other: other(500, "")},
		{RequestId: "req-interrupted", Username: "carol", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 103, Quota: 5, PromptTokens: 50, CompletionTokens: 10, UseTime: 5, Other: other(0, "interrupted")},
		{RequestId: "req-retried", Username: "dave", ModelName: "gpt-4o", Type: LogTypeError, CreatedAt: 104, Content: "status_code=401", Other: other(401, "")},
		{RequestId: "req-retried", Username: "dave", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: 105, Quota: 20, PromptTokens: 200, CompletionTokens: 40, UseTime: 7, Other: other(0, "ok")},
	}
	require.NoError(t, db.Create(&rows).Error)

	summary, err := SummarizeLogs(LogFilter{}, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 5, summary.Requests)
	assert.EqualValues(t, 3, summary.SuccessRequests)
	assert.EqualValues(t, 2, summary.FailedRequests)
	assert.EqualValues(t, 1, summary.Failed4xx)
	assert.EqualValues(t, 1, summary.Failed5xx)
	assert.EqualValues(t, 1, summary.InterruptedRequests)
	assert.EqualValues(t, 1, summary.RetriedRequests)
	assert.EqualValues(t, 3, summary.ErrorRows)
	assert.EqualValues(t, 1, summary.StatusCodes["403"])
	assert.EqualValues(t, 1, summary.StatusCodes["500"])
	assert.NotContains(t, summary.StatusCodes, "401")

	require.NotEmpty(t, summary.Series)
	var bucket *LogSummaryBucket
	for i := range summary.Series {
		if summary.Series[i].Requests == 5 {
			bucket = &summary.Series[i]
			break
		}
	}
	require.NotNil(t, bucket)
	assert.EqualValues(t, 1, bucket.Failed4xx)
	assert.EqualValues(t, 1, bucket.Failed5xx)
	assert.EqualValues(t, 1, bucket.Interrupted)
	assert.Equal(t, 7, bucket.P95)
}

func TestLogSummaryCountsRequests(t *testing.T) {
	db := setupLogRequestTestDB(t)
	now := time.Now().Unix()
	rows := []*Log{
		{RequestId: "req-ok", Username: "alice", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: now, Quota: 10, PromptTokens: 100, CompletionTokens: 20, UseTime: 3},
		{RequestId: "req-failed", Username: "bob", ModelName: "gpt-4o", Type: LogTypeError, CreatedAt: now - 10, Content: "status_code=403"},
		{RequestId: "req-failed", Username: "bob", ModelName: "gpt-4o", Type: LogTypeError, CreatedAt: now - 9, Content: "status_code=403"},
		{RequestId: "req-retried", Username: "carol", ModelName: "gpt-4o", Type: LogTypeError, CreatedAt: now - 8, Content: "status_code=401"},
		{RequestId: "req-retried", Username: "carol", ModelName: "gpt-4o", Type: LogTypeConsume, CreatedAt: now - 7, Quota: 30, PromptTokens: 200, CompletionTokens: 40, UseTime: 7},
	}
	require.NoError(t, db.Create(&rows).Error)

	summary, err := SummarizeLogs(LogFilter{}, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 3, summary.Requests)
	assert.EqualValues(t, 2, summary.SuccessRequests)
	assert.EqualValues(t, 1, summary.FailedRequests)
	assert.EqualValues(t, 2, summary.RetriedRequests)
	assert.EqualValues(t, 3, summary.ErrorRows)
	assert.EqualValues(t, 40, summary.Quota)
	assert.EqualValues(t, 300, summary.PromptTokens)
	assert.EqualValues(t, 60, summary.CompletionTokens)
	assert.EqualValues(t, 360, summary.Tokens)
	assert.InDelta(t, 33.3333, summary.ErrorRate, 0.01)
	assert.Equal(t, 2, summary.Rpm)
	assert.Equal(t, 360, summary.Tpm)
	assert.NotEmpty(t, summary.Series)
}

func TestLogSummaryRejectsOversizedRange(t *testing.T) {
	db := setupLogRequestTestDB(t)
	// Exercise the public limit with real rows, without generating request payloads.
	require.NoError(t, db.Exec(`
WITH RECURSIVE entries(n) AS (
  SELECT 1 UNION ALL SELECT n + 1 FROM entries WHERE n < 200001
)
INSERT INTO logs (request_id, type, created_at, quota)
SELECT 'request', 2, 100, 1 FROM entries`).Error)
	summary, err := SummarizeLogs(LogFilter{}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Narrow the time range")
	assert.Zero(t, summary.Quota, "a partial total must never look like a complete summary")
}

func TestSummaryExcludesNonRelayLogs(t *testing.T) {
	db := setupLogRequestTestDB(t)
	rows := []*Log{
		{UserId: 1, RequestId: "request", CreatedAt: 20, Type: LogTypeConsume, Quota: 100, UseTime: 3},
		{UserId: 1, RequestId: "bonus", CreatedAt: 21, Type: LogTypeSystem, Content: "New user sign-up bonus 100"},
		{UserId: 1, RequestId: "topup", CreatedAt: 22, Type: LogTypeTopup},
	}
	require.NoError(t, db.Create(&rows).Error)
	summary, err := SummarizeLogs(LogFilter{}, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 1, summary.Requests)
	assert.Equal(t, 3, summary.P50)
}

func TestSessionKeepsCombinedSubscriptionCost(t *testing.T) {
	for _, tc := range []struct {
		name         string
		first        string
		second       string
		source       string
		subscription int64
		wallet       int64
	}{
		{"subscriptions", `{"billing_source":"subscription","subscription_consumed":10}`, `{"billing_source":"subscription","subscription_consumed":20,"subscription_id":2,"subscription_remain":100}`, "subscription", 30, 0},
		{"missing consumption", `{"billing_source":"subscription"}`, `{"billing_source":"subscription","subscription_consumed":20}`, "subscription", 30, 0},
		{"explicit zero", `{"billing_source":"subscription","subscription_consumed":0}`, `{"billing_source":"subscription","subscription_consumed":20}`, "subscription", 20, 0},
		{"mixed sources", `{"billing_source":"wallet"}`, `{"billing_source":"subscription","subscription_consumed":20}`, "mixed", 20, 10},
		{"mixed with zero subscription deduction", `{"billing_source":"wallet"}`, `{"billing_source":"subscription","subscription_consumed":0}`, "mixed", 0, 10},
		{"legacy", `{}`, `{}`, "", 0, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupLogRequestTestDB(t)
			rows := []*Log{
				{UserId: 1, TokenId: 1, ModelName: "model", RequestId: "a", CreatedAt: 100, Type: LogTypeConsume, Quota: 10, Other: tc.first},
				{UserId: 1, TokenId: 1, ModelName: "model", RequestId: "b", CreatedAt: 101, Type: LogTypeConsume, Quota: 20, Other: tc.second},
			}
			require.NoError(t, db.Create(&rows).Error)
			sessions, _, err := GetAllLogSessions(LogFilter{}, 0, 10)
			require.NoError(t, err)
			require.Len(t, sessions, 1)
			assert.EqualValues(t, tc.subscription+tc.wallet, sessions[0].Quota)
			var other map[string]any
			require.NoError(t, common.UnmarshalJsonStr(sessions[0].Other, &other))
			assert.EqualValues(t, tc.subscription, other["subscription_consumed"])
			assert.EqualValues(t, tc.wallet, other["wallet_quota_deducted"])
			if tc.source == "" {
				assert.NotContains(t, other, "billing_source")
			} else {
				assert.Equal(t, tc.source, other["billing_source"])
			}
			assert.NotContains(t, other, "subscription_id")
			assert.NotContains(t, other, "subscription_remain")
		})
	}
}
