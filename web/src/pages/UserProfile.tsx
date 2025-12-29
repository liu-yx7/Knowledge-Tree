import copy from "copy-to-clipboard";
import { ExternalLinkIcon, UserMinusIcon, UserPlusIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { useParams } from "react-router-dom";
import { MemoRenderContext } from "@/components/MasonryView";
import MemoView from "@/components/MemoView";
import PagedMemoList from "@/components/PagedMemoList";
import UserAvatar from "@/components/UserAvatar";
import { Button } from "@/components/ui/button";
import { useMemoFilters, useMemoSorting } from "@/hooks";
import useLoading from "@/hooks/useLoading";
import { subscriptionStore, userStore } from "@/store";
import { State } from "@/types/proto/api/v1/common";
import { Memo } from "@/types/proto/api/v1/memo_service";
import { User } from "@/types/proto/api/v1/user_service";
import { useTranslate } from "@/utils/i18n";

const UserProfile = observer(() => {
  const t = useTranslate();
  const params = useParams();
  const loadingState = useLoading();
  const [user, setUser] = useState<User>();
  const [isFollowing, setIsFollowing] = useState(false);
  const [isFollowLoading, setIsFollowLoading] = useState(false);
  const [followerCount, setFollowerCount] = useState(0);
  const [followingCount, setFollowingCount] = useState(0);

  const currentUser = userStore.state.currentUser;
  const isOwnProfile = currentUser === user?.name;

  useEffect(() => {
    const username = params.username;
    if (!username) {
      throw new Error("username is required");
    }

    userStore
      .getOrFetchUserByUsername(username)
      .then((user) => {
        setUser(user);
        loadingState.setFinish();

        // Fetch subscription status and counts
        if (user) {
          subscriptionStore.fetchSubscriptionCounts(user.name).then((counts) => {
            setFollowerCount(counts.followerCount);
            setFollowingCount(counts.followingCount);
          });

          if (currentUser && currentUser !== user.name) {
            subscriptionStore.fetchSubscriptionStatus(user.name).then((status) => {
              setIsFollowing(status.isFollowing);
            });
          }
        }
      })
      .catch((error) => {
        console.error(error);
        toast.error(t("message.user-not-found"));
      });
  }, [params.username, currentUser]);

  // Build filter using unified hook (no shortcuts, but includes pinned)
  const memoFilter = useMemoFilters({
    creatorName: user?.name,
    includeShortcuts: false,
    includePinned: true,
  });

  // Get sorting logic using unified hook
  const { listSort, orderBy } = useMemoSorting({
    pinnedFirst: true,
    state: State.NORMAL,
  });

  const handleCopyProfileLink = () => {
    if (!user) {
      return;
    }

    copy(`${window.location.origin}/u/${encodeURIComponent(user.username)}`);
    toast.success(t("message.copied"));
  };

  const handleToggleFollow = async () => {
    if (!user || !currentUser) return;

    setIsFollowLoading(true);
    try {
      if (isFollowing) {
        await subscriptionStore.unfollow(user.name);
        setIsFollowing(false);
        setFollowerCount((prev) => Math.max(0, prev - 1));
        toast.success(t("subscription.unfollowed"));
      } else {
        await subscriptionStore.follow(user.name);
        setIsFollowing(true);
        setFollowerCount((prev) => prev + 1);
        toast.success(t("subscription.followed"));
      }
    } catch (error) {
      console.error(error);
      toast.error(t("subscription.error"));
    } finally {
      setIsFollowLoading(false);
    }
  };

  return (
    <section className="w-full min-h-full flex flex-col justify-start items-center">
      {!loadingState.isLoading &&
        (user ? (
          <>
            {/* User profile header - centered with max width */}
            <div className="w-full max-w-4xl mx-auto mb-8">
              <div className="w-full flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 py-6 border-b border-border">
                <div className="flex items-center gap-4">
                  <UserAvatar className="w-20! h-20! drop-shadow rounded-full" avatarUrl={user?.avatarUrl} />
                  <div className="flex flex-col justify-center items-start">
                    <h1 className="text-2xl sm:text-3xl font-semibold text-foreground">{user.displayName || user.username}</h1>
                    {user.username && user.displayName && <p className="text-sm text-muted-foreground">@{user.username}</p>}
                    <div className="flex items-center gap-4 mt-1 text-sm text-muted-foreground">
                      <span>
                        <strong className="text-foreground">{followerCount}</strong> {t("subscription.followers")}
                      </span>
                      <span>
                        <strong className="text-foreground">{followingCount}</strong> {t("subscription.following")}
                      </span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {currentUser && !isOwnProfile && (
                    <Button
                      variant={isFollowing ? "outline" : "default"}
                      onClick={handleToggleFollow}
                      disabled={isFollowLoading}
                      className="shrink-0"
                    >
                      {isFollowing ? (
                        <>
                          <UserMinusIcon className="w-4 h-4 mr-1" />
                          {t("subscription.unfollow")}
                        </>
                      ) : (
                        <>
                          <UserPlusIcon className="w-4 h-4 mr-1" />
                          {t("subscription.follow")}
                        </>
                      )}
                    </Button>
                  )}
                  <Button variant="outline" onClick={handleCopyProfileLink} className="shrink-0">
                    {t("common.share")}
                    <ExternalLinkIcon className="ml-1 w-4 h-auto opacity-60" />
                  </Button>
                </div>
              </div>
              {user.description && (
                <div className="py-4">
                  <p className="text-base text-foreground/80 whitespace-pre-wrap">{user.description}</p>
                </div>
              )}
            </div>

            {/* Memo list - full width for proper masonry layout */}
            <PagedMemoList
              renderer={(memo: Memo, context?: MemoRenderContext) => (
                <MemoView key={`${memo.name}-${memo.displayTime}`} memo={memo} showVisibility showPinned compact={context?.compact} />
              )}
              listSort={listSort}
              orderBy={orderBy}
              filter={memoFilter}
            />
          </>
        ) : (
          <div className="w-full max-w-3xl mx-auto">
            <p className="text-center text-muted-foreground mt-8">Not found</p>
          </div>
        ))}
    </section>
  );
});

export default UserProfile;
