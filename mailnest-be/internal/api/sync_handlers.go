package api

import (
	"errors"
	"net/http"
	"strconv"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleListSyncJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 50)
	accountID, _ := strconv.ParseInt(r.URL.Query().Get("accountId"), 10, 64)
	jobs, total, err := a.store.ListSyncJobs(storage.ListSyncJobsQuery{
		UserID:    userID,
		AccountID: accountID,
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取同步任务失败")
		return
	}
	items := make([]map[string]any, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, syncJobPayload(job))
	}
	response.OK(w, "获取成功", map[string]any{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}

func (a *App) handleSyncJobDetail(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	jobID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || jobID <= 0 {
		response.Error(w, http.StatusBadRequest, "同步任务 ID 格式错误")
		return
	}
	job, err := a.store.FindSyncJobByID(userID, jobID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "同步任务不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取同步任务失败")
		return
	}
	response.OK(w, "获取成功", syncJobPayload(job))
}

func (a *App) handleSyncJobEvents(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	jobID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || jobID <= 0 {
		response.Error(w, http.StatusBadRequest, "同步任务 ID 格式错误")
		return
	}
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 100)
	events, total, err := a.store.ListSyncJobEvents(storage.ListSyncJobEventsQuery{
		UserID: userID,
		JobID:  jobID,
		Level:  r.URL.Query().Get("level"),
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取同步日志失败")
		return
	}
	items := make([]map[string]any, 0, len(events))
	for _, event := range events {
		items = append(items, syncJobEventPayload(event))
	}
	response.OK(w, "获取成功", map[string]any{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}
