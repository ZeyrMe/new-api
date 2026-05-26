package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	AffiliateRewardStatusPending     = "pending"
	AffiliateRewardStatusAvailable   = "available"
	AffiliateRewardStatusTransferred = "transferred"
	AffiliateRewardStatusVoided      = "voided"

	AffiliateRewardTriggerFirstTopup     = "first_topup"
	AffiliateRewardTriggerRecurringTopup = "recurring_topup"
	AffiliateRewardTriggerSubscription   = "subscription_order"
)

type AffiliateRewardLog struct {
	Id          int     `json:"id"`
	RewardKey   string  `json:"reward_key" gorm:"type:varchar(128);uniqueIndex"`
	InviterId   int     `json:"inviter_id" gorm:"index"`
	InviteeId   int     `json:"invitee_id" gorm:"index"`
	TopupId     int     `json:"topup_id" gorm:"index"`
	TradeNo     string  `json:"trade_no" gorm:"type:varchar(255);index"`
	TriggerType string  `json:"trigger_type" gorm:"type:varchar(32);index"`
	BasisQuota  int     `json:"basis_quota"`
	BasisMoney  float64 `json:"basis_money"`
	RewardRate  float64 `json:"reward_rate"`
	RewardQuota int     `json:"reward_quota"`
	Status      string  `json:"status" gorm:"type:varchar(32);index"`
	EligibleAt  int64   `json:"eligible_at" gorm:"index"`
	SettledAt   int64   `json:"settled_at"`
	VoidReason  string  `json:"void_reason" gorm:"type:varchar(255)"`
	CreatedAt   int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt   int64   `json:"updated_at" gorm:"bigint"`
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
	if event.BasisQuota < setting.MinTopupQuota {
		return nil
	}

	var invitee User
	if err := tx.Select("id", "inviter_id", "created_at").Where("id = ?", event.InviteeId).First(&invitee).Error; err != nil {
		return err
	}
	if invitee.InviterId == 0 || invitee.InviterId == invitee.Id {
		return nil
	}
	if setting.AttributionWindowDays > 0 && invitee.CreatedAt > 0 &&
		event.CompletedAt > invitee.CreatedAt+int64(setting.AttributionWindowDays)*86400 {
		return nil
	}
	var tradeRewardCount int64
	if err := tx.Model(&AffiliateRewardLog{}).Where("trade_no = ?", event.TradeNo).Count(&tradeRewardCount).Error; err != nil {
		return err
	}
	if tradeRewardCount > 0 {
		return nil
	}

	firstKey := fmt.Sprintf("first:%d", invitee.Id)
	var firstCount int64
	if err := tx.Model(&AffiliateRewardLog{}).Where("reward_key = ?", firstKey).Count(&firstCount).Error; err != nil {
		return err
	}
	if firstCount == 0 && setting.FirstRewardEnabled && setting.FirstRewardRate > 0 {
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

	var existingCount int64
	if err := tx.Model(&AffiliateRewardLog{}).
		Where("inviter_id = ? AND invitee_id = ? AND status <> ?", inviterId, inviteeId, AffiliateRewardStatusVoided).
		Count(&existingCount).Error; err != nil {
		return err
	}

	log := &AffiliateRewardLog{
		RewardKey:   rewardKey,
		InviterId:   inviterId,
		InviteeId:   inviteeId,
		TopupId:     event.TopupId,
		TradeNo:     event.TradeNo,
		TriggerType: triggerType,
		BasisQuota:  event.BasisQuota,
		BasisMoney:  event.BasisMoney,
		RewardRate:  rate,
		RewardQuota: rewardQuota,
		Status:      status,
		EligibleAt:  eligibleAt,
		SettledAt:   settledAt,
		CreatedAt:   event.CompletedAt,
		UpdatedAt:   event.CompletedAt,
	}
	if err := tx.Create(log).Error; err != nil {
		return err
	}

	updates := map[string]interface{}{
		"aff_history": gorm.Expr("aff_history + ?", rewardQuota),
	}
	if status == AffiliateRewardStatusAvailable {
		updates["aff_quota"] = gorm.Expr("aff_quota + ?", rewardQuota)
	}
	if existingCount == 0 {
		updates["aff_count"] = gorm.Expr("aff_count + ?", 1)
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

func GetPendingAffiliateRewardQuota(userId int) (int, error) {
	var total int64
	err := DB.Model(&AffiliateRewardLog{}).
		Where("inviter_id = ? AND status = ?", userId, AffiliateRewardStatusPending).
		Select("COALESCE(SUM(reward_quota), 0)").Scan(&total).Error
	return int(total), err
}

func GetUserAffiliateRewards(userId int, pageInfo *common.PageInfo) ([]*AffiliateRewardLog, int64, error) {
	var rewards []*AffiliateRewardLog
	var total int64
	query := DB.Model(&AffiliateRewardLog{}).Where("inviter_id = ?", userId)
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

func MarkAffiliateRewardsTransferredTx(tx *gorm.DB, userId int, quota int) error {
	if tx == nil || userId <= 0 || quota <= 0 {
		return nil
	}
	remaining := quota
	var rewards []AffiliateRewardLog
	if err := tx.Where("inviter_id = ? AND status = ?", userId, AffiliateRewardStatusAvailable).
		Order("eligible_at asc, id asc").
		Find(&rewards).Error; err != nil {
		return err
	}
	now := common.GetTimestamp()
	for _, reward := range rewards {
		if remaining < reward.RewardQuota {
			break
		}
		result := tx.Model(&AffiliateRewardLog{}).
			Where("id = ? AND status = ?", reward.Id, AffiliateRewardStatusAvailable).
			Updates(map[string]interface{}{
				"status":     AffiliateRewardStatusTransferred,
				"settled_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			remaining -= reward.RewardQuota
		}
		if remaining <= 0 {
			break
		}
	}
	return nil
}
