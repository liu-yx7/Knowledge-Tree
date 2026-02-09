package v1

import (
	"context"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/sync/semaphore"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/plugin/markdown"
	"github.com/usememos/memos/plugin/ragflow"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/server/runner/ragflowsync"
	"github.com/usememos/memos/store"
)

type APIV1Service struct {
	v1pb.UnimplementedInstanceServiceServer
	v1pb.UnimplementedAuthServiceServer
	v1pb.UnimplementedUserServiceServer
	v1pb.UnimplementedMemoServiceServer
	v1pb.UnimplementedAttachmentServiceServer
	v1pb.UnimplementedShortcutServiceServer
	v1pb.UnimplementedActivityServiceServer
	v1pb.UnimplementedIdentityProviderServiceServer
	v1pb.UnimplementedAIServiceServer
	v1pb.UnimplementedRAGFlowServiceServer

	Secret          string
	Profile         *profile.Profile
	Store           *store.Store
	MarkdownService markdown.Service
	RAGFlowClient   *ragflow.Client // 替代原有的 LLMManager

	// RAGFlow Provisioner - 用户无感知的 RAGFlow 账户自动配置
	RAGFlowProvisioner *ragflow.Provisioner

	// RAGFlow 同步 Runner - 用于触发同步事件
	RAGFlowSyncRunner *ragflowsync.Runner

	// thumbnailSemaphore limits concurrent thumbnail generation to prevent memory exhaustion
	thumbnailSemaphore *semaphore.Weighted
}

func NewAPIV1Service(secret string, profile *profile.Profile, store *store.Store, ragflowClient *ragflow.Client, ragflowProvisioner *ragflow.Provisioner, ragflowSyncRunner *ragflowsync.Runner) *APIV1Service {
	markdownService := markdown.NewService(
		markdown.WithTagExtension(),
	)
	return &APIV1Service{
		Secret:             secret,
		Profile:            profile,
		Store:              store,
		MarkdownService:    markdownService,
		RAGFlowClient:      ragflowClient,
		RAGFlowProvisioner: ragflowProvisioner,
		RAGFlowSyncRunner:  ragflowSyncRunner,
		thumbnailSemaphore: semaphore.NewWeighted(3), // Limit to 3 concurrent thumbnail generations
	}
}

