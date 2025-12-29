package v1

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) Follow(ctx context.Context, req *v1pb.FollowRequest) (*v1pb.Subscription, error) {
	currentUserID, ok := ctx.Value(UserIDContextKey).(int32)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	targetUserID, err := ExtractUserIDFromName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
	}

	if currentUserID == targetUserID {
		return nil, status.Errorf(codes.InvalidArgument, "cannot follow yourself")
	}

	// Check if target user exists
	targetUser, err := s.Store.GetUser(ctx, &store.FindUser{ID: &targetUserID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}
	if targetUser == nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	// Check if already following
	existing, err := s.Store.GetSubscription(ctx, &store.FindSubscription{
		FollowerID:  &currentUserID,
		FollowingID: &targetUserID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check subscription: %v", err)
	}
	if existing != nil {
		return nil, status.Errorf(codes.AlreadyExists, "already following this user")
	}

	subscription, err := s.Store.CreateSubscription(ctx, &store.Subscription{
		FollowerID:  currentUserID,
		FollowingID: targetUserID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create subscription: %v", err)
	}

	return &v1pb.Subscription{
		Name:          fmt.Sprintf("users/%d/subscriptions/%d", currentUserID, subscription.ID),
		FollowingUser: req.Name,
		CreateTime:    timestamppb.New(time.Unix(subscription.CreatedTs, 0)),
	}, nil
}

func (s *APIV1Service) Unfollow(ctx context.Context, req *v1pb.UnfollowRequest) (*emptypb.Empty, error) {
	currentUserID, ok := ctx.Value(UserIDContextKey).(int32)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	targetUserID, err := ExtractUserIDFromName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
	}

	err = s.Store.DeleteSubscription(ctx, &store.DeleteSubscription{
		FollowerID:  &currentUserID,
		FollowingID: &targetUserID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete subscription: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) GetSubscriptionStatus(ctx context.Context, req *v1pb.GetSubscriptionStatusRequest) (*v1pb.SubscriptionStatus, error) {
	currentUserID, ok := ctx.Value(UserIDContextKey).(int32)
	if !ok {
		// Not authenticated - return not following
		return &v1pb.SubscriptionStatus{IsFollowing: false}, nil
	}

	targetUserID, err := ExtractUserIDFromName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
	}

	subscription, err := s.Store.GetSubscription(ctx, &store.FindSubscription{
		FollowerID:  &currentUserID,
		FollowingID: &targetUserID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get subscription: %v", err)
	}

	result := &v1pb.SubscriptionStatus{IsFollowing: subscription != nil}
	if subscription != nil {
		result.Subscription = &v1pb.Subscription{
			Name:          fmt.Sprintf("users/%d/subscriptions/%d", currentUserID, subscription.ID),
			FollowingUser: req.Name,
			CreateTime:    timestamppb.New(time.Unix(subscription.CreatedTs, 0)),
		}
	}
	return result, nil
}

func (s *APIV1Service) ListFollowing(ctx context.Context, req *v1pb.ListFollowingRequest) (*v1pb.ListFollowingResponse, error) {
	userID, err := ExtractUserIDFromName(req.Parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
	}

	subscriptions, err := s.Store.ListSubscriptions(ctx, &store.FindSubscription{
		FollowerID: &userID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list subscriptions: %v", err)
	}

	response := &v1pb.ListFollowingResponse{
		Following: make([]*v1pb.FollowedUser, len(subscriptions)),
		TotalSize: int32(len(subscriptions)),
	}
	for i, sub := range subscriptions {
		response.Following[i] = &v1pb.FollowedUser{
			User:       fmt.Sprintf("users/%d", sub.FollowingID),
			CreateTime: timestamppb.New(time.Unix(sub.CreatedTs, 0)),
		}
	}
	return response, nil
}

func (s *APIV1Service) ListFollowers(ctx context.Context, req *v1pb.ListFollowersRequest) (*v1pb.ListFollowersResponse, error) {
	userID, err := ExtractUserIDFromName(req.Parent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
	}

	subscriptions, err := s.Store.ListSubscriptions(ctx, &store.FindSubscription{
		FollowingID: &userID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list followers: %v", err)
	}

	response := &v1pb.ListFollowersResponse{
		Followers: make([]*v1pb.Follower, len(subscriptions)),
		TotalSize: int32(len(subscriptions)),
	}
	for i, sub := range subscriptions {
		response.Followers[i] = &v1pb.Follower{
			User:       fmt.Sprintf("users/%d", sub.FollowerID),
			CreateTime: timestamppb.New(time.Unix(sub.CreatedTs, 0)),
		}
	}
	return response, nil
}

func (s *APIV1Service) GetSubscriptionCounts(ctx context.Context, req *v1pb.GetSubscriptionCountsRequest) (*v1pb.SubscriptionCounts, error) {
	userID, err := ExtractUserIDFromName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
	}

	counts, err := s.Store.GetSubscriptionCounts(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get subscription counts: %v", err)
	}

	return &v1pb.SubscriptionCounts{
		FollowerCount:  counts.FollowerCount,
		FollowingCount: counts.FollowingCount,
	}, nil
}
