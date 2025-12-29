import { makeAutoObservable } from "mobx";
import { subscriptionServiceClient } from "@/grpcweb";
import { Subscription, SubscriptionCounts, SubscriptionStatus } from "@/types/proto/api/v1/subscription_service";

class SubscriptionStore {
  // Cache subscription status by user name
  subscriptionStatusByUser: Record<string, SubscriptionStatus> = {};
  // Cache subscription counts by user name
  subscriptionCountsByUser: Record<string, SubscriptionCounts> = {};

  constructor() {
    makeAutoObservable(this);
  }

  async fetchSubscriptionStatus(userName: string): Promise<SubscriptionStatus> {
    const status = await subscriptionServiceClient.getSubscriptionStatus({ name: userName });
    this.subscriptionStatusByUser[userName] = status;
    return status;
  }

  async follow(userName: string): Promise<Subscription> {
    const subscription = await subscriptionServiceClient.follow({ name: userName });
    this.subscriptionStatusByUser[userName] = { isFollowing: true, subscription };
    // Invalidate counts cache
    delete this.subscriptionCountsByUser[userName];
    return subscription;
  }

  async unfollow(userName: string): Promise<void> {
    await subscriptionServiceClient.unfollow({ name: userName });
    this.subscriptionStatusByUser[userName] = { isFollowing: false };
    // Invalidate counts cache
    delete this.subscriptionCountsByUser[userName];
  }

  async fetchSubscriptionCounts(userName: string): Promise<SubscriptionCounts> {
    const counts = await subscriptionServiceClient.getSubscriptionCounts({ name: userName });
    this.subscriptionCountsByUser[userName] = counts;
    return counts;
  }

  async listFollowing(userName: string) {
    const response = await subscriptionServiceClient.listFollowing({ parent: userName });
    return response.following;
  }

  async listFollowers(userName: string) {
    const response = await subscriptionServiceClient.listFollowers({ parent: userName });
    return response.followers;
  }

  getSubscriptionStatus(userName: string): SubscriptionStatus | undefined {
    return this.subscriptionStatusByUser[userName];
  }

  getSubscriptionCounts(userName: string): SubscriptionCounts | undefined {
    return this.subscriptionCountsByUser[userName];
  }

  // Clear all cached data
  reset() {
    this.subscriptionStatusByUser = {};
    this.subscriptionCountsByUser = {};
  }
}

const subscriptionStore = new SubscriptionStore();

export default subscriptionStore;
