package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func configureAffiliateRewardTest(t *testing.T, mutate func(*operation_setting.AffiliateSetting)) {
	t.Helper()

	affiliateSetting := operation_setting.GetAffiliateSetting()
	originalAffiliateSetting := *affiliateSetting
	paymentSetting := operation_setting.GetPaymentSetting()
	originalPaymentSetting := *paymentSetting
	originalQuotaPerUnit := common.QuotaPerUnit

	t.Cleanup(func() {
		*affiliateSetting = originalAffiliateSetting
		*paymentSetting = originalPaymentSetting
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	*affiliateSetting = operation_setting.AffiliateSetting{
		Enabled:                   true,
		AttributionWindowDays:     30,
		MinTopupQuota:             0,
		SettlementDelayDays:       0,
		FirstRewardEnabled:        true,
		FirstRewardRate:           0.1,
		FirstRewardCapQuota:       0,
		RecurringRewardEnabled:    false,
		RecurringRewardRate:       0.02,
		RecurringWindowDays:       90,
		RecurringMaxCount:         0,
		RecurringCapPerInvitee:    0,
		IncludeSubscriptionOrders: true,
	}
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	common.QuotaPerUnit = 100

	if mutate != nil {
		mutate(affiliateSetting)
	}
}

func insertAffiliateUserForTest(t *testing.T, id int, username string, inviterId int, createdAt int64) *User {
	t.Helper()
	user := &User{
		Id:        id,
		Username:  username,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   fmt.Sprintf("AFF%d", id),
		InviterId: inviterId,
		CreatedAt: createdAt,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func insertAffiliateTopUpForTest(t *testing.T, tradeNo string, userId int, amount int64, money float64, provider string) *TopUp {
	t.Helper()
	topup := &TopUp{
		UserId:          userId,
		Amount:          amount,
		Money:           money,
		TradeNo:         tradeNo,
		PaymentMethod:   provider,
		PaymentProvider: provider,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, topup.Insert())
	return topup
}

func markAffiliateTopUpSuccessForTest(t *testing.T, topup *TopUp, completeTime int64) {
	t.Helper()
	topup.Status = common.TopUpStatusSuccess
	topup.CompleteTime = completeTime
	require.NoError(t, DB.Save(topup).Error)
}

func getAffiliateRewardLogCountForTest(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(&AffiliateRewardLog{}).Count(&count).Error)
	return count
}

func getAffiliateUserForTest(t *testing.T, id int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.Where("id = ?", id).First(&user).Error)
	return user
}

func insertAffiliateRewardLogForTest(t *testing.T, reward AffiliateRewardLog) AffiliateRewardLog {
	t.Helper()
	if reward.CreatedAt == 0 {
		reward.CreatedAt = common.GetTimestamp()
	}
	if reward.UpdatedAt == 0 {
		reward.UpdatedAt = reward.CreatedAt
	}
	require.NoError(t, DB.Create(&reward).Error)
	return reward
}

func TestUserInsertBindsInviterAndGrantsRegistrationReward(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, nil)

	originalQuotaForNewUser := common.QuotaForNewUser
	originalQuotaForInvitee := common.QuotaForInvitee
	originalQuotaForInviter := common.QuotaForInviter
	t.Cleanup(func() {
		common.QuotaForNewUser = originalQuotaForNewUser
		common.QuotaForInvitee = originalQuotaForInvitee
		common.QuotaForInviter = originalQuotaForInviter
	})
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 50
	common.QuotaForInviter = 900

	inviter := insertAffiliateUserForTest(t, 1001, "inviter_register", 0, common.GetTimestamp())
	invitee := &User{
		Username:  "invitee_register",
		Password:  "password123",
		Status:    common.UserStatusEnabled,
		Group:     "default",
		InviterId: inviter.Id,
	}
	require.NoError(t, invitee.Insert(inviter.Id))

	reloadedInviter := getAffiliateUserForTest(t, inviter.Id)
	reloadedInvitee := getAffiliateUserForTest(t, invitee.Id)
	assert.Equal(t, inviter.Id, reloadedInvitee.InviterId)
	assert.Equal(t, 50, reloadedInvitee.Quota)
	assert.Equal(t, 900, reloadedInviter.AffQuota)
	assert.Equal(t, 900, reloadedInviter.AffHistoryQuota)
	assert.Equal(t, 1, reloadedInviter.AffCount)
	assert.EqualValues(t, 0, getAffiliateRewardLogCountForTest(t))
}

func TestApplyAffiliateRewardFirstTopUpPendingSettleTransferAndIdempotency(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, func(setting *operation_setting.AffiliateSetting) {
		setting.SettlementDelayDays = 1
	})
	common.QuotaPerUnit = 1

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1101, "inviter_first", 0, now-10*86400)
	invitee := insertAffiliateUserForTest(t, 1102, "invitee_first", inviter.Id, now-10*86400)
	topup := insertAffiliateTopUpForTest(t, "first-topup-order", invitee.Id, 10, 10, PaymentProviderEpay)
	completedAt := now - 2*86400
	markAffiliateTopUpSuccessForTest(t, topup, completedAt)
	event := AffiliateRewardTopUpEvent{
		InviteeId:     invitee.Id,
		TopupId:       topup.Id,
		TradeNo:       topup.TradeNo,
		BasisQuota:    1000,
		BasisMoney:    10,
		TriggerSource: PaymentProviderEpay,
		CompletedAt:   completedAt,
	}

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ApplyAffiliateRewardOnTopUpSuccess(tx, event)
	}))
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ApplyAffiliateRewardOnTopUpSuccess(tx, event)
	}))

	assert.EqualValues(t, 1, getAffiliateRewardLogCountForTest(t))
	reloadedInviter := getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	assert.Equal(t, 100, reloadedInviter.AffHistoryQuota)
	assert.Equal(t, 0, reloadedInviter.AffCount)

	var reward AffiliateRewardLog
	require.NoError(t, DB.Where("reward_key = ?", fmt.Sprintf("first:%d", invitee.Id)).First(&reward).Error)
	assert.Equal(t, AffiliateRewardStatusPending, reward.Status)
	assert.Equal(t, AffiliateRewardTriggerFirstTopup, reward.TriggerType)
	assert.Equal(t, AffiliateRewardSourceTopup, reward.SourceType)
	assert.Equal(t, PaymentProviderEpay, reward.PaymentProvider)
	assert.Equal(t, 100, reward.RewardQuota)
	assert.EqualValues(t, 0, reward.SettledAt)
	assert.EqualValues(t, 0, reward.TransferredAt)

	settled, err := SettleAvailableAffiliateRewards(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 100, settled)
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 100, reloadedInviter.AffQuota)
	require.NoError(t, DB.First(&reward, reward.Id).Error)
	settledAt := reward.SettledAt
	assert.Equal(t, AffiliateRewardStatusAvailable, reward.Status)
	assert.Greater(t, settledAt, int64(0))
	assert.EqualValues(t, 0, reward.TransferredAt)

	require.NoError(t, reloadedInviter.TransferAffQuotaToQuota(100))
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	assert.Equal(t, 100, reloadedInviter.Quota)
	require.NoError(t, DB.First(&reward, reward.Id).Error)
	assert.Equal(t, AffiliateRewardStatusTransferred, reward.Status)
	assert.Equal(t, 100, reward.TransferredQuota)
	assert.Equal(t, settledAt, reward.SettledAt)
	assert.Greater(t, reward.TransferredAt, int64(0))
}

