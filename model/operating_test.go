package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openOperatingTestDB 打开一个方言的测试库。MySQL/PostgreSQL 需要对应的 DSN，
// 未配置时跳过，而不是用 SQLite 的结果冒充跨库兼容。
func openOperatingTestDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	var driver gorm.Dialector
	switch dialect {
	case "sqlite":
		driver = sqlite.Open(":memory:")
	case "mysql":
		dsn := os.Getenv("TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("TEST_MYSQL_DSN is not configured")
		}
		driver = mysql.Open(dsn)
	case "postgres":
		dsn := os.Getenv("TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("TEST_POSTGRES_DSN is not configured")
		}
		driver = postgres.Open(dsn)
	default:
		t.Fatalf("unknown dialect %q", dialect)
	}
	db, err := gorm.Open(driver, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// SQLite 内存库按连接隔离，单连接保证建表与查询看到同一个库。
	sqlDB.SetMaxOpenConns(1)

	var version string
	versionSQL := "SELECT version()"
	if dialect == "sqlite" {
		versionSQL = "SELECT sqlite_version()"
	}
	require.NoError(t, db.Raw(versionSQL).Scan(&version).Error)
	t.Logf("%s version: %s", dialect, version)
	return db
}

// setupOperatingTestDB 用独立库接管 DB/LOG_DB，并在用例结束后删表清理。
func setupOperatingTestDB(t *testing.T, dialect string) {
	t.Helper()
	db := openOperatingTestDB(t, dialect)
	previousDB, previousLogDB := DB, LOG_DB
	DB, LOG_DB = db, db

	tables := []any{&User{}, &TopUp{}, &Checkin{}, &Redemption{}, &OperatingExpense{}, &Log{}}
	for _, table := range tables {
		if db.Migrator().HasTable(table) {
			require.NoError(t, db.Migrator().DropTable(table))
		}
	}
	require.NoError(t, DB.AutoMigrate(&User{}, &TopUp{}, &Checkin{}, &Redemption{}, &OperatingExpense{}))
	require.NoError(t, LOG_DB.AutoMigrate(&Log{}))

	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		for _, table := range tables {
			assert.NoError(t, db.Migrator().DropTable(table))
		}
		sqlDB, err := db.DB()
		if err == nil {
			assert.NoError(t, sqlDB.Close())
		}
	})
}

