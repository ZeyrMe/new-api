package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AffiliateRewardStatusPending     = "pending"
	AffiliateRewardStatusAvailable   = "available"
	AffiliateRewardStatusTransferred = "transferred"
	AffiliateRewardStatusVoided      = "voided"

	AffiliateRewardTriggerFirstTopup     = "first_topup"
	AffiliateRewardTriggerRecurringTopup = "recurring_topup"
	AffiliateRewardTriggerSubscription   = "subscription_order"

	AffiliateRewardSourceTopup        = "topup"
	AffiliateRewardSourceSubscription = "subscription_order"
)

type AffiliateRewardLog struct {
	Id               int     `json:"id"`
	RewardKey        string  `json:"reward_key" gorm:"type:varchar(128);uniqueIndex"`
	InviterId        int     `json:"inviter_id" gorm:"index"`
	InviteeId        int     `json:"invitee_id" gorm:"index"`
	TopupId          int     `json:"topup_id" gorm:"index"`
	TradeNo          string  `json:"trade_no" gorm:"type:varchar(255);index"`
	TriggerType      string  `json:"trigger_type" gorm:"type:varchar(32);index"`
	SourceType       string  `json:"source_type" gorm:"type:varchar(32);index"`
	PaymentProvider  string  `json:"payment_provider" gorm:"type:varchar(50);index"`
	BasisQuota       int     `json:"basis_quota"`
	BasisMoney       float64 `json:"basis_money"`
	RewardRate       float64 `json:"reward_rate"`
	RewardQuota      int     `json:"reward_quota"`
	TransferredQuota int     `json:"transferred_quota" gorm:"default:0"`
	Status           string  `json:"status" gorm:"type:varchar(32);index"`
	EligibleAt       int64   `json:"eligible_at" gorm:"index"`
	SettledAt        int64   `json:"settled_at"`
	TransferredAt    int64   `json:"transferred_at"`
	VoidReason       string  `json:"void_reason" gorm:"type:varchar(255)"`
	CreatedAt        int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt        int64   `json:"updated_at" gorm:"bigint"`
}

func (r *AffiliateRewardLog) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	if r.UpdatedAt == 0 {
		r.UpdatedAt = now
	}
	return nil
}

func (r *AffiliateRewardLog) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

type AffiliateRewardTopUpEvent struct {
	InviteeId           int
	TopupId             int
	TradeNo             string
	BasisQuota          int
	BasisMoney          float64
	TriggerSource       string
	CompletedAt         int64
	IncludeSubscription bool
}

type AffiliateRewardQuery struct {
	InviterId       int
	Keyword         string
	Status          string
	TriggerType     string
	SourceType      string
	PaymentProvider string
	StartTime       int64
	EndTime         int64
}

type AffiliateRewardSummary struct {
	TotalRewardQuota       int64 `json:"total_reward_quota"`
	PendingRewardQuota     int64 `json:"pending_reward_quota"`
	AvailableRewardQuota   int64 `json:"available_reward_quota"`
	TransferredRewardQuota int64 `json:"transferred_reward_quota"`
	VoidedRewardQuota      int64 `json:"voided_reward_quota"`
	EffectiveInviteeCount  int64 `json:"effective_invitee_count"`
	TotalRecords           int64 `json:"total_records"`
}

