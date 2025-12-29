# Subscribe Feature Implementation Prompt

## Overview

Implement a **user subscription system** that allows users to subscribe to other users. When viewing a user's profile page, a "Subscribe" button should appear (if not already subscribed) or an "Unsubscribe" button (if already subscribed). Users can view memos from all their subscribed users in a unified feed.

## Feature Requirements

### Core Functionality

1. **Subscribe to User**: A logged-in user can subscribe to another user's profile
2. **Unsubscribe from User**: A logged-in user can unsubscribe from a previously subscribed user
3. **Subscription Status**: Show subscription status on user profile pages
4. **Subscriptions List**: Users can view their list of subscribed users
5. **Subscribers List** (optional): Users can see who has subscribed to them
6. **Subscription Feed**: View memos from all subscribed users in a single feed

### UI/UX Requirements

1. **User Profile Page** (`web/src/pages/UserProfile.tsx`):

   - Add a "Subscribe" / "Unsubscribe" button next to the Share button
   - Button should not appear on the user's own profile
   - Show subscriber count (optional)

2. **Subscription Feed Page** (new page):

   - List memos from all subscribed users
   - Use existing `PagedMemoList` component
   - Accessible from sidebar/navigation

3. **Subscriptions Management** (optional):
   - List of subscribed users
   - Ability to unsubscribe from the list

---

## Implementation Guide

### Phase 1: Database Layer

#### 1.1 Create Subscription Model (`store/subscription.go`)

```go
package store

import "context"

type Subscription struct {
    ID            int32
    SubscriberID  int32  // The user who is subscribing
    SubscribedID  int32  // The user being subscribed to
    CreatedTs     int64
}

type FindSubscription struct {
    ID            *int32
    SubscriberID  *int32
    SubscribedID  *int32
}

type DeleteSubscription struct {
    ID            *int32
    SubscriberID  *int32
    SubscribedID  *int32
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
```

#### 1.2 Update Driver Interface (`store/driver.go`)

Add to the `Driver` interface:

```go
// Subscription model related methods.
CreateSubscription(ctx context.Context, create *Subscription) (*Subscription, error)
ListSubscriptions(ctx context.Context, find *FindSubscription) ([]*Subscription, error)
DeleteSubscription(ctx context.Context, delete *DeleteSubscription) error
```

#### 1.3 SQLite Implementation (`store/db/sqlite/subscription.go`)

