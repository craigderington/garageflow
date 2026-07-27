package events

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	TypeRepairOrderCreated  = "repair_order:created"
	TypeEstimateSent        = "estimate:sent"
	TypeEstimateApproved    = "estimate:approved"
	TypeMechanicClockedIn   = "mechanic:clocked_in"
	TypeInventoryLowStock   = "inventory:low_stock"
)

type RepairOrderCreatedPayload struct {
	RepairOrderID string `json:"repair_order_id"`
	ShopID        string `json:"shop_id"`
	CustomerID    string `json:"customer_id"`
}

type EstimateSentPayload struct {
	EstimateID    string `json:"estimate_id"`
	RepairOrderID string `json:"repair_order_id"`
	ShopID        string `json:"shop_id"`
}

type EstimateApprovedPayload struct {
	EstimateID    string `json:"estimate_id"`
	RepairOrderID string `json:"repair_order_id"`
	ShopID        string `json:"shop_id"`
}

type MechanicClockedInPayload struct {
	MechanicID    string `json:"mechanic_id"`
	RepairOrderID string `json:"repair_order_id"`
	ShopID        string `json:"shop_id"`
}

type InventoryLowStockPayload struct {
	PartID string `json:"part_id"`
	ShopID string `json:"shop_id"`
	Name   string `json:"name"`
	Stock  int    `json:"stock"`
}

type Bus struct {
	client *asynq.Client
}

func NewBus(rdb *redis.Client) *Bus {
	return &Bus{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: rdb.Options().Addr}),
	}
}

func (b *Bus) Publish(ctx context.Context, eventType string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	task := asynq.NewTask(eventType, data)
	_, err = b.client.Enqueue(task)
	return err
}