func TestAffiliateRewardWindowsKeepFirstAndRecurringIndependent(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, func(setting *operation_setting.AffiliateSetting) {
		setting.AttributionWindowDays = 7
		setting.FirstRewardEnabled = true
		setting.FirstRewardRate = 0.5
		setting.RecurringRewardEnabled = true
		setting.RecurringRewardRate = 0.1
		setting.RecurringWindowDays = 90
	})

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1151, "inviter_window", 0, now-40*86400)
	invitee := insertAffiliateUserForTest(t, 1152, "invitee_window", inviter.Id, now-40*86400)
	topup := insertAffiliateTopUpForTest(t, "window-first-outside-attribution", invitee.Id, 10, 10, PaymentProviderEpay)
	markAffiliateTopUpSuccessForTest(t, topup, now)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ApplyAffiliateRewardOnTopUpSuccess(tx, AffiliateRewardTopUpEvent{
			InviteeId:     invitee.Id,
			TopupId:       topup.Id,
			TradeNo:       topup.TradeNo,
			BasisQuota:    1000,
			BasisMoney:    topup.Money,
			TriggerSource: PaymentProviderEpay,
			CompletedAt:   topup.CompleteTime,
		})
	}))

	assert.EqualValues(t, 1, getAffiliateRewardLogCountForTest(t))
	var reward AffiliateRewardLog
	require.NoError(t, DB.Where("trade_no = ?", topup.TradeNo).First(&reward).Error)
	assert.Equal(t, AffiliateRewardTriggerRecurringTopup, reward.TriggerType)
	assert.Equal(t, 100, reward.RewardQuota)
	assert.Equal(t, 100, getAffiliateUserForTest(t, inviter.Id).AffQuota)
}

