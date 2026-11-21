package queue

import "context"

// Queue provides bounded backpressure so a busy inbox cannot exhaust memory.
type Queue[T any] struct{ items chan T }

func New[T any](capacity int) *Queue[T] { return &Queue[T]{items: make(chan T, capacity)} }
func (q *Queue[T]) Push(ctx context.Context, item T) error {
	select {
	case q.items <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (q *Queue[T]) Pop(ctx context.Context) (T, error) {
	select {
	case item := <-q.items:
		return item, nil
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}
func (q *Queue[T]) Len() int { return len(q.items) }
