package model

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFreeQuotaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	previousDB, previousLogDB := DB, LOG_DB
	previousType := common.LogDatabaseType()
	DB, LOG_DB = db, db
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetLogDatabaseType(previousType)
		assert.NoError(t, sqlDB.Close())
	})

	require.NoError(t, db.AutoMigrate(&Log{}, &AuditLog{}))
	require.NoError(t, db.Exec("CREATE TABLE users (id integer primary key, username text, created_at bigint, deleted_at datetime)").Error)
	require.NoError(t, db.Exec("CREATE TABLE redemptions (id integer primary key, user_id integer, used_user_id integer, quota integer, status integer, redeemed_time bigint, deleted_at datetime)").Error)
	require.NoError(t, db.Exec("CREATE TABLE checkins (id integer primary key, user_id integer, checkin_date text, quota_awarded integer, created_at bigint)").Error)
	return db
}

func TestGetFreeQuotaStatsUsesConsumedQuota(t *testing.T) {
	db := setupFreeQuotaTestDB(t)

	previousNew := common.QuotaForNewUser
	previousInvitee := common.QuotaForInvitee
	previousInviter := common.QuotaForInviter
	common.QuotaForNewUser = 1000
	common.QuotaForInvitee = 500
	common.QuotaForInviter = 200
	t.Cleanup(func() {
		common.QuotaForNewUser = previousNew
		common.QuotaForInvitee = previousInvitee
		common.QuotaForInviter = previousInviter
	})

	for id := 1; id <= 10; id++ {
		require.NoError(t, db.Exec(
			"INSERT INTO users (id, username) VALUES (?, ?)",
			id,
			common.LogText("user%d", id),
		).Error)
	}

	structuredGrant := func(grantType string, quota int) string {
		return common.MapToJsonStr(map[string]any{
			"grant_type":  grantType,
			"grant_quota": quota,
		})
	}
	adminParams := map[string]any{
		"target_user_id":  4,
		"target_username": "user4",
		"mode":            "add",
		"requested_quota": 700,
		"quota":           700,
		"from":            0,
		"to":              700,
	}
	adminOperation := func() *AuditOperation {
		return &AuditOperation{Action: "user.quota_add", Params: adminParams}
	}
	adminOther := common.MapToJsonStr(map[string]any{
		"op": map[string]any{
			"action": "user.quota_add",
			"params": adminParams,
		},
	})
	adminParams2 := map[string]any{
		"target_user_id":  9,
		"target_username": "user9",
		"mode":            "add",
		"quota":           500,
	}
	adminOther2 := common.MapToJsonStr(map[string]any{
		"op": map[string]any{
			"action": "user.quota_add",
			"params": adminParams2,
		},
	})
	onlineTopupOther := common.MapToJsonStr(map[string]any{
		"admin_info": map[string]any{
			"payment_method":          "alipay",
			"callback_payment_method": "epay",
		},
		"content_parts": []any{
			map[string]any{
				"key": "Online top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}",
				"params": map[string]any{
					"quota":  "¥10.000000",
					"amount": "10.000000",
				},
			},
		},
	})

	rows := []*Log{
		{UserId: 1, Username: "user1", Type: LogTypeSystem, Content: "New user sign-up bonus $2", CreatedAt: 16, Other: structuredGrant("new_user", 1000)},
		{UserId: 2, Username: "user2", Type: LogTypeSystem, Content: "New user sign-up bonus $2", CreatedAt: 10, Other: structuredGrant("new_user", 1000)},
		{UserId: 3, Username: "user3", Type: LogTypeSystem, Content: "New user sign-up bonus $1", CreatedAt: 16, Other: structuredGrant("new_user", 500)},
		{UserId: 7, Username: "user7", Type: LogTypeSystem, Content: "New user sign-up bonus $2", CreatedAt: 16, Other: structuredGrant("new_user", 1000)},
		{UserId: 8, Username: "user8", Type: LogTypeSystem, Content: "New user sign-up bonus 1000", CreatedAt: 16, Other: "{}"},

		{UserId: 1, Username: "user1", Type: LogTypeConsume, Content: "consume", CreatedAt: 20, Quota: 400},
		{UserId: 1, Username: "user1", Type: LogTypeConsume, Content: "consume", CreatedAt: 40, Quota: 300},
		{UserId: 1, Username: "user1", Type: LogTypeRefund, Content: "refund", CreatedAt: 25, Quota: 100},
		{UserId: 2, Username: "user2", Type: LogTypeConsume, Content: "consume", CreatedAt: 12, Quota: 400},
		{UserId: 2, Username: "user2", Type: LogTypeConsume, Content: "consume", CreatedAt: 20, Quota: 400},
		{UserId: 3, Username: "user3", Type: LogTypeConsume, Content: "consume", CreatedAt: 20, Quota: 800},
		{UserId: 7, Username: "user7", Type: LogTypeConsume, Content: "consume", CreatedAt: 40, Quota: 100},
		{UserId: 8, Username: "user8", Type: LogTypeConsume, Content: "consume", CreatedAt: 20, Quota: 100},
		{UserId: 4, Username: "user4", Type: LogTypeConsume, Content: "consume", CreatedAt: 20, Quota: 200},
		{UserId: 5, Username: "user5", Type: LogTypeConsume, Content: "consume", CreatedAt: 20, Quota: 100},
		{UserId: 5, Username: "user5", Type: LogTypeRefund, Content: "refund", CreatedAt: 25, Quota: 50},
		{UserId: 6, Username: "user6", Type: LogTypeConsume, Content: "consume", CreatedAt: 20, Quota: 25},
		{UserId: 9, Username: "user9", Type: LogTypeConsume, Content: "consume", CreatedAt: 20, Quota: 100},
		{UserId: 10, Username: "user10", Type: LogTypeConsume, Content: "consume", CreatedAt: 20, Quota: 250},

		{UserId: 4, Username: "user4", Type: LogTypeTopup, Content: "Increased user quota by 700", CreatedAt: 16, RequestId: "admin-1", Other: adminOther},
		{UserId: 9, Username: "user9", Type: LogTypeTopup, Content: "Increased user quota by 500", CreatedAt: 16, RequestId: "admin-2", Other: adminOther2},
		{UserId: 1, Username: "user1", Type: LogTypeTopup, Content: "Online top-up succeeded, amount added: ¥10.000000, payment amount: 10.000000", CreatedAt: 16, Other: onlineTopupOther},
		{UserId: 5, Username: "user5", Type: LogTypeTopup, Content: "Topped up ¥5.000002 with a redemption code", CreatedAt: 16},
	}
	require.NoError(t, db.Create(&rows).Error)
	require.NoError(t, db.Create(&AuditLog{
		EventId:   "audit-grant-1",
		UserId:    1,
		ActorRole: common.RoleAdminUser,
		CreatedAt: 16,
		Action:    "user.quota_add",
		RequestId: "admin-1",
		Other: AuditOther{
			Op: adminOperation(),
		},
	}).Error)

	require.NoError(t, db.Exec(
		"INSERT INTO redemptions (id, user_id, used_user_id, quota, status, redeemed_time) VALUES (1, 5, 5, 300, ?, 16)",
		common.RedemptionCodeStatusUsed,
	).Error)
	require.NoError(t, db.Exec("INSERT INTO checkins (id, user_id, checkin_date, quota_awarded, created_at) VALUES (1, 6, '2026-11-15', 50, 16)").Error)

	stats, err := GetFreeQuotaStats(15, 30)
	require.NoError(t, err)

	byKey := make(map[string]FreeQuotaSource, len(stats.Sources))
	for _, source := range stats.Sources {
		byKey[source.Key] = source
	}
	require.Len(t, byKey, 7)

	// User 1: 400 consumed at t=20, 100 refunded at t=25, 300 consumed after end.
	// User 2: 400 consumed before start depletes the grant, 400 remains in range.
	// User 3: consumed 800 but only a 500 grant exists.
	// User 8: legacy log-text grant.
	assert.EqualValues(t, 1300, byKey[FreeQuotaSourceNewUser].Quota)
	assert.EqualValues(t, 3500, byKey[FreeQuotaSourceNewUser].Granted)
	assert.EqualValues(t, 4, byKey[FreeQuotaSourceNewUser].Count)
	assert.EqualValues(t, 4, byKey[FreeQuotaSourceNewUser].GrantCount)
	assert.Equal(t, "mixed", byKey[FreeQuotaSourceNewUser].Method)

	assert.EqualValues(t, 50, byKey[FreeQuotaSourceRedemption].Quota)
	assert.EqualValues(t, 300, byKey[FreeQuotaSourceRedemption].Granted)
	assert.EqualValues(t, 1, byKey[FreeQuotaSourceRedemption].Count)
	assert.Equal(t, "exact", byKey[FreeQuotaSourceRedemption].Method)

	assert.EqualValues(t, 25, byKey[FreeQuotaSourceCheckin].Quota)
	assert.EqualValues(t, 50, byKey[FreeQuotaSourceCheckin].Granted)

	assert.EqualValues(t, 300, byKey[FreeQuotaSourceAdmin].Quota)
	assert.EqualValues(t, 1200, byKey[FreeQuotaSourceAdmin].Granted)
	assert.EqualValues(t, 2, byKey[FreeQuotaSourceAdmin].GrantCount, "audit and topup rows share a request id")
	assert.Equal(t, "exact", byKey[FreeQuotaSourceAdmin].Method)

	assert.EqualValues(t, 1675, stats.Total)
	assert.EqualValues(t, 1675, stats.FreeTotal)
	assert.EqualValues(t, 2225, stats.ConsumptionTotal)
	assert.EqualValues(t, 5050, stats.Granted)

	onlineQuota, ok := parseGrantQuota("amount added: ¥10.000000")
	require.True(t, ok)
	assert.EqualValues(t, 1, stats.OnlineTopupCount)
	assert.EqualValues(t, onlineQuota, stats.OnlineTopupQuota)
	assert.InDelta(t, 10.0, stats.OnlineTopupAmount, 0.000001)
	require.Len(t, stats.OnlineTopupRecords, 1)
	assert.EqualValues(t, 1, stats.OnlineTopupRecords[0].UserId)
	assert.Equal(t, "user1", stats.OnlineTopupRecords[0].Username)
	assert.EqualValues(t, onlineQuota, stats.OnlineTopupRecords[0].Quota)
	assert.InDelta(t, 10.0, stats.OnlineTopupRecords[0].Amount, 0.000001)
	assert.Equal(t, "alipay", stats.OnlineTopupRecords[0].PaymentMethod)

	assert.EqualValues(t, 550, byKey[QuotaSourcePaid].Quota)
	assert.EqualValues(t, 2, byKey[QuotaSourcePaid].Count)
	assert.Equal(t, "paid", byKey[QuotaSourcePaid].Kind)
	assert.Equal(t, "exact", byKey[QuotaSourcePaid].Method)
	paidByUser := make(map[int]FreeQuotaUser)
	for _, user := range byKey[QuotaSourcePaid].Users {
		paidByUser[user.UserId] = user
	}
	assert.EqualValues(t, 300, paidByUser[3].Used, "user3 consumed 800 but only 500 was free")
	assert.EqualValues(t, 250, paidByUser[10].Used, "consumption without grants belongs to the paid source")
	assert.EqualValues(t, 0, paidByUser[10].Granted)

	newUserUsers := make(map[int]FreeQuotaUser)
	for _, user := range byKey[FreeQuotaSourceNewUser].Users {
		newUserUsers[user.UserId] = user
	}
	require.Len(t, newUserUsers, 4)
	assert.EqualValues(t, 300, newUserUsers[1].Used)
	assert.EqualValues(t, 1000, newUserUsers[1].Granted)
	assert.EqualValues(t, 1, newUserUsers[1].GrantCount)
	assert.EqualValues(t, 400, newUserUsers[2].Used)
	assert.EqualValues(t, 500, newUserUsers[3].Used)
	assert.EqualValues(t, 100, newUserUsers[8].Used)
	assert.Equal(t, "log_text", newUserUsers[8].Method)
	assert.NotContains(t, newUserUsers, 7, "grants without usage are not listed")

	adminUsers := byKey[FreeQuotaSourceAdmin].Users
	require.Len(t, adminUsers, 2)
	adminByUser := make(map[int]FreeQuotaUser)
	for _, user := range adminUsers {
		adminByUser[user.UserId] = user
	}
	assert.EqualValues(t, 200, adminByUser[4].Used)
	assert.EqualValues(t, 700, adminByUser[4].Granted)
	assert.EqualValues(t, 100, adminByUser[9].Used)
	assert.EqualValues(t, 500, adminByUser[9].Granted, "topup-only grants are counted without an audit row")
}