func TestApplyAffiliateRewardRecurringLimitsThresholdCapsAndSelfInvite(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, func(setting *operation_setting.AffiliateSetting) {
		setting.MinTopupQuota = 100
		setting.FirstRewardEnabled = false
		setting.RecurringRewardEnabled = true
		setting.RecurringRewardRate = 0.2
		setting.RecurringWindowDays = 7
		setting.RecurringMaxCount = 2
		setting.RecurringCapPerInvitee = 50
	})

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1201, "inviter_recurring", 0, now-2*86400)
	invitee := insertAffiliateUserForTest(t, 1202, "invitee_recurring", inviter.Id, now-2*86400)

	events := []AffiliateRewardTopUpEvent{
		{InviteeId: invitee.Id, TradeNo: "recurring-below-min", BasisQuota: 99, BasisMoney: 1, CompletedAt: now},
		{InviteeId: invitee.Id, TradeNo: "recurring-one", BasisQuota: 100, BasisMoney: 1, CompletedAt: now},
		{InviteeId: invitee.Id, TradeNo: "recurring-two", BasisQuota: 200, BasisMoney: 2, CompletedAt: now},
		{InviteeId: invitee.Id, TradeNo: "recurring-three", BasisQuota: 1000, BasisMoney: 10, CompletedAt: now},
	}
	for _, event := range events {
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			return ApplyAffiliateRewardOnTopUpSuccess(tx, event)
		}))
	}
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ApplyAffiliateRewardOnTopUpSuccess(tx, events[1])
	}))

	reloadedInviter := getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 50, reloadedInviter.AffQuota)
	assert.Equal(t, 50, reloadedInviter.AffHistoryQuota)
	assert.Equal(t, 0, reloadedInviter.AffCount)
	assert.EqualValues(t, 2, getAffiliateRewardLogCountForTest(t))

	expiredInvitee := insertAffiliateUserForTest(t, 1203, "invitee_expired", inviter.Id, now-10*86400)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ApplyAffiliateRewardOnTopUpSuccess(tx, AffiliateRewardTopUpEvent{
			InviteeId:   expiredInvitee.Id,
			TradeNo:     "recurring-expired",
			BasisQuota:  1000,
			BasisMoney:  10,
			CompletedAt: now,
		})
	}))

	selfInvitee := insertAffiliateUserForTest(t, 1204, "invitee_self", 1204, now)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ApplyAffiliateRewardOnTopUpSuccess(tx, AffiliateRewardTopUpEvent{
			InviteeId:   selfInvitee.Id,
			TradeNo:     "recurring-self",
			BasisQuota:  1000,
			BasisMoney:  10,
			CompletedAt: now,
		})
	}))

	assert.EqualValues(t, 2, getAffiliateRewardLogCountForTest(t))
}

func TestCreateAffiliateRewardDuplicateRewardKeyIsIdempotent(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, nil)

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1231, "inviter_duplicate_key", 0, now)
	invitee := insertAffiliateUserForTest(t, 1232, "invitee_duplicate_key", inviter.Id, now)
	event := AffiliateRewardTopUpEvent{
		InviteeId:     invitee.Id,
		TopupId:       1,
		TradeNo:       "duplicate-key-one",
		BasisQuota:    1000,
		BasisMoney:    10,
		TriggerSource: PaymentProviderEpay,
		CompletedAt:   now,
	}

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return createAffiliateRewardTx(tx, inviter.Id, invitee.Id, event, AffiliateRewardTriggerFirstTopup, "duplicate-reward-key", 0.1, 0)
	}))
	event.TradeNo = "duplicate-key-two"
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return createAffiliateRewardTx(tx, inviter.Id, invitee.Id, event, AffiliateRewardTriggerFirstTopup, "duplicate-reward-key", 0.1, 0)
	}))

	assert.EqualValues(t, 1, getAffiliateRewardLogCountForTest(t))
	reloadedInviter := getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 100, reloadedInviter.AffQuota)
	assert.Equal(t, 100, reloadedInviter.AffHistoryQuota)
}

