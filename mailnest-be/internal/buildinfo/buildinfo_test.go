package buildinfo

import "testing"

func TestCurrent(t *testing.T) {
	oldVersion, oldCommit, oldTime := Version, Commit, BuildTime
	t.Cleanup(func() { Version, Commit, BuildTime = oldVersion, oldCommit, oldTime })
	Version, Commit, BuildTime = "", "", " "
	info := Current()
	if info.Version != "0.3.0" || info.Commit != "unknown" || info.BuildTime != "unknown" {
		t.Fatalf("默认版本信息不正确：%+v", info)
	}
	Version, Commit, BuildTime = " 0.2.1 ", "abc1234", "2026-10-17T08:00:00Z"
	info = Current()
	if info.Version != "0.2.1" || info.Commit != Commit || info.BuildTime != BuildTime {
		t.Fatalf("未使用构建注入信息：%+v", info)
	}
}