func ApplyAffiliateRewardOnTopUpSuccess(tx *gorm.DB, event AffiliateRewardTopUpEvent) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	if event.InviteeId <= 0 || event.TradeNo == "" || event.BasisQuota <= 0 {
		return nil
	}
	if event.CompletedAt <= 0 {
		event.CompletedAt = common.GetTimestamp()
	}
	setting := operation_setting.GetAffiliateSetting()
	if setting == nil || !setting.Enabled || !operation_setting.IsPaymentComplianceConfirmed() {
		return nil
	}
	if event.IncludeSubscription && !setting.IncludeSubscriptionOrders {
		return nil
	}

	var invitee User
	if err := tx.Select("id", "inviter_id", "created_at").Where("id = ?", event.InviteeId).First(&invitee).Error; err != nil {
		return err
	}
	if invitee.InviterId == 0 || invitee.InviterId == invitee.Id {
		return nil
	}
	var tradeRewardCount int64
	if err := tx.Model(&AffiliateRewardLog{}).Where("trade_no = ?", event.TradeNo).Count(&tradeRewardCount).Error; err != nil {
		return err
	}
	if tradeRewardCount > 0 {
		return nil
	}

	isFirstSuccessfulOrder, err := isFirstSuccessfulAffiliateOrderTx(tx, event)
	if err != nil {
		return err
	}
	if event.BasisQuota < setting.MinTopupQuota {
		return nil
	}

	firstKey := fmt.Sprintf("first:%d", invitee.Id)
	if isFirstSuccessfulOrder && setting.FirstRewardEnabled && setting.FirstRewardRate > 0 &&
		isWithinAffiliateRewardWindow(invitee.CreatedAt, event.CompletedAt, setting.AttributionWindowDays) {
		var firstRewardCount int64
		if err := tx.Model(&AffiliateRewardLog{}).Where("reward_key = ?", firstKey).Count(&firstRewardCount).Error; err != nil {
			return err
		}
		if firstRewardCount > 0 {
			return nil
		}
		return createAffiliateRewardTx(tx, invitee.InviterId, invitee.Id, event, AffiliateRewardTriggerFirstTopup, firstKey, setting.FirstRewardRate, setting.FirstRewardCapQuota)
	}

	if !setting.RecurringRewardEnabled {
		return nil
	}
	recurringTriggerTypes := []string{
		AffiliateRewardTriggerRecurringTopup,
		AffiliateRewardTriggerSubscription,
	}
	if setting.RecurringWindowDays > 0 && invitee.CreatedAt > 0 &&
		event.CompletedAt > invitee.CreatedAt+int64(setting.RecurringWindowDays)*86400 {
		return nil
	}
	if setting.RecurringMaxCount > 0 {
		var recurringCount int64
		if err := tx.Model(&AffiliateRewardLog{}).
			Where("inviter_id = ? AND invitee_id = ? AND trigger_type IN ? AND status <> ?",
				invitee.InviterId, invitee.Id, recurringTriggerTypes, AffiliateRewardStatusVoided).
			Count(&recurringCount).Error; err != nil {
			return err
		}
		if recurringCount >= int64(setting.RecurringMaxCount) {
			return nil
		}
	}
	capQuota := 0
	if setting.RecurringCapPerInvitee > 0 {
		var used int64
		if err := tx.Model(&AffiliateRewardLog{}).
			Where("inviter_id = ? AND invitee_id = ? AND trigger_type IN ? AND status <> ?",
				invitee.InviterId, invitee.Id, recurringTriggerTypes, AffiliateRewardStatusVoided).
			Select("COALESCE(SUM(reward_quota), 0)").Scan(&used).Error; err != nil {
			return err
		}
		remaining := setting.RecurringCapPerInvitee - int(used)
		if remaining <= 0 {
			return nil
		}
		capQuota = remaining
	}
	rewardKey := fmt.Sprintf("recurring:%s", event.TradeNo)
	triggerType := AffiliateRewardTriggerRecurringTopup
	if event.IncludeSubscription {
		triggerType = AffiliateRewardTriggerSubscription
	}
	return createAffiliateRewardTx(tx, invitee.InviterId, invitee.Id, event, triggerType, rewardKey, setting.RecurringRewardRate, capQuota)
}

func isWithinAffiliateRewardWindow(inviteeCreatedAt int64, completedAt int64, windowDays int) bool {
	return windowDays <= 0 || inviteeCreatedAt <= 0 || completedAt <= inviteeCreatedAt+int64(windowDays)*86400
}

func ApplyAffiliateRewardForTopUpTx(tx *gorm.DB, topUp *TopUp, basisQuota int, triggerSource string, includeSubscription bool) error {
	if topUp == nil {
		return nil
	}
	return ApplyAffiliateRewardOnTopUpSuccess(tx, AffiliateRewardTopUpEvent{
		InviteeId:           topUp.UserId,
		TopupId:             topUp.Id,
		TradeNo:             topUp.TradeNo,
		BasisQuota:          basisQuota,
		BasisMoney:          topUp.Money,
		TriggerSource:       triggerSource,
		CompletedAt:         topUp.CompleteTime,
		IncludeSubscription: includeSubscription,
	})
}

func isFirstSuccessfulAffiliateOrderTx(tx *gorm.DB, event AffiliateRewardTopUpEvent) (bool, error) {
	var first TopUp
	err := tx.Select("id", "trade_no", "complete_time").
		Where("user_id = ? AND status = ? AND money > 0", event.InviteeId, common.TopUpStatusSuccess).
		Order("complete_time asc, id asc").
		First(&first).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return first.TradeNo == event.TradeNo, nil
}

