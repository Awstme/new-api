package model

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// Free quota sources. The order is the order shown in the dashboard.
const (
	FreeQuotaSourceNewUser    = "new_user"
	FreeQuotaSourceInvitee    = "invitee"
	FreeQuotaSourceInviter    = "inviter"
	FreeQuotaSourceRedemption = "redemption"
	FreeQuotaSourceCheckin    = "checkin"
	FreeQuotaSourceAdmin      = "admin_add"
	QuotaSourcePaid           = "paid"
	quotaGrantInviterTransfer = "inviter_transfer"
)

var freeQuotaSourceOrder = []string{
	FreeQuotaSourceNewUser,
	FreeQuotaSourceInvitee,
	FreeQuotaSourceInviter,
	FreeQuotaSourceRedemption,
	FreeQuotaSourceCheckin,
	FreeQuotaSourceAdmin,
}

// FreeQuotaUser is one user's consumed free quota under a source. Granted and
// GrantCount only describe grants inside the requested range; Used is the part
// of the free grant that was actually consumed inside the range.
type FreeQuotaUser struct {
	UserId     int    `json:"user_id"`
	Username   string `json:"username"`
	Granted    int64  `json:"granted"`
	Used       int64  `json:"used"`
	GrantCount int64  `json:"grant_count"`
	LastUsedAt int64  `json:"last_used_at"`
	Method     string `json:"method"`
}

// FreeQuotaSource aggregates consumed free quota. Quota is the consumed amount;
// Granted is shown for context and is not added to the total.
type FreeQuotaSource struct {
	Key        string          `json:"key"`
	Kind       string          `json:"kind"`
	Quota      int64           `json:"quota"`
	Granted    int64           `json:"granted"`
	Count      int64           `json:"count"`
	GrantCount int64           `json:"grant_count"`
	Method     string          `json:"method"`
	Users      []FreeQuotaUser `json:"users"`
}

type FreeQuotaStats struct {
	// Total is the consumed free quota. It is kept as the compatibility field
	// for the dashboard card that shows the free portion.
	Total int64 `json:"total"`
	// FreeTotal mirrors Total with an explicit name.
	FreeTotal int64 `json:"free_total"`
	// ConsumptionTotal is total consumption, including the paid/normal source.
	ConsumptionTotal int64 `json:"consumption_total"`
	// Online top-ups are cash recharges, kept separate from consumption sources.
	OnlineTopupCount   int64               `json:"online_topup_count"`
	OnlineTopupQuota   int64               `json:"online_topup_quota"`
	OnlineTopupAmount  float64             `json:"online_topup_amount"`
	OnlineTopupRecords []OnlineTopupRecord `json:"online_topup_records"`
	Granted            int64               `json:"granted"`
	Sources            []FreeQuotaSource   `json:"sources"`
}

type OnlineTopupRecord struct {
	UserId        int     `json:"user_id"`
	Username      string  `json:"username"`
	CreatedAt     int64   `json:"created_at"`
	Quota         int64   `json:"quota"`
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
	RequestId     string  `json:"request_id"`
}

type freeQuotaGrant struct {
	UserId   int
	Username string
	Time     int64
	Source   string
	Quota    int64
	Method   string
}

type freeQuotaConsumption struct {
	UserId   int
	Username string
	Time     int64
	Type     int
	Quota    int64
	TaskId   string
}

type rangeQuotaConsumption struct {
	UserId     int
	Username   string
	Quota      int64
	LastUsedAt int64
}

var (
	grantAmountPattern   = regexp.MustCompile(`[-+]?[0-9][0-9,]*(?:\.[0-9]+)?`)
	paymentAmountPattern = regexp.MustCompile(`payment amount:\s*([0-9][0-9,]*(?:\.[0-9]+)?)`)
)