func TestTransferAffiliateRewardPartiallyConsumesLedger(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, nil)
	common.QuotaPerUnit = 1

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1251, "inviter_partial", 0, now)
	invitee := insertAffiliateUserForTest(t, 1252, "invitee_partial", inviter.Id, now)
	inviter.AffQuota = 100
	inviter.AffHistoryQuota = 100
	require.NoError(t, DB.Save(inviter).Error)

	reward := &AffiliateRewardLog{
		RewardKey:   "partial-transfer",
		InviterId:   inviter.Id,
		InviteeId:   invitee.Id,
		TradeNo:     "partial-transfer-order",
		TriggerType: AffiliateRewardTriggerRecurringTopup,
		BasisQuota:  1000,
		BasisMoney:  10,
		RewardRate:  0.1,
		RewardQuota: 100,
		Status:      AffiliateRewardStatusAvailable,
		EligibleAt:  now,
		SettledAt:   now,
	}
	require.NoError(t, DB.Create(reward).Error)

	reloadedInviter := getAffiliateUserForTest(t, inviter.Id)
	require.NoError(t, reloadedInviter.TransferAffQuotaToQuota(40))
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 60, reloadedInviter.AffQuota)
	assert.Equal(t, 40, reloadedInviter.Quota)
	require.NoError(t, DB.First(reward, reward.Id).Error)
	assert.Equal(t, AffiliateRewardStatusAvailable, reward.Status)
	assert.Equal(t, 40, reward.TransferredQuota)
	assert.Greater(t, reward.TransferredAt, int64(0))
	firstTransferredAt := reward.TransferredAt

	require.NoError(t, reloadedInviter.TransferAffQuotaToQuota(60))
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	assert.Equal(t, 100, reloadedInviter.Quota)
	require.NoError(t, DB.First(reward, reward.Id).Error)
	assert.Equal(t, AffiliateRewardStatusTransferred, reward.Status)
	assert.Equal(t, 100, reward.TransferredQuota)
	assert.GreaterOrEqual(t, reward.TransferredAt, firstTransferredAt)
}

func TestVoidAffiliateRewardReversesLedgerByStatusAndIsIdempotent(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, nil)

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1281, "inviter_void", 0, now)
	invitee := insertAffiliateUserForTest(t, 1282, "invitee_void", inviter.Id, now)
	inviter.AffQuota = 380
	inviter.AffHistoryQuota = 1000
	inviter.Quota = 520
	require.NoError(t, DB.Save(inviter).Error)

	pendingReward := insertAffiliateRewardLogForTest(t, AffiliateRewardLog{
		RewardKey:   "void-pending",
		InviterId:   inviter.Id,
		InviteeId:   invitee.Id,
		TradeNo:     "void-pending-order",
		TriggerType: AffiliateRewardTriggerFirstTopup,
		SourceType:  AffiliateRewardSourceTopup,
		BasisQuota:  1000,
		BasisMoney:  10,
		RewardRate:  0.1,
		RewardQuota: 100,
		Status:      AffiliateRewardStatusPending,
		EligibleAt:  now + 86400,
	})
	availableReward := insertAffiliateRewardLogForTest(t, AffiliateRewardLog{
		RewardKey:   "void-available",
		InviterId:   inviter.Id,
		InviteeId:   invitee.Id,
		TradeNo:     "void-available-order",
		TriggerType: AffiliateRewardTriggerRecurringTopup,
		SourceType:  AffiliateRewardSourceTopup,
		BasisQuota:  1000,
		BasisMoney:  10,
		RewardRate:  0.2,
		RewardQuota: 200,
		Status:      AffiliateRewardStatusAvailable,
		EligibleAt:  now,
		SettledAt:   now,
	})
	partialReward := insertAffiliateRewardLogForTest(t, AffiliateRewardLog{
		RewardKey:        "void-partial",
		InviterId:        inviter.Id,
		InviteeId:        invitee.Id,
		TradeNo:          "void-partial-order",
		TriggerType:      AffiliateRewardTriggerRecurringTopup,
		SourceType:       AffiliateRewardSourceTopup,
		BasisQuota:       1000,
		BasisMoney:       10,
		RewardRate:       0.3,
		RewardQuota:      300,
		TransferredQuota: 120,
		Status:           AffiliateRewardStatusAvailable,
		EligibleAt:       now,
		SettledAt:        now,
		TransferredAt:    now,
	})
	transferredReward := insertAffiliateRewardLogForTest(t, AffiliateRewardLog{
		RewardKey:        "void-transferred",
		InviterId:        inviter.Id,
		InviteeId:        invitee.Id,
		TradeNo:          "void-transferred-order",
		TriggerType:      AffiliateRewardTriggerSubscription,
		SourceType:       AffiliateRewardSourceSubscription,
		BasisQuota:       1000,
		BasisMoney:       10,
		RewardRate:       0.4,
		RewardQuota:      400,
		TransferredQuota: 400,
		Status:           AffiliateRewardStatusTransferred,
		EligibleAt:       now,
		SettledAt:        now,
		TransferredAt:    now,
	})

	reward, err := VoidAffiliateReward(pendingReward.Id, "chargeback pending")
	require.NoError(t, err)
	assert.Equal(t, AffiliateRewardStatusVoided, reward.Status)
	assert.Equal(t, "chargeback pending", reward.VoidReason)
	reloadedInviter := getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 380, reloadedInviter.AffQuota)
	assert.Equal(t, 900, reloadedInviter.AffHistoryQuota)
	assert.Equal(t, 520, reloadedInviter.Quota)

	_, err = VoidAffiliateReward(availableReward.Id, "chargeback available")
	require.NoError(t, err)
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 180, reloadedInviter.AffQuota)
	assert.Equal(t, 700, reloadedInviter.AffHistoryQuota)
	assert.Equal(t, 520, reloadedInviter.Quota)

	_, err = VoidAffiliateReward(partialReward.Id, "chargeback partial")
	require.NoError(t, err)
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	assert.Equal(t, 400, reloadedInviter.AffHistoryQuota)
	assert.Equal(t, 400, reloadedInviter.Quota)

	_, err = VoidAffiliateReward(transferredReward.Id, "chargeback transferred")
	require.NoError(t, err)
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	assert.Equal(t, 0, reloadedInviter.AffHistoryQuota)
	assert.Equal(t, 0, reloadedInviter.Quota)

	_, err = VoidAffiliateReward(transferredReward.Id, "second void ignored")
	require.NoError(t, err)
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	assert.Equal(t, 0, reloadedInviter.AffHistoryQuota)
	assert.Equal(t, 0, reloadedInviter.Quota)
	require.NoError(t, DB.First(&transferredReward, transferredReward.Id).Error)
	assert.Equal(t, "chargeback transferred", transferredReward.VoidReason)
}