func TestParseGrantQuota(t *testing.T) {
	quotaPerUnit := common.QuotaPerUnit
	if quotaPerUnit <= 0 {
		t.Skip("quotaPerUnit is not configured")
	}
	quota, ok := parseGrantQuota("New user sign-up bonus ＄0.002")
	require.True(t, ok)
	assert.EqualValues(t, int64(math.Round(0.002*quotaPerUnit)), quota)

	quota, ok = parseGrantQuota("Invitation code bonus $0.002")
	require.True(t, ok)
	assert.EqualValues(t, int64(math.Round(0.002*quotaPerUnit)), quota)

	quota, ok = parseGrantQuota("Invitation code bonus 1000")
	require.True(t, ok)
	assert.EqualValues(t, 1000, quota)

	_, ok = parseGrantQuota("no amount here")
	assert.False(t, ok)
}

func TestSubscriptionDoesNotConsumeWalletGrant(t *testing.T) {
	db := setupFreeQuotaTestDB(t)
	rows := []*Log{
		{UserId: 1, Username: "alice", RequestId: "grant", CreatedAt: 10, Type: LogTypeSystem, Content: "New user sign-up bonus 100", Other: `{"grant_type":"new_user","grant_quota":100}`},
		{UserId: 1, Username: "alice", RequestId: "subscription", CreatedAt: 20, Type: LogTypeConsume, Quota: 100, Other: `{"task_id":"subscription-task","billing_source":"subscription","wallet_quota_deducted":0,"subscription_consumed":100}`},
		// Historical polling logs omitted the source; inherit it from the task's submit log.
		{UserId: 1, Username: "alice", RequestId: "subscription-delta", CreatedAt: 21, Type: LogTypeConsume, Quota: 50, Other: `{"task_id":"subscription-task"}`},
		{UserId: 1, Username: "alice", RequestId: "subscription-refund", CreatedAt: 22, Type: LogTypeRefund, Quota: 150, Other: `{"task_id":"subscription-task"}`},
		{UserId: 1, Username: "alice", RequestId: "wallet", CreatedAt: 40, Type: LogTypeConsume, Quota: 100, Other: `{"billing_source":"wallet"}`},
	}
	require.NoError(t, db.Create(&rows).Error)
	stats, err := GetFreeQuotaStats(35, 50)
	require.NoError(t, err)
	assert.EqualValues(t, 100, stats.FreeTotal)
}

