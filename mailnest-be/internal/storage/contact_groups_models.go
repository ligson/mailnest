package storage

import (
	"errors"
	"time"
)

var (
	ErrContactGroupName    = errors.New("分组名称已存在")
	ErrContactGroupChanged = errors.New("分组已被修改，请重新打开后再保存")
	ErrContactGroupMembers = errors.New("所选联系人不存在或不属于当前用户")
)

// 分组是用户私有通讯录数据；多对多成员独立存储，不修改原联系人表。
type ContactGroup struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	UserID      int64  `gorm:"not null;uniqueIndex:idx_contact_group_name,priority:1"`
	Name        string `gorm:"size:80;not null"`
	NameKey     string `gorm:"size:80;not null;uniqueIndex:idx_contact_group_name,priority:2"`
	Description string `gorm:"type:text;not null"`
	Revision    int64  `gorm:"not null;default:1"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	MemberCount int64 `gorm:"->;-:migration"`
}

func (ContactGroup) TableName() string { return "contact_groups" }

type contactGroupMember struct {
	UserID    int64 `gorm:"primaryKey;autoIncrement:false;index:idx_contact_group_contact,priority:1"`
	GroupID   int64 `gorm:"primaryKey;autoIncrement:false"`
	ContactID int64 `gorm:"primaryKey;autoIncrement:false;index:idx_contact_group_contact,priority:2"`
}

func (contactGroupMember) TableName() string { return "contact_group_members" }

type ContactGroupInput struct {
	Name, Description string
	ContactIDs        []int64
	Revision          int64
}

// 选组及成员编辑只读取地址摘要，不加载备注、邮件正文或整个通讯录。
type ContactGroupAddress struct {
	ID          int64
	Email       string
	DisplayName string
	Nickname    string
}