func TestVoidTransferredAffiliateRewardAllowsNegativeMainQuota(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, nil)

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1283, "inviter_void_negative", 0, now)
	invitee := insertAffiliateUserForTest(t, 1284, "invitee_void_negative", inviter.Id, now)
	inviter.Quota = 50
	inviter.AffHistoryQuota = 100
	require.NoError(t, DB.Save(inviter).Error)
	reward := insertAffiliateRewardLogForTest(t, AffiliateRewardLog{
		RewardKey:        "void-negative",
		InviterId:        inviter.Id,
		InviteeId:        invitee.Id,
		TradeNo:          "void-negative-order",
		TriggerType:      AffiliateRewardTriggerRecurringTopup,
		SourceType:       AffiliateRewardSourceTopup,
		BasisQuota:       1000,
		BasisMoney:       10,
		RewardRate:       0.1,
		RewardQuota:      100,
		TransferredQuota: 100,
		Status:           AffiliateRewardStatusTransferred,
		EligibleAt:       now,
		SettledAt:        now,
		TransferredAt:    now,
	})

	_, err := VoidAffiliateReward(reward.Id, "chargeback after transfer")
	require.NoError(t, err)

	reloadedInviter := getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, -50, reloadedInviter.Quota)
	assert.Equal(t, 0, reloadedInviter.AffHistoryQuota)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	var logCount int64
	require.NoError(t, DB.Model(&Log{}).Where("user_id = ? AND type = ?", inviter.Id, LogTypeSystem).Count(&logCount).Error)
	assert.EqualValues(t, 1, logCount)
}

