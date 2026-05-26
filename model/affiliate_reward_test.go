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
	assert.Equal(t, 100, reward.RewardQuota)

	settled, err := SettleAvailableAffiliateRewards(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, 100, settled)
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 100, reloadedInviter.AffQuota)

	require.NoError(t, reloadedInviter.TransferAffQuotaToQuota(100))
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	assert.Equal(t, 100, reloadedInviter.Quota)
	require.NoError(t, DB.First(&reward, reward.Id).Error)
	assert.Equal(t, AffiliateRewardStatusTransferred, reward.Status)
	assert.Equal(t, 100, reward.TransferredQuota)
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

	require.NoError(t, reloadedInviter.TransferAffQuotaToQuota(60))
	reloadedInviter = getAffiliateUserForTest(t, inviter.Id)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	assert.Equal(t, 100, reloadedInviter.Quota)
	require.NoError(t, DB.First(reward, reward.Id).Error)
	assert.Equal(t, AffiliateRewardStatusTransferred, reward.Status)
	assert.Equal(t, 100, reward.TransferredQuota)
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