// GetFreeQuotaStats returns how much quota was actually consumed in the
// requested range, grouped by source. Free sources are attributed to grants in
// chronological order (oldest grant first), capped at the granted amount. The
// paid source is the remainder of total consumption after free attribution.
func GetFreeQuotaStats(startTimestamp int64, endTimestamp int64) (FreeQuotaStats, error) {
	stats := FreeQuotaStats{Sources: make([]FreeQuotaSource, 0, len(freeQuotaSourceOrder)+1)}
	if endTimestamp <= 0 {
		endTimestamp = common.GetTimestamp()
	}
	if startTimestamp < 0 {
		startTimestamp = 0
	}

	grants, usernames, err := loadFreeQuotaGrants(endTimestamp)
	if err != nil {
		return stats, err
	}

	rangeConsumption, rangeNames, err := loadRangeQuotaConsumption(startTimestamp, endTimestamp)
	if err != nil {
		return stats, err
	}
	for userId, username := range rangeNames {
		if username != "" {
			usernames[userId] = username
		}
	}

	userIDs := make(map[int]struct{}, len(grants))
	for _, grant := range grants {
		userIDs[grant.UserId] = struct{}{}
	}

	consumption, consumptionNames, err := loadFreeQuotaConsumption(endTimestamp, userIDs)
	if err != nil {
		return stats, err
	}
	for userId, username := range consumptionNames {
		if username != "" {
			usernames[userId] = username
		}
	}
	if err := fillFreeQuotaUsernames(usernames); err != nil {
		return stats, err
	}

	bySource := make(map[string]map[int]*freeQuotaUserAggregate, len(freeQuotaSourceOrder))
	for _, source := range freeQuotaSourceOrder {
		bySource[source] = make(map[int]*freeQuotaUserAggregate)
	}
	sourceGranted := make(map[string]int64, len(freeQuotaSourceOrder))
	sourceGrantCount := make(map[string]int64, len(freeQuotaSourceOrder))
	sourceUsed := make(map[string]int64, len(freeQuotaSourceOrder))
	sourceMethods := make(map[string]map[string]int64, len(freeQuotaSourceOrder))
	for _, source := range freeQuotaSourceOrder {
		sourceMethods[source] = make(map[string]int64)
	}

	inRange := func(timestamp int64) bool {
		return timestamp >= startTimestamp && timestamp <= endTimestamp
	}
	ensureAggregate := func(source string, userId int) *freeQuotaUserAggregate {
		sourceUsers := bySource[source]
		aggregate, ok := sourceUsers[userId]
		if !ok {
			aggregate = &freeQuotaUserAggregate{
				User: FreeQuotaUser{
					UserId:   userId,
					Username: usernames[userId],
				},
				Methods: make(map[string]int64),
			}
			sourceUsers[userId] = aggregate
		}
		return aggregate
	}

	for index := range grants {
		grant := &grants[index]
		if grant.Time > 0 && grant.Time <= endTimestamp {
			aggregate := ensureAggregate(grant.Source, grant.UserId)
			if inRange(grant.Time) {
				aggregate.User.Granted += grant.Quota
				aggregate.User.GrantCount++
				sourceGranted[grant.Source] += grant.Quota
				sourceGrantCount[grant.Source]++
			}
		}
	}

	eventsByUser := make(map[int][]freeQuotaEvent, len(userIDs))
	for index := range grants {
		grant := &grants[index]
		if grant.Time <= 0 {
			continue
		}
		eventsByUser[grant.UserId] = append(eventsByUser[grant.UserId], freeQuotaEvent{
			Time:  grant.Time,
			Order: 0,
			Grant: grant,
		})
	}
	for _, item := range consumption {
		order := 1
		if item.Type == LogTypeRefund {
			order = 2
		}
		eventsByUser[item.UserId] = append(eventsByUser[item.UserId], freeQuotaEvent{
			Time:   item.Time,
			Order:  order,
			Quota:  item.Quota,
			TaskId: item.TaskId,
		})
	}

	for userId, events := range eventsByUser {
		sort.SliceStable(events, func(i, j int) bool {
			if events[i].Time != events[j].Time {
				return events[i].Time < events[j].Time
			}
			return events[i].Order < events[j].Order
		})
		simulateFreeQuotaEvents(userId, events, inRange, ensureAggregate, sourceUsed, sourceMethods)
	}

	// Refunds from earlier periods can make a source negative. Keep the
	// displayed attribution nonnegative and within each user's net consumption,
	// retaining the established source order when offsets cross sources.
	remainingByUser := make(map[int]int64, len(rangeConsumption))
	for _, item := range rangeConsumption {
		remainingByUser[item.UserId] = max(0, item.Quota)
	}
	for _, source := range freeQuotaSourceOrder {
		sourceUsed[source] = 0
		clear(sourceMethods[source])
		for userId, aggregate := range bySource[source] {
			used := min(max(0, aggregate.User.Used), remainingByUser[userId])
			aggregate.User.Used = used
			remainingByUser[userId] -= used
			sourceUsed[source] += used
			for _, method := range []string{"exact", "log_text", "config"} {
				weight := min(max(0, aggregate.Methods[method]), used)
				aggregate.Methods[method] = weight
				sourceMethods[source][method] += weight
				used -= weight
			}
		}
	}

	stats.Sources = make([]FreeQuotaSource, 0, len(freeQuotaSourceOrder))
	for _, source := range freeQuotaSourceOrder {
		users := make([]FreeQuotaUser, 0, len(bySource[source]))
		for _, aggregate := range bySource[source] {
			if aggregate.User.Used <= 0 {
				continue
			}
			aggregate.User.Used = max(0, aggregate.User.Used)
			aggregate.User.Method = combineFreeQuotaMethods(aggregate.Methods)
			users = append(users, aggregate.User)
		}
		sort.Slice(users, func(i, j int) bool {
			if users[i].Used != users[j].Used {
				return users[i].Used > users[j].Used
			}
			return users[i].Username < users[j].Username
		})
		used := max(0, sourceUsed[source])
		method := "exact"
		if used > 0 {
			method = combineFreeQuotaMethods(sourceMethods[source])
		}
		stats.Sources = append(stats.Sources, FreeQuotaSource{
			Key:        source,
			Kind:       "free",
			Quota:      used,
			Granted:    sourceGranted[source],
			Count:      int64(len(users)),
			GrantCount: sourceGrantCount[source],
			Method:     method,
			Users:      users,
		})
		stats.Total += used
		stats.Granted += sourceGranted[source]
	}

	freeUsedByUser := make(map[int]int64, len(userIDs))
	for _, source := range freeQuotaSourceOrder {
		for userId, aggregate := range bySource[source] {
			if aggregate.User.Used > 0 {
				freeUsedByUser[userId] += aggregate.User.Used
			}
		}
	}

	paidUsers := make([]FreeQuotaUser, 0, len(rangeConsumption))
	var paidTotal int64
	for _, item := range rangeConsumption {
		normal := item.Quota - freeUsedByUser[item.UserId]
		if normal <= 0 || item.UserId <= 0 {
			continue
		}
		paidUsers = append(paidUsers, FreeQuotaUser{
			UserId:     item.UserId,
			Username:   firstNonEmpty(item.Username, usernames[item.UserId]),
			Used:       normal,
			LastUsedAt: item.LastUsedAt,
			Method:     "exact",
		})
		paidTotal += normal
	}
	sort.Slice(paidUsers, func(i, j int) bool {
		if paidUsers[i].Used != paidUsers[j].Used {
			return paidUsers[i].Used > paidUsers[j].Used
		}
		return paidUsers[i].Username < paidUsers[j].Username
	})
	stats.Sources = append(stats.Sources, FreeQuotaSource{
		Key:    QuotaSourcePaid,
		Kind:   "paid",
		Quota:  paidTotal,
		Count:  int64(len(paidUsers)),
		Method: "exact",
		Users:  paidUsers,
	})
	stats.FreeTotal = stats.Total
	stats.ConsumptionTotal = stats.Total + paidTotal

	onlineCount, onlineQuota, onlineAmount, onlineRecords, err := loadOnlineTopupStats(startTimestamp, endTimestamp)
	if err != nil {
		return stats, err
	}
	stats.OnlineTopupCount = onlineCount
	stats.OnlineTopupQuota = onlineQuota
	stats.OnlineTopupAmount = onlineAmount
	stats.OnlineTopupRecords = onlineRecords
	return stats, nil
}

