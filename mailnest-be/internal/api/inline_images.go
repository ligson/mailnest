package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mailnest-be/internal/storage"

	"golang.org/x/image/tiff"
)

var cidReferencePattern = regexp.MustCompile(`(?i)cid:(?:<[^>]+>|[^"'\s>]+)`)

const inlineAttachmentURLTTL = time.Hour

const maxInlineImageTransformBytes = 1 << 20

// attachmentPayloads 会把被 HTML cid: 引用的附件标记为 inline，前端据此避免重复展示。
func attachmentPayloads(messageID int64, attachments []storage.MailAttachment, inlineContentIDs map[string]bool) []map[string]any {
	items := make([]map[string]any, 0, len(attachments))
	for _, attachment := range attachments {
		inline := attachment.Inline || inlineContentIDs[normalizeContentID(nullableStringValue(attachment.ContentID))]
		items = append(items, map[string]any{
			"id":          strconv.FormatInt(attachment.ID, 10),
			"messageId":   strconv.FormatInt(messageID, 10),
			"filename":    attachment.Filename,
			"contentType": nullableString(attachment.ContentType),
			"contentId":   nullableString(attachment.ContentID),
			"inline":      inline,
			"size":        attachment.Size,
			"downloadUrl": fmt.Sprintf("/api/v1/messages/%d/attachments/%d/content", messageID, attachment.ID),
		})
	}
	return items
}

func rewriteInlineCIDImages(htmlBody string, attachments []storage.MailAttachment, inlineContentIDs map[string]bool, userID, messageID int64, secret string) string {
	if strings.TrimSpace(htmlBody) == "" {
		return htmlBody
	}
	replacements := make(map[string]string)
	for _, attachment := range attachments {
		contentID := nullableStringValue(attachment.ContentID)
		normalizedContentID := normalizeContentID(contentID)
		if normalizedContentID == "" || (!attachment.Inline && !inlineContentIDs[normalizedContentID]) {
			continue
		}
		contentType := "application/octet-stream"
		if attachment.ContentType.Valid && strings.TrimSpace(attachment.ContentType.String) != "" {
			contentType = attachment.ContentType.String
		}
		if browserCanDisplayImage(contentType, attachment.FilePath) {
			replacements[normalizedContentID] = inlineAttachmentContentURL(secret, userID, messageID, attachment.ID)
			continue
		}
		// 浏览器不能直接显示 TIFF 等格式时，小图转成 data URL；大图给占位图，避免详情接口超时。
		if attachment.Size > maxInlineImageTransformBytes {
			replacements[normalizedContentID] = inlineImagePlaceholderDataURL("内嵌图片较大，请在附件中查看")
			continue
		}
		data, err := os.ReadFile(attachment.FilePath)
		if err != nil {
			continue
		}
		if len(data) > maxInlineImageTransformBytes {
			replacements[normalizedContentID] = inlineImagePlaceholderDataURL("内嵌图片较大，请在附件中查看")
			continue
		}
		contentType, data = browserDisplayableInlineImage(contentType, attachment.FilePath, data)
		replacements[normalizedContentID] = "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return rewriteCIDReferences(htmlBody, replacements)
}

func referencedInlineContentIDs(htmlBody string, attachments []storage.MailAttachment) map[string]bool {
	referenced := make(map[string]bool)
	if strings.TrimSpace(htmlBody) == "" || len(attachments) == 0 {
		return referenced
	}
	htmlContentIDs := extractCIDReferences(htmlBody)
	for _, attachment := range attachments {
		contentID := nullableStringValue(attachment.ContentID)
		normalizedContentID := normalizeContentID(contentID)
		if normalizedContentID == "" {
			continue
		}
		if htmlContentIDs[normalizedContentID] {
			referenced[normalizedContentID] = true
		}
	}
	return referenced
}

func rewriteCIDReferences(htmlBody string, replacements map[string]string) string {
	return cidReferencePattern.ReplaceAllStringFunc(htmlBody, func(reference string) string {
		normalizedContentID := normalizeContentID(reference)
		if replacement, ok := replacements[normalizedContentID]; ok {
			return replacement
		}
		return inlineImagePlaceholderDataURL("内嵌图片缺失")
	})
}

func inlineAttachmentContentURL(secret string, userID, messageID, attachmentID int64) string {
	expiresAt := time.Now().Add(inlineAttachmentURLTTL).Unix()
	signature := inlineAttachmentSignature(secret, userID, messageID, attachmentID, expiresAt)
	return fmt.Sprintf(
		"/api/v1/messages/%d/attachments/%d/inline-content?uid=%d&exp=%d&sig=%s",
		messageID,
		attachmentID,
		userID,
		expiresAt,
		url.QueryEscape(signature),
	)
}

func validInlineAttachmentSignature(secret string, userID, messageID, attachmentID, expiresAt int64, signature string) bool {
	if strings.TrimSpace(signature) == "" {
		return false
	}
	expected := inlineAttachmentSignature(secret, userID, messageID, attachmentID, expiresAt)
	return hmac.Equal([]byte(signature), []byte(expected))
}

func inlineAttachmentSignature(secret string, userID, messageID, attachmentID, expiresAt int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d:%d:%d:%d", userID, messageID, attachmentID, expiresAt)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func extractCIDReferences(htmlBody string) map[string]bool {
	contentIDs := make(map[string]bool)
	for _, reference := range cidReferencePattern.FindAllString(htmlBody, -1) {
		normalizedContentID := normalizeContentID(reference)
		if normalizedContentID != "" {
			contentIDs[normalizedContentID] = true
		}
	}
	return contentIDs
}

func normalizeContentID(contentID string) string {
	contentID = strings.TrimSpace(contentID)
	if decoded, err := url.PathUnescape(contentID); err == nil {
		contentID = decoded
	}
	contentID = strings.TrimSpace(contentID)
	if strings.HasPrefix(strings.ToLower(contentID), "cid:") {
		contentID = strings.TrimSpace(contentID[4:])
	}
	contentID = strings.TrimPrefix(strings.TrimSuffix(contentID, ">"), "<")
	return strings.ToLower(contentID)
}

func browserDisplayableInlineImage(contentType string, filePath string, data []byte) (string, []byte) {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	extension := strings.ToLower(filepath.Ext(filePath))
	if mediaType != "image/tiff" && mediaType != "image/tif" && extension != ".tif" && extension != ".tiff" {
		return contentType, data
	}

	image, err := tiff.Decode(bytes.NewReader(data))
	if err != nil {
		return contentType, data
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image); err != nil {
		return contentType, data
	}
	return "image/png", pngData.Bytes()
}

func browserCanDisplayImage(contentType string, filePath string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch mediaType {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp", "image/svg+xml", "image/bmp", "image/x-icon":
		return true
	case "image/tiff", "image/tif":
		return false
	}
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".ico":
		return true
	default:
		return false
	}
}

func missingInlineImagePlaceholderDataURL() string {
	return inlineImagePlaceholderDataURL("内嵌图片缺失")
}

func inlineImagePlaceholderDataURL(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		text = "内嵌图片不可显示"
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="260" height="52" viewBox="0 0 260 52"><rect width="260" height="52" rx="6" fill="#f8fafc" stroke="#cbd5e1"/><text x="50%%" y="50%%" dominant-baseline="middle" text-anchor="middle" fill="#64748b" font-size="14" font-family="Arial, sans-serif">%s</text></svg>`, htmlEscapeText(text))
	return "data:image/svg+xml;charset=utf-8," + url.PathEscape(svg)
}

func htmlEscapeText(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	return value
}
