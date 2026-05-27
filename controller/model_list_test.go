package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type listModelsResponse struct {
	Success bool               `json:"success"`
	Data    []dto.OpenAIModels `json:"data"`
	Object  string             `json:"object"`
}

type controllerDBTestState struct {
	db              *gorm.DB
	logDB           *gorm.DB
	usingSQLite     bool
	usingMySQL      bool
	usingPostgreSQL bool
	redisEnabled    bool
}

func saveControllerDBTestState() controllerDBTestState {
	return controllerDBTestState{
		db:              model.DB,
		logDB:           model.LOG_DB,
		usingSQLite:     common.UsingSQLite,
		usingMySQL:      common.UsingMySQL,
		usingPostgreSQL: common.UsingPostgreSQL,
		redisEnabled:    common.RedisEnabled,
	}
}

func (state controllerDBTestState) restore() {
	model.DB = state.db
	model.LOG_DB = state.logDB
	common.UsingSQLite = state.usingSQLite
	common.UsingMySQL = state.usingMySQL
	common.UsingPostgreSQL = state.usingPostgreSQL
	common.RedisEnabled = state.redisEnabled
}

func closeControllerTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}

func setupModelListControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalState := saveControllerDBTestState()
	initModelListColumnNames(t)

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}, &model.AffiliateRewardLog{}, &model.Log{}))
	model.InvalidatePricingCache()

	t.Cleanup(func() {
		model.InvalidatePricingCache()
		originalState.restore()
		closeControllerTestDB(t, db)
	})

	return db
}

func initModelListColumnNames(t *testing.T) {
	t.Helper()

	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalState := saveControllerDBTestState()
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	defer func() {
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		originalState.restore()
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	}()

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s_init?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	require.NoError(t, os.Setenv("SQL_DSN", "local"))

	require.NoError(t, model.InitDB())
	if model.DB != nil {
		closeControllerTestDB(t, model.DB)
	}
}

func withTieredBillingConfig(t *testing.T, modes map[string]string, exprs map[string]string) {
	t.Helper()

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		if strings.HasPrefix(key, "billing_setting.") {
			saved[key] = value
		}
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		model.InvalidatePricingCache()
	})

	modeBytes, err := common.Marshal(modes)
	require.NoError(t, err)
	exprBytes, err := common.Marshal(exprs)
	require.NoError(t, err)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": string(modeBytes),
		"billing_setting.billing_expr": string(exprBytes),
	}))
	model.InvalidatePricingCache()
}

func withSelfUseModeDisabled(t *testing.T) {
	t.Helper()

	original := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = false
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = original
	})
}

func decodeListModelsResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]struct{} {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload listModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "list", payload.Object)

	ids := make(map[string]struct{}, len(payload.Data))
	for _, item := range payload.Data {
		ids[item.Id] = struct{}{}
	}
	return ids
}

func pricingByModelName(pricings []model.Pricing) map[string]model.Pricing {
	byName := make(map[string]model.Pricing, len(pricings))
	for _, pricing := range pricings {
		byName[pricing.ModelName] = pricing
	}
	return byName
}

type adminAffiliateRewardsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Page     int                          `json:"page"`
		PageSize int                          `json:"page_size"`
		Total    int                          `json:"total"`
		Items    []model.AffiliateRewardLog   `json:"items"`
		Summary  model.AffiliateRewardSummary `json:"summary"`
	} `json:"data"`
}