type freeQuotaEvent struct {
	Time   int64
	Order  int
	Quota  int64
	Grant  *freeQuotaGrant
	TaskId string
}

type freeQuotaBucket struct {
	Source    string
	Method    string
	Remaining int64
}

type freeQuotaChunk struct {
	// Bucket -1 represents normal wallet quota, which must also be tracked so
	// its refunds cannot restore an unrelated free grant.
	Bucket int
	Time   int64
	Amount int64
	TaskId string
}

func simulateFreeQuotaEvents(
	userId int,
	events []freeQuotaEvent,
	inRange func(int64) bool,
	ensureAggregate func(string, int) *freeQuotaUserAggregate,
	sourceUsed map[string]int64,
	sourceMethods map[string]map[string]int64,
) {
	buckets := make([]freeQuotaBucket, 0, 8)
	chunks := make([]freeQuotaChunk, 0, 16)

	for _, event := range events {
		if event.Grant != nil {
			if event.Grant.Quota <= 0 {
				continue
			}
			buckets = append(buckets, freeQuotaBucket{
				Source:    event.Grant.Source,
				Method:    event.Grant.Method,
				Remaining: event.Grant.Quota,
			})
			continue
		}
		amount := max(0, event.Quota)
		if amount == 0 {
			continue
		}
		if event.Order == 2 {
			refundQuotaChunks(userId, amount, event.Time, event.TaskId, buckets, chunks, inRange, ensureAggregate, sourceUsed, sourceMethods)
			continue
		}
		for index := range buckets {
			if amount == 0 {
				break
			}
			bucket := &buckets[index]
			if bucket.Remaining <= 0 {
				continue
			}
			take := min(bucket.Remaining, amount)
			bucket.Remaining -= take
			chunks = append(chunks, freeQuotaChunk{Bucket: index, Time: event.Time, Amount: take, TaskId: event.TaskId})
			amount -= take
			if !inRange(event.Time) {
				continue
			}
			sourceUsed[bucket.Source] += take
			sourceMethods[bucket.Source][bucket.Method] += take
			aggregate := ensureAggregate(bucket.Source, userId)
			aggregate.User.Used += take
			aggregate.Methods[bucket.Method] += take
			if event.Time > aggregate.User.LastUsedAt {
				aggregate.User.LastUsedAt = event.Time
			}
		}
		if amount > 0 {
			chunks = append(chunks, freeQuotaChunk{Bucket: -1, Time: event.Time, Amount: amount, TaskId: event.TaskId})
		}
	}

	for _, chunk := range chunks {
		if chunk.Bucket < 0 || chunk.Amount <= 0 || !inRange(chunk.Time) {
			continue
		}
		bucket := buckets[chunk.Bucket]
		aggregate := ensureAggregate(bucket.Source, userId)
		if chunk.Time > aggregate.User.LastUsedAt {
			aggregate.User.LastUsedAt = chunk.Time
		}
	}
}

