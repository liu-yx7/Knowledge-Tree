package mysql

import (
	"context"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateSubscription(ctx context.Context, create *store.Subscription) (*store.Subscription, error) {
	stmt := `
		INSERT INTO subscription (follower_id, following_id)
		VALUES (?, ?)
	`
	result, err := d.db.ExecContext(ctx, stmt, create.FollowerID, create.FollowingID)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	create.ID = int32(id)

	// Fetch the created timestamp
	err = d.db.QueryRowContext(ctx, `SELECT created_ts FROM subscription WHERE id = ?`, create.ID).Scan(&create.CreatedTs)
	if err != nil {
		return nil, err
	}

	return create, nil
}

func (d *DB) ListSubscriptions(ctx context.Context, find *store.FindSubscription) ([]*store.Subscription, error) {
	where, args := []string{"1 = 1"}, []any{}

	if v := find.ID; v != nil {
		where, args = append(where, "id = ?"), append(args, *v)
	}
	if v := find.FollowerID; v != nil {
		where, args = append(where, "follower_id = ?"), append(args, *v)
	}
	if v := find.FollowingID; v != nil {
		where, args = append(where, "following_id = ?"), append(args, *v)
	}

	query := `
		SELECT id, follower_id, following_id, created_ts
		FROM subscription
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_ts DESC
	`

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*store.Subscription, 0)
	for rows.Next() {
		var sub store.Subscription
		if err := rows.Scan(&sub.ID, &sub.FollowerID, &sub.FollowingID, &sub.CreatedTs); err != nil {
			return nil, err
		}
		list = append(list, &sub)
	}
	return list, rows.Err()
}

func (d *DB) DeleteSubscription(ctx context.Context, delete *store.DeleteSubscription) error {
	where, args := []string{}, []any{}

	if v := delete.ID; v != nil {
		where, args = append(where, "id = ?"), append(args, *v)
	}
	if v := delete.FollowerID; v != nil {
		where, args = append(where, "follower_id = ?"), append(args, *v)
	}
	if v := delete.FollowingID; v != nil {
		where, args = append(where, "following_id = ?"), append(args, *v)
	}

	if len(where) == 0 {
		return nil
	}

	_, err := d.db.ExecContext(ctx, `DELETE FROM subscription WHERE `+strings.Join(where, " AND "), args...)
	return err
}

func (d *DB) GetSubscriptionCounts(ctx context.Context, userID int32) (*store.SubscriptionCounts, error) {
	counts := &store.SubscriptionCounts{}

	err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscription WHERE following_id = ?`, userID).Scan(&counts.FollowerCount)
	if err != nil {
		return nil, err
	}

	err = d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscription WHERE follower_id = ?`, userID).Scan(&counts.FollowingCount)
	if err != nil {
		return nil, err
	}

	return counts, nil
}
