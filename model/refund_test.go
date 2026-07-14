package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRefundRequestIdempotencyAndAtomicApproval(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&User{}, &TopUp{}, &RefundRequest{}))
	user := &User{Id: 991, Username: "refund_user", Status: common.UserStatusEnabled, Quota: int(common.QuotaPerUnit) * 20}
	require.NoError(t, DB.Create(user).Error)
	topUp := &TopUp{UserId: user.Id, Amount: 10, Money: 10, TradeNo: "refund-topup", PaymentProvider: PaymentProviderStripe, PaymentMethod: PaymentMethodStripe, Status: common.TopUpStatusSuccess, CreateTime: time.Now().Unix()}
	require.NoError(t, topUp.Insert())
	key := "refund-key"
	first := &RefundRequest{UserId: user.Id, TopUpId: topUp.Id, Amount: 3, IdempotencyKey: key, Status: RefundStatusPending}
	require.NoError(t, first.Insert())
	replayed := GetRefundRequestByIdempotency(user.Id, key)
	require.NotNil(t, replayed)
	require.Equal(t, first.Id, replayed.Id)
	require.NoError(t, first.Approve(7))
	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	require.Equal(t, int(common.QuotaPerUnit)*17, updated.Quota)
	require.Equal(t, RefundStatusApproved, GetRefundRequest(first.Id).Status)
	require.Error(t, first.Approve(7))
	second := &RefundRequest{UserId: user.Id, TopUpId: topUp.Id, Amount: 8, IdempotencyKey: "refund-key-2", Status: RefundStatusPending}
	require.NoError(t, second.Insert())
	require.Error(t, second.Approve(7))
}
