package v1

import (
	"net/http"
	"regexp"

	"github.com/labstack/echo/v5"
)

// imageIDPattern 校验 image_id 格式（{kb_id}-{chunk_id}，均为十六进制或字母数字）
// 防止路径遍历攻击
var imageIDPattern = regexp.MustCompile(`^[a-zA-Z0-9]+-[a-zA-Z0-9]+$`)

// handleRAGFlowImage 代理 RAGFlow chunk 截图
// GET /api/v1/ragflow/image/:imageId
//
// 职责：
//  1. 验证 imageId 格式，防止路径遍历
//  2. 通过系统级 RAGFlowClient 从 RAGFlow 获取图片（RAGFlow 侧该端点无需认证）
//  3. 将 JPEG 数据透传给前端，设置 Cache-Control 长缓存（chunk 图片不可变）
func (s *APIV1Service) handleRAGFlowImage(c *echo.Context) error {
	imageID := c.Param("imageId")
	if imageID == "" || !imageIDPattern.MatchString(imageID) {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid image_id format",
		})
	}

	if s.RAGFlowClient == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "RAGFlow not configured",
		})
	}

	data, err := s.RAGFlowClient.GetDocumentImage(c.Request().Context(), imageID)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": "failed to fetch image from RAGFlow",
		})
	}

	// chunk 截图是不可变的（由文档解析生成），可长期缓存
	c.Response().Header().Set("Cache-Control", "public, max-age=86400, immutable")
	return c.Blob(http.StatusOK, "image/jpeg", data)
}
