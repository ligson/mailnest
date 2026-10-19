package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBatchContactsChunkingAndLimits(t *testing.T) {
	s := performanceStore(t)
	ctx := context.Background()
	suffix := fmt.Sprint(time.Now().UnixNano())
	owner, err := s.CreateUser("chunks"+suffix, "chunks"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]contactModel, 405)
	for i := range rows {
		email := fmt.Sprintf("chunk%d@example.com", i)
		rows[i] = contactModel{UserID: owner.ID, Email: email, EmailKey: email, Source: "manual"}
	}
	if err := s.db.gormDB.CreateInBatches(&rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	g, err := s.SaveContactGroup(ctx, owner.ID, 0, ContactGroupInput{Name: "分块验证"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.BatchUpdateContacts(ctx, owner.ID, ContactBatchInput{ContactIDs: ids, Company: &ContactCompanyChange{Mode: "set", Value: "批量公司"}, Groups: &ContactGroupsChange{Mode: "add", IDs: []int64{g.ID}}})
	if err != nil || result.CompanyUpdatedCount != 405 || result.GroupMembershipChangedCount != 405 {
		t.Fatal("跨批写入错误", result, err)
	}
	for _, i := range []int{0, 200, 404} {
		c, err := s.FindContactByID(owner.ID, ids[i])
		if err != nil || c.Company.String != "批量公司" {
			t.Fatal("首中末联系人错误", i, c, err)
		}
	}
	for _, test := range []struct {
		ids   []int64
		limit int
	}{
		{nil, 100}, {[]int64{0}, 100}, {[]int64{-1}, 100}, {[]int64{1, 2, 3}, 2},
	} {
		if _, err := sortedContactBatchIDs(test.ids, test.limit); !errors.Is(err, ErrContactBatchInput) {
			t.Fatal("未拒绝非法ID或超限", test)
		}
	}
	groups := make([]int64, 101)
	for i := range groups {
		groups[i] = int64(i + 1)
	}
	pairLimitIDs := make([]int64, 501)
	for i := range pairLimitIDs {
		pairLimitIDs[i] = int64(i + 1)
	}
	for _, input := range []ContactBatchInput{
		{ContactIDs: make([]int64, 10001), Company: &ContactCompanyChange{Mode: "clear"}},
		{ContactIDs: ids, Groups: &ContactGroupsChange{Mode: "add", IDs: groups}},
		{ContactIDs: pairLimitIDs, Groups: &ContactGroupsChange{Mode: "add", IDs: groups[:100]}},
	} {
		if _, err := s.BatchUpdateContacts(ctx, owner.ID, input); !errors.Is(err, ErrContactBatchInput) {
			t.Fatal("未拒绝超限请求", err)
		}
	}
}

func TestBatchContactsCompanyGroupsAndIsolation(t *testing.T) {
	s := performanceStore(t)
	ctx := context.Background()
	suffix := fmt.Sprint(time.Now().UnixNano())
	owner, err := s.CreateUser("batch"+suffix, "batch"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateUser("otherbatch"+suffix, "otherbatch"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	contact := func(userID int64, email, company string) Contact {
		t.Helper()
		c, err := s.CreateContact(CreateContactParams{UserID: userID, Email: email, Company: company, Nickname: "昵称", DisplayName: "姓名", Phone: "123", Notes: "备注", Source: "auto"})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	a := contact(owner.ID, "a@example.com", "旧公司")
	b := contact(owner.ID, "b@example.com", "")
	foreign := contact(other.ID, "foreign@example.com", "他人公司")
	group := func(userID int64, name string, ids ...int64) ContactGroup {
		t.Helper()
		g, err := s.SaveContactGroup(ctx, userID, 0, ContactGroupInput{Name: name, ContactIDs: ids})
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	g1 := group(owner.ID, "项目一", a.ID)
	g2 := group(owner.ID, "项目二", b.ID)
	preserved := group(owner.ID, "保持不变", a.ID)
	hidden := group(other.ID, "他人分组", foreign.ID)
	update := func(input ContactBatchInput) ContactBatchResult {
		t.Helper()
		result, err := s.BatchUpdateContacts(ctx, owner.ID, input)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	company := func(id int64) Contact {
		t.Helper()
		c, err := s.FindContactByID(owner.ID, id)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if _, err := s.ResolveContacts(ctx, owner.ID, []string{a.Email, b.Email}); err != nil {
		t.Fatal(err)
	}
	filled := update(ContactBatchInput{ContactIDs: []int64{a.ID, b.ID, b.ID}, Company: &ContactCompanyChange{Mode: "fill_empty", Value: " 新公司 "}})
	if filled.SelectedCount != 2 || filled.CompanyUpdatedCount != 1 || company(a.ID).Company.String != "旧公司" || company(b.ID).Company.String != "新公司" {
		t.Fatal("补全公司或去重错误", filled)
	}
	for _, value := range []string{"Acme", "ACME"} {
		result := update(ContactBatchInput{ContactIDs: []int64{a.ID, b.ID}, Company: &ContactCompanyChange{Mode: "set", Value: value}})
		if result.CompanyUpdatedCount != 2 || company(a.ID).Company.String != value {
			t.Fatal("统一设置或大小写更新失败", result)
		}
	}
	if result := update(ContactBatchInput{ContactIDs: []int64{a.ID, b.ID}, Company: &ContactCompanyChange{Mode: "set", Value: "ACME"}}); result.CompanyUpdatedCount != 0 {
		t.Fatal("重复设置不应计为变化", result)
	}
	resolved, err := s.ResolveContacts(ctx, owner.ID, []string{a.Email})
	if err != nil || len(resolved.Items) != 1 || resolved.Items[0].Company.String != "ACME" {
		t.Fatal("联系人缓存未失效", err)
	}
	result := update(ContactBatchInput{ContactIDs: []int64{a.ID}, Company: &ContactCompanyChange{Mode: "clear"}})
	current := company(a.ID)
	if result.CompanyUpdatedCount != 1 || current.Company.Valid || current.Nickname.String != "昵称" || current.DisplayName.String != "姓名" || current.Phone.String != "123" || current.Notes.String != "备注" || company(b.ID).Company.String != "ACME" {
		t.Fatal("清空公司影响了其他资料", result, current)
	}
	input := ContactBatchInput{ContactIDs: []int64{b.ID, a.ID}, Groups: &ContactGroupsChange{Mode: "add", IDs: []int64{g2.ID, g1.ID, g1.ID}}}
	if result := update(input); result.GroupMembershipChangedCount != 2 {
		t.Fatal("多组加入计数错误", result)
	}
	if result := update(input); result.GroupMembershipChangedCount != 0 {
		t.Fatal("重复加入没有去重", result)
	}
	if _, err := s.SaveContactGroup(ctx, owner.ID, g1.ID, ContactGroupInput{Name: g1.Name, Revision: g1.Revision}); !errors.Is(err, ErrContactGroupChanged) {
		t.Fatal("批量变更未保护旧表单", err)
	}
	input.ContactIDs = []int64{a.ID}
	input.Groups.Mode = "remove"
	if result := update(input); result.GroupMembershipChangedCount != 2 {
		t.Fatal("多组移出计数错误", result)
	}
	_, members, err := s.FindContactGroup(ctx, owner.ID, preserved.ID)
	if err != nil || len(members) != 1 || members[0].ID != a.ID {
		t.Fatal("移出操作影响其他分组", err)
	}
	for _, bad := range []ContactBatchInput{
		{ContactIDs: []int64{a.ID, foreign.ID}, Company: &ContactCompanyChange{Mode: "set", Value: "不可写入"}},
		{ContactIDs: []int64{a.ID}, Company: &ContactCompanyChange{Mode: "set", Value: "不可写入"}, Groups: &ContactGroupsChange{Mode: "add", IDs: []int64{g1.ID, hidden.ID}}},
		{ContactIDs: []int64{a.ID, 9223372036854775806}, Company: &ContactCompanyChange{Mode: "set", Value: "不可写入"}},
	} {
		result, err := s.BatchUpdateContacts(ctx, owner.ID, bad)
		if err == nil || result != (ContactBatchResult{}) || company(a.ID).Company.Valid {
			t.Fatal("无效成员/越权没有整批回滚", err, result)
		}
	}
	for _, bad := range []ContactBatchInput{
		{}, {ContactIDs: []int64{a.ID}},
		{ContactIDs: []int64{a.ID}, Company: &ContactCompanyChange{Mode: "bad"}},
		{ContactIDs: []int64{a.ID}, Company: &ContactCompanyChange{Mode: "set", Value: " "}},
		{ContactIDs: []int64{a.ID}, Company: &ContactCompanyChange{Mode: "set", Value: strings.Repeat("中", 121)}},
		{ContactIDs: []int64{a.ID}, Company: &ContactCompanyChange{Mode: "clear", Value: "不应带值"}},
		{ContactIDs: []int64{a.ID}, Groups: &ContactGroupsChange{Mode: "replace", IDs: []int64{g1.ID}}},
		{ContactIDs: []int64{a.ID}, Groups: &ContactGroupsChange{Mode: "add"}},
	} {
		if _, err := s.BatchUpdateContacts(ctx, owner.ID, bad); !errors.Is(err, ErrContactBatchInput) {
			t.Fatal("无效参数应拒绝", err, bad)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.BatchUpdateContacts(cancelled, owner.ID, ContactBatchInput{ContactIDs: []int64{a.ID}, Company: &ContactCompanyChange{Mode: "set", Value: "取消"}}); err == nil || company(a.ID).Company.Valid {
		t.Fatal("取消请求后产生写入", err)
	}
	combined := update(ContactBatchInput{ContactIDs: []int64{a.ID}, Company: &ContactCompanyChange{Mode: "set", Value: "最终公司"}, Groups: &ContactGroupsChange{Mode: "add", IDs: []int64{g1.ID, g2.ID}}})
	if combined.CompanyUpdatedCount != 1 || combined.GroupMembershipChangedCount != 2 {
		t.Fatal("公司与分组原子更新失败", combined)
	}
	untouched, err := s.FindContactByID(other.ID, foreign.ID)
	if err != nil || untouched.Company.String != "他人公司" {
		t.Fatal("修改了其他用户", err)
	}
}
