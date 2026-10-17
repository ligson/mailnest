package storage

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) ListContactGroups(ctx context.Context, userID int64, keyword string, limit, offset int) ([]ContactGroup, int64, error) {
	db := s.db.gormDB.WithContext(ctx).Model(&ContactGroup{}).Where("user_id = ?", userID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		db = db.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	groups := []ContactGroup{}
	err := db.Select("contact_groups.*, (SELECT COUNT(*) FROM contact_group_members gm JOIN contacts c ON c.id=gm.contact_id AND c.user_id=gm.user_id WHERE gm.user_id=contact_groups.user_id AND gm.group_id=contact_groups.id) AS member_count").Order("name_key, id").Limit(limit).Offset(offset).Find(&groups).Error
	return groups, total, err
}

func (s *Store) FindContactGroup(ctx context.Context, userID, id int64) (ContactGroup, []ContactGroupAddress, error) {
	var group ContactGroup
	members := []ContactGroupAddress{}
	err := s.db.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id=? AND id=?", userID, id).First(&group).Error; err != nil {
			return groupNotFound(err)
		}
		return groupAddresses(tx, userID, []int64{id}, &members)
	})
	group.MemberCount = int64(len(members))
	return group, members, err
}

func groupNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func groupAddresses(tx *gorm.DB, userID int64, groupIDs []int64, out *[]ContactGroupAddress) error {
	return tx.Table("contacts c").Select("c.id,c.email,COALESCE(c.display_name,'') AS display_name,COALESCE(c.nickname,'') AS nickname").Where("c.user_id=? AND EXISTS (SELECT 1 FROM contact_group_members gm WHERE gm.user_id=c.user_id AND gm.contact_id=c.id AND gm.group_id IN ?)", userID, groupIDs).Order("c.email_key, c.id").Scan(out).Error
}

// 确认选择时读取当前成员；同一联系人属于多个组也只返回一次。
func (s *Store) ResolveContactGroups(ctx context.Context, userID int64, groupIDs []int64) ([]ContactGroupAddress, error) {
	result := []ContactGroupAddress{}
	err := s.db.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&ContactGroup{}).Where("user_id=? AND id IN ?", userID, groupIDs).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(groupIDs)) {
			return ErrNotFound
		}
		return groupAddresses(tx, userID, groupIDs, &result)
	})
	return result, err
}

func lockGroup(tx *gorm.DB, userID, id int64) (ContactGroup, error) {
	var group ContactGroup
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id=? AND id=?", userID, id).First(&group).Error
	return group, groupNotFound(err)
}

func validateGroupContacts(tx *gorm.DB, userID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	var owned []int64
	// 顺序加锁，避免并发删除联系人后留下悬空关系。
	for start := 0; start < len(ids); start += 200 {
		end := min(start+200, len(ids))
		var batch []int64
		if err := tx.Model(&contactModel{}).Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id=? AND id IN ?", userID, ids[start:end]).Order("id").Pluck("id", &batch).Error; err != nil {
			return err
		}
		owned = append(owned, batch...)
	}
	if len(owned) != len(ids) {
		return ErrContactGroupMembers
	}
	return nil
}

func insertGroupMembers(tx *gorm.DB, userID, groupID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	members := make([]contactGroupMember, 0, len(ids))
	for _, id := range ids {
		members = append(members, contactGroupMember{UserID: userID, GroupID: groupID, ContactID: id})
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&members, 200).Error
}

func (s *Store) SaveContactGroup(ctx context.Context, userID, id int64, input ContactGroupInput) (ContactGroup, error) {
	var group ContactGroup
	err := s.db.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if id > 0 {
			var err error
			group, err = lockGroup(tx, userID, id)
			if err != nil {
				return err
			}
			if group.Revision != input.Revision {
				return ErrContactGroupChanged
			}
		}
		key := strings.ToLower(strings.TrimSpace(input.Name))
		var duplicate int64
		if err := tx.Model(&ContactGroup{}).Where("user_id=? AND name_key=? AND id<>?", userID, key, id).Count(&duplicate).Error; err != nil {
			return err
		}
		if duplicate > 0 {
			return ErrContactGroupName
		}
		if err := validateGroupContacts(tx, userID, input.ContactIDs); err != nil {
			return err
		}
		if id == 0 {
			group = ContactGroup{UserID: userID, Name: input.Name, NameKey: key, Description: input.Description, Revision: 1}
			if err := tx.Create(&group).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&group).Updates(map[string]any{"name": input.Name, "name_key": key, "description": input.Description, "revision": group.Revision + 1}).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id=? AND group_id=?", userID, id).Delete(&contactGroupMember{}).Error; err != nil {
				return err
			}
			// 显式返回持久化后的修订号，避免 GORM 对表达式更新的回填差异。
			if err := tx.Where("user_id=? AND id=?", userID, id).First(&group).Error; err != nil {
				return err
			}
		}
		return insertGroupMembers(tx, userID, group.ID, input.ContactIDs)
	})
	group.MemberCount = int64(len(input.ContactIDs))
	return group, err
}

func (s *Store) ChangeContactGroupMembers(ctx context.Context, userID, id int64, ids []int64, remove bool) error {
	return s.db.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		group, err := lockGroup(tx, userID, id)
		if err != nil {
			return err
		}
		if err = validateGroupContacts(tx, userID, ids); err != nil {
			return err
		}
		if remove {
			err = tx.Where("user_id=? AND group_id=? AND contact_id IN ?", userID, id, ids).Delete(&contactGroupMember{}).Error
		} else {
			err = insertGroupMembers(tx, userID, id, ids)
		}
		if err != nil {
			return err
		}
		return tx.Model(&group).Update("revision", group.Revision+1).Error
	})
}

func (s *Store) DeleteContactGroup(ctx context.Context, userID, id int64) error {
	return s.db.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockGroup(tx, userID, id); err != nil {
			return err
		}
		if err := tx.Where("user_id=? AND group_id=?", userID, id).Delete(&contactGroupMember{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id=? AND id=?", userID, id).Delete(&ContactGroup{}).Error
	})
}