// freeQuotaUserAggregate mirrors the aggregate type declared in
// GetFreeQuotaStats. It is declared at package level so the event simulation
// helper can update it without duplicating the shape.
type freeQuotaUserAggregate struct {
	User    FreeQuotaUser
	Methods map[string]int64
}

func refundQuotaChunks(
	userId int,
	amount int64,
	refundTime int64,
	taskId string,
	buckets []freeQuotaBucket,
	chunks []freeQuotaChunk,
	inRange func(int64) bool,
	ensureAggregate func(string, int) *freeQuotaUserAggregate,
	sourceUsed map[string]int64,
	sourceMethods map[string]map[string]int64,
) {
	for index := len(chunks) - 1; index >= 0 && amount > 0; index-- {
		chunk := &chunks[index]
		if chunk.Amount <= 0 || (taskId != "" && chunk.TaskId != taskId) {
			continue
		}
		take := min(chunk.Amount, amount)
		chunk.Amount -= take
		amount -= take
		if chunk.Bucket < 0 {
			continue
		}
		bucket := &buckets[chunk.Bucket]
		bucket.Remaining += take
		// Attribute refunds to their posting period, matching range consumption.
		if !inRange(refundTime) {
			continue
		}
		sourceUsed[bucket.Source] -= take
		sourceMethods[bucket.Source][bucket.Method] -= take
		aggregate := ensureAggregate(bucket.Source, userId)
		aggregate.User.Used -= take
		aggregate.Methods[bucket.Method] -= take
	}
}

func combineFreeQuotaMethods(weights map[string]int64) string {
	present := make([]string, 0, 3)
	for _, method := range []string{"exact", "log_text", "config"} {
		if weights[method] > 0 {
			present = append(present, method)
		}
	}
	switch len(present) {
	case 0:
		return "exact"
	case 1:
		return present[0]
	default:
		return "mixed"
	}
}

func loadFreeQuotaGrants(endTimestamp int64) ([]freeQuotaGrant, map[int]string, error) {
	usernames := make(map[int]string)
	grants := make([]freeQuotaGrant, 0, 64)

	systemGrants, err := loadSystemGrantEvents(endTimestamp, usernames)
	if err != nil {
		return nil, nil, err
	}
	grants = append(grants, systemGrants...)

	redemptionGrants, err := loadRedemptionGrantEvents(endTimestamp)
	if err != nil {
		return nil, nil, err
	}
	grants = append(grants, redemptionGrants...)

	checkinGrants, err := loadCheckinGrantEvents(endTimestamp)
	if err != nil {
		return nil, nil, err
	}
	grants = append(grants, checkinGrants...)

	adminGrants, err := loadAdminGrantEvents(endTimestamp)
	if err != nil {
		return nil, nil, err
	}
	grants = append(grants, adminGrants...)

	return grants, usernames, nil
}

