package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"mailnest-be/internal/mail"
)

const maxComposeAttachmentCount = 20

const maxComposeAttachmentBytes = 25 << 20

const maxImportEMLBytes = 50 << 20

const maxImportEMLBatchBytes = 200 << 20

const maxImportEMLBatchCount = 100

type importEMLUpload struct {
	Filename string
	Data     []byte
	Error    string
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

// decodeSendMessageRequest 同时兼容 JSON 和 multipart 两种写信请求：
// 普通邮件走 JSON，带本地附件上传时走 multipart。
func decodeSendMessageRequest(r *http.Request) (sendMessageRequest, []mail.OutgoingAttachment, error) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		var req sendMessageRequest
		if err := decodeJSON(r, &req); err != nil {
			return sendMessageRequest{}, nil, errors.New("请求参数格式错误")
		}
		return req, nil, nil
	}

	if err := r.ParseMultipartForm(maxComposeAttachmentBytes + 1<<20); err != nil {
		return sendMessageRequest{}, nil, errors.New("读取发信表单失败")
	}
	form := r.MultipartForm
	req := sendMessageRequest{
		DraftID:              strings.TrimSpace(formValue(form, "draftId")),
		AccountID:            strings.TrimSpace(formValue(form, "accountId")),
		To:                   formAddressValues(form, "to"),
		CC:                   formAddressValues(form, "cc"),
		BCC:                  formAddressValues(form, "bcc"),
		Subject:              formValue(form, "subject"),
		TextBody:             formValue(form, "textBody"),
		HTMLBody:             formValue(form, "htmlBody"),
		ComposeMode:          formValue(form, "composeMode"),
		SourceMessageID:      formValue(form, "sourceMessageId"),
		ForwardAttachmentIDs: formStringValues(form, "forwardAttachmentIds"),
	}
	attachments, err := readComposeAttachments(form)
	if err != nil {
		return sendMessageRequest{}, nil, err
	}
	return req, attachments, nil
}

func formStringValues(form *multipart.Form, key string) []string {
	raw := strings.TrimSpace(formValue(form, key))
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '，' || r == '；'
	})
}

func formValue(form *multipart.Form, key string) string {
	if form == nil || len(form.Value[key]) == 0 {
		return ""
	}
	return form.Value[key][0]
}

func formAddressValues(form *multipart.Form, key string) []string {
	raw := strings.TrimSpace(formValue(form, key))
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '，' || r == '；'
	})
}

func readComposeAttachments(form *multipart.Form) ([]mail.OutgoingAttachment, error) {
	if form == nil {
		return nil, nil
	}
	fileHeaders := form.File["attachments"]
	if len(fileHeaders) > maxComposeAttachmentCount {
		return nil, fmt.Errorf("附件不能超过 %d 个", maxComposeAttachmentCount)
	}
	// 附件先整体读入内存交给 SMTP 组包，必须在入口限制总大小，避免大文件撑爆进程。
	attachments := make([]mail.OutgoingAttachment, 0, len(fileHeaders))
	var total int64
	for _, header := range fileHeaders {
		if header == nil || strings.TrimSpace(header.Filename) == "" {
			continue
		}
		file, err := header.Open()
		if err != nil {
			return nil, errors.New("读取附件失败")
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxComposeAttachmentBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return nil, errors.New("读取附件失败")
		}
		total += int64(len(data))
		if total > maxComposeAttachmentBytes {
			return nil, fmt.Errorf("附件总大小不能超过 %d MB", maxComposeAttachmentBytes>>20)
		}
		contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		attachments = append(attachments, mail.OutgoingAttachment{
			Filename:    filepath.Base(header.Filename),
			ContentType: contentType,
			Data:        data,
		})
	}
	return attachments, nil
}

func readImportEMLFiles(r *http.Request) ([]importEMLUpload, error) {
	if err := r.ParseMultipartForm(maxImportEMLBatchBytes + 1<<20); err != nil {
		return nil, errors.New("读取 EML 表单失败")
	}
	headers := importEMLFileHeaders(r.MultipartForm)
	if len(headers) == 0 {
		return nil, errors.New("请选择 EML 文件")
	}
	if len(headers) > maxImportEMLBatchCount {
		return nil, fmt.Errorf("一次最多导入 %d 个 EML 文件", maxImportEMLBatchCount)
	}
	items := make([]importEMLUpload, 0, len(headers))
	var total int64
	for _, header := range headers {
		item := importEMLUpload{Filename: importEMLFilename(header)}
		data, err := readImportEMLPart(header)
		if err != nil {
			item.Error = err.Error()
			items = append(items, item)
			continue
		}
		total += int64(len(data))
		if total > maxImportEMLBatchBytes {
			item.Error = fmt.Sprintf("本次导入文件总大小不能超过 %d MB", maxImportEMLBatchBytes>>20)
			items = append(items, item)
			continue
		}
		item.Data = data
		items = append(items, item)
	}
	return items, nil
}

func importEMLFileHeaders(form *multipart.Form) []*multipart.FileHeader {
	if form == nil {
		return nil
	}
	headers := make([]*multipart.FileHeader, 0, len(form.File["file"])+len(form.File["eml"]))
	headers = append(headers, form.File["file"]...)
	headers = append(headers, form.File["eml"]...)
	return headers
}

func readImportEMLPart(header *multipart.FileHeader) ([]byte, error) {
	filename := importEMLFilename(header)
	if filename == "" {
		return nil, errors.New("EML 文件名为空")
	}
	if !strings.EqualFold(filepath.Ext(filename), ".eml") {
		return nil, errors.New("只支持导入 .eml 文件")
	}
	file, err := header.Open()
	if err != nil {
		return nil, errors.New("读取 EML 文件失败")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxImportEMLBytes+1))
	if err != nil {
		return nil, errors.New("读取 EML 文件失败")
	}
	if len(data) == 0 {
		return nil, errors.New("EML 文件为空")
	}
	if len(data) > maxImportEMLBytes {
		return nil, fmt.Errorf("EML 文件不能超过 %d MB", maxImportEMLBytes>>20)
	}
	return data, nil
}

func importEMLFilename(header *multipart.FileHeader) string {
	if header == nil {
		return "import.eml"
	}
	filename := filepath.Base(strings.TrimSpace(header.Filename))
	if filename == "" {
		return "import.eml"
	}
	return filename
}
