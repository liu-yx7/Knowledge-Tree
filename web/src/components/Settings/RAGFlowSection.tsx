import { useState } from "react";
import { RefreshCw, CheckCircle, XCircle, Clock, AlertCircle, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import {
  useSyncStatus,
  useSyncStats,
  useTriggerSync,
  useContentSyncStates,
  formatSyncStatus,
  getSyncStatusColor,
  formatContentType,
} from "@/hooks/useRAGFlowQueries";
import SettingSection from "./SettingSection";
import SettingRow from "./SettingRow";

/**
 * RAGFlow 同步状态管理组件
 * 仅管理员可见，用于查看和管理 RAGFlow 同步状态
 */
const RAGFlowSection = () => {
  const [selectedStatusFilter, setSelectedStatusFilter] = useState<string>("");
  
  const { data: syncStatus, isLoading: statusLoading, error: statusError } = useSyncStatus();
  const { data: syncStats, isLoading: statsLoading } = useSyncStats();
  const { data: syncStatesData, isLoading: statesLoading } = useContentSyncStates({
    statusFilter: selectedStatusFilter,
    pageSize: 10,
  });
  const triggerSync = useTriggerSync();

  const handleTriggerSync = async () => {
    try {
      await triggerSync.mutateAsync();
    } catch (error) {
      console.error("触发同步失败:", error);
    }
  };

  // 如果 API 调用失败（可能是权限不足或服务未启用），显示提示
  if (statusError) {
    return (
      <SettingSection title="RAGFlow 同步">
        <div className="p-4 text-center text-muted-foreground">
          <AlertCircle className="w-8 h-8 mx-auto mb-2" />
          <p className="text-sm">RAGFlow 服务未启用或您没有访问权限</p>
        </div>
      </SettingSection>
    );
  }

  return (
    <SettingSection title="RAGFlow 同步">
      {/* 服务状态 */}
      <SettingRow
        label="服务状态"
        description="RAGFlow 同步服务的运行状态"
      >
        {statusLoading ? (
          <Loader2 className="w-4 h-4 animate-spin" />
        ) : syncStatus ? (
          <div className="flex items-center gap-3">
            {/* 健康状态 */}
            <div className="flex items-center gap-1.5">
              {syncStatus.healthy ? (
                <CheckCircle className="w-4 h-4 text-green-500" />
              ) : (
                <XCircle className="w-4 h-4 text-red-500" />
              )}
              <span className={cn(
                "text-sm",
                syncStatus.healthy ? "text-green-600" : "text-red-600"
              )}>
                {syncStatus.healthy ? "健康" : "不健康"}
              </span>
            </div>

            {/* 熔断器状态 */}
            {syncStatus.circuitOpen && (
              <Badge variant="destructive" className="text-xs">
                熔断器已打开
              </Badge>
            )}

            {/* Runner 状态 */}
            <Badge variant={syncStatus.runnerActive ? "default" : "secondary"} className="text-xs">
              {syncStatus.runnerActive ? "运行中" : "已停止"}
            </Badge>
          </div>
        ) : (
          <span className="text-sm text-muted-foreground">未知</span>
        )}
      </SettingRow>

      {/* 同步统计 */}
      <SettingRow
        label="同步统计"
        description="内容同步状态统计"
      >
        {statsLoading ? (
          <Loader2 className="w-4 h-4 animate-spin" />
        ) : syncStats ? (
          <div className="flex flex-wrap gap-3">
            <StatBadge
              icon={<Clock className="w-3 h-3" />}
              label="待同步"
              value={syncStats.pendingCount}
              variant="warning"
            />
            <StatBadge
              icon={<CheckCircle className="w-3 h-3" />}
              label="已同步"
              value={syncStats.syncedCount}
              variant="success"
            />
            <StatBadge
              icon={<XCircle className="w-3 h-3" />}
              label="失败"
              value={syncStats.failedCount}
              variant="error"
            />
            <StatBadge
              icon={<AlertCircle className="w-3 h-3" />}
              label="跳过"
              value={syncStats.skippedCount}
              variant="muted"
            />
          </div>
        ) : null}
      </SettingRow>

      {/* 手动触发同步 */}
      <SettingRow
        label="手动同步"
        description="立即触发一次批量同步"
      >
        <Button
          variant="outline"
          size="sm"
          onClick={handleTriggerSync}
          disabled={triggerSync.isPending || !syncStatus?.runnerActive}
        >
          {triggerSync.isPending ? (
            <Loader2 className="w-4 h-4 mr-2 animate-spin" />
          ) : (
            <RefreshCw className="w-4 h-4 mr-2" />
          )}
          触发同步
        </Button>
      </SettingRow>

      {/* 同步状态列表 */}
      <SettingRow
        label="同步记录"
        description="最近的内容同步状态"
        vertical
      >
        <div className="w-full">
          {/* 过滤器 */}
          <div className="flex gap-2 mb-3">
            {["", "pending", "synced", "failed", "skipped"].map((status) => (
              <Button
                key={status || "all"}
                variant={selectedStatusFilter === status ? "default" : "outline"}
                size="sm"
                className="text-xs h-7"
                onClick={() => setSelectedStatusFilter(status)}
              >
                {status ? formatSyncStatus(status) : "全部"}
              </Button>
            ))}
          </div>

          {/* 列表 */}
          {statesLoading ? (
            <div className="flex justify-center py-4">
              <Loader2 className="w-5 h-5 animate-spin" />
            </div>
          ) : syncStatesData?.syncStates && syncStatesData.syncStates.length > 0 ? (
            <div className="space-y-2">
              {syncStatesData.syncStates.map((state, index) => (
                <div
                  key={`${state.contentType}-${state.contentUid}-${index}`}
                  className="flex items-center justify-between p-2 rounded-md bg-muted/50"
                >
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="text-xs">
                      {formatContentType(state.contentType)}
                    </Badge>
                    <span className="text-sm font-mono truncate max-w-[200px]">
                      {state.contentUid}
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    {state.retryCount > 0 && (
                      <span className="text-xs text-muted-foreground">
                        重试 {state.retryCount}
                      </span>
                    )}
                    <Badge className={cn("text-xs", getSyncStatusColor(state.status))}>
                      {formatSyncStatus(state.status)}
                    </Badge>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center text-sm text-muted-foreground py-4">
              暂无同步记录
            </div>
          )}
        </div>
      </SettingRow>
    </SettingSection>
  );
};

// 统计徽章组件
interface StatBadgeProps {
  icon: React.ReactNode;
  label: string;
  value: number;
  variant: "success" | "warning" | "error" | "muted";
}

const StatBadge = ({ icon, label, value, variant }: StatBadgeProps) => {
  const variantStyles = {
    success: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
    warning: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400",
    error: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
    muted: "bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-400",
  };

  return (
    <div className={cn(
      "flex items-center gap-1.5 px-2 py-1 rounded-md text-xs",
      variantStyles[variant]
    )}>
      {icon}
      <span>{label}</span>
      <span className="font-semibold">{value}</span>
    </div>
  );
};

export default RAGFlowSection;