func TestAffiliateRewardAdminFiltersAndSummary(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, nil)

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1291, "inviter_filter", 0, now)
	otherInviter := insertAffiliateUserForTest(t, 1292, "other_inviter_filter", 0, now)
	inviteeOne := insertAffiliateUserForTest(t, 1293, "invitee_filter_one", inviter.Id, now)
	inviteeTwo := insertAffiliateUserForTest(t, 1294, "invitee_filter_two", inviter.Id, now)
	inviteeVoided := insertAffiliateUserForTest(t, 1295, "invitee_filter_voided", inviter.Id, now)
	otherInvitee := insertAffiliateUserForTest(t, 1296, "other_invitee_filter", otherInviter.Id, now)

	insertAffiliateRewardLogForTest(t, AffiliateRewardLog{
		RewardKey:       "filter-pending",
		InviterId:       inviter.Id,
		InviteeId:       inviteeOne.Id,
		TradeNo:         "filter-pending-order",
		TriggerType:     AffiliateRewardTriggerFirstTopup,
		SourceType:      AffiliateRewardSourceTopup,
		PaymentProvider: PaymentProviderEpay,
		RewardQuota:     100,
		Status:          AffiliateRewardStatusPending,
		EligibleAt:      now + 86400,
		CreatedAt:       now - 100,
	})
	insertAffiliateRewardLogForTest(t, AffiliateRewardLog{
		RewardKey:        "filter-available",
		InviterId:        inviter.Id,
		InviteeId:        inviteeTwo.Id,
		TradeNo:          "filter-available-order",
		TriggerType:      AffiliateRewardTriggerRecurringTopup,
		SourceType:       AffiliateRewardSourceTopup,
		PaymentProvider:  PaymentProviderStripe,
		RewardQuota:      200,
		TransferredQuota: 50,
		Status:           AffiliateRewardStatusAvailable,
		EligibleAt:       now,
		SettledAt:        now,
		CreatedAt:        now - 90,
	})
	insertAffiliateRewardLogForTest(t, AffiliateRewardLog{
		RewardKey:        "filter-subscription",
		InviterId:        inviter.Id,
		InviteeId:        inviteeTwo.Id,
		TradeNo:          "filter-subscription-order",
		TriggerType:      AffiliateRewardTriggerSubscription,
		SourceType:       AffiliateRewardSourceSubscription,
		PaymentProvider:  PaymentProviderStripe,
		RewardQuota:      300,
		TransferredQuota: 300,
		Status:           AffiliateRewardStatusTransferred,
		EligibleAt:       now,
		SettledAt:        now,
		TransferredAt:    now,
		CreatedAt:        now - 80,
	})
	insertAffiliateRewardLogForTest(t, AffiliateRewardLog{
		RewardKey:        "filter-voided",
		InviterId:        inviter.Id,
		InviteeId:        inviteeVoided.Id,
		TradeNo:          "filter-voided-order",
		TriggerType:      AffiliateRewardTriggerRecurringTopup,
		SourceType:       AffiliateRewardSourceTopup,
		PaymentProvider:  PaymentProviderCreem,
		RewardQuota:      400,
		TransferredQuota: 100,
		Status:           AffiliateRewardStatusVoided,
		EligibleAt:       now,
		SettledAt:        now,
		TransferredAt:    now,
		VoidReason:       "chargeback",
		CreatedAt:        now - 70,
	})
	insertAffiliateRewardLogForTest(t, AffiliateRewardLog{
		RewardKey:       "filter-other",
		InviterId:       otherInviter.Id,
		InviteeId:       otherInvitee.Id,
		TradeNo:         "filter-other-order",
		TriggerType:     AffiliateRewardTriggerRecurringTopup,
		SourceType:      AffiliateRewardSourceTopup,
		PaymentProvider: PaymentProviderEpay,
		RewardQuota:     500,
		Status:          AffiliateRewardStatusAvailable,
		EligibleAt:      now,
		SettledAt:       now,
		CreatedAt:       now - 60,
	})

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	userRewards, total, err := GetUserAffiliateRewards(inviter.Id, pageInfo, AffiliateRewardQuery{})
	require.NoError(t, err)
	assert.EqualValues(t, 4, total)
	require.Len(t, userRewards, 4)
	for _, reward := range userRewards {
		assert.Equal(t, inviter.Id, reward.InviterId)
	}

	stripeRewards, total, err := GetAffiliateRewards(AffiliateRewardQuery{PaymentProvider: PaymentProviderStripe}, pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, stripeRewards, 2)

	subscriptionRewards, total, err := GetAffiliateRewards(AffiliateRewardQuery{SourceType: AffiliateRewardSourceSubscription}, pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, subscriptionRewards, 1)
	assert.Equal(t, AffiliateRewardTriggerSubscription, subscriptionRewards[0].TriggerType)

	keywordRewards, total, err := GetAffiliateRewards(AffiliateRewardQuery{Keyword: fmt.Sprintf("%d", inviteeTwo.Id)}, pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, keywordRewards, 2)

	windowRewards, total, err := GetAffiliateRewards(AffiliateRewardQuery{
		InviterId: inviter.Id,
		StartTime: now - 95,
		EndTime:   now - 75,
	}, pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, windowRewards, 2)

	summary, err := GetAffiliateRewardSummary(AffiliateRewardQuery{InviterId: inviter.Id})
	require.NoError(t, err)
	assert.EqualValues(t, 600, summary.TotalRewardQuota)
	assert.EqualValues(t, 100, summary.PendingRewardQuota)
	assert.EqualValues(t, 150, summary.AvailableRewardQuota)
	assert.EqualValues(t, 350, summary.TransferredRewardQuota)
	assert.EqualValues(t, 400, summary.VoidedRewardQuota)
	assert.EqualValues(t, 2, summary.EffectiveInviteeCount)
	assert.EqualValues(t, 4, summary.TotalRecords)
}

