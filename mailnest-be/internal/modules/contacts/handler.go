package contacts

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

type Handler struct {
	store *storage.Store
	user  func(*http.Request) (int64, bool)
}

// Register 由统一装配层注入现有鉴权，不另建登录态或响应协议。
func Register(mux *http.ServeMux, store *storage.Store, auth func(http.Handler) http.Handler, user func(*http.Request) (int64, bool)) {
	h := &Handler{store: store, user: user}
	for path, fn := range map[string]http.HandlerFunc{
		"PATCH /api/v1/contacts/batch":             h.batchUpdate,
		"GET /api/v1/contact-groups":               h.list,
		"POST /api/v1/contact-groups":              h.save,
		"POST /api/v1/contact-groups/resolve":      h.resolve,
		"GET /api/v1/contact-groups/{id}":          h.detail,
		"PUT /api/v1/contact-groups/{id}":          h.save,
		"DELETE /api/v1/contact-groups/{id}":       h.remove,
		"POST /api/v1/contact-groups/{id}/members": h.changeMembers,
	} {
		mux.Handle(path, auth(fn))
	}
}

func groupPayload(g storage.ContactGroup) map[string]any {
	return map[string]any{"id": strconv.FormatInt(g.ID, 10), "name": g.Name, "description": g.Description, "revision": g.Revision, "memberCount": g.MemberCount, "createdAt": g.CreatedAt, "updatedAt": g.UpdatedAt}
}

func addressesPayload(members []storage.ContactGroupAddress) []map[string]any {
	items := make([]map[string]any, 0, len(members))
	for _, c := range members {
		name := c.Nickname
		if name == "" {
			name = c.DisplayName
		}
		if name == "" {
			name = c.Email
		}
		items = append(items, map[string]any{"id": strconv.FormatInt(c.ID, 10), "email": c.Email, "displayName": c.DisplayName, "nickname": c.Nickname, "name": name})
	}
	return items
}

func fail(w http.ResponseWriter, r *http.Request, userID int64, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		response.Error(w, 404, "分组不存在或无权访问")
	case errors.Is(err, storage.ErrContactGroupMembers):
		response.Error(w, 400, err.Error())
	case errors.Is(err, storage.ErrContactGroupName), errors.Is(err, storage.ErrContactGroupChanged):
		response.Error(w, 409, err.Error())
	default:
		if r.Context().Err() != nil {
			return
		}
		log.Printf("联系人分组操作失败 user_id=%d method=%s error=%v", userID, r.Method, err)
		response.Error(w, 500, "联系人分组操作失败，请稍后重试")
	}
}

func idFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, 400, "分组 ID 格式错误")
		return 0, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		response.Error(w, 400, "请求参数格式错误")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		response.Error(w, 400, "请求参数格式错误")
		return false
	}
	return true
}

func parseIDs(w http.ResponseWriter, raw []string, maxSize int) ([]int64, bool) {
	if len(raw) > maxSize {
		response.Error(w, 400, "本次选择过多，请分批操作")
		return nil, false
	}
	ids := make([]int64, 0, len(raw))
	seen := map[int64]bool{}
	for _, v := range raw {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.Error(w, 400, "所选 ID 格式错误")
			return nil, false
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.user(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if size < 1 || size > 100 {
		size = 50
	}
	if page > 1000000 {
		response.Error(w, 400, "页码过大")
		return
	}
	items, total, err := h.store.ListContactGroups(r.Context(), userID, r.URL.Query().Get("keyword"), size, (page-1)*size)
	if err != nil {
		fail(w, r, userID, err)
		return
	}
	payload := make([]map[string]any, 0, len(items))
	for _, g := range items {
		payload = append(payload, groupPayload(g))
	}
	response.OK(w, "获取成功", map[string]any{"items": payload, "total": total, "page": page, "pageSize": size})
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.user(r)
	id, ok := idFromPath(w, r)
	if !ok {
		return
	}
	group, members, err := h.store.FindContactGroup(r.Context(), userID, id)
	if err != nil {
		fail(w, r, userID, err)
		return
	}
	response.OK(w, "获取成功", map[string]any{"group": groupPayload(group), "members": addressesPayload(members)})
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.user(r)
	var id int64
	if r.Method == http.MethodPut {
		var ok bool
		id, ok = idFromPath(w, r)
		if !ok {
			return
		}
	}
	var input struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		ContactIDs  []string `json:"contactIds"`
		Revision    int64    `json:"revision"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 80 || utf8.RuneCountInString(input.Description) > 500 {
		response.Error(w, 400, "分组名称须为 1～80 个字符，备注不能超过 500 个字符")
		return
	}
	ids, ok := parseIDs(w, input.ContactIDs, 10000)
	if !ok {
		return
	}
	group, err := h.store.SaveContactGroup(r.Context(), userID, id, storage.ContactGroupInput{Name: input.Name, Description: input.Description, ContactIDs: ids, Revision: input.Revision})
	if err != nil {
		fail(w, r, userID, err)
		return
	}
	log.Printf("联系人分组已保存 user_id=%d group_id=%d member_count=%d", userID, group.ID, len(ids))
	if id == 0 {
		response.Created(w, "分组已创建", groupPayload(group))
	} else {
		response.OK(w, "分组已更新", groupPayload(group))
	}
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.user(r)
	id, ok := idFromPath(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteContactGroup(r.Context(), userID, id); err != nil {
		fail(w, r, userID, err)
		return
	}
	log.Printf("联系人分组已删除 user_id=%d group_id=%d", userID, id)
	response.OK(w, "分组已删除，联系人保留", nil)
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.user(r)
	var input struct {
		GroupIDs []string `json:"groupIds"`
	}
	if !decode(w, r, &input) {
		return
	}
	ids, ok := parseIDs(w, input.GroupIDs, 100)
	if !ok {
		return
	}
	if len(ids) == 0 {
		response.Error(w, 400, "请选择联系人分组")
		return
	}
	members, err := h.store.ResolveContactGroups(r.Context(), userID, ids)
	if err != nil {
		fail(w, r, userID, err)
		return
	}
	response.OK(w, "获取成功", map[string]any{"items": addressesPayload(members)})
}

func (h *Handler) changeMembers(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.user(r)
	id, ok := idFromPath(w, r)
	if !ok {
		return
	}
	var input struct {
		ContactIDs []string `json:"contactIds"`
		Action     string   `json:"action"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Action != "add" && input.Action != "remove" {
		response.Error(w, 400, "请选择加入分组或移出分组")
		return
	}
	ids, ok := parseIDs(w, input.ContactIDs, 10000)
	if !ok {
		return
	}
	if len(ids) == 0 {
		response.Error(w, 400, "请选择联系人")
		return
	}
	if err := h.store.ChangeContactGroupMembers(r.Context(), userID, id, ids, input.Action == "remove"); err != nil {
		fail(w, r, userID, err)
		return
	}
	response.OK(w, "分组成员已更新", nil)
}
