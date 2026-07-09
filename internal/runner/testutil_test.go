package runner

import (
	"os"
	"testing"
)

// withTempHome 把 HOME / USERPROFILE / LocalAppData 重定向到 t.TempDir()，
// 避免测试污染用户缓存目录（~/.egou、%LocalAppData%\egou 等）。
// 测试结束自动恢复原值（t.Cleanup）。
// 返回临时目录路径，供调用方在临时目录下构造测试数据。
//
// P2-15：吸取 NxEGO6 .trae/rules/project_rules.md §5.1 的测试约定。
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	old := map[string]string{
		"HOME":         os.Getenv("HOME"),
		"USERPROFILE":  os.Getenv("USERPROFILE"),
		"LocalAppData": os.Getenv("LocalAppData"),
	}
	os.Setenv("HOME", dir)
	os.Setenv("USERPROFILE", dir)
	os.Setenv("LocalAppData", dir)

	t.Cleanup(func() {
		for k, v := range old {
			os.Setenv(k, v)
		}
	})
	return dir
}

// setupTestStore 在 withTempHome 的基础上，创建一个空的 egou/runtime-frontend
// 目录结构，用于测试 runtimeCacheDir / binCacheDir 相关逻辑。
// 返回缓存目录路径。
func setupTestStore(t *testing.T) string {
	t.Helper()
	home := withTempHome(t)
	cacheDir, err := runtimeCacheDir()
	if err != nil {
		t.Fatalf("runtimeCacheDir 失败: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("创建缓存目录失败: %v", err)
	}
	_ = home // 仅用于隔离 HOME，不直接使用
	return cacheDir
}