func insertControllerAffiliateUser(t *testing.T, db *gorm.DB, id int, username string, inviterId int) model.User {
	t.Helper()
	user := model.User{
		Id:        id,
		Username:  username,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   fmt.Sprintf("AFF%d", id),
		InviterId: inviterId,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func insertControllerAffiliateReward(t *testing.T, db *gorm.DB, reward model.AffiliateRewardLog) model.AffiliateRewardLog {
	t.Helper()
	if reward.CreatedAt == 0 {
		reward.CreatedAt = common.GetTimestamp()
	}
	if reward.UpdatedAt == 0 {
		reward.UpdatedAt = reward.CreatedAt
	}
	require.NoError(t, db.Create(&reward).Error)
	return reward
}

func TestGetAdminAffiliateRewardsFiltersAndSummarizes(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	now := common.GetTimestamp()
	inviter := insertControllerAffiliateUser(t, db, 9101, "admin_aff_inviter", 0)
	invitee := insertControllerAffiliateUser(t, db, 9102, "admin_aff_invitee", inviter.Id)
	otherInvitee := insertControllerAffiliateUser(t, db, 9103, "admin_aff_other_invitee", inviter.Id)

	insertControllerAffiliateReward(t, db, model.AffiliateRewardLog{
		RewardKey:        "controller-admin-stripe",
		InviterId:        inviter.Id,
		InviteeId:        invitee.Id,
		TradeNo:          "controller-admin-stripe-order",
		TriggerType:      model.AffiliateRewardTriggerSubscription,
		SourceType:       model.AffiliateRewardSourceSubscription,
		PaymentProvider:  model.PaymentProviderStripe,
		RewardQuota:      300,
		Status:           model.AffiliateRewardStatusTransferred,
		TransferredQuota: 300,
		CreatedAt:        now - 10,
	})
	insertControllerAffiliateReward(t, db, model.AffiliateRewardLog{
		RewardKey:       "controller-admin-epay",
		InviterId:       inviter.Id,
		InviteeId:       otherInvitee.Id,
		TradeNo:         "controller-admin-epay-order",
		TriggerType:     model.AffiliateRewardTriggerFirstTopup,
		SourceType:      model.AffiliateRewardSourceTopup,
		PaymentProvider: model.PaymentProviderEpay,
		RewardQuota:     100,
		Status:          model.AffiliateRewardStatusPending,
		CreatedAt:       now - 5,
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/user/aff_rewards/admin?p=1&page_size=10&payment_provider=stripe", nil)

	GetAdminAffiliateRewards(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload adminAffiliateRewardsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, 1, payload.Data.Page)
	require.Equal(t, 10, payload.Data.PageSize)
	require.Equal(t, 1, payload.Data.Total)
	require.Len(t, payload.Data.Items, 1)
	assert.Equal(t, model.PaymentProviderStripe, payload.Data.Items[0].PaymentProvider)
	assert.EqualValues(t, 300, payload.Data.Summary.TotalRewardQuota)
	assert.EqualValues(t, 300, payload.Data.Summary.TransferredRewardQuota)
	assert.EqualValues(t, 1, payload.Data.Summary.EffectiveInviteeCount)
}

func TestGetAdminAffiliateRewardsRejectsInvalidFilter(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		fieldName string
	}{
		{name: "status", query: "status=unknown", fieldName: "status"},
		{name: "trigger type", query: "trigger_type=unknown", fieldName: "trigger_type"},
		{name: "source type", query: "source_type=unknown", fieldName: "source_type"},
		{name: "payment provider", query: "payment_provider=unknown", fieldName: "payment_provider"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupModelListControllerTestDB(t)

			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/user/aff_rewards/admin?"+tt.query, nil)

			GetAdminAffiliateRewards(context)

			require.Equal(t, http.StatusOK, recorder.Code)
			var payload struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
			assert.False(t, payload.Success)
			assert.Contains(t, payload.Message, tt.fieldName)
		})
	}
}

func TestGetAdminAffiliateRewardsSettlesDuePendingRewards(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	now := common.GetTimestamp()
	inviter := insertControllerAffiliateUser(t, db, 9104, "admin_aff_settle_inviter", 0)
	invitee := insertControllerAffiliateUser(t, db, 9105, "admin_aff_settle_invitee", inviter.Id)
	insertControllerAffiliateReward(t, db, model.AffiliateRewardLog{
		RewardKey:       "controller-admin-due-pending",
		InviterId:       inviter.Id,
		InviteeId:       invitee.Id,
		TradeNo:         "controller-admin-due-pending-order",
		TriggerType:     model.AffiliateRewardTriggerFirstTopup,
		SourceType:      model.AffiliateRewardSourceTopup,
		PaymentProvider: model.PaymentProviderEpay,
		RewardQuota:     120,
		Status:          model.AffiliateRewardStatusPending,
		EligibleAt:      now - 1,
		CreatedAt:       now - 10,
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/user/aff_rewards/admin?p=1&page_size=10", nil)

	GetAdminAffiliateRewards(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload adminAffiliateRewardsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Len(t, payload.Data.Items, 1)
	assert.Equal(t, model.AffiliateRewardStatusAvailable, payload.Data.Items[0].Status)
	assert.EqualValues(t, 120, payload.Data.Summary.AvailableRewardQuota)
	var reloadedInviter model.User
	require.NoError(t, db.First(&reloadedInviter, inviter.Id).Error)
	assert.Equal(t, 120, reloadedInviter.AffQuota)
}

func TestGetAffiliateRewardsOnlyReturnsCurrentUserRewards(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	now := common.GetTimestamp()
	inviter := insertControllerAffiliateUser(t, db, 9106, "user_aff_inviter", 0)
	invitee := insertControllerAffiliateUser(t, db, 9107, "user_aff_invitee", inviter.Id)
	otherInviter := insertControllerAffiliateUser(t, db, 9108, "user_aff_other_inviter", 0)
	otherInvitee := insertControllerAffiliateUser(t, db, 9109, "user_aff_other_invitee", otherInviter.Id)
	insertControllerAffiliateReward(t, db, model.AffiliateRewardLog{
		RewardKey:       "controller-user-own",
		InviterId:       inviter.Id,
		InviteeId:       invitee.Id,
		TradeNo:         "controller-user-own-order",
		TriggerType:     model.AffiliateRewardTriggerRecurringTopup,
		SourceType:      model.AffiliateRewardSourceTopup,
		PaymentProvider: model.PaymentProviderEpay,
		RewardQuota:     100,
		Status:          model.AffiliateRewardStatusAvailable,
		EligibleAt:      now,
		CreatedAt:       now - 10,
	})
	insertControllerAffiliateReward(t, db, model.AffiliateRewardLog{
		RewardKey:       "controller-user-other",
		InviterId:       otherInviter.Id,
		InviteeId:       otherInvitee.Id,
		TradeNo:         "controller-user-other-order",
		TriggerType:     model.AffiliateRewardTriggerRecurringTopup,
		SourceType:      model.AffiliateRewardSourceTopup,
		PaymentProvider: model.PaymentProviderEpay,
		RewardQuota:     200,
		Status:          model.AffiliateRewardStatusAvailable,
		EligibleAt:      now,
		CreatedAt:       now - 5,
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("id", inviter.Id)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/user/aff_rewards?p=1&page_size=10", nil)

	GetAffiliateRewards(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Total int                        `json:"total"`
			Items []model.AffiliateRewardLog `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, 1, payload.Data.Total)
	require.Len(t, payload.Data.Items, 1)
	assert.Equal(t, inviter.Id, payload.Data.Items[0].InviterId)
}

func TestVoidAffiliateRewardHandlerReversesLedger(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	now := common.GetTimestamp()
	inviter := insertControllerAffiliateUser(t, db, 9111, "admin_void_inviter", 0)
	invitee := insertControllerAffiliateUser(t, db, 9112, "admin_void_invitee", inviter.Id)
	inviter.AffQuota = 70
	inviter.AffHistoryQuota = 100
	inviter.Quota = 30
	require.NoError(t, db.Save(&inviter).Error)

	reward := insertControllerAffiliateReward(t, db, model.AffiliateRewardLog{
		RewardKey:        "controller-void-partial",
		InviterId:        inviter.Id,
		InviteeId:        invitee.Id,
		TradeNo:          "controller-void-partial-order",
		TriggerType:      model.AffiliateRewardTriggerRecurringTopup,
		SourceType:       model.AffiliateRewardSourceTopup,
		PaymentProvider:  model.PaymentProviderEpay,
		RewardQuota:      100,
		TransferredQuota: 30,
		Status:           model.AffiliateRewardStatusAvailable,
		EligibleAt:       now,
		SettledAt:        now,
		TransferredAt:    now,
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: strconv.Itoa(reward.Id)}}
	context.Request = httptest.NewRequest(http.MethodPost, "/api/user/aff_rewards/1/void", strings.NewReader(`{"reason":"chargeback"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	VoidAffiliateReward(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var reloadedInviter model.User
	require.NoError(t, db.Where("id = ?", inviter.Id).First(&reloadedInviter).Error)
	assert.Equal(t, 0, reloadedInviter.AffQuota)
	assert.Equal(t, 0, reloadedInviter.AffHistoryQuota)
	assert.Equal(t, 0, reloadedInviter.Quota)
	var reloadedReward model.AffiliateRewardLog
	require.NoError(t, db.First(&reloadedReward, reward.Id).Error)
	assert.Equal(t, model.AffiliateRewardStatusVoided, reloadedReward.Status)
	assert.Equal(t, "chargeback", reloadedReward.VoidReason)
}

func TestListModelsIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-tiered-visible-model":      "tiered_expr",
		"zz-tiered-empty-expr-model":   "tiered_expr",
		"zz-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-tiered-empty-expr-model": "   ",
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1001,
		Username: "model-list-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-tiered-visible-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-empty-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-missing-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-unpriced-model", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1001)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-tiered-visible-model")
	require.NotContains(t, ids, "zz-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-unpriced-model")

	pricingByName := pricingByModelName(model.GetPricing())
	visiblePricing, ok := pricingByName["zz-tiered-visible-model"]
	require.True(t, ok)
	require.Equal(t, "tiered_expr", visiblePricing.BillingMode)
	require.NotEmpty(t, visiblePricing.BillingExpr)

	emptyExprPricing, ok := pricingByName["zz-tiered-empty-expr-model"]
	require.True(t, ok)
	require.Empty(t, emptyExprPricing.BillingMode)
	require.Empty(t, emptyExprPricing.BillingExpr)

	missingExprPricing, ok := pricingByName["zz-tiered-missing-expr-model"]
	require.True(t, ok)
	require.Empty(t, missingExprPricing.BillingMode)
	require.Empty(t, missingExprPricing.BillingExpr)
}

func TestListModelsTokenLimitIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-token-tiered-visible-model":      "tiered_expr",
		"zz-token-tiered-empty-expr-model":   "tiered_expr",
		"zz-token-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-token-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-token-tiered-empty-expr-model": "",
	})
	setupModelListControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{
		"zz-token-tiered-visible-model":      true,
		"zz-token-tiered-empty-expr-model":   true,
		"zz-token-tiered-missing-expr-model": true,
		"zz-token-unpriced-model":            true,
	})

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-token-tiered-visible-model")
	require.NotContains(t, ids, "zz-token-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-token-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-token-unpriced-model")
}

func TestUpdateOptionRejectsInvalidAffiliateSettings(t *testing.T) {
	setupModelListControllerTestDB(t)

	affiliateSetting := operation_setting.GetAffiliateSetting()
	originalAffiliateSetting := *affiliateSetting
	paymentSetting := operation_setting.GetPaymentSetting()
	originalPaymentSetting := *paymentSetting
	t.Cleanup(func() {
		*affiliateSetting = originalAffiliateSetting
		*paymentSetting = originalPaymentSetting
	})

	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	cases := []struct {
		name     string
		key      string
		value    string
		contains string
	}{
		{
			name:     "invalid bool",
			key:      "affiliate_setting.enabled",
			value:    `"maybe"`,
			contains: "必须是布尔值",
		},
		{
			name:     "negative integer",
			key:      "affiliate_setting.min_topup_quota",
			value:    `-1`,
			contains: "不能为负数",
		},
		{
			name:     "invalid integer",
			key:      "affiliate_setting.recurring_max_count",
			value:    `"abc"`,
			contains: "必须是整数",
		},
		{
			name:     "negative rate",
			key:      "affiliate_setting.first_reward_rate",
			value:    `-0.1`,
			contains: "不能为负数",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(
				http.MethodPut,
				"/api/option",
				strings.NewReader(fmt.Sprintf(`{"key":%q,"value":%s}`, tc.key, tc.value)),
			)

			UpdateOption(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.False(t, response.Success)
			require.Contains(t, response.Message, tc.contains)
		})
	}
}

func TestUpdateOptionRejectsAffiliateRewardsWithoutCompliance(t *testing.T) {
	setupModelListControllerTestDB(t)

	affiliateSetting := operation_setting.GetAffiliateSetting()
	originalAffiliateSetting := *affiliateSetting
	paymentSetting := operation_setting.GetPaymentSetting()
	originalPaymentSetting := *paymentSetting
	t.Cleanup(func() {
		*affiliateSetting = originalAffiliateSetting
		*paymentSetting = originalPaymentSetting
	})

	*affiliateSetting = operation_setting.AffiliateSetting{
		Enabled:                false,
		FirstRewardEnabled:     true,
		FirstRewardRate:        0.1,
		RecurringRewardEnabled: false,
		RecurringRewardRate:    0,
	}
	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = ""

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option",
		strings.NewReader(`{"key":"affiliate_setting.enabled","value":true}`),
	)

	UpdateOption(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.NotEmpty(t, response.Message)
}