```go
package sqlite

import (
    "context"
    "strings"

    "github.com/usememos/memos/store"
)

func (d *DB) CreateSubscription(ctx context.Context, create *store.Subscription) (*store.Subscription, error) {
    stmt := `
        INSERT INTO subscription (subscriber_id, subscribed_id)
        VALUES (?, ?)
        RETURNING id, created_ts
    `
    if err := d.db.QueryRowContext(ctx, stmt, create.SubscriberID, create.SubscribedID).Scan(
        &create.ID,
        &create.CreatedTs,
    ); err != nil {
        return nil, err
    }
    return create, nil
}

func (d *DB) ListSubscriptions(ctx context.Context, find *store.FindSubscription) ([]*store.Subscription, error) {
    where, args := []string{"1 = 1"}, []any{}

    if v := find.ID; v != nil {
        where, args = append(where, "id = ?"), append(args, *v)
    }
    if v := find.SubscriberID; v != nil {
        where, args = append(where, "subscriber_id = ?"), append(args, *v)
    }
    if v := find.SubscribedID; v != nil {
        where, args = append(where, "subscribed_id = ?"), append(args, *v)
    }

    query := `
        SELECT id, subscriber_id, subscribed_id, created_ts
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
        if err := rows.Scan(&sub.ID, &sub.SubscriberID, &sub.SubscribedID, &sub.CreatedTs); err != nil {
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
    if v := delete.SubscriberID; v != nil {
        where, args = append(where, "subscriber_id = ?"), append(args, *v)
    }
    if v := delete.SubscribedID; v != nil {
        where, args = append(where, "subscribed_id = ?"), append(args, *v)
    }

    if len(where) == 0 {
        return nil
    }

    _, err := d.db.ExecContext(ctx, `DELETE FROM subscription WHERE `+strings.Join(where, " AND "), args...)
    return err
}
```

#### 1.4 Database Migration

**SQLite** (`store/migration/sqlite/0.27/00__subscription.sql`):

```sql
-- subscription table
CREATE TABLE subscription (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subscriber_id INTEGER NOT NULL,
    subscribed_id INTEGER NOT NULL,
    created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
    UNIQUE(subscriber_id, subscribed_id),
    FOREIGN KEY (subscriber_id) REFERENCES user(id) ON DELETE CASCADE,
    FOREIGN KEY (subscribed_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX idx_subscription_subscriber_id ON subscription(subscriber_id);
CREATE INDEX idx_subscription_subscribed_id ON subscription(subscribed_id);
```

**Update LATEST.sql** (`store/migration/sqlite/LATEST.sql`):
Add the subscription table definition.

**MySQL and PostgreSQL**: Create similar migration files in respective directories.

---

### Phase 2: Protocol Buffers

#### 2.1 Create Subscription Service (`proto/api/v1/subscription_service.proto`)

```protobuf
syntax = "proto3";

package memos.api.v1;

import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "google/api/resource.proto";
import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

option go_package = "gen/api/v1";

service SubscriptionService {
    // Subscribe to a user.
    rpc Subscribe(SubscribeRequest) returns (Subscription) {
        option (google.api.http) = {
            post: "/api/v1/{name=users/*}:subscribe"
            body: "*"
        };
    }

    // Unsubscribe from a user.
    rpc Unsubscribe(UnsubscribeRequest) returns (google.protobuf.Empty) {
        option (google.api.http) = {
            post: "/api/v1/{name=users/*}:unsubscribe"
            body: "*"
        };
    }

    // Check if current user is subscribed to a user.
    rpc GetSubscriptionStatus(GetSubscriptionStatusRequest) returns (SubscriptionStatus) {
        option (google.api.http) = {
            get: "/api/v1/{name=users/*}/subscription"
        };
    }

    // List users the current user is subscribed to.
    rpc ListSubscriptions(ListSubscriptionsRequest) returns (ListSubscriptionsResponse) {
        option (google.api.http) = {
            get: "/api/v1/{parent=users/*}/subscriptions"
        };
    }

    // List subscribers of a user.
    rpc ListSubscribers(ListSubscribersRequest) returns (ListSubscribersResponse) {
        option (google.api.http) = {
            get: "/api/v1/{parent=users/*}/subscribers"
        };
    }
}

message Subscription {
    option (google.api.resource) = {
        type: "memos.api.v1/Subscription"
        pattern: "users/{user}/subscriptions/{subscription}"
    };

    // The resource name.
    string name = 1 [(google.api.field_behavior) = IDENTIFIER];

    // The user being subscribed to.
    string subscribed_user = 2 [(google.api.resource_reference) = {type: "memos.api.v1/User"}];

    // The creation timestamp.
    google.protobuf.Timestamp create_time = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
}

message SubscribeRequest {
    // The user to subscribe to.
    // Format: users/{user}
    string name = 1 [
        (google.api.field_behavior) = REQUIRED,
        (google.api.resource_reference) = {type: "memos.api.v1/User"}
    ];
}

message UnsubscribeRequest {
    // The user to unsubscribe from.
    // Format: users/{user}
    string name = 1 [
        (google.api.field_behavior) = REQUIRED,
        (google.api.resource_reference) = {type: "memos.api.v1/User"}
    ];
}

message GetSubscriptionStatusRequest {
    // The user to check subscription status for.
    // Format: users/{user}
    string name = 1 [
        (google.api.field_behavior) = REQUIRED,
        (google.api.resource_reference) = {type: "memos.api.v1/User"}
    ];
}

message SubscriptionStatus {
    // Whether the current user is subscribed to the target user.
    bool is_subscribed = 1;

    // The subscription details if subscribed.
    Subscription subscription = 2;
}

message ListSubscriptionsRequest {
    // The user whose subscriptions to list.
    // Format: users/{user}
    string parent = 1 [
        (google.api.field_behavior) = REQUIRED,
        (google.api.resource_reference) = {type: "memos.api.v1/User"}
    ];

    int32 page_size = 2 [(google.api.field_behavior) = OPTIONAL];
    string page_token = 3 [(google.api.field_behavior) = OPTIONAL];
}

message ListSubscriptionsResponse {
    repeated Subscription subscriptions = 1;
    string next_page_token = 2;
    int32 total_size = 3;
}

message ListSubscribersRequest {
    // The user whose subscribers to list.
    // Format: users/{user}
    string parent = 1 [
        (google.api.field_behavior) = REQUIRED,
        (google.api.resource_reference) = {type: "memos.api.v1/User"}
    ];

    int32 page_size = 2 [(google.api.field_behavior) = OPTIONAL];
    string page_token = 3 [(google.api.field_behavior) = OPTIONAL];
}

message ListSubscribersResponse {
    // List of subscribers (users who subscribe to the parent user).
    repeated Subscriber subscribers = 1;
    string next_page_token = 2;
    int32 total_size = 3;
}

message Subscriber {
    // The subscriber user resource.
    string user = 1 [(google.api.resource_reference) = {type: "memos.api.v1/User"}];

    // When the subscription was created.
    google.protobuf.Timestamp create_time = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
}
```

#### 2.2 Generate Code

```bash
cd proto
buf generate
```

---

### Phase 3: Backend API Service

#### 3.1 Create Subscription Service (`server/router/api/v1/subscription_service.go`)

```go
package v1

import (
    "context"
    "fmt"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/emptypb"
    "google.golang.org/protobuf/types/known/timestamppb"

    v1pb "github.com/usememos/memos/proto/gen/api/v1"
    "github.com/usememos/memos/store"
)

func (s *APIV1Service) Subscribe(ctx context.Context, req *v1pb.SubscribeRequest) (*v1pb.Subscription, error) {
    currentUserID, ok := ctx.Value(userIDContextKey).(int32)
    if !ok {
        return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
    }

    targetUserID, err := ExtractUserIDFromName(req.Name)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
    }

    if currentUserID == targetUserID {
        return nil, status.Errorf(codes.InvalidArgument, "cannot subscribe to yourself")
    }

    // Check if target user exists
    targetUser, err := s.Store.GetUser(ctx, &store.FindUser{ID: &targetUserID})
    if err != nil || targetUser == nil {
        return nil, status.Errorf(codes.NotFound, "user not found")
    }

    // Check if already subscribed
    existing, err := s.Store.GetSubscription(ctx, &store.FindSubscription{
        SubscriberID: &currentUserID,
        SubscribedID: &targetUserID,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to check subscription: %v", err)
    }
    if existing != nil {
        return nil, status.Errorf(codes.AlreadyExists, "already subscribed")
    }

    subscription, err := s.Store.CreateSubscription(ctx, &store.Subscription{
        SubscriberID: currentUserID,
        SubscribedID: targetUserID,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to create subscription: %v", err)
    }

    return &v1pb.Subscription{
        Name:           fmt.Sprintf("users/%d/subscriptions/%d", currentUserID, subscription.ID),
        SubscribedUser: req.Name,
        CreateTime:     timestamppb.New(time.Unix(subscription.CreatedTs, 0)),
    }, nil
}

func (s *APIV1Service) Unsubscribe(ctx context.Context, req *v1pb.UnsubscribeRequest) (*emptypb.Empty, error) {
    currentUserID, ok := ctx.Value(userIDContextKey).(int32)
    if !ok {
        return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
    }

    targetUserID, err := ExtractUserIDFromName(req.Name)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
    }

    err = s.Store.DeleteSubscription(ctx, &store.DeleteSubscription{
        SubscriberID: &currentUserID,
        SubscribedID: &targetUserID,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to delete subscription: %v", err)
    }

    return &emptypb.Empty{}, nil
}

func (s *APIV1Service) GetSubscriptionStatus(ctx context.Context, req *v1pb.GetSubscriptionStatusRequest) (*v1pb.SubscriptionStatus, error) {
    currentUserID, ok := ctx.Value(userIDContextKey).(int32)
    if !ok {
        return &v1pb.SubscriptionStatus{IsSubscribed: false}, nil
    }

    targetUserID, err := ExtractUserIDFromName(req.Name)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
    }

    subscription, err := s.Store.GetSubscription(ctx, &store.FindSubscription{
        SubscriberID: &currentUserID,
        SubscribedID: &targetUserID,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get subscription: %v", err)
    }

    result := &v1pb.SubscriptionStatus{IsSubscribed: subscription != nil}
    if subscription != nil {
        result.Subscription = &v1pb.Subscription{
            Name:           fmt.Sprintf("users/%d/subscriptions/%d", currentUserID, subscription.ID),
            SubscribedUser: req.Name,
            CreateTime:     timestamppb.New(time.Unix(subscription.CreatedTs, 0)),
        }
    }
    return result, nil
}

func (s *APIV1Service) ListSubscriptions(ctx context.Context, req *v1pb.ListSubscriptionsRequest) (*v1pb.ListSubscriptionsResponse, error) {
    userID, err := ExtractUserIDFromName(req.Parent)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
    }

    subscriptions, err := s.Store.ListSubscriptions(ctx, &store.FindSubscription{
        SubscriberID: &userID,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to list subscriptions: %v", err)
    }

    response := &v1pb.ListSubscriptionsResponse{
        Subscriptions: make([]*v1pb.Subscription, len(subscriptions)),
        TotalSize:     int32(len(subscriptions)),
    }
    for i, sub := range subscriptions {
        response.Subscriptions[i] = &v1pb.Subscription{
            Name:           fmt.Sprintf("users/%d/subscriptions/%d", userID, sub.ID),
            SubscribedUser: fmt.Sprintf("users/%d", sub.SubscribedID),
            CreateTime:     timestamppb.New(time.Unix(sub.CreatedTs, 0)),
        }
    }
    return response, nil
}

func (s *APIV1Service) ListSubscribers(ctx context.Context, req *v1pb.ListSubscribersRequest) (*v1pb.ListSubscribersResponse, error) {
    userID, err := ExtractUserIDFromName(req.Parent)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid user name: %v", err)
    }

    subscriptions, err := s.Store.ListSubscriptions(ctx, &store.FindSubscription{
        SubscribedID: &userID,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to list subscribers: %v", err)
    }

    response := &v1pb.ListSubscribersResponse{
        Subscribers: make([]*v1pb.Subscriber, len(subscriptions)),
        TotalSize:   int32(len(subscriptions)),
    }
    for i, sub := range subscriptions {
        response.Subscribers[i] = &v1pb.Subscriber{
            User:       fmt.Sprintf("users/%d", sub.SubscriberID),
            CreateTime: timestamppb.New(time.Unix(sub.CreatedTs, 0)),
        }
    }
    return response, nil
}
```

#### 3.2 Register Service

Update `server/router/api/v1/v1.go` to register the subscription service.

#### 3.3 Update ACL Config (`server/router/api/v1/acl_config.go`)

Add public endpoints if needed (GetSubscriptionStatus might be public for guest viewing).

---

### Phase 4: Frontend Implementation

#### 4.1 Add gRPC Client (`web/src/grpcweb.ts`)

```typescript
import { SubscriptionServiceDefinition } from "@/types/proto/api/v1/subscription_service";

export const subscriptionServiceClient = createClient(
  SubscriptionServiceDefinition,
  channel
);
```

#### 4.2 Create Subscription Store (`web/src/store/subscription.ts`)

```typescript
import { makeAutoObservable } from "mobx";
import { subscriptionServiceClient } from "@/grpcweb";
import {
  Subscription,
  SubscriptionStatus,
} from "@/types/proto/api/v1/subscription_service";

class SubscriptionStore {
  subscriptionStatusByUser: Record<string, SubscriptionStatus> = {};
  subscriptions: Subscription[] = [];

  constructor() {
    makeAutoObservable(this);
  }

  async fetchSubscriptionStatus(userName: string): Promise<SubscriptionStatus> {
    const status = await subscriptionServiceClient.getSubscriptionStatus({
      name: userName,
    });
    this.subscriptionStatusByUser[userName] = status;
    return status;
  }

  async subscribe(userName: string): Promise<Subscription> {
    const subscription = await subscriptionServiceClient.subscribe({
      name: userName,
    });
    this.subscriptionStatusByUser[userName] = {
      isSubscribed: true,
      subscription,
    };
    return subscription;
  }

  async unsubscribe(userName: string): Promise<void> {
    await subscriptionServiceClient.unsubscribe({ name: userName });
    this.subscriptionStatusByUser[userName] = { isSubscribed: false };
  }

  async fetchSubscriptions(parent: string): Promise<void> {
    const response = await subscriptionServiceClient.listSubscriptions({
      parent,
    });
    this.subscriptions = response.subscriptions;
  }

  getSubscriptionStatus(userName: string): SubscriptionStatus | undefined {
    return this.subscriptionStatusByUser[userName];
  }
}

export const subscriptionStore = new SubscriptionStore();
export default subscriptionStore;
```

#### 4.3 Update User Profile Page (`web/src/pages/UserProfile.tsx`)

Add the subscribe/unsubscribe button:

```tsx
import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
import { userStore, subscriptionStore } from "@/store";
import { Button } from "@/components/ui/button";
import { UserPlusIcon, UserMinusIcon } from "lucide-react";

const UserProfile = observer(() => {
  const [isSubscribed, setIsSubscribed] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const currentUser = userStore.state.currentUser;
  const isOwnProfile = currentUser === user?.name;

  useEffect(() => {
    if (user && currentUser && !isOwnProfile) {
      subscriptionStore.fetchSubscriptionStatus(user.name).then((status) => {
        setIsSubscribed(status.isSubscribed);
      });
    }
  }, [user, currentUser, isOwnProfile]);

  const handleToggleSubscription = async () => {
    if (!user) return;
    setIsLoading(true);
    try {
      if (isSubscribed) {
        await subscriptionStore.unsubscribe(user.name);
        setIsSubscribed(false);
        toast.success(t("subscription.unsubscribed"));
      } else {
        await subscriptionStore.subscribe(user.name);
        setIsSubscribed(true);
        toast.success(t("subscription.subscribed"));
      }
    } catch (error) {
      toast.error(t("subscription.error"));
    } finally {
      setIsLoading(false);
    }
  };

  // In the JSX, add next to the Share button:
  {
    currentUser && !isOwnProfile && (
      <Button
        variant={isSubscribed ? "outline" : "default"}
        onClick={handleToggleSubscription}
        disabled={isLoading}
      >
        {isSubscribed ? (
          <>
            <UserMinusIcon className="w-4 h-4 mr-1" />
            {t("subscription.unsubscribe")}
          </>
        ) : (
          <>
            <UserPlusIcon className="w-4 h-4 mr-1" />
            {t("subscription.subscribe")}
          </>
        )}
      </Button>
    );
  }
});
```

#### 4.4 Create Subscription Feed Page (`web/src/pages/SubscriptionFeed.tsx`)

```tsx
import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
import PagedMemoList from "@/components/PagedMemoList";
import MemoView from "@/components/MemoView";
import { subscriptionStore, userStore } from "@/store";
import { useMemoFilters, useMemoSorting } from "@/hooks";

const SubscriptionFeed = observer(() => {
  const [subscribedUserNames, setSubscribedUserNames] = useState<string[]>([]);

  useEffect(() => {
    const currentUser = userStore.state.currentUser;
    if (currentUser) {
      subscriptionStore.fetchSubscriptions(currentUser).then(() => {
        const names = subscriptionStore.subscriptions.map(
          (s) => s.subscribedUser
        );
        setSubscribedUserNames(names);
      });
    }
  }, []);

  // Build filter for subscribed users' memos
  const memoFilter = useMemoFilters({
    creatorNames: subscribedUserNames,
    includeShortcuts: false,
    includePinned: false,
  });

  const { listSort, orderBy } = useMemoSorting({
    pinnedFirst: false,
  });

  if (subscribedUserNames.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-64">
        <p className="text-muted-foreground">No subscriptions yet</p>
      </div>
    );
  }

  return (
    <section className="w-full min-h-full flex flex-col">
      <h1 className="text-2xl font-semibold mb-4">Subscription Feed</h1>
      <PagedMemoList
        renderer={(memo) => (
          <MemoView key={memo.name} memo={memo} showVisibility />
        )}
        listSort={listSort}
        orderBy={orderBy}
        filter={memoFilter}
      />
    </section>
  );
});

export default SubscriptionFeed;
```

#### 4.5 Add Route (`web/src/router/index.tsx`)

```tsx
{
  path: "/subscriptions",
  element: <SubscriptionFeed />,
}
```

#### 4.6 Add Navigation Link

Update sidebar/navigation to include a link to the subscription feed.

#### 4.7 Add i18n Translations (`web/src/locales/en.json`)

```json
{
  "subscription": {
    "subscribe": "Subscribe",
    "unsubscribe": "Unsubscribe",
    "subscribed": "Subscribed successfully",
    "unsubscribed": "Unsubscribed successfully",
    "error": "Failed to update subscription",
    "feed": "Subscription Feed",
    "no-subscriptions": "No subscriptions yet"
  }
}
```

---

## Testing Checklist

### Backend Tests

- [ ] Create subscription successfully
- [ ] Prevent duplicate subscriptions
- [ ] Prevent self-subscription
- [ ] Delete subscription successfully
- [ ] List subscriptions returns correct data
- [ ] List subscribers returns correct data
- [ ] Subscription status returns correct value

### Frontend Tests

- [ ] Subscribe button appears on other users' profiles
- [ ] Subscribe button does not appear on own profile
- [ ] Subscribe/unsubscribe toggles correctly
- [ ] Subscription feed displays memos from subscribed users
- [ ] Empty state shown when no subscriptions

### Integration Tests

- [ ] Full subscribe/unsubscribe flow works end-to-end
- [ ] Subscription feed filters memos correctly

---

## File Checklist

### Backend (Go)

- [ ] `store/subscription.go` - Subscription model and store methods
- [ ] `store/driver.go` - Add subscription methods to Driver interface
- [ ] `store/db/sqlite/subscription.go` - SQLite implementation
- [ ] `store/db/mysql/subscription.go` - MySQL implementation
- [ ] `store/db/postgres/subscription.go` - PostgreSQL implementation
- [ ] `store/migration/sqlite/0.27/00__subscription.sql` - SQLite migration
- [ ] `store/migration/mysql/0.27/00__subscription.sql` - MySQL migration
- [ ] `store/migration/postgres/0.27/00__subscription.sql` - PostgreSQL migration
- [ ] `proto/api/v1/subscription_service.proto` - Protobuf definitions
- [ ] `server/router/api/v1/subscription_service.go` - API implementation
- [ ] `server/router/api/v1/v1.go` - Register subscription service
- [ ] `server/router/api/v1/acl_config.go` - Update ACL if needed

### Frontend (TypeScript/React)

- [ ] `web/src/grpcweb.ts` - Add subscription service client
- [ ] `web/src/store/subscription.ts` - Subscription MobX store
- [ ] `web/src/store/index.ts` - Export subscription store
- [ ] `web/src/pages/UserProfile.tsx` - Add subscribe button
- [ ] `web/src/pages/SubscriptionFeed.tsx` - New subscription feed page
- [ ] `web/src/router/index.tsx` - Add subscription feed route
- [ ] `web/src/locales/en.json` - Add translations
- [ ] `web/src/locales/zh-Hans.json` - Add Chinese translations

---

## Notes

- Follow existing code patterns and conventions as documented in `CLAUDE.md`
- Run `buf generate` after modifying proto files
- Test with all three database drivers (SQLite, MySQL, PostgreSQL)
- Ensure proper error handling and loading states in the frontend
- Consider adding subscriber count to user profiles (optional enhancement)
