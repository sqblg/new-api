package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	RefundStatusPending  = "pending"
	RefundStatusApproved = "approved"
	RefundStatusRejected = "rejected"
)

type RefundRequest struct {
	Id             int    `json:"id"`
	UserId         int    `json:"user_id" gorm:"index"`
	TopUpId        int    `json:"topup_id" gorm:"index"`
	Amount         int64  `json:"amount"`
	Reason         string `json:"reason" gorm:"type:varchar(500)"`
	IdempotencyKey string `json:"-" gorm:"type:varchar(255);index:idx_refund_user_idempotency,unique"`
	Status         string `json:"status" gorm:"type:varchar(32);index"`
	OperatorId     int    `json:"operator_id"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (r *RefundRequest) Insert() error {
	if r.CreatedAt == 0 {
		r.CreatedAt = time.Now().Unix()
	}
	r.UpdatedAt = r.CreatedAt
	return DB.Create(r).Error
}

func GetRefundRequestByIdempotency(userId int, key string) *RefundRequest {
	if userId <= 0 || key == "" {
		return nil
	}
	var request RefundRequest
	if err := DB.Where("user_id = ? AND idempotency_key = ?", userId, key).First(&request).Error; err != nil {
		return nil
	}
	return &request
}

func GetRefundRequest(id int) *RefundRequest {
	var request RefundRequest
	if err := DB.First(&request, id).Error; err != nil {
		return nil
	}
	return &request
}

func ListRefundRequests(offset, limit int) ([]RefundRequest, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var requests []RefundRequest
	var total int64
	if err := DB.Model(&RefundRequest{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := DB.Order("id desc").Offset(offset).Limit(limit).Find(&requests).Error; err != nil {
		return nil, 0, err
	}
	return requests, total, nil
}

func (r *RefundRequest) Approve(operatorId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var current RefundRequest
		if err := tx.First(&current, r.Id).Error; err != nil {
			return err
		}
		if current.Status != RefundStatusPending {
			return errors.New("refund request is no longer pending")
		}
		var topUp TopUp
		if err := tx.First(&topUp, current.TopUpId).Error; err != nil {
			return err
		}
		if topUp.UserId != current.UserId || topUp.Status != common.TopUpStatusSuccess || current.Amount <= 0 || current.Amount > topUp.Amount {
			return errors.New("refund request is not eligible")
		}
		// The quota is credited in the same unit as TopUp.Amount. Keep the
		// refund decision and balance debit in one transaction.
		quota := current.Amount * int64(common.QuotaPerUnit)
		result := tx.Model(&User{}).Where("id = ? AND quota >= ?", current.UserId, quota).Update("quota", gorm.Expr("quota - ?", quota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("insufficient quota for refund")
		}
		current.Status = RefundStatusApproved
		current.OperatorId = operatorId
		current.UpdatedAt = time.Now().Unix()
		return tx.Save(&current).Error
	})
}
