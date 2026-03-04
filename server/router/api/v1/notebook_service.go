package v1

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/memos/internal/util"
	"github.com/usememos/memos/plugin/ragflow"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) ListNotebooks(ctx context.Context, _ *v1pb.ListNotebooksRequest) (*v1pb.ListNotebooksResponse, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	notebooks, err := s.Store.ListNotebooks(ctx, &store.FindNotebook{
		CreatorID: &userID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list notebooks: %v", err)
	}

	// Auto-create default notebook if none exists (first-time user).
	if len(notebooks) == 0 {
		notebook, err := s.ensureDefaultNotebook(ctx, userID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create default notebook: %v", err)
		}
		notebooks = []*store.Notebook{notebook}
	}

	response := &v1pb.ListNotebooksResponse{}
	for _, nb := range notebooks {
		response.Notebooks = append(response.Notebooks, convertNotebookToProto(nb))
	}
	return response, nil
}

func (s *APIV1Service) GetNotebook(ctx context.Context, req *v1pb.GetNotebookRequest) (*v1pb.Notebook, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	notebook, err := s.Store.GetNotebook(ctx, &store.FindNotebook{
		ID: &req.Id,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get notebook: %v", err)
	}
	if notebook == nil {
		return nil, status.Errorf(codes.NotFound, "notebook not found")
	}
	if notebook.CreatorID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	return convertNotebookToProto(notebook), nil
}

func (s *APIV1Service) CreateNotebook(ctx context.Context, req *v1pb.CreateNotebookRequest) (*v1pb.Notebook, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if req.Title == "" {
		return nil, status.Errorf(codes.InvalidArgument, "title is required")
	}

	// Create a new RAGFlow Dataset for this notebook.
	datasetID := ""
	if s.RAGFlowProvisioner != nil {
		user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
		if err == nil && user != nil {
			dsID, err := s.createNotebookDataset(ctx, userID, user.Username, req.Title)
			if err != nil {
				slog.Warn("CreateNotebook: failed to create RAGFlow dataset, continuing without binding",
					slog.Int("userID", int(userID)),
					slog.Any("error", err))
			} else {
				datasetID = dsID
			}
		}
	}

	create := &store.Notebook{
		UID:       util.GenUUID(),
		CreatorID: userID,
		Title:     req.Title,
		Icon:      req.Icon,
		IsDefault: false,
		DatasetID: datasetID,
	}

	notebook, err := s.Store.CreateNotebook(ctx, create)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create notebook: %v", err)
	}

	// Update the user's RAGFlow Assistant to include all notebook datasets.
	if s.RAGFlowProvisioner != nil && datasetID != "" {
		if err := s.syncAssistantDatasets(ctx, userID); err != nil {
			slog.Warn("CreateNotebook: failed to sync assistant datasets",
				slog.Int("userID", int(userID)),
				slog.Any("error", err))
		}
	}

	return convertNotebookToProto(notebook), nil
}

func (s *APIV1Service) UpdateNotebook(ctx context.Context, req *v1pb.UpdateNotebookRequest) (*v1pb.Notebook, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}
	if req.Notebook == nil {
		return nil, status.Errorf(codes.InvalidArgument, "notebook is required")
	}

	notebook, err := s.Store.GetNotebook(ctx, &store.FindNotebook{
		ID: &req.Notebook.Id,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get notebook: %v", err)
	}
	if notebook == nil {
		return nil, status.Errorf(codes.NotFound, "notebook not found")
	}
	if notebook.CreatorID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	update := &store.UpdateNotebook{
		ID: notebook.ID,
	}

	for _, path := range req.UpdateMask.GetPaths() {
		switch path {
		case "title":
			update.Title = &req.Notebook.Title
		case "icon":
			update.Icon = &req.Notebook.Icon
		}
	}

	if err := s.Store.UpdateNotebook(ctx, update); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update notebook: %v", err)
	}

	updated, err := s.Store.GetNotebook(ctx, &store.FindNotebook{
		ID: &notebook.ID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get updated notebook: %v", err)
	}
	return convertNotebookToProto(updated), nil
}

func (s *APIV1Service) DeleteNotebook(ctx context.Context, req *v1pb.DeleteNotebookRequest) (*emptypb.Empty, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	notebook, err := s.Store.GetNotebook(ctx, &store.FindNotebook{
		ID: &req.Id,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get notebook: %v", err)
	}
	if notebook == nil {
		return nil, status.Errorf(codes.NotFound, "notebook not found")
	}
	if notebook.CreatorID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}
	if notebook.IsDefault {
		return nil, status.Errorf(codes.FailedPrecondition, "cannot delete default notebook")
	}

	// Move memos from this notebook to the default notebook.
	isDefault := true
	defaultNotebook, err := s.Store.GetNotebook(ctx, &store.FindNotebook{
		CreatorID: &userID,
		IsDefault: &isDefault,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find default notebook: %v", err)
	}

	if defaultNotebook != nil {
		// Reassign memos to default notebook before deletion.
		memos, err := s.Store.ListMemos(ctx, &store.FindMemo{
			CreatorID:  &userID,
			NotebookID: &notebook.ID,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to list memos: %v", err)
		}
		for _, memo := range memos {
			if err := s.Store.UpdateMemo(ctx, &store.UpdateMemo{
				ID:         memo.ID,
				NotebookID: &defaultNotebook.ID,
			}); err != nil {
				slog.Warn("DeleteNotebook: failed to reassign memo",
					slog.Int("memoID", int(memo.ID)),
					slog.Any("error", err))
			}
		}
	}

	if err := s.Store.DeleteNotebook(ctx, &store.DeleteNotebook{ID: notebook.ID}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete notebook: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ensureDefaultNotebook creates a default notebook for a user if none exists.
func (s *APIV1Service) ensureDefaultNotebook(ctx context.Context, userID int32) (*store.Notebook, error) {
	// Try to get existing RAGFlow dataset ID for this user.
	datasetID := ""
	mapping, err := s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})
	if err == nil && mapping != nil {
		datasetID = mapping.DatasetID
	}

	create := &store.Notebook{
		UID:       util.GenUUID(),
		CreatorID: userID,
		Title:     "Default",
		Icon:      "📚",
		IsDefault: true,
		DatasetID: datasetID,
	}

	notebook, err := s.Store.CreateNotebook(ctx, create)
	if err != nil {
		return nil, err
	}

	// Provision a RAGFlow dataset if the notebook has no dataset yet.
	if s.RAGFlowProvisioner != nil && notebook.DatasetID == "" {
		user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
		if err == nil && user != nil {
			dsID, err := s.createNotebookDataset(ctx, userID, user.Username, "Default")
			if err != nil {
				slog.Warn("ensureDefaultNotebook: failed to create RAGFlow dataset, continuing without binding",
					slog.Int("userID", int(userID)),
					slog.Any("error", err))
			} else {
				updatedDatasetID := dsID
				if updateErr := s.Store.UpdateNotebook(ctx, &store.UpdateNotebook{ID: notebook.ID, DatasetID: &updatedDatasetID}); updateErr != nil {
					slog.Warn("ensureDefaultNotebook: failed to update notebook datasetID",
						slog.Int("notebookID", int(notebook.ID)),
						slog.Any("error", updateErr))
				} else {
					notebook.DatasetID = dsID
				}
				if syncErr := s.syncAssistantDatasets(ctx, userID); syncErr != nil {
					slog.Warn("ensureDefaultNotebook: failed to sync assistant datasets",
						slog.Int("userID", int(userID)),
						slog.Any("error", syncErr))
				}
			}
		}
	}

	slog.Info("ensureDefaultNotebook: created default notebook",
		slog.Int("userID", int(userID)),
		slog.Int("notebookID", int(notebook.ID)),
		slog.String("datasetID", notebook.DatasetID))

	return notebook, nil
}

// createNotebookDataset creates a new RAGFlow Dataset for a custom notebook.
func (s *APIV1Service) createNotebookDataset(ctx context.Context, userID int32, username, title string) (string, error) {
	client, err := s.RAGFlowProvisioner.GetClientForUser(ctx, userID, username)
	if err != nil {
		return "", err
	}

	datasetName := "knowtree_nb_" + util.GenUUID()[:8]
	dataset, err := client.CreateDataset(ctx, &ragflow.CreateDatasetRequest{
		Name:        datasetName,
		Description: title,
		ChunkMethod: ragflow.ChunkMethodNaive,
	})
	if err != nil {
		return "", err
	}

	return dataset.ID, nil
}

func convertNotebookToProto(nb *store.Notebook) *v1pb.Notebook {
	proto := &v1pb.Notebook{
		Id:        nb.ID,
		Uid:       nb.UID,
		CreatorId: nb.CreatorID,
		Title:     nb.Title,
		Icon:      nb.Icon,
		IsDefault: nb.IsDefault,
		DatasetId: nb.DatasetID,
	}
	if nb.CreatedTs != 0 {
		proto.CreateTime = &timestamppb.Timestamp{Seconds: nb.CreatedTs}
	}
	if nb.UpdatedTs != 0 {
		proto.UpdateTime = &timestamppb.Timestamp{Seconds: nb.UpdatedTs}
	}
	return proto
}

// syncAssistantDatasets collects all dataset IDs from the user's notebooks
// and updates the RAGFlow Assistant to use them all.
func (s *APIV1Service) syncAssistantDatasets(ctx context.Context, userID int32) error {
	notebooks, err := s.Store.ListNotebooks(ctx, &store.FindNotebook{CreatorID: &userID})
	if err != nil {
		return err
	}

	var datasetIDs []string
	for _, nb := range notebooks {
		if nb.DatasetID != "" {
			datasetIDs = append(datasetIDs, nb.DatasetID)
		}
	}
	if len(datasetIDs) == 0 {
		return nil
	}

	return s.RAGFlowProvisioner.UpdateUserAssistantDatasets(ctx, userID, datasetIDs)
}
