package storage

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

var ErrContactBatchInput = errors.New("批量设置参数无效")

type ContactCompanyChange struct {
	Mode  string `json:"mode"`
	Value string `json:"value"`
}

type ContactGroupsChange struct {
	Mode string
	IDs  []int64
}

type ContactBatchInput struct {
	ContactIDs []int64
	Company    *ContactCompanyChange
	Groups     *ContactGroupsChange
}

type ContactBatchResult struct {
	SelectedCount               int   `json:"selectedCount"`
	CompanyUpdatedCount         int64 `json:"companyUpdatedCount"`
	GroupMembershipChangedCount int64 `json:"groupMembershipChangedCount"`
}

func sortedContactBatchIDs(ids []int64, limit int) ([]int64, error) {
	if len(ids) == 0 || len(ids) > limit {
		return nil, ErrContactBatchInput
	}
	seen := make(map[int64]bool, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrContactBatchInput
		}
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

// BatchUpdateContacts 只修改明确选中的字段；归属验证、公司和所有分组变更在同一事务。
func (s *Store) BatchUpdateContacts(ctx context.Context, userID int64, input ContactBatchInput) (ContactBatchResult, error) {
	result := ContactBatchResult{}
	ids, err := sortedContactBatchIDs(input.ContactIDs, 10000)
	if err != nil || (input.Company == nil && input.Groups == nil) {
		return result, ErrContactBatchInput
	}
	companyValue := ""
	if input.Company != nil {
		companyValue = strings.TrimSpace(input.Company.Value)
		if input.Company.Mode != "set" && input.Company.Mode != "fill_empty" && input.Company.Mode != "clear" {
			return result, ErrContactBatchInput
		}
		if utf8.RuneCountInString(companyValue) > 120 || (input.Company.Mode != "clear" && companyValue == "") || (input.Company.Mode == "clear" && companyValue != "") {
			return result, ErrContactBatchInput
		}
	}
	var groupIDs []int64
	if input.Groups != nil {
		if input.Groups.Mode != "add" && input.Groups.Mode != "remove" {
			return result, ErrContactBatchInput
		}
		groupIDs, err = sortedContactBatchIDs(input.Groups.IDs, 100)
		if err != nil || len(groupIDs)*len(ids) > 50000 {
			return result, ErrContactBatchInput
		}
	}
	err = s.db.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 与既有分组维护保持“分组→联系人”的顺序；多组按 ID 加锁，避免反向选组死锁。
		for _, id := range groupIDs {
			if _, err := lockGroup(tx, userID, id); err != nil {
				return err
			}
		}
		if err := validateGroupContacts(tx, userID, ids); err != nil {
			return err
		}
		for start := 0; start < len(ids); start += 200 {
			batch := ids[start:min(start+200, len(ids))]
			if input.Company != nil {
				count, err := updateContactCompanies(tx, userID, batch, input.Company.Mode, companyValue)
				if err != nil {
					return err
				}
				result.CompanyUpdatedCount += count
			}
			for _, groupID := range groupIDs {
				if input.Groups.Mode == "remove" {
					deleted := tx.Where("user_id=? AND group_id=? AND contact_id IN ?", userID, groupID, batch).Delete(&contactGroupMember{})
					if deleted.Error != nil {
						return deleted.Error
					}
					result.GroupMembershipChangedCount += deleted.RowsAffected
				} else {
					// 先读取现有关系，重试或重复加入不改动其他组，也不重复计算已有成员。
					var existing []int64
					if err := tx.Model(&contactGroupMember{}).Where("user_id=? AND group_id=? AND contact_id IN ?", userID, groupID, batch).Pluck("contact_id", &existing).Error; err != nil {
						return err
					}
					present := make(map[int64]bool, len(existing))
					for _, id := range existing {
						present[id] = true
					}
					missing := make([]int64, 0, len(batch))
					for _, id := range batch {
						if !present[id] {
							missing = append(missing, id)
						}
					}
					if err := insertGroupMembers(tx, userID, groupID, missing); err != nil {
						return err
					}
					result.GroupMembershipChangedCount += int64(len(missing))
				}
			}
		}
		for _, id := range groupIDs {
			if err := tx.Model(&ContactGroup{}).Where("user_id=? AND id=?", userID, id).Update("revision", gorm.Expr("revision + 1")).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ContactBatchResult{}, err
	}
	result.SelectedCount = len(ids)
	if result.CompanyUpdatedCount > 0 {
		// 旧 SQL 包装层无法观察 GORM 写入，必须在事务成功后失效当前用户联系人缓存。
		s.db.cacheVersions.bump(cacheMutation{domain: "contacts", userID: userID})
	}
	return result, nil
}

func updateContactCompanies(tx *gorm.DB, userID int64, ids []int64, mode, company string) (int64, error) {
	var rows []contactModel
	if err := tx.Select("id", "company").Where("user_id=? AND id IN ?", userID, ids).Find(&rows).Error; err != nil {
		return 0, err
	}
	changed := make([]int64, 0, len(rows))
	for _, row := range rows {
		if mode == "fill_empty" && row.Company != nil && strings.TrimSpace(*row.Company) != "" {
			continue
		}
		// 在 Go 中精确比较，避免 MySQL 不区分大小写的排序规则漏掉 Acme→ACME 这类修改。
		if mode == "clear" {
			if row.Company != nil {
				changed = append(changed, row.ID)
			}
		} else if row.Company == nil || *row.Company != company {
			changed = append(changed, row.ID)
		}
	}
	if len(changed) == 0 {
		return 0, nil
	}
	var value any = company
	if mode == "clear" {
		value = nil
	}
	updated := tx.Model(&contactModel{}).Where("user_id=? AND id IN ?", userID, changed).Updates(map[string]any{"company": value, "source": "manual"})
	return updated.RowsAffected, updated.Error
}