// seedOperatingSummary 写入一套固定样本：期间内/外的充值、投入、消费、余额与发放。
func seedOperatingSummary(t *testing.T, startAt, endAt int64) {
	t.Helper()

	// 收入：两条成功充值（不同渠道语义）+ 一条待支付订单，后者不应计入。
	topUps := []TopUp{
		{UserId: 1, Amount: 2, Money: 1.8, TradeNo: "t1", Status: common.TopUpStatusSuccess, PaymentProvider: PaymentProviderEpay, CompleteTime: startAt + 60},
		{UserId: 2, Amount: 100_000, Money: 10, TradeNo: "t2", Status: common.TopUpStatusSuccess, PaymentProvider: PaymentProviderCreem, CompleteTime: startAt + 120},
		{UserId: 3, Amount: 5, Money: 5, TradeNo: "t3", Status: common.TopUpStatusPending, PaymentProvider: PaymentProviderEpay, CompleteTime: startAt + 180},
		{UserId: 4, Amount: 5, Money: 5, TradeNo: "t4", Status: common.TopUpStatusSuccess, PaymentProvider: PaymentProviderEpay, CompleteTime: endAt + 10},
	}
	require.NoError(t, DB.Create(&topUps).Error)

	// 投入：期间内两笔，期间外一笔。
	require.NoError(t, DB.Create(&OperatingExpense{
		Category: OperatingExpenseCategoryServer, AmountCents: 12_345, OccurredAt: startAt + 10, OperatorId: 1, CreatedAt: startAt,
	}).Error)
	require.NoError(t, DB.Create(&OperatingExpense{
		Category: OperatingExpenseCategoryUpstreamPackage, AmountCents: 776_655, OccurredAt: startAt + 20, OperatorId: 1, CreatedAt: startAt,
	}).Error)
	require.NoError(t, DB.Create(&OperatingExpense{
		Category: OperatingExpenseCategoryOther, AmountCents: 999, OccurredAt: endAt + 1, OperatorId: 1, CreatedAt: startAt,
	}).Error)

	// 消费：同一用户两次，另一用户一次，外加一条期间外日志。
	require.NoError(t, LOG_DB.Create(&Log{UserId: 1, Type: LogTypeConsume, Quota: 30_000, CreatedAt: startAt + 30}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 1, Type: LogTypeConsume, Quota: 20_000, CreatedAt: startAt + 40}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 2, Type: LogTypeConsume, Quota: 5_000, CreatedAt: startAt + 50}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 3, Type: LogTypeConsume, Quota: 7_000, CreatedAt: endAt + 5}).Error)
	// 零额度消费日志（零估价任务）不产生消费，也不计入去重人数。
	require.NoError(t, LOG_DB.Create(&Log{UserId: 4, Type: LogTypeConsume, Quota: 0, CreatedAt: startAt + 60}).Error)

	// 余额：软删用户不计入。
	require.NoError(t, DB.Create(&User{Id: 11, Username: "alice", AffCode: "aff11", Quota: 100_000}).Error)
	require.NoError(t, DB.Create(&User{Id: 12, Username: "bob", AffCode: "aff12", Quota: 20_000}).Error)
	require.NoError(t, DB.Create(&User{Id: 13, Username: "gone", AffCode: "aff13", Quota: 50_000}).Error)
	require.NoError(t, DB.Delete(&User{}, 13).Error)

	// 发放构成：签到（期内/期外各一）、兑换码、管理员调整、新用户赠送。
	require.NoError(t, DB.Create(&Checkin{UserId: 11, CheckinDate: "2023-11-14", QuotaAwarded: 7_000, CreatedAt: startAt + 100}).Error)
	require.NoError(t, DB.Create(&Checkin{UserId: 11, CheckinDate: "2023-11-15", QuotaAwarded: 3_000, CreatedAt: endAt + 100}).Error)
	require.NoError(t, DB.Create(&Redemption{
		Key: "code-1", Status: common.RedemptionCodeStatusUsed, Quota: 50_000,
		RedeemedTime: startAt + 200, UsedUserId: 11,
	}).Error)

	adminOther := `{"op":{"action":"user.quota_add","params":{"quota":25000,"from":0,"to":25000,"target_user_id":11}}}`
	require.NoError(t, LOG_DB.Create(&Log{
		UserId: 11, Type: LogTypeTopup, CreatedAt: startAt + 300,
		Content: "Increased user quota by " + logger.FormatQuota(25_000), Other: adminOther,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId: 11, Type: LogTypeSystem, CreatedAt: startAt + 400,
		Content: "New user sign-up bonus " + logger.FormatQuota(500_000),
	}).Error)
	// 邀请人和被邀请人的奖励都要计入邀请来源。
	require.NoError(t, LOG_DB.Create(&Log{
		UserId: 12, Type: LogTypeSystem, CreatedAt: startAt + 420,
		Content: "Invitation code bonus " + logger.FormatQuota(100_000),
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId: 11, Type: LogTypeSystem, CreatedAt: startAt + 440,
		Content: "Bonus for inviting a user " + logger.FormatQuota(200_000),
	}).Error)
}

func TestGetOperatingSummary(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			setupOperatingTestDB(t, dialect)
			startAt, endAt := int64(1_700_000_000), int64(1_700_086_400)
			seedOperatingSummary(t, startAt, endAt)

			summary, err := GetOperatingSummary(startAt, endAt)
			require.NoError(t, err)

			assert.EqualValues(t, 2, summary.Revenue.Orders)
			// Epay 的 money 已是人民币（1.8）；Creem 的 10 是美元，按 Price 换算。
			rate := operation_setting.USDExchangeRate
			assert.EqualValues(t, int64(1.8*100)+int64(10*rate*100), summary.Revenue.AmountCents)
			assert.True(t, summary.Revenue.MixedCurrency)
			assert.EqualValues(t, 100_000+int64(common.QuotaPerUnit*2), summary.Revenue.CreditedQuota)

			assert.EqualValues(t, 2, summary.Cost.Entries)
			assert.EqualValues(t, 789_000, summary.Cost.AmountCents)

			assert.EqualValues(t, 55_000, summary.Consumption.Quota)
			assert.EqualValues(t, 2, summary.Consumption.Users)

			// 软删用户的余额仍然存在，必须计入快照，不能被默认软删除过滤静默排除。
			assert.EqualValues(t, 3, summary.Balance.Users)
			assert.EqualValues(t, 170_000, summary.Balance.Quota)
			assert.NotZero(t, summary.Balance.SnapshotAt)

			byKey := make(map[string]OperatingGrant, len(summary.Grants))
			for _, grant := range summary.Grants {
				byKey[grant.Key] = grant
			}
			assert.EqualValues(t, 1, byKey["checkin"].Count)
			assert.EqualValues(t, 7_000, byKey["checkin"].Quota)
			assert.EqualValues(t, 1, byKey["redemption"].Count)
			assert.EqualValues(t, 50_000, byKey["redemption"].Quota)
			assert.EqualValues(t, 1, byKey["admin"].Count)
			assert.EqualValues(t, 25_000, byKey["admin"].Quota)
			assert.EqualValues(t, 1, byKey["signup"].Count)
			assert.EqualValues(t, 500_000, byKey["signup"].Quota)
			assert.EqualValues(t, 2, byKey["invite"].Count)
			assert.EqualValues(t, 300_000, byKey["invite"].Quota)
			assert.EqualValues(t, 2, byKey["paid"].Count)
			assert.EqualValues(t, 100_000+int64(common.QuotaPerUnit*2), byKey["paid"].Quota)
		})
	}
}