func loadSystemGrantEvents(endTimestamp int64, usernames map[int]string) ([]freeQuotaGrant, error) {
	var rows []struct {
		UserId    int    `gorm:"column:user_id"`
		Username  string `gorm:"column:username"`
		CreatedAt int64  `gorm:"column:created_at"`
		Content   string `gorm:"column:content"`
		Other     string `gorm:"column:other"`
	}
	err := LOG_DB.Table("logs").
		Select("user_id, username, created_at, content, other").
		Where("type = ? AND created_at <= ? AND (content LIKE ? OR content LIKE ? OR content LIKE ?)",
			LogTypeSystem, endTimestamp,
			"New user sign-up bonus%", "Invitation code bonus%", "Transferred invitation quota%").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	grants := make([]freeQuotaGrant, 0, len(rows))
	for _, row := range rows {
		source, fallback, ok := systemGrantSource(row.Content)
		if !ok {
			continue
		}
		var other struct {
			GrantQuota float64 `json:"grant_quota"`
			GrantType  string  `json:"grant_type"`
		}
		_ = common.UnmarshalJsonStr(row.Other, &other)
		if other.GrantType == quotaGrantInviterTransfer {
			source = FreeQuotaSourceInviter
		} else if isFreeQuotaSource(other.GrantType) {
			source = other.GrantType
		}
		var quota int64
		method := "exact"
		switch {
		case other.GrantQuota > 0:
			quota = int64(other.GrantQuota)
		default:
			if parsed, parsedOK := parseGrantQuota(row.Content); parsedOK {
				quota = parsed
				method = "log_text"
			} else {
				quota = int64(max(0, fallback))
				method = "config"
			}
		}
		if quota <= 0 || row.UserId <= 0 {
			continue
		}
		if row.Username != "" {
			usernames[row.UserId] = row.Username
		}
		grants = append(grants, freeQuotaGrant{
			UserId:   row.UserId,
			Username: row.Username,
			Time:     row.CreatedAt,
			Source:   source,
			Quota:    quota,
			Method:   method,
		})
	}
	return grants, nil
}

func systemGrantSource(content string) (string, int, bool) {
	switch {
	case strings.HasPrefix(content, "New user sign-up bonus"):
		return FreeQuotaSourceNewUser, common.QuotaForNewUser, true
	case strings.HasPrefix(content, "Invitation code bonus"):
		return FreeQuotaSourceInvitee, common.QuotaForInvitee, true
	case strings.HasPrefix(content, "Transferred invitation quota"):
		return FreeQuotaSourceInviter, 0, true
	default:
		return "", 0, false
	}
}

func loadRedemptionGrantEvents(endTimestamp int64) ([]freeQuotaGrant, error) {
	var rows []struct {
		UserId       int   `gorm:"column:user_id"`
		UsedUserId   int   `gorm:"column:used_user_id"`
		Quota        int64 `gorm:"column:quota"`
		RedeemedTime int64 `gorm:"column:redeemed_time"`
	}
	err := DB.Table("redemptions").
		Select("user_id, used_user_id, quota, redeemed_time").
		Where("status = ? AND redeemed_time > 0 AND redeemed_time <= ?", common.RedemptionCodeStatusUsed, endTimestamp).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	grants := make([]freeQuotaGrant, 0, len(rows))
	for _, row := range rows {
		userId := row.UsedUserId
		if userId == 0 {
			userId = row.UserId
		}
		if userId <= 0 || row.Quota <= 0 || row.RedeemedTime <= 0 {
			continue
		}
		grants = append(grants, freeQuotaGrant{
			UserId: userId,
			Time:   row.RedeemedTime,
			Source: FreeQuotaSourceRedemption,
			Quota:  row.Quota,
			Method: "exact",
		})
	}
	return grants, nil
}

func loadCheckinGrantEvents(endTimestamp int64) ([]freeQuotaGrant, error) {
	var rows []struct {
		UserId       int   `gorm:"column:user_id"`
		QuotaAwarded int64 `gorm:"column:quota_awarded"`
		CreatedAt    int64 `gorm:"column:created_at"`
	}
	err := DB.Table("checkins").
		Select("user_id, quota_awarded, created_at").
		Where("created_at <= ?", endTimestamp).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	grants := make([]freeQuotaGrant, 0, len(rows))
	for _, row := range rows {
		if row.UserId <= 0 || row.QuotaAwarded <= 0 || row.CreatedAt <= 0 {
			continue
		}
		grants = append(grants, freeQuotaGrant{
			UserId: row.UserId,
			Time:   row.CreatedAt,
			Source: FreeQuotaSourceCheckin,
			Quota:  row.QuotaAwarded,
			Method: "exact",
		})
	}
	return grants, nil
}