// RegisterGateway registers the gRPC-Gateway and Connect handlers with the given Echo instance.
func (s *APIV1Service) RegisterGateway(ctx context.Context, echoServer *echo.Echo) error {
	// Auth middleware for gRPC-Gateway - runs after routing, has access to method name.
	// Uses the same PublicMethods config as the Connect AuthInterceptor.
	authenticator := auth.NewAuthenticator(s.Store, s.Secret)
	gatewayAuthMiddleware := func(next runtime.HandlerFunc) runtime.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
			ctx := r.Context()

			// Get the RPC method name from context (set by grpc-gateway after routing)
			rpcMethod, ok := runtime.RPCMethod(ctx)

			// Extract credentials from HTTP headers
			authHeader := r.Header.Get("Authorization")

			result := authenticator.Authenticate(ctx, authHeader)

			// Enforce authentication for non-public methods
			// If rpcMethod cannot be determined, allow through, service layer will handle visibility checks
			if result == nil && ok && !IsPublicMethod(rpcMethod) {
				http.Error(w, `{"code": 16, "message": "authentication required"}`, http.StatusUnauthorized)
				return
			}

			// Set context based on auth result (may be nil for public endpoints)
			if result != nil {
				if result.Claims != nil {
					// Access Token V2 - stateless, use claims
					ctx = auth.SetUserClaimsInContext(ctx, result.Claims)
					ctx = context.WithValue(ctx, auth.UserIDContextKey, result.Claims.UserID)
				} else if result.User != nil {
					// PAT - have full user
					ctx = auth.SetUserInContext(ctx, result.User, result.AccessToken)
				}
				r = r.WithContext(ctx)
			}

			next(w, r, pathParams)
		}
	}

	// Create gRPC-Gateway mux with auth middleware.
	gwMux := runtime.NewServeMux(
		runtime.WithMiddlewares(gatewayAuthMiddleware),
	)
	if err := v1pb.RegisterInstanceServiceHandlerServer(ctx, gwMux, s); err != nil {
		return err
	}
	if err := v1pb.RegisterAuthServiceHandlerServer(ctx, gwMux, s); err != nil {
		return err
	}
	if err := v1pb.RegisterUserServiceHandlerServer(ctx, gwMux, s); err != nil {
		return err
	}
	if err := v1pb.RegisterMemoServiceHandlerServer(ctx, gwMux, s); err != nil {
		return err
	}
	if err := v1pb.RegisterAttachmentServiceHandlerServer(ctx, gwMux, s); err != nil {
		return err
	}
	if err := v1pb.RegisterShortcutServiceHandlerServer(ctx, gwMux, s); err != nil {
		return err
	}
	if err := v1pb.RegisterActivityServiceHandlerServer(ctx, gwMux, s); err != nil {
		return err
	}
	if err := v1pb.RegisterIdentityProviderServiceHandlerServer(ctx, gwMux, s); err != nil {
		return err
	}
	if err := v1pb.RegisterAIServiceHandlerServer(ctx, gwMux, s); err != nil {
		return err
	}
	gwGroup := echoServer.Group("")
	gwGroup.Use(middleware.CORS())
	handler := echo.WrapHandler(gwMux)

	gwGroup.Any("/api/v1/*", handler)
	gwGroup.Any("/file/*", handler)

	// Connect handlers for browser clients (replaces grpc-web).
	logStacktraces := s.Profile.Demo
	connectInterceptors := connect.WithInterceptors(
		NewMetadataInterceptor(), // Convert HTTP headers to gRPC metadata first
		NewLoggingInterceptor(logStacktraces),
		NewRecoveryInterceptor(logStacktraces),
		NewAuthInterceptor(s.Store, s.Secret),
	)
	connectMux := http.NewServeMux()
	connectHandler := NewConnectServiceHandler(s)
	connectHandler.RegisterConnectHandlers(connectMux, connectInterceptors)

	// Wrap with CORS for browser access
	corsHandler := middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: func(_ string) (bool, error) {
			return true, nil
		},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders:     []string{"*"},
		AllowCredentials: true,
	})
	connectGroup := echoServer.Group("", corsHandler)
	connectGroup.Any("/memos.api.v1.*", echo.WrapHandler(connectMux))

	return nil
}

// getUserRAGFlowClient 获取绑定了用户 API Key 的 RAGFlow 客户端
// 优先通过 Provisioner 自动配置（注册/登录/生成 API Key），实现用户无感知接入
// 如果 Provisioner 未配置，降级到被动查询 ragflow_user_mapping 表
func (s *APIV1Service) getUserRAGFlowClient(ctx context.Context, userID int32) *ragflow.Client {
	// 优先路径：通过 Provisioner 自动配置（P2 核心能力）
	if s.RAGFlowProvisioner != nil {
		// 获取用户信息（Provisioner 需要 username 生成 RAGFlow 邮箱）
		user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
		if err != nil || user == nil {
			slog.Warn("getUserRAGFlowClient: 获取用户信息失败",
				slog.Int("userID", int(userID)),
				slog.Any("error", err))
			return nil
		}

		client, err := s.RAGFlowProvisioner.GetClientForUser(ctx, userID, user.Username)
		if err != nil {
			slog.Warn("getUserRAGFlowClient: Provisioner 自动配置失败",
				slog.Int("userID", int(userID)),
				slog.Any("error", err))
			return nil
		}
		return client
	}

	// 降级路径：被动查询映射表（Provisioner 未配置时的兼容模式）
	if s.RAGFlowClient == nil {
		return nil
	}

	mapping, err := s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})
	if err != nil || mapping == nil || mapping.APIKey == "" {
		return nil
	}

	return s.RAGFlowClient.WithAPIKey(mapping.APIKey)
}