func TestOperatingExpenseLifecycle(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			setupOperatingTestDB(t, dialect)
			startAt := int64(1_700_000_000)

			expense, err := CreateOperatingExpense(OperatingExpenseInput{
				Category:    OperatingExpenseCategoryServer,
				AmountCents: 5_000,
				OccurredAt:  startAt,
				Note:        "monthly host",
				OperatorId:  1,
			})
			require.NoError(t, err)
			require.NotZero(t, expense.Id)

			expenses, err := GetOperatingExpenses(startAt, startAt+60)
			require.NoError(t, err)
			require.Len(t, expenses, 1)
			assert.EqualValues(t, 5_000, expenses[0].AmountCents)

			require.NoError(t, DeleteOperatingExpense(expense.Id))
			expenses, err = GetOperatingExpenses(startAt, startAt+60)
			require.NoError(t, err)
			assert.Empty(t, expenses)
		})
	}
}

func TestCreateOperatingExpenseRejectsInvalidInput(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			setupOperatingTestDB(t, dialect)

			_, err := CreateOperatingExpense(OperatingExpenseInput{
				Category:    "unknown",
				AmountCents: 100,
				OccurredAt:  1_700_000_000,
			})
			require.Error(t, err)

			_, err = CreateOperatingExpense(OperatingExpenseInput{
				Category:    OperatingExpenseCategoryServer,
				AmountCents: 0,
				OccurredAt:  1_700_000_000,
			})
			require.Error(t, err)
		})
	}
}

// TestOperatingExpenseMigrationIdempotent 验证建表可重复执行且不改变已有数据。
func TestOperatingExpenseMigrationIdempotent(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			setupOperatingTestDB(t, dialect)

			expense := &OperatingExpense{
				Category:    OperatingExpenseCategoryUpstreamBalance,
				AmountCents: 42_000,
				OccurredAt:  1_700_000_500,
				OperatorId:  1,
				CreatedAt:   1_700_000_000,
			}
			require.NoError(t, DB.Create(expense).Error)

			for range 2 {
				require.NoError(t, DB.AutoMigrate(&OperatingExpense{}))
			}

			var rows []OperatingExpense
			require.NoError(t, DB.Find(&rows).Error)
			require.Len(t, rows, 1)
			assert.EqualValues(t, 42_000, rows[0].AmountCents)
			assert.Equal(t, OperatingExpenseCategoryUpstreamBalance, rows[0].Category)
		})
	}
}

// TestOperatingAdminGrantSkipsOverride 固定 override 不计入发放额度：
// 一条历史 override 的 from/to 差额可能包含单位换算，当成发放会算出天文数字。
func TestOperatingAdminGrantSkipsOverride(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			setupOperatingTestDB(t, dialect)
			startAt, endAt := int64(1_700_000_000), int64(1_700_086_400)

			logs := []Log{
				{UserId: 1, Type: LogTypeTopup, CreatedAt: startAt + 10, Content: "Increased user quota by x",
					Other: `{"op":{"action":"user.quota_add","params":{"quota":7000,"from":0,"to":7000}}}`},
				{UserId: 2, Type: LogTypeTopup, CreatedAt: startAt + 20, Content: "Decreased user quota by x",
					Other: `{"op":{"action":"user.quota_subtract","params":{"quota":2000,"from":2000,"to":0}}}`},
				// 单位换算或脏数据造成的超大幅度 override，不应计入发放额度。
				{UserId: 3, Type: LogTypeTopup, CreatedAt: startAt + 30, Content: "Overrode user quota from x to y",
					Other: `{"op":{"action":"user.quota_override","params":{"from":4015085375,"to":7142857}}}`},
			}
			require.NoError(t, LOG_DB.Create(&logs).Error)

			grant, err := getOperatingAdminGrant(startAt, endAt)
			require.NoError(t, err)
			assert.EqualValues(t, 2, grant.Count)
			assert.EqualValues(t, 5_000, grant.Quota)
		})
	}
}

func TestParseQuotaFromLogText(t *testing.T) {
	quotaPerUnit := int64(common.QuotaPerUnit)

	// FormatQuota 的美元展示与解析应能往返。
	quota, ok := parseQuotaFromLogText(logger.FormatQuota(int(quotaPerUnit)))
	require.True(t, ok)
	assert.EqualValues(t, quotaPerUnit, quota)

	quota, ok = parseQuotaFromLogText(" 250000 ")
	require.True(t, ok)
	assert.EqualValues(t, 250_000, quota)

	_, ok = parseQuotaFromLogText("not a number")
	assert.False(t, ok)
}
