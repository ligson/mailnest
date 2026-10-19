package contacts

import (
	"errors"
	"log"
	"net/http"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (h *Handler) batchUpdate(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.user(r)
	var input struct {
		ContactIDs []string                      `json:"contactIds"`
		Company    *storage.ContactCompanyChange `json:"company"`
		Groups     *struct {
			Mode     string   `json:"mode"`
			GroupIDs []string `json:"groupIds"`
		} `json:"groups"`
	}
	if !decode(w, r, &input) {
		return
	}
	ids, ok := parseIDs(w, input.ContactIDs, 10000)
	if !ok {
		return
	}
	change := storage.ContactBatchInput{ContactIDs: ids, Company: input.Company}
	if input.Groups != nil {
		groupIDs, ok := parseIDs(w, input.Groups.GroupIDs, 100)
		if !ok {
			return
		}
		change.Groups = &storage.ContactGroupsChange{Mode: input.Groups.Mode, IDs: groupIDs}
	}
	result, err := h.store.BatchUpdateContacts(r.Context(), userID, change)
	if errors.Is(err, storage.ErrContactBatchInput) {
		response.Error(w, 400, "请选择联系人及有效的批量操作；公司名称限 120 字，单次最多 10000 人、100 个分组及 50000 对成员关系")
		return
	}
	if err != nil {
		fail(w, r, userID, err)
		return
	}
	log.Printf("联系人批量设置完成 user_id=%d selected=%d company_updated=%d membership_changed=%d", userID, result.SelectedCount, result.CompanyUpdatedCount, result.GroupMembershipChangedCount)
	response.OK(w, "批量设置成功", result)
}
