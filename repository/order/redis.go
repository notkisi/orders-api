package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/notkisi/orders-api/model"
	"github.com/redis/go-redis/v9"
)

type OrderRepo struct {
	DB *redis.Client
}

func orderIDKey(id uint64) string {
	return fmt.Sprintf("order:%d", id)
}

// as these methods preform network requests they can fail, have error returned
func (o *OrderRepo) Insert(ctx context.Context, order model.Order) error {
	data, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("failed to encode order: %w", err)
	}

	// generate a new key to store into redis
	key := orderIDKey(order.OrderID)

	// start a new transaction, and add actions to it
	txn := o.DB.TxPipeline()

	// store in DB, use NX to not overrite existing data
	res := txn.SetNX(ctx, key, string(data), 0)
	if err := res.Err(); err != nil {
		txn.Discard()
		return fmt.Errorf("failed to set: %w", err)
	}

	// insert orderid into set that will be used for pagination
	// wrap this in transaction so if setnx succeeds but sadd fails
	// dont end up in a partial state
	if err := txn.SAdd(ctx, "orders", key).Err(); err != nil {
		txn.Discard()
		return fmt.Errorf("failed to add orders to set: %w", err)
	}

	// attempt to execute a transaction
	if _, err := txn.Exec(ctx); err != nil {
		return fmt.Errorf("failed to exec: %w", err)
	}

	return nil
}

var ErrNotExist = errors.New("order does not exist")

func (o *OrderRepo) FindByID(ctx context.Context, id uint64) (model.Order, error) {
	key := orderIDKey(id)

	value, err := o.DB.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return model.Order{}, ErrNotExist
	} else if err != nil {
		return model.Order{}, fmt.Errorf("get order: %w", err)
	}

	var order model.Order
	err = json.Unmarshal([]byte(value), &order)
	if err != nil {
		return model.Order{}, fmt.Errorf("failed to decode order json: %w", err)
	}

	return order, nil
}

func (o *OrderRepo) DeleteByID(ctx context.Context, id uint64) error {
	key := orderIDKey(id)

	txn := o.DB.TxPipeline()

	err := txn.Del(ctx, key).Err()
	if errors.Is(err, redis.Nil) {
		txn.Discard()
		return ErrNotExist
	} else if err != nil {
		txn.Discard()
		return fmt.Errorf("delete order: %w", err)
	}

	if err := txn.SRem(ctx, "orders", key).Err(); err != nil {
		txn.Discard()
		return fmt.Errorf("failed to remove from orders set: %w", err)
	}

	if _, err := txn.Exec(ctx); err != nil {
		return fmt.Errorf("DELETEBYID failed to exec : %w", err)
	}

	return nil
}

func (o *OrderRepo) Update(ctx context.Context, order model.Order) error {
	data, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("UPDATE failed to encode order: %w", err)
	}

	key := orderIDKey(order.OrderID)

	err = o.DB.SetXX(ctx, key, string(data), 0).Err()
	if errors.Is(err, redis.Nil) {
		return ErrNotExist
	} else if err != nil {
		return fmt.Errorf("update order fail: %w", err)
	}

	return nil
}

// its expensive to retrieve all records from redis
// use pagination here
type FindAllPage struct {
	Size   uint64
	Offset uint64
}

type FindResult struct {
	Orders []model.Order
	Cursor uint64
}

func (o *OrderRepo) FindAll(ctx context.Context, page FindAllPage) (FindResult, error) {
	res := o.DB.SScan(ctx, "orders", page.Offset, "*", int64(page.Size))

	keys, cursor, err := res.Result()
	if err != nil {
		return FindResult{}, fmt.Errorf("failed to get order ids: %w", err)
	}

	// dont send empty list of keys to MGet
	if len(keys) == 0 {
		return FindResult{
			Orders: []model.Order{},
		}, nil
	}

	// get value from all retrieved keys
	// everything returned in xs is of interface type, cast it
	xs, err := o.DB.MGet(ctx, keys...).Result()
	if err != nil {
		return FindResult{}, fmt.Errorf("failed to get keys values from set", err)
	}

	orders := make([]model.Order, len(xs))
	for i, x := range xs {
		// cast interface{} to string so we can unmarshal
		x := x.(string)

		var order model.Order
		err := json.Unmarshal([]byte(x), &order)
		if err != nil {
			return FindResult{}, fmt.Errorf("FindAll: failed to decode order json: %w", err)
		}

		orders[i] = order
	}

	return FindResult{
		Orders: orders,
		Cursor: cursor,
	}, nil
}