func loadAdminGrantEvents(endTimestamp int64) ([]freeQuotaGrant, error) {
	seen := make(map[string]struct{})
	grants := make([]freeQuotaGrant, 0, 16)

	var audits []struct {
		RequestId string `gorm:"column:request_id"`
		CreatedAt int64  `gorm:"column:created_at"`
		Other     string `gorm:"column:other"`
	}
	err := LOG_DB.Table("audit_logs").
		Select("request_id, created_at, other").
		Where("action IN ? AND created_at <= ?", []string{"user.quota_add", "user.quota_override"}, endTimestamp).
		Find(&audits).Error
	if err != nil {
		return nil, err
	}
	for _, row := range audits {
		userId, username, quota, ok := parseAdminGrantOperation(row.Other)
		if !ok || quota <= 0 || userId <= 0 || row.CreatedAt <= 0 {
			continue
		}
		if row.RequestId != "" {
			if _, exists := seen[row.RequestId]; exists {
				continue
			}
			seen[row.RequestId] = struct{}{}
		}
		grants = append(grants, freeQuotaGrant{
			UserId:   userId,
			Username: username,
			Time:     row.CreatedAt,
			Source:   FreeQuotaSourceAdmin,
			Quota:    quota,
			Method:   "exact",
		})
	}

	var rows []struct {
		UserId    int    `gorm:"column:user_id"`
		Username  string `gorm:"column:username"`
		CreatedAt int64  `gorm:"column:created_at"`
		RequestId string `gorm:"column:request_id"`
		Content   string `gorm:"column:content"`
		Other     string `gorm:"column:other"`
	}
	err = LOG_DB.Table("logs").
		Select("user_id, username, created_at, request_id, content, other").
		Where("type = ? AND created_at <= ? AND (other LIKE ? OR other LIKE ? OR content LIKE ? OR content LIKE ?)",
			LogTypeTopup, endTimestamp,
			"%\"user.quota_add\"%", "%\"user.quota_override\"%",
			"Increased user quota by %", "Overrode user quota from %").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.RequestId != "" {
			if _, exists := seen[row.RequestId]; exists {
				continue
			}
		}
		userId, username, quota, ok := parseAdminGrantOperation(row.Other)
		method := "exact"
		if !ok || quota <= 0 {
			if parsed, parsedOK := parseAdminGrantContent(row.Content); parsedOK && parsed > 0 {
				userId, quota, ok = row.UserId, parsed, true
				method = "log_text"
			}
		}
		if !ok || quota <= 0 || userId <= 0 {
			continue
		}
		if row.RequestId != "" {
			seen[row.RequestId] = struct{}{}
		}
		username = firstNonEmpty(username, row.Username)
		grants = append(grants, freeQuotaGrant{
			UserId:   userId,
			Username: username,
			Time:     row.CreatedAt,
			Source:   FreeQuotaSourceAdmin,
			Quota:    quota,
			Method:   method,
		})
	}
	return grants, nil
}

func parseAdminGrantOperation(other string) (int, string, int64, bool) {
	var payload struct {
		Op *struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		} `json:"op"`
	}
	if err := common.UnmarshalJsonStr(other, &payload); err != nil || payload.Op == nil {
		return 0, "", 0, false
	}
	params := payload.Op.Params
	userId := int(freeQuotaNumber(params["target_user_id"]))
	username, _ := params["target_username"].(string)
	switch payload.Op.Action {
	case "user.quota_add":
		quota := freeQuotaNumber(params["quota"])
		return userId, username, quota, quota > 0
	case "user.quota_override":
		from := freeQuotaNumber(params["from"])
		to := freeQuotaNumber(params["to"])
		quota := to - from
		return userId, username, quota, quota > 0
	default:
		return 0, "", 0, false
	}
}

func loadOnlineTopupStats(startTimestamp int64, endTimestamp int64) (int64, int64, float64, []OnlineTopupRecord, error) {
	var rows []struct {
		UserId    int    `gorm:"column:user_id"`
		Username  string `gorm:"column:username"`
		CreatedAt int64  `gorm:"column:created_at"`
		RequestId string `gorm:"column:request_id"`
		Content   string `gorm:"column:content"`
		Other     string `gorm:"column:other"`
	}
	err := LOG_DB.Table("logs").
		Select("user_id, username, created_at, request_id, content, other").
		Where("type = ? AND created_at >= ? AND created_at <= ? AND content LIKE ?",
			LogTypeTopup, startTimestamp, endTimestamp, "%top-up succeeded, amount added:%").
		Order("created_at desc").
		Find(&rows).Error
	if err != nil {
		return 0, 0, 0, nil, err
	}

	records := make([]OnlineTopupRecord, 0, len(rows))
	var quota int64
	var amount float64
	for _, row := range rows {
		rowQuota, rowAmount := parseOnlineTopupAmount(row.Content, row.Other)
		quota += rowQuota
		amount += rowAmount
		paymentMethod := ""
		var meta struct {
			AdminInfo struct {
				PaymentMethod         string `json:"payment_method"`
				CallbackPaymentMethod string `json:"callback_payment_method"`
			} `json:"admin_info"`
		}
		if err := common.UnmarshalJsonStr(row.Other, &meta); err == nil {
			paymentMethod = firstNonEmpty(meta.AdminInfo.PaymentMethod, meta.AdminInfo.CallbackPaymentMethod)
		}
		records = append(records, OnlineTopupRecord{
			UserId:        row.UserId,
			Username:      row.Username,
			CreatedAt:     row.CreatedAt,
			Quota:         rowQuota,
			Amount:        rowAmount,
			PaymentMethod: paymentMethod,
			RequestId:     row.RequestId,
		})
	}
	return int64(len(records)), quota, amount, records, nil
}

