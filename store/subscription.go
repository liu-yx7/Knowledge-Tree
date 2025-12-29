package store

import (
	"context"
)

// Subscription represents a follow relationship between users.
// FollowerID follows FollowingID.
type Subscription struct {
	ID          int32
	FollowerID  int32 // The user who is following
	FollowingID int32 // The user being followed
	CreatedTs   int64
}

// FindSubscription is the query for finding subscriptions.
type FindSubscription struct {
	ID          *int32
	FollowerID  *int32
	FollowingID *int32
}

// DeleteSubscription is the query for deleting subscriptions.
type DeleteSubscription struct {
	ID          *int32
	FollowerID  *int32
	FollowingID *int32
}

// SubscriptionCounts holds the follower and following counts for a user.
type SubscriptionCounts struct {
	FollowerCount  int32
	FollowingCount int32
}

func (s *Store) CreateSubscription(ctx context.Context, create *Subscription) (*Subscription, error) {
	return s.driver.CreateSubscription(ctx, create)
}

func (s *Store) ListSubscriptions(ctx context.Context, find *FindSubscription) ([]*Subscription, error) {
	return s.driver.ListSubscriptions(ctx, find)
}

func (s *Store) GetSubscription(ctx context.Context, find *FindSubscription) (*Subscription, error) {
	list, err := s.ListSubscriptions(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (s *Store) DeleteSubscription(ctx context.Context, delete *DeleteSubscription) error {
	return s.driver.DeleteSubscription(ctx, delete)
}

func (s *Store) GetSubscriptionCounts(ctx context.Context, userID int32) (*SubscriptionCounts, error) {
	return s.driver.GetSubscriptionCounts(ctx, userID)
}
