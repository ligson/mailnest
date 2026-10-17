package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestContactGroupsMembershipIsolationAndMigration(t *testing.T) {
	s := performanceStore(t)
	ctx := context.Background()
	suffix := fmt.Sprint(time.Now().UnixNano())
	owner, err := s.CreateUser("groups"+suffix, "groups"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateUser("foreign"+suffix, "foreign"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	contact := func(user int64, email string) Contact {
		c, e := s.CreateContact(CreateContactParams{UserID: user, Email: email, Nickname: "昵称", Source: "manual"})
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	a := contact(owner.ID, "alice@example.com")
	b := contact(owner.ID, "bob@example.com")
	foreign := contact(other.ID, "secret@example.com")
	first, err := s.SaveContactGroup(ctx, owner.ID, 0, ContactGroupInput{Name: "项目组", ContactIDs: []int64{a.ID, b.ID}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SaveContactGroup(ctx, owner.ID, 0, ContactGroupInput{Name: "客户", ContactIDs: []int64{a.ID}})
	if err != nil {
		t.Fatal(err)
	}
	hidden, err := s.SaveContactGroup(ctx, other.ID, 0, ContactGroupInput{Name: "项目组", ContactIDs: []int64{foreign.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveContactGroup(ctx, owner.ID, 0, ContactGroupInput{Name: "项目组"}); !errors.Is(err, ErrContactGroupName) {
		t.Fatalf("同用户重名应冲突: %v", err)
	}
	if _, err = s.SaveContactGroup(ctx, owner.ID, 0, ContactGroupInput{Name: "越权组", ContactIDs: []int64{foreign.ID}}); !errors.Is(err, ErrContactGroupMembers) {
		t.Fatalf("跨用户成员应回滚: %v", err)
	}
	groups, total, err := s.ListContactGroups(ctx, owner.ID, "", 50, 0)
	if err != nil || total != 2 || len(groups) != 2 {
		t.Fatalf("组列表隔离或回滚失败: %v %d %+v", err, total, groups)
	}
	for _, g := range groups {
		if g.ID == first.ID && g.MemberCount != 2 {
			t.Fatal("成员计数不准确")
		}
	}
	if _, _, err = s.FindContactGroup(ctx, other.ID, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("可访问异用户组", err)
	}
	if _, err = s.ResolveContactGroups(ctx, owner.ID, []int64{first.ID, hidden.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatal("可展开异用户组", err)
	}
	resolved, err := s.ResolveContactGroups(ctx, owner.ID, []int64{first.ID, second.ID})
	if err != nil || len(resolved) != 2 {
		t.Fatalf("多对多展开未去重: %+v %v", resolved, err)
	}
	filtered, count, err := s.ListContacts(ListContactsQuery{UserID: owner.ID, GroupID: second.ID, Limit: 20})
	if err != nil || count != 1 || len(filtered) != 1 || filtered[0].ID != a.ID {
		t.Fatal("分组筛选错误", err)
	}
	if _, _, err = s.ListContacts(ListContactsQuery{UserID: other.ID, GroupID: first.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatal("分组筛选越权", err)
	}
	if err = s.ChangeContactGroupMembers(ctx, owner.ID, second.ID, []int64{b.ID}, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveContactGroup(ctx, owner.ID, second.ID, ContactGroupInput{Name: "旧表单", Revision: second.Revision, ContactIDs: []int64{a.ID}}); !errors.Is(err, ErrContactGroupChanged) {
		t.Fatal("旧表单覆盖了新成员", err)
	}
	current, members, err := s.FindContactGroup(ctx, owner.ID, second.ID)
	if err != nil || len(members) != 2 {
		t.Fatal("冲突后成员被覆盖", err)
	}
	if _, err = s.SaveContactGroup(ctx, owner.ID, second.ID, ContactGroupInput{Name: "客户更新", Revision: current.Revision, ContactIDs: []int64{a.ID, foreign.ID}}); !errors.Is(err, ErrContactGroupMembers) {
		t.Fatal("跨用户更新未拒绝", err)
	}
	current, members, err = s.FindContactGroup(ctx, owner.ID, second.ID)
	if err != nil || current.Name != "客户" || len(members) != 2 {
		t.Fatal("非法更新未原子回滚", err)
	}
	updated, err := s.SaveContactGroup(ctx, owner.ID, second.ID, ContactGroupInput{Name: "客户更新", Revision: current.Revision, ContactIDs: []int64{a.ID}})
	if err != nil || updated.Revision <= current.Revision || updated.MemberCount != 1 {
		t.Fatal("编辑未正确持久化", err)
	}
	if err = s.migrate(); err != nil {
		t.Fatal("重复迁移失败", err)
	}
	if _, members, err = s.FindContactGroup(ctx, owner.ID, first.ID); err != nil || len(members) != 2 {
		t.Fatal("重复迁移影响数据", err)
	}
	if err = s.DeleteContact(owner.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{first.ID, second.ID} {
		_, members, err = s.FindContactGroup(ctx, owner.ID, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range members {
			if m.ID == a.ID {
				t.Fatal("删除联系人后保留悬空成员")
			}
		}
	}
	if err = s.DeleteContactGroup(ctx, owner.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.FindContactByID(owner.ID, b.ID); err != nil {
		t.Fatal("删除组误删联系人", err)
	}
	if err = s.DeleteContactGroup(ctx, other.ID, second.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("可删除异用户组", err)
	}
}