func parseOnlineTopupAmount(content string, other string) (int64, float64) {
	var quota int64
	var amount float64

	var payload struct {
		ContentParts []struct {
			Key    string         `json:"key"`
			Params map[string]any `json:"params"`
		} `json:"content_parts"`
	}
	if err := common.UnmarshalJsonStr(other, &payload); err == nil {
		for _, part := range payload.ContentParts {
			if !strings.Contains(part.Key, "top-up succeeded") || !strings.Contains(part.Key, "amount added") {
				continue
			}
			if value, ok := part.Params["quota"]; ok {
				quota = parseOnlineTopupQuota(value)
			}
			if value, ok := part.Params["amount"]; ok {
				amount = parseOnlineTopupMoney(value)
			}
			if quota > 0 {
				break
			}
		}
	}

	if quota <= 0 {
		if index := strings.Index(content, "amount added:"); index >= 0 {
			rest := content[index+len("amount added:"):]
			if comma := strings.Index(rest, ","); comma >= 0 {
				rest = rest[:comma]
			}
			rest = strings.TrimSpace(rest)
			if parsed, ok := parseGrantQuota("amount added: " + rest); ok {
				quota = parsed
			}
		}
	}
	if amount <= 0 {
		if match := paymentAmountPattern.FindStringSubmatch(content); len(match) > 1 {
			if parsed, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64); err == nil {
				amount = parsed
			}
		}
	}
	return quota, amount
}

func parseOnlineTopupQuota(value any) int64 {
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0
		}
		if parsed, ok := parseGrantQuota("amount added: " + text); ok {
			return parsed
		}
		return 0
	default:
		return freeQuotaNumber(value)
	}
}

func parseOnlineTopupMoney(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(typed), ",", ""), 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func loadRangeQuotaConsumption(startTimestamp int64, endTimestamp int64) ([]rangeQuotaConsumption, map[int]string, error) {
	usernames := make(map[int]string)
	var rows []struct {
		UserId    int    `gorm:"column:user_id"`
		Username  string `gorm:"column:username"`
		CreatedAt int64  `gorm:"column:created_at"`
		Type      int    `gorm:"column:type"`
		Quota     int64  `gorm:"column:quota"`
	}
	err := LOG_DB.Table("logs").
		Select("user_id, username, created_at, type, quota").
		Where("created_at >= ? AND created_at <= ? AND type IN ?", startTimestamp, endTimestamp, []int{LogTypeConsume, LogTypeRefund}).
		Find(&rows).Error
	if err != nil {
		return nil, nil, err
	}

	type aggregate struct {
		Quota      int64
		LastUsedAt int64
		Username   string
	}
	byUser := make(map[int]*aggregate)
	for _, row := range rows {
		item, ok := byUser[row.UserId]
		if !ok {
			item = &aggregate{}
			byUser[row.UserId] = item
		}
		if row.Type == LogTypeRefund {
			item.Quota -= row.Quota
			continue
		}
		item.Quota += row.Quota
		if row.Quota > 0 && row.CreatedAt > item.LastUsedAt {
			item.LastUsedAt = row.CreatedAt
		}
		if row.Username != "" {
			item.Username = row.Username
			usernames[row.UserId] = row.Username
		}
	}

	consumption := make([]rangeQuotaConsumption, 0, len(byUser))
	for userId, item := range byUser {
		if userId <= 0 || item.Quota <= 0 {
			continue
		}
		consumption = append(consumption, rangeQuotaConsumption{
			UserId:     userId,
			Username:   item.Username,
			Quota:      item.Quota,
			LastUsedAt: item.LastUsedAt,
		})
	}
	return consumption, usernames, nil
}