func TestTransferAffiliateRewardAllowsLegacyInviteBalanceRemainder(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, nil)
	common.QuotaPerUnit = 1

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1261, "inviter_legacy_transfer", 0, now)
	invitee := insertAffiliateUserForTest(t, 1262, "invitee_legacy_transfer", inviter.Id, now)
	inviter.AffQuota = 100
	inviter.AffHistoryQuota = 100
	require.NoError(t, DB.Save(inviter).Error)

	reward := &AffiliateRewardLog{
		RewardKey:   "legacy-remainder",
		InviterId:   inviter.Id,
		InviteeId:   invitee.Id,
		TradeNo:     "legacy-remainder-order",
		TriggerType: AffiliateRewardTriggerFirstTopup,
		BasisQuota:  400,
		BasisMoney:  4,
		RewardRate:  0.1,
		RewardQuota: 40,
		Status:      AffiliateRewardStatusAvailable,
		EligibleAt:  now,
		SettledAt:   now,
	}
	require.NoError(t, DB.Create(reward).Error)

	reloadedInviter := getAffiliateUserForTest(t, inviter.Id)
	require.NoError(t, reloadedInviter.TransferAffQuotaToQuota(100))

	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	assert.Equal(t, 100, reloadedInviter.Quota)
	require.NoError(t, DB.First(reward, reward.Id).Error)
	assert.Equal(t, AffiliateRewardStatusTransferred, reward.Status)
	assert.Equal(t, 40, reward.TransferredQuota)
	assert.EqualValues(t, 1, getAffiliateRewardLogCountForTest(t))
}

func TestFirstSuccessfulOrderBelowThresholdConsumesFirstRewardSemantics(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, func(setting *operation_setting.AffiliateSetting) {
		setting.MinTopupQuota = 100
		setting.FirstRewardEnabled = true
		setting.FirstRewardRate = 0.5
		setting.RecurringRewardEnabled = true
		setting.RecurringRewardRate = 0.1
	})

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1271, "inviter_first_semantics", 0, now-86400)
	invitee := insertAffiliateUserForTest(t, 1272, "invitee_first_semantics", inviter.Id, now-86400)
	firstTopup := insertAffiliateTopUpForTest(t, "first-under-threshold", invitee.Id, 1, 1, PaymentProviderEpay)
	secondTopup := insertAffiliateTopUpForTest(t, "second-recurring", invitee.Id, 2, 2, PaymentProviderEpay)
	markAffiliateTopUpSuccessForTest(t, firstTopup, now)
	markAffiliateTopUpSuccessForTest(t, secondTopup, now+1)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ApplyAffiliateRewardOnTopUpSuccess(tx, AffiliateRewardTopUpEvent{
			InviteeId:     invitee.Id,
			TopupId:       firstTopup.Id,
			TradeNo:       firstTopup.TradeNo,
			BasisQuota:    50,
			BasisMoney:    firstTopup.Money,
			TriggerSource: PaymentProviderEpay,
			CompletedAt:   firstTopup.CompleteTime,
		})
	}))
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ApplyAffiliateRewardOnTopUpSuccess(tx, AffiliateRewardTopUpEvent{
			InviteeId:     invitee.Id,
			TopupId:       secondTopup.Id,
			TradeNo:       secondTopup.TradeNo,
			BasisQuota:    200,
			BasisMoney:    secondTopup.Money,
			TriggerSource: PaymentProviderEpay,
			CompletedAt:   secondTopup.CompleteTime,
		})
	}))

	assert.EqualValues(t, 1, getAffiliateRewardLogCountForTest(t))
	var reward AffiliateRewardLog
	require.NoError(t, DB.Where("trade_no = ?", secondTopup.TradeNo).First(&reward).Error)
	assert.Equal(t, AffiliateRewardTriggerRecurringTopup, reward.TriggerType)
	assert.Equal(t, "recurring:"+secondTopup.TradeNo, reward.RewardKey)
	assert.Equal(t, 20, reward.RewardQuota)
	assert.Equal(t, 20, getAffiliateUserForTest(t, inviter.Id).AffQuota)
}

