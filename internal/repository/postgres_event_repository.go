package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type PostgresEventRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresEventRepository(pool *pgxpool.Pool) *PostgresEventRepository {
	return &PostgresEventRepository{pool: pool}
}

func (r *PostgresEventRepository) Insert(ctx context.Context, event domain.NotificationEvent) error {
	event.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())

	_, err := r.pool.Exec(ctx,
		"INSERT INTO events (id, type, payload, status, retry_count) VALUES ($1, $2, $3, $4, $5)",
		event.ID, event.Type, event.Payload, event.Status, event.RetryCount,
	)
	return err
}

func (r *PostgresEventRepository) FindByStatus(ctx context.Context, status domain.EventStatus) ([]domain.NotificationEvent, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT id, type, payload, status, retry_count FROM events WHERE status = $1",
		status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.NotificationEvent
	for rows.Next() {
		var e domain.NotificationEvent
		if err := rows.Scan(&e.ID, &e.Type, &e.Payload, &e.Status, &e.RetryCount); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *PostgresEventRepository) FindByStatuses(ctx context.Context, statuses ...domain.EventStatus) ([]domain.NotificationEvent, error) {
	statusValues := make([]string, 0, len(statuses))
	for _, status := range statuses {
		statusValues = append(statusValues, string(status))
	}

	rows, err := r.pool.Query(ctx,
		"SELECT id, type, payload, status, retry_count FROM events WHERE status = ANY($1::text[])",
		statusValues,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.NotificationEvent
	for rows.Next() {
		var e domain.NotificationEvent
		if err := rows.Scan(&e.ID, &e.Type, &e.Payload, &e.Status, &e.RetryCount); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (r *PostgresEventRepository) UpdateStatus(ctx context.Context, id string, status domain.EventStatus, retryCount int) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE events SET status=$1, retry_count=$2 WHERE id=$3",
		status, retryCount, id,
	)
	return err
}

var _ domain.EventRepository = (*PostgresEventRepository)(nil)