func loadFreeQuotaConsumption(endTimestamp int64, userIDs map[int]struct{}) ([]freeQuotaConsumption, map[int]string, error) {
	usernames := make(map[int]string)
	if len(userIDs) == 0 {
		return nil, usernames, nil
	}
	ids := make([]int, 0, len(userIDs))
	for userId := range userIDs {
		ids = append(ids, userId)
	}
	var rows []struct {
		UserId    int    `gorm:"column:user_id"`
		Username  string `gorm:"column:username"`
		CreatedAt int64  `gorm:"column:created_at"`
		Type      int    `gorm:"column:type"`
		Quota     int64  `gorm:"column:quota"`
		Other     string `gorm:"column:other"`
	}
	err := LOG_DB.Table("logs").
		Select("user_id, username, created_at, type, quota, other").
		Where("user_id IN ? AND created_at <= ? AND type IN ?", ids, endTimestamp, []int{LogTypeConsume, LogTypeRefund}).
		Order("created_at ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, nil, err
	}
	type taskOwner struct {
		UserId int
		TaskId string
	}
	billing := make([]struct {
		Source              string `json:"billing_source"`
		TaskId              string `json:"task_id"`
		WalletQuotaDeducted *int64 `json:"wallet_quota_deducted"`
	}, len(rows))
	sourcesByTask := make(map[taskOwner]string)
	for i, row := range rows {
		_ = common.UnmarshalJsonStr(row.Other, &billing[i])
		if billing[i].TaskId != "" && billing[i].Source != "" {
			sourcesByTask[taskOwner{row.UserId, billing[i].TaskId}] = billing[i].Source
		}
	}
	consumption := make([]freeQuotaConsumption, 0, len(rows))
	for i, row := range rows {
		source := billing[i].Source
		if source == "" && billing[i].TaskId != "" {
			source = sourcesByTask[taskOwner{row.UserId, billing[i].TaskId}]
		}
		quota := row.Quota
		if billing[i].WalletQuotaDeducted != nil {
			quota = *billing[i].WalletQuotaDeducted
		}
		if source == "subscription" || quota <= 0 {
			continue
		}
		if row.Username != "" {
			usernames[row.UserId] = row.Username
		}
		consumption = append(consumption, freeQuotaConsumption{
			UserId:   row.UserId,
			Username: row.Username,
			Time:     row.CreatedAt,
			Type:     row.Type,
			Quota:    quota,
			TaskId:   billing[i].TaskId,
		})
	}
	return consumption, usernames, nil
}

func fillFreeQuotaUsernames(usernames map[int]string) error {
	missing := make([]int, 0)
	for userId, username := range usernames {
		if username == "" {
			missing = append(missing, userId)
		}
	}
	if len(missing) > 0 {
		var rows []struct {
			Id       int    `gorm:"column:id"`
			Username string `gorm:"column:username"`
		}
		if err := DB.Table("users").Select("id, username").Where("id IN ?", missing).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			usernames[row.Id] = row.Username
		}
	}
	for userId, username := range usernames {
		if username == "" {
			usernames[userId] = fmt.Sprintf("#%d", userId)
		}
	}
	return nil
}

func isFreeQuotaSource(source string) bool {
	for _, candidate := range freeQuotaSourceOrder {
		if source == candidate {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// parseGrantQuota converts the amount written in a historical grant log back
// to quota units. It follows the current display settings, so historical logs
// written under a different display configuration are approximations.
func parseGrantQuota(content string) (int64, bool) {
	matches := grantAmountPattern.FindAllString(content, -1)
	if len(matches) == 0 {
		return 0, false
	}
	lastMatch := matches[len(matches)-1]
	amountText := strings.ReplaceAll(lastMatch, ",", "")
	amount, err := strconv.ParseFloat(amountText, 64)
	if err != nil {
		return 0, false
	}
	index := strings.LastIndex(content, lastMatch)
	prefix := strings.TrimSpace(content[:index])
	if prefix == "" {
		return 0, false
	}

	quotaPerUnit := common.QuotaPerUnit
	switch {
	case strings.HasSuffix(prefix, "¥") || strings.HasSuffix(prefix, "￥"):
		rate := operation_setting.USDExchangeRate
		if rate <= 0 {
			rate = 1
		}
		return int64(math.Round(amount / rate * quotaPerUnit)), true
	case strings.HasSuffix(prefix, "＄") || strings.HasSuffix(prefix, "$"):
		return int64(math.Round(amount * quotaPerUnit)), true
	default:
		custom := operation_setting.GetGeneralSetting()
		if custom.CustomCurrencySymbol != "" && strings.HasSuffix(prefix, custom.CustomCurrencySymbol) {
			rate := custom.CustomCurrencyExchangeRate
			if rate <= 0 {
				rate = 1
			}
			return int64(math.Round(amount / rate * quotaPerUnit)), true
		}
	}
	// Tokens display mode stores the raw quota as a plain number.
	return int64(math.Round(amount)), true
}

func parseAdminGrantContent(content string) (int64, bool) {
	if strings.HasPrefix(content, "Increased user quota by ") {
		quota, ok := parseGrantQuota(content)
		return quota, ok
	}
	if strings.HasPrefix(content, "Overrode user quota from ") {
		matches := grantAmountPattern.FindAllString(content, -1)
		if len(matches) < 2 {
			return 0, false
		}
		from, errFrom := strconv.ParseInt(strings.ReplaceAll(matches[len(matches)-2], ",", ""), 10, 64)
		to, errTo := strconv.ParseInt(strings.ReplaceAll(matches[len(matches)-1], ",", ""), 10, 64)
		if errFrom != nil || errTo != nil {
			return 0, false
		}
		return max(0, to-from), true
	}
	return 0, false
}

func freeQuotaNumber(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}