func TestRefundRestoresOnlyTheOriginalGrantSource(t *testing.T) {
	db := setupFreeQuotaTestDB(t)
	rows := []*Log{
		{UserId: 1, Username: "alice", CreatedAt: 10, Type: LogTypeSystem, Content: "New user sign-up bonus 100", Other: `{"grant_type":"new_user","grant_quota":100}`},
		{UserId: 1, Username: "alice", CreatedAt: 20, Type: LogTypeConsume, Quota: 100, Other: `{"task_id":"first-task"}`},
		{UserId: 1, Username: "alice", CreatedAt: 21, Type: LogTypeSystem, Content: "Invitation code bonus 100", Other: `{"grant_type":"invitee","grant_quota":100}`},
		{UserId: 1, Username: "alice", CreatedAt: 22, Type: LogTypeConsume, Quota: 150, Other: `{"task_id":"second-task"}`},
		{UserId: 1, Username: "alice", CreatedAt: 23, Type: LogTypeRefund, Quota: 50, Other: `{"task_id":"first-task"}`},
		{UserId: 1, Username: "alice", CreatedAt: 24, Type: LogTypeRefund, Quota: 70, Other: `{"task_id":"second-task"}`},
	}
	require.NoError(t, db.Create(&rows).Error)
	stats, err := GetFreeQuotaStats(15, 30)
	require.NoError(t, err)
	used := make(map[string]int64)
	for _, source := range stats.Sources {
		used[source.Key] = source.Quota
	}
	assert.EqualValues(t, 50, used[FreeQuotaSourceNewUser])
	assert.EqualValues(t, 80, used[FreeQuotaSourceInvitee])
	assert.Zero(t, used[QuotaSourcePaid])
	assert.EqualValues(t, 130, stats.ConsumptionTotal)
}

