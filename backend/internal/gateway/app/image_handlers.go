package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxImageSize = 10 * 1024 * 1024

var (
	errImageTooLarge    = errors.New("image exceeds the maximum size")
	errUnsupportedImage = errors.New("image format is not supported")
	errInvalidImage     = errors.New("image data is invalid")
)

func (a *App) uploadImageMessageHandler(c *gin.Context) {
	if a.services == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat service unavailable"})
		return
	}
	if a.objects == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "image storage unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImageSize+1024*1024)
	if err := c.Request.ParseMultipartForm(1024 * 1024); err != nil {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
		var limitError *http.MaxBytesError
		if errors.As(err, &limitError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "image must be between 1 byte and 10 MB"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart image upload"})
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	conversationID := strings.TrimSpace(c.PostForm("conversation_id"))
	if _, err := uuid.Parse(conversationID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid conversation_id is required"})
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}
	if fileHeader.Size < 1 || fileHeader.Size > maxImageSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "image must be between 1 byte and 10 MB"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read image file"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxImageSize+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read image file"})
		return
	}
	if len(data) == 0 || len(data) > maxImageSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "image must be between 1 byte and 10 MB"})
		return
	}

	mediaType, extension, err := inspectImage(data)
	if err != nil {
		statusCode, message := http.StatusBadRequest, "image file is invalid"
		if errors.Is(err, errImageTooLarge) {
			statusCode, message = http.StatusRequestEntityTooLarge, "image must be between 1 byte and 10 MB"
		} else if errors.Is(err, errUnsupportedImage) {
			statusCode, message = http.StatusUnsupportedMediaType, "only JPEG, PNG, and GIF images are supported"
		}
		c.JSON(statusCode, gin.H{"error": message})
		return
	}

	userID, _ := c.Get("user_id")
	ownerID, ok := userID.(string)
	if !ok || ownerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user is required"})
		return
	}
	mediaID := uuid.NewString()
	storageKey := path.Join("images", ownerID, mediaID+extension)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	if err := a.objects.put(ctx, storageKey, data, mediaType); err != nil {
		log.Printf("store uploaded image: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "image storage failed"})
		return
	}
	message, err := a.services.SendImageMessage(ctx, conversationID, ownerID, mediaID, storageKey, mediaType, int64(len(data)))
	if err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if cleanupErr := a.objects.remove(cleanupCtx, storageKey); cleanupErr != nil {
			log.Printf("remove image after message persistence failed: %v", cleanupErr)
		}
		cleanupCancel()
		log.Printf("persist uploaded image message: %v", err)
		c.JSON(grpcHTTPStatus(err), gin.H{"error": "image message could not be saved"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": message})
}

func inspectImage(data []byte) (string, string, error) {
	if len(data) == 0 || len(data) > maxImageSize {
		return "", "", errImageTooLarge
	}
	mediaType := http.DetectContentType(data)
	extensions := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif"}
	extension, supported := extensions[mediaType]
	if !supported {
		return "", "", errUnsupportedImage
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		return "", "", errInvalidImage
	}
	return mediaType, extension, nil
}

func (a *App) getMediaHandler(c *gin.Context) {
	if a.services == nil || a.objects == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "image service unavailable"})
		return
	}
	userID, _ := c.Get("user_id")
	ownerID, ok := userID.(string)
	if !ok || ownerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated user is required"})
		return
	}
	media, err := a.services.GetMediaObject(c.Request.Context(), c.Param("id"), ownerID)
	if err != nil {
		c.JSON(grpcHTTPStatus(err), gin.H{"error": "image not found or access denied"})
		return
	}
	if media.MediaType != "image/jpeg" && media.MediaType != "image/png" && media.MediaType != "image/gif" {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "unsupported stored image type"})
		return
	}
	object, info, err := a.objects.get(c.Request.Context(), media.StorageKey)
	if err != nil {
		log.Printf("read image from MinIO: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "image could not be loaded"})
		return
	}
	defer object.Close()
	if info.Size != media.ByteSize {
		log.Printf("stored image size mismatch for media %s", c.Param("id"))
		c.JSON(http.StatusBadGateway, gin.H{"error": "stored image metadata is invalid"})
		return
	}
	c.Header("Content-Type", media.MediaType)
	c.Header("Content-Length", fmt.Sprint(media.ByteSize))
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, object); err != nil {
		log.Printf("stream image to client: %v", err)
	}
}

func grpcHTTPStatus(err error) int {
	switch status.Code(err) {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.ResourceExhausted:
		return http.StatusRequestEntityTooLarge
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadGateway
	}
}