func createAffiliateRewardTx(tx *gorm.DB, inviterId int, inviteeId int, event AffiliateRewardTopUpEvent, triggerType string, rewardKey string, rate float64, capQuota int) error {
	if rate <= 0 {
		return nil
	}
	rewardQuota := int(decimal.NewFromInt(int64(event.BasisQuota)).
		Mul(decimal.NewFromFloat(rate)).
		Floor().
		IntPart())
	if capQuota > 0 && rewardQuota > capQuota {
		rewardQuota = capQuota
	}
	if rewardQuota <= 0 {
		return nil
	}

	setting := operation_setting.GetAffiliateSetting()
	delayDays := 0
	if setting != nil && setting.SettlementDelayDays > 0 {
		delayDays = setting.SettlementDelayDays
	}
	status := AffiliateRewardStatusAvailable
	eligibleAt := event.CompletedAt
	settledAt := event.CompletedAt
	if delayDays > 0 {
		status = AffiliateRewardStatusPending
		eligibleAt = event.CompletedAt + int64(delayDays)*86400
		settledAt = 0
	}
	sourceType := AffiliateRewardSourceTopup
	if event.IncludeSubscription {
		sourceType = AffiliateRewardSourceSubscription
	}

	log := &AffiliateRewardLog{
		RewardKey:       rewardKey,
		InviterId:       inviterId,
		InviteeId:       inviteeId,
		TopupId:         event.TopupId,
		TradeNo:         event.TradeNo,
		TriggerType:     triggerType,
		SourceType:      sourceType,
		PaymentProvider: event.TriggerSource,
		BasisQuota:      event.BasisQuota,
		BasisMoney:      event.BasisMoney,
		RewardRate:      rate,
		RewardQuota:     rewardQuota,
		Status:          status,
		EligibleAt:      eligibleAt,
		SettledAt:       settledAt,
		CreatedAt:       event.CompletedAt,
		UpdatedAt:       event.CompletedAt,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "reward_key"}},
		DoNothing: true,
	}).Create(log)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}

	updates := map[string]interface{}{
		"aff_history": gorm.Expr("aff_history + ?", rewardQuota),
	}
	if status == AffiliateRewardStatusAvailable {
		updates["aff_quota"] = gorm.Expr("aff_quota + ?", rewardQuota)
	}
	if err := tx.Model(&User{}).Where("id = ?", inviterId).Updates(updates).Error; err != nil {
		return err
	}
	common.SysLog(fmt.Sprintf("affiliate reward created inviter=%d invitee=%d trade_no=%s reward=%s status=%s",
		inviterId, inviteeId, event.TradeNo, logger.FormatQuota(rewardQuota), status))
	return nil
}

