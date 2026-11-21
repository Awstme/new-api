package model

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/shopspring/decimal"
)

// 经营分析的统计口径：
//   - 期间收入、投入、消费按 [startAt, endAt) 左闭右开统计，边界由调用方按北京时间切好。
//   - 当前余额是展示时点的快照，不随后所选期间变成历史期末余额。
//   - 发放构成只回答「这期间被发放了多少额度」，不与剩余余额倒推来源。

const (
	OperatingExpenseCategoryUpstreamPackage = "upstream_package"
	OperatingExpenseCategoryUpstreamBalance = "upstream_balance"
	OperatingExpenseCategoryServer          = "server"
	OperatingExpenseCategoryOther           = "other"
)

// OperatingExpense 是管理员手工录入的经营支出。金额以整数分保存，避免二进制
// 浮点在期间汇总时累积误差。
type OperatingExpense struct {
	Id          int    `json:"id"`
	Category    string `json:"category" gorm:"type:varchar(32);index"`
	AmountCents int64  `json:"amount_cents"`
	OccurredAt  int64  `json:"occurred_at" gorm:"bigint;index"`
	Note        string `json:"note" gorm:"type:varchar(255)"`
	OperatorId  int    `json:"operator_id"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
}

func (expense *OperatingExpense) Create() error {
	return DB.Create(expense).Error
}

func (expense *OperatingExpense) Delete() error {
	return DB.Delete(&OperatingExpense{}, expense.Id).Error
}

// GetOperatingExpenses 返回期间内的投入明细，按发生时间倒序。
func GetOperatingExpenses(startAt, endAt int64) ([]OperatingExpense, error) {
	expenses := make([]OperatingExpense, 0, 16)
	err := DB.Where("occurred_at >= ? AND occurred_at < ?", startAt, endAt).
		Order("occurred_at desc, id desc").
		Find(&expenses).Error
	return expenses, err
}

// OperatingRevenue 是期间内的钱包实收。Money 是各渠道记录的实付金额，CreditedQuota
// 是这些订单实际到账的额度——两者单位不同，不能互相推导。
type OperatingRevenue struct {
	Orders int64 `json:"orders"`
	// AmountCents 是统一换算到人民币分的实收合计；MixedCurrency 标记期间内是否
	// 混有不同记账币种的渠道，此时前端不应把它当成可直接记账的现金金额。
	AmountCents   int64 `json:"amount_cents"`
	CreditedQuota int64 `json:"credited_quota"`
	MixedCurrency bool  `json:"mixed_currency"`
}

type OperatingCost struct {
	Entries     int64 `json:"entries"`
	AmountCents int64 `json:"amount_cents"`
}

type OperatingConsumption struct {
	Quota int64 `json:"quota"`
	Users int64 `json:"users"`
}

type OperatingBalance struct {
	Users int64 `json:"users"`
	Quota int64 `json:"quota"`
	// SnapshotAt 是余额快照的时点，避免把「当前余额」误读成期末余额。
	SnapshotAt int64 `json:"snapshot_at"`
}

// OperatingGrant 是一类发放来源在期间内的合计。Accuracy 说明额度是否可信：
// exact 来自结构化表，estimated 是对历史日志文本的还原。
type OperatingGrant struct {
	Key      string `json:"key"`
	Count    int64  `json:"count"`
	Quota    int64  `json:"quota"`
	Accuracy string `json:"accuracy"`
}

type OperatingSummary struct {
	StartTimestamp int64                `json:"start_timestamp"`
	EndTimestamp   int64                `json:"end_timestamp"`
	Revenue        OperatingRevenue     `json:"revenue"`
	Cost           OperatingCost        `json:"cost"`
	Consumption    OperatingConsumption `json:"consumption"`
	Balance        OperatingBalance     `json:"balance"`
	Grants         []OperatingGrant     `json:"grants"`
	// LogsAvailable 表示消费与发放来源是否能从日志库读取。日志库不可用或记录
	// 已被清理时，这两部分不完整，不能被当成零消费展示。
	LogsAvailable bool `json:"logs_available"`
}

const (
	operatingGrantExact     = "exact"
	operatingGrantEstimated = "estimated"
)

// GetOperatingSummary 汇总一个期间内的经营分析数据。所有查询都按时间列过滤，
// 不加载全量历史，所以耗时不随站点总记录数增长。
func GetOperatingSummary(startAt, endAt int64) (OperatingSummary, error) {
	summary := OperatingSummary{
		StartTimestamp: startAt,
		EndTimestamp:   endAt,
		Grants:         make([]OperatingGrant, 0, 6),
	}

	revenue, err := getOperatingRevenue(startAt, endAt)
	if err != nil {
		return summary, err
	}
	summary.Revenue = revenue

	cost, err := getOperatingCost(startAt, endAt)
	if err != nil {
		return summary, err
	}
	summary.Cost = cost

	// 日志库是独立配置的可清理存储：它不可用时只让消费与发放不完整，
	// 主库的收入、投入与余额仍然要能看，不能整体失败。
	consumption, err := getOperatingConsumption(startAt, endAt)
	if err != nil {
		common.SysLog(common.LogText("operating analysis: consume logs unavailable: %s", err.Error()))
	} else {
		summary.Consumption = consumption
		summary.LogsAvailable = true
	}

	balance, err := getOperatingBalance()
	if err != nil {
		return summary, err
	}
	summary.Balance = balance

	if !summary.LogsAvailable {
		return summary, nil
	}

	grants, err := getOperatingGrants(startAt, endAt, revenue)
	if err != nil {
		common.SysLog(common.LogText("operating analysis: grant logs unavailable: %s", err.Error()))
		return summary, nil
	}
	summary.Grants = grants
	return summary, nil
}

func getOperatingRevenue(startAt, endAt int64) (OperatingRevenue, error) {
	var revenue OperatingRevenue
	// 各支付渠道对 amount/money 的语义不同（有的记额度、有的记实付），所以按渠道
	// 分组取回，再在 Go 里按各自规则换算到账额度。
	var rows []struct {
		Provider string
		Orders   int64
		Amount   int64
		Money    float64
	}
	err := DB.Model(&TopUp{}).
		Where("status = ? AND complete_time >= ? AND complete_time < ?", common.TopUpStatusSuccess, startAt, endAt).
		Select("payment_provider AS provider, COUNT(*) AS orders, COALESCE(SUM(amount), 0) AS amount, COALESCE(SUM(money), 0) AS money").
		Group("payment_provider").
		Scan(&rows).Error
	if err != nil {
		return revenue, err
	}
	currencies := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		credited, err := operatingCreditedQuota(row.Provider, row.Amount, row.Money)
		if err != nil {
			return revenue, err
		}
		revenue.Orders += row.Orders
		revenue.CreditedQuota += credited
		revenue.AmountCents += operatingRevenueCents(row.Provider, row.Money)
		currencies[operatingProviderCurrency(row.Provider)] = struct{}{}
	}
	// 各渠道的 money 记账币种不同（Epay/Waffo 是人民币，Stripe 等是美元），
	// 混在一起相加得到的数字不能称为某个币种的实收，必须显式标记。
	revenue.MixedCurrency = len(currencies) > 1
	return revenue, nil
}

// operatingProviderCurrency 返回该渠道 money 字段的记账币种。Waffo 的币种由
// 配置决定（默认 USD），不能按渠道名硬编码成人民币。
func operatingProviderCurrency(provider string) string {
	switch provider {
	case PaymentProviderEpay, PaymentProviderWaffoPancake:
		return "CNY"
	case PaymentProviderWaffo:
		if currency := setting.WaffoCurrency; currency != "" {
			return currency
		}
		return "USD"
	case PaymentProviderStripe, PaymentProviderCreem:
		return "USD"
	default:
		return "UNKNOWN"
	}
}

// operatingRevenueCents 把渠道 money 换算为人民币分。Epay/Waffo 的金额已经按
// 站内价格折算成人民币；Stripe/Creem 记录的是美元金额，按 USD 汇率换算。
func operatingRevenueCents(provider string, money float64) int64 {
	rate := operation_setting.USDExchangeRate
	if rate <= 0 {
		rate = 1
	}
	switch operatingProviderCurrency(provider) {
	case "CNY":
		return int64(money * 100)
	case "USD":
		return int64(money * rate * 100)
	default:
		return 0
	}
}

// operatingCreditedQuota 复现各渠道把订单字段换算成入账额度的规则。
func operatingCreditedQuota(provider string, amount int64, money float64) (int64, error) {
	if provider == PaymentProviderCreem {
		// Creem 的产品额度直接存在 amount 上，不按 QuotaPerUnit 换算。
		return amount, nil
	}
	base := decimal.NewFromInt(amount)
	if provider == PaymentProviderStripe {
		// Stripe 的 amount 是用户填写的金额，实际到账按折后 money 计算。
		base = decimal.NewFromFloat(money)
	}
	credited, err := common.WalletQuotaFromDecimalStrict(base.Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	if err != nil {
		return 0, err
	}
	return int64(credited), nil
}

func getOperatingCost(startAt, endAt int64) (OperatingCost, error) {
	var cost OperatingCost
	err := DB.Model(&OperatingExpense{}).
		Where("occurred_at >= ? AND occurred_at < ?", startAt, endAt).
		Select("COUNT(*) AS entries, COALESCE(SUM(amount_cents), 0) AS amount_cents").
		Scan(&cost).Error
	return cost, err
}

// getOperatingConsumption 统计期间内的最终消费。只有正额度日志算作消费，
// 零额度（例如零估价任务）不产生消费用户，否则去重人数会被放大。
func getOperatingConsumption(startAt, endAt int64) (OperatingConsumption, error) {
	var consumption OperatingConsumption
	err := LOG_DB.Table("logs").
		Where("type = ? AND quota > 0 AND created_at >= ? AND created_at < ?", LogTypeConsume, startAt, endAt).
		Select("COALESCE(SUM(quota), 0) AS quota, COUNT(DISTINCT user_id) AS users").
		Scan(&consumption).Error
	return consumption, err
}

// getOperatingBalance 统计当前余额快照。软删除用户的余额仍然真实存在，
// 因此显式包含他们，不依赖 GORM 的默认软删除过滤，避免余额被静默遗漏。
func getOperatingBalance() (OperatingBalance, error) {
	var balance OperatingBalance
	err := DB.Model(&User{}).
		Unscoped().
		Select("COUNT(*) AS users, COALESCE(SUM(quota), 0) AS quota").
		Scan(&balance).Error
	balance.SnapshotAt = common.GetTimestamp()
	return balance, err
}

func getOperatingGrants(startAt, endAt int64, revenue OperatingRevenue) ([]OperatingGrant, error) {
	grants := make([]OperatingGrant, 0, 6)

	checkin, err := getOperatingCheckinGrant(startAt, endAt)
	if err != nil {
		return nil, err
	}
	grants = append(grants, checkin)

	redemption, err := getOperatingRedemptionGrant(startAt, endAt)
	if err != nil {
		return nil, err
	}
	grants = append(grants, redemption)

	admin, err := getOperatingAdminGrant(startAt, endAt)
	if err != nil {
		return nil, err
	}
	grants = append(grants, admin)

	signup, invite, err := getOperatingSignupGrants(startAt, endAt)
	if err != nil {
		return nil, err
	}
	grants = append(grants, signup, invite)

	grants = append(grants, OperatingGrant{
		Key:      "paid",
		Count:    revenue.Orders,
		Quota:    revenue.CreditedQuota,
		Accuracy: operatingGrantExact,
	})
	return grants, nil
}

func getOperatingCheckinGrant(startAt, endAt int64) (OperatingGrant, error) {
	var row struct {
		Count int64
		Quota int64
	}
	err := DB.Model(&Checkin{}).
		Where("created_at >= ? AND created_at < ?", startAt, endAt).
		Select("COUNT(*) AS count, COALESCE(SUM(quota_awarded), 0) AS quota").
		Scan(&row).Error
	return OperatingGrant{Key: "checkin", Count: row.Count, Quota: row.Quota, Accuracy: operatingGrantExact}, err
}

func getOperatingRedemptionGrant(startAt, endAt int64) (OperatingGrant, error) {
	var row struct {
		Count int64
		Quota int64
	}
	err := DB.Model(&Redemption{}).
		Where("status = ? AND redeemed_time >= ? AND redeemed_time < ?", common.RedemptionCodeStatusUsed, startAt, endAt).
		Select("COUNT(*) AS count, COALESCE(SUM(quota), 0) AS quota").
		Scan(&row).Error
	return OperatingGrant{Key: "redemption", Count: row.Count, Quota: row.Quota, Accuracy: operatingGrantExact}, err
}

// getOperatingAdminGrant 从管理员调整日志里还原净增额度。日志的 other.op.params 保存了
// 结构化的 before/after 与调整额，因此这一项不需要解析展示文案。
func getOperatingAdminGrant(startAt, endAt int64) (OperatingGrant, error) {
	grant := OperatingGrant{Key: "admin", Accuracy: operatingGrantExact}
	var rows []struct {
		Other string
	}
	err := LOG_DB.Table("logs").
		Where("type = ? AND created_at >= ? AND created_at < ?", LogTypeTopup, startAt, endAt).
		Where("content LIKE ? OR content LIKE ? OR content LIKE ?",
			"Increased user quota by%", "Decreased user quota by%", "Overrode user quota from%").
		Select("other").
		Find(&rows).Error
	if err != nil {
		return grant, err
	}
	for _, row := range rows {
		var meta struct {
			Op struct {
				Action string `json:"action"`
				Params struct {
					Quota int64 `json:"quota"`
				} `json:"params"`
			} `json:"op"`
		}
		if err := common.UnmarshalJsonStr(row.Other, &meta); err != nil {
			continue
		}
		delta := int64(0)
		switch meta.Op.Action {
		case "user.quota_add":
			delta = meta.Op.Params.Quota
		case "user.quota_subtract":
			delta = -meta.Op.Params.Quota
		default:
			// override 是绝对值重置，差额可能包含单位换算或历史脏数据，
			// 当成「发放额度」会把一项重置放大成天文数字。只计入调整次数。
			continue
		}
		grant.Count++
		grant.Quota += delta
	}
	return grant, nil
}

// getOperatingSignupGrants 统计新用户赠送与邀请奖励。这两类额度只存在于系统日志里，
// 额度数字需要从历史展示文案还原，因此标记为 estimated。
func getOperatingSignupGrants(startAt, endAt int64) (signup OperatingGrant, invite OperatingGrant, err error) {
	signup = OperatingGrant{Key: "signup", Accuracy: operatingGrantEstimated}
	invite = OperatingGrant{Key: "invite", Accuracy: operatingGrantEstimated}
	var rows []struct {
		Content string
	}
	err = LOG_DB.Table("logs").
		Where("type = ? AND created_at >= ? AND created_at < ?", LogTypeSystem, startAt, endAt).
		Where("content LIKE ? OR content LIKE ? OR content LIKE ?",
			"New user sign-up bonus%", "Invitation code bonus%", "Bonus for inviting a user%").
		Select("content").
		Find(&rows).Error
	if err != nil {
		return signup, invite, err
	}
	for _, row := range rows {
		target := &signup
		rest := row.Content
		if trimmed, ok := strings.CutPrefix(rest, "New user sign-up bonus"); ok {
			rest = trimmed
		} else if trimmed, ok := strings.CutPrefix(rest, "Invitation code bonus"); ok {
			target = &invite
			rest = trimmed
		} else if trimmed, ok := strings.CutPrefix(rest, "Bonus for inviting a user"); ok {
			// 邀请人的奖励也是一笔真实发放，漏掉会让邀请来源偏低。
			target = &invite
			rest = trimmed
		} else {
			continue
		}
		target.Count++
		if quota, ok := parseQuotaFromLogText(rest); ok {
			target.Quota += quota
		}
	}
	return signup, invite, nil
}

// operatingQuotaTextPattern 拆出历史日志里的「货币符号 + 数字」。
var operatingQuotaTextPattern = regexp.MustCompile(`^\s*([^\d\s.,+-]*)\s*(-?[0-9][0-9,]*\.?[0-9]*)\s*$`)

// parseQuotaFromLogText 把 logger.FormatQuota 写进日志的展示值还原成额度。
// 符号决定当时的展示模式：＄ 为美元、¥ 为人民币、无符号为纯额度、其余为自定义货币。
func parseQuotaFromLogText(text string) (int64, bool) {
	matches := operatingQuotaTextPattern.FindStringSubmatch(text)
	if matches == nil {
		return 0, false
	}
	symbol := matches[1]
	amount, err := strconv.ParseFloat(strings.ReplaceAll(matches[2], ",", ""), 64)
	if err != nil {
		return 0, false
	}

	quotaPerUnit := common.QuotaPerUnit
	rate := 1.0
	switch symbol {
	case "＄", "$":
		rate = 1
	case "¥":
		rate = operation_setting.USDExchangeRate
	default:
		if symbol == "" {
			// 纯额度展示，日志里就是额度本身。
			return int64(amount), true
		}
		rate = operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
	}
	if rate <= 0 {
		return 0, false
	}
	return int64(amount / rate * quotaPerUnit), true
}

// OperatingExpenseInput 是管理员提交的一笔投入。
type OperatingExpenseInput struct {
	Category    string
	AmountCents int64
	OccurredAt  int64
	Note        string
	OperatorId  int
}

func (input OperatingExpenseInput) Validate() error {
	switch input.Category {
	case OperatingExpenseCategoryUpstreamPackage,
		OperatingExpenseCategoryUpstreamBalance,
		OperatingExpenseCategoryServer,
		OperatingExpenseCategoryOther:
	default:
		return common.NewMessage("Invalid expense category")
	}
	if input.AmountCents <= 0 {
		return common.NewMessage("Expense amount must be greater than zero")
	}
	if input.AmountCents > 100_000_000_00 {
		return common.NewMessage("Expense amount is too large")
	}
	if input.OccurredAt <= 0 {
		return common.NewMessage("Expense date is required")
	}
	if len([]rune(input.Note)) > 200 {
		return common.NewMessage("Expense note is too long")
	}
	return nil
}

func CreateOperatingExpense(input OperatingExpenseInput) (*OperatingExpense, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	expense := &OperatingExpense{
		Category:    input.Category,
		AmountCents: input.AmountCents,
		OccurredAt:  input.OccurredAt,
		Note:        input.Note,
		OperatorId:  input.OperatorId,
		CreatedAt:   common.GetTimestamp(),
	}
	if err := expense.Create(); err != nil {
		return nil, err
	}
	return expense, nil
}

func DeleteOperatingExpense(id int) error {
	expense := &OperatingExpense{Id: id}
	return expense.Delete()
}