func TestManualCompleteTopUpAppliesAffiliateRewardOnce(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, func(setting *operation_setting.AffiliateSetting) {
		setting.FirstRewardEnabled = false
		setting.RecurringRewardEnabled = true
		setting.RecurringRewardRate = 0.1
	})

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1301, "inviter_admin", 0, now)
	invitee := insertAffiliateUserForTest(t, 1302, "invitee_admin", inviter.Id, now)
	insertAffiliateTopUpForTest(t, "admin-topup-order", invitee.Id, 2, 2, PaymentProviderEpay)

	require.NoError(t, ManualCompleteTopUp("admin-topup-order", "127.0.0.1"))
	require.NoError(t, ManualCompleteTopUp("admin-topup-order", "127.0.0.1"))

	reloadedInvitee := getAffiliateUserForTest(t, invitee.Id)
	reloadedInviter := getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 200, reloadedInvitee.Quota)
	assert.Equal(t, 20, reloadedInviter.AffQuota)
	assert.EqualValues(t, 1, getAffiliateRewardLogCountForTest(t))
	var reward AffiliateRewardLog
	require.NoError(t, DB.Where("trade_no = ?", "admin-topup-order").First(&reward).Error)
	assert.Equal(t, AffiliateRewardSourceTopup, reward.SourceType)
	assert.Equal(t, PaymentProviderEpay, reward.PaymentProvider)
	assert.Equal(t, AffiliateRewardTriggerRecurringTopup, reward.TriggerType)
}

func TestCompleteSubscriptionOrderRespectsAffiliateIncludeSwitch(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, func(setting *operation_setting.AffiliateSetting) {
		setting.FirstRewardEnabled = false
		setting.RecurringRewardEnabled = true
		setting.RecurringRewardRate = 0.1
		setting.IncludeSubscriptionOrders = false
	})

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1401, "inviter_subscription", 0, now)
	inviteeDisabled := insertAffiliateUserForTest(t, 1402, "invitee_sub_disabled", inviter.Id, now)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 1403)
	insertSubscriptionOrderForPaymentGuardTest(t, "subscription-no-reward", inviteeDisabled.Id, plan.Id, PaymentProviderStripe)
	require.NoError(t, CompleteSubscriptionOrder("subscription-no-reward", `{"provider":"stripe"}`, PaymentProviderStripe, ""))
	assert.EqualValues(t, 0, getAffiliateRewardLogCountForTest(t))

	operation_setting.GetAffiliateSetting().IncludeSubscriptionOrders = true
	inviteeEnabled := insertAffiliateUserForTest(t, 1404, "invitee_sub_enabled", inviter.Id, now)
	insertSubscriptionOrderForPaymentGuardTest(t, "subscription-reward", inviteeEnabled.Id, plan.Id, PaymentProviderStripe)
	require.NoError(t, CompleteSubscriptionOrder("subscription-reward", `{"provider":"stripe"}`, PaymentProviderStripe, ""))

	var reward AffiliateRewardLog
	require.NoError(t, DB.Where("trade_no = ?", "subscription-reward").First(&reward).Error)
	assert.Equal(t, AffiliateRewardTriggerSubscription, reward.TriggerType)
	assert.Equal(t, 999, reward.BasisQuota)
	assert.Equal(t, 99, reward.RewardQuota)
}

func TestAffiliateRewardRequiresPaymentCompliance(t *testing.T) {
	truncateTables(t)
	configureAffiliateRewardTest(t, func(setting *operation_setting.AffiliateSetting) {
		setting.FirstRewardRate = 0.1
	})

	paymentSetting := operation_setting.GetPaymentSetting()
	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = ""

	now := common.GetTimestamp()
	inviter := insertAffiliateUserForTest(t, 1501, "inviter_no_compliance", 0, now)
	invitee := insertAffiliateUserForTest(t, 1502, "invitee_no_compliance", inviter.Id, now)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ApplyAffiliateRewardOnTopUpSuccess(tx, AffiliateRewardTopUpEvent{
			InviteeId:   invitee.Id,
			TradeNo:     "no-compliance",
			BasisQuota:  1000,
			BasisMoney:  10,
			CompletedAt: now,
		})
	}))

	assert.EqualValues(t, 0, getAffiliateRewardLogCountForTest(t))
	assert.True(t, operation_setting.WouldAffiliateSettingRequireCompliance("affiliate_setting.enabled", "true"))
}