func SettleAvailableAffiliateRewards(userId int) (int, error) {
	if userId <= 0 {
		return 0, nil
	}
	now := common.GetTimestamp()
	total := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var rewards []AffiliateRewardLog
		if err := tx.Where("inviter_id = ? AND status = ? AND eligible_at <= ?", userId, AffiliateRewardStatusPending, now).
			Order("id asc").
			Find(&rewards).Error; err != nil {
			return err
		}
		for _, reward := range rewards {
			result := tx.Model(&AffiliateRewardLog{}).
				Where("id = ? AND status = ?", reward.Id, AffiliateRewardStatusPending).
				Updates(map[string]interface{}{
					"status":     AffiliateRewardStatusAvailable,
					"settled_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				total += reward.RewardQuota
			}
		}
		if total > 0 {
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("aff_quota", gorm.Expr("aff_quota + ?", total)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return total, err
}

func SettleAllAvailableAffiliateRewards() (int, error) {
	now := common.GetTimestamp()
	total := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var rewards []AffiliateRewardLog
		if err := tx.Where("status = ? AND eligible_at <= ?", AffiliateRewardStatusPending, now).
			Order("inviter_id asc, id asc").
			Find(&rewards).Error; err != nil {
			return err
		}
		quotaByInviter := make(map[int]int)
		for _, reward := range rewards {
			result := tx.Model(&AffiliateRewardLog{}).
				Where("id = ? AND status = ?", reward.Id, AffiliateRewardStatusPending).
				Updates(map[string]interface{}{
					"status":     AffiliateRewardStatusAvailable,
					"settled_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				total += reward.RewardQuota
				quotaByInviter[reward.InviterId] += reward.RewardQuota
			}
		}
		for inviterId, quota := range quotaByInviter {
			if quota <= 0 {
				continue
			}
			if err := tx.Model(&User{}).Where("id = ?", inviterId).
				Update("aff_quota", gorm.Expr("aff_quota + ?", quota)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return total, err
}

func GetPendingAffiliateRewardQuota(userId int) (int, error) {
	var total int64
	err := DB.Model(&AffiliateRewardLog{}).
		Where("inviter_id = ? AND status = ?", userId, AffiliateRewardStatusPending).
		Select("COALESCE(SUM(reward_quota), 0)").Scan(&total).Error
	return int(total), err
}

func GetEffectiveAffiliateInviteeCount(userId int) (int64, error) {
	if userId <= 0 {
		return 0, nil
	}
	var count int64
	err := DB.Model(&AffiliateRewardLog{}).
		Where("inviter_id = ? AND status <> ?", userId, AffiliateRewardStatusVoided).
		Distinct("invitee_id").
		Count(&count).Error
	return count, err
}

func escapeAffiliateLike(keyword string) string {
	keyword = strings.ReplaceAll(keyword, "!", "!!")
	keyword = strings.ReplaceAll(keyword, "%", "!%")
	keyword = strings.ReplaceAll(keyword, "_", "!_")
	return "%" + keyword + "%"
}

func applyAffiliateRewardQuery(query *gorm.DB, filter AffiliateRewardQuery) *gorm.DB {
	if filter.InviterId > 0 {
		query = query.Where("inviter_id = ?", filter.InviterId)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		pattern := escapeAffiliateLike(keyword)
		if numericValue, err := strconv.Atoi(keyword); err == nil {
			query = query.Where(
				"(reward_key LIKE ? ESCAPE '!' OR trade_no LIKE ? ESCAPE '!' OR void_reason LIKE ? ESCAPE '!' OR id = ? OR inviter_id = ? OR invitee_id = ? OR topup_id = ?)",
				pattern, pattern, pattern, numericValue, numericValue, numericValue, numericValue,
			)
		} else {
			query = query.Where(
				"(reward_key LIKE ? ESCAPE '!' OR trade_no LIKE ? ESCAPE '!' OR void_reason LIKE ? ESCAPE '!')",
				pattern, pattern, pattern,
			)
		}
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if triggerType := strings.TrimSpace(filter.TriggerType); triggerType != "" {
		query = query.Where("trigger_type = ?", triggerType)
	}
	if sourceType := strings.TrimSpace(filter.SourceType); sourceType != "" {
		query = query.Where("source_type = ?", sourceType)
	}
	if paymentProvider := strings.TrimSpace(filter.PaymentProvider); paymentProvider != "" {
		query = query.Where("payment_provider = ?", paymentProvider)
	}
	if filter.StartTime > 0 {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("created_at <= ?", filter.EndTime)
	}
	return query
}

func GetAffiliateRewards(filter AffiliateRewardQuery, pageInfo *common.PageInfo) ([]*AffiliateRewardLog, int64, error) {
	var rewards []*AffiliateRewardLog
	var total int64
	query := applyAffiliateRewardQuery(DB.Model(&AffiliateRewardLog{}), filter)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&rewards).Error; err != nil {
		return nil, 0, err
	}
	return rewards, total, nil
}

func GetAffiliateRewardSummary(filter AffiliateRewardQuery) (*AffiliateRewardSummary, error) {
	query := applyAffiliateRewardQuery(DB.Model(&AffiliateRewardLog{}), filter)
	var summary AffiliateRewardSummary
	err := query.Select(
		`COALESCE(SUM(CASE WHEN status <> ? THEN reward_quota ELSE 0 END), 0) AS total_reward_quota,
		COALESCE(SUM(CASE WHEN status = ? THEN reward_quota ELSE 0 END), 0) AS pending_reward_quota,
		COALESCE(SUM(CASE WHEN status = ? THEN reward_quota - transferred_quota ELSE 0 END), 0) AS available_reward_quota,
		COALESCE(SUM(CASE WHEN status <> ? THEN transferred_quota ELSE 0 END), 0) AS transferred_reward_quota,
		COALESCE(SUM(CASE WHEN status = ? THEN reward_quota ELSE 0 END), 0) AS voided_reward_quota,
		COUNT(*) AS total_records`,
		AffiliateRewardStatusVoided,
		AffiliateRewardStatusPending,
		AffiliateRewardStatusAvailable,
		AffiliateRewardStatusVoided,
		AffiliateRewardStatusVoided,
	).Scan(&summary).Error
	if err != nil {
		return nil, err
	}
	countQuery := applyAffiliateRewardQuery(DB.Model(&AffiliateRewardLog{}), filter).
		Where("status <> ?", AffiliateRewardStatusVoided).
		Distinct("invitee_id")
	if err := countQuery.Count(&summary.EffectiveInviteeCount).Error; err != nil {
		return nil, err
	}
	return &summary, nil
}

func GetUserAffiliateRewards(userId int, pageInfo *common.PageInfo, filter AffiliateRewardQuery) ([]*AffiliateRewardLog, int64, error) {
	filter.InviterId = userId
	return GetAffiliateRewards(filter, pageInfo)
}

func VoidAffiliateReward(rewardId int, reason string) (*AffiliateRewardLog, error) {
	if rewardId <= 0 {
		return nil, errors.New("invalid reward id")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("void reason is required")
	}
	if len(reason) > 255 {
		return nil, errors.New("void reason is too long")
	}
	var reward AffiliateRewardLog
	voidedNow := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", rewardId).First(&reward).Error; err != nil {
			return err
		}
		if reward.Status == AffiliateRewardStatusVoided {
			return nil
		}

		untransferredQuota := reward.RewardQuota - reward.TransferredQuota
		if untransferredQuota < 0 {
			untransferredQuota = 0
		}
		userUpdates := map[string]interface{}{
			"aff_history": gorm.Expr("aff_history - ?", reward.RewardQuota),
		}
		if reward.Status == AffiliateRewardStatusAvailable && untransferredQuota > 0 {
			userUpdates["aff_quota"] = gorm.Expr("aff_quota - ?", untransferredQuota)
		}
		if reward.TransferredQuota > 0 {
			userUpdates["quota"] = gorm.Expr("quota - ?", reward.TransferredQuota)
		}
		if err := tx.Model(&User{}).Where("id = ?", reward.InviterId).Updates(userUpdates).Error; err != nil {
			return err
		}
		if err := tx.Model(&AffiliateRewardLog{}).Where("id = ?", reward.Id).Updates(map[string]interface{}{
			"status":      AffiliateRewardStatusVoided,
			"void_reason": reason,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", rewardId).First(&reward).Error; err != nil {
			return err
		}
		voidedNow = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if voidedNow {
		common.SysLog(fmt.Sprintf("affiliate reward voided reward_id=%d inviter=%d invitee=%d reward=%s transferred=%s reason=%q status=%s",
			reward.Id, reward.InviterId, reward.InviteeId, logger.FormatQuota(reward.RewardQuota),
			logger.FormatQuota(reward.TransferredQuota), reason, reward.Status))
		RecordLog(reward.InviterId, LogTypeSystem, fmt.Sprintf("邀请返利已作废，奖励ID：%d，返利：%s，已转出：%s，原因：%s",
			reward.Id, logger.LogQuota(reward.RewardQuota), logger.LogQuota(reward.TransferredQuota), reason))
	}
	return &reward, nil
}

func MarkAffiliateRewardsTransferredTx(tx *gorm.DB, userId int, quota int) error {
	if tx == nil || userId <= 0 || quota <= 0 {
		return nil
	}
	remaining := quota
	var rewards []AffiliateRewardLog
	if err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("inviter_id = ? AND status = ? AND reward_quota > transferred_quota", userId, AffiliateRewardStatusAvailable).
		Order("eligible_at asc, id asc").
		Find(&rewards).Error; err != nil {
		return err
	}
	now := common.GetTimestamp()
	for _, reward := range rewards {
		availableQuota := reward.RewardQuota - reward.TransferredQuota
		if availableQuota <= 0 {
			continue
		}
		consumeQuota := availableQuota
		if remaining < consumeQuota {
			consumeQuota = remaining
		}
		newTransferredQuota := reward.TransferredQuota + consumeQuota
		updates := map[string]interface{}{
			"transferred_quota": newTransferredQuota,
			"transferred_at":    now,
		}
		if newTransferredQuota >= reward.RewardQuota {
			updates["status"] = AffiliateRewardStatusTransferred
		}
		result := tx.Model(&AffiliateRewardLog{}).
			Where("id = ? AND status = ? AND transferred_quota = ?", reward.Id, AffiliateRewardStatusAvailable, reward.TransferredQuota).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			remaining -= consumeQuota
		}
		if remaining <= 0 {
			break
		}
	}
	return nil
}