func TestPaidTaskRefundDoesNotRestoreFreeGrant(t *testing.T) {
	for _, tc := range []struct {
		name        string
		refundTask  string
		refundQuota int
		wantFree    int64
	}{
		{"paid task", "paid-task", 100, 100},
		{"earlier free task", "free-task", 100, 0},
		{"partial paid refund", "paid-task", 40, 100},
		{"unknown task", "unknown", 100, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupFreeQuotaTestDB(t)
			rows := []*Log{
				{UserId: 1, Username: "alice", RequestId: "grant", CreatedAt: 10, Type: LogTypeSystem, Content: "New user sign-up bonus 100", Other: `{"grant_type":"new_user","grant_quota":100}`},
				{UserId: 1, Username: "alice", RequestId: "free-task", CreatedAt: 20, Type: LogTypeConsume, Quota: 100, Other: `{"task_id":"free-task"}`},
				{UserId: 1, Username: "alice", RequestId: "paid-task", CreatedAt: 21, Type: LogTypeConsume, Quota: 100, Other: `{"task_id":"paid-task"}`},
				{UserId: 1, Username: "alice", RequestId: "refund", CreatedAt: 22, Type: LogTypeRefund, Quota: tc.refundQuota, Other: common.MapToJsonStr(map[string]any{"task_id": tc.refundTask})},
			}
			require.NoError(t, db.Create(&rows).Error)
			stats, err := GetFreeQuotaStats(15, 30)
			require.NoError(t, err)
			assert.Equal(t, tc.wantFree, stats.FreeTotal)
			assert.EqualValues(t, 200-tc.refundQuota, stats.ConsumptionTotal)
		})
	}
}

func TestUntransferredInviterBonusIsNotWalletGrant(t *testing.T) {
	db := setupFreeQuotaTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO users (id, username) VALUES (1, 'alice')").Error)
	rows := []*Log{
		{UserId: 1, Username: "alice", RequestId: "grant", CreatedAt: 10, Type: LogTypeSystem, Content: "Bonus for inviting a user 100", Other: `{"grant_type":"inviter","grant_quota":100}`},
		{UserId: 1, Username: "alice", RequestId: "wallet", CreatedAt: 20, Type: LogTypeConsume, Quota: 100},
	}
	require.NoError(t, db.Create(&rows).Error)
	stats, err := GetFreeQuotaStats(15, 30)
	require.NoError(t, err)
	assert.Zero(t, stats.FreeTotal)
}

func TestInvitationTransferRecordsOnlyCreditedQuota(t *testing.T) {
	db := setupFreeQuotaTestDB(t)
	require.NoError(t, db.AutoMigrate(&User{}))
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })
	user := User{Id: 1, Username: "inviter", Quota: 500, AffQuota: 200}
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error)
	require.NoError(t, user.TransferAffQuotaToQuota(100))
	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	assert.Equal(t, 600, stored.Quota)
	assert.Equal(t, 100, stored.AffQuota)
	var grants []Log
	require.NoError(t, db.Where("type = ?", LogTypeSystem).Find(&grants).Error)
	require.Len(t, grants, 1)
	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(grants[0].Other, &other))
	assert.Equal(t, "inviter_transfer", other["grant_type"])
	assert.EqualValues(t, 100, other["grant_quota"])
	grantTime := grants[0].CreatedAt
	require.NoError(t, db.Create(&Log{UserId: user.Id, Username: user.Username, Type: LogTypeConsume, CreatedAt: grantTime + 1, Quota: 50}).Error)
	stats, err := GetFreeQuotaStats(grantTime, grantTime+1)
	require.NoError(t, err)
	assert.EqualValues(t, 100, stats.Granted)
	assert.EqualValues(t, 50, stats.FreeTotal)
	require.Error(t, user.TransferAffQuotaToQuota(200))
	var count int64
	require.NoError(t, db.Model(&Log{}).Where("type = ?", LogTypeSystem).Count(&count).Error)
	assert.EqualValues(t, 1, count, "failed transfers cannot grant quota")
}
