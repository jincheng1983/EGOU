// system.go 实现系统级方法：健康检查、窗口全屏/最大化切换、文件管理器打开、文件指纹。
//
// CheckSignature 改用 Go 原生 crypto/sha256 + os.Stat 计算文件指纹
// （规约 §9 禁止 PowerShell，统一 Go 实现）。

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"egou/internal/runner"
)

// OpenInExplorer 在系统文件管理器中打开指定目录。
// 目录不存在时会自动创建。Windows 用 explorer.exe，macOS 用 open，Linux 用 xdg-open。
// 成功返回空字符串，失败返回错误信息。
func (s *IDEService) OpenInExplorer(path string) string {
	if path == "" {
		return "路径为空"
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err.Error()
	}
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return err.Error()
			}
		} else {
			return err.Error()
		}
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", absPath)
	case "darwin":
		cmd = exec.Command("open", absPath)
	default:
		cmd = exec.Command("xdg-open", absPath)
	}
	if err := cmd.Start(); err != nil {
		return err.Error()
	}
	return ""
}

// CheckSignature 计算文件指纹（SHA256 + 大小）。
// 规约 §9 禁止 PowerShell，统一用 Go 原生 crypto/sha256 + os.Stat 实现。
// 返回 JSON 字符串：{ status, sha256, size, sizeText }
//   - status: "Computed"（计算成功）/ "Error"
//   - sha256: 文件 SHA256 哈希（小写 hex，64 字符）
//   - size: 文件字节数
//   - sizeText: 人类可读大小（如 "11.77 MB"）
//
// sigResult 是 CheckSignature 的 JSON 返回结构，用 encoding/json 序列化，
// 避免手拼 JSON 字符串导致的转义错误。
type sigResult struct {
	Status   string `json:"status"`
	SHA256   string `json:"sha256,omitempty"`
	Size     int64  `json:"size,omitempty"`
	SizeText string `json:"sizeText,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (s *IDEService) CheckSignature(filePath string) string {
	makeErr := func(msg string) string {
		b, _ := json.Marshal(sigResult{Status: "Error", Error: msg})
		return string(b)
	}
	if filePath == "" {
		return makeErr("文件路径为空")
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return makeErr("路径解析失败")
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return makeErr("文件不存在")
	}
	if info.IsDir() {
		return makeErr("路径指向目录，非文件")
	}
	f, err := os.Open(absPath)
	if err != nil {
		return makeErr("打开文件失败: " + err.Error())
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return makeErr("计算哈希失败: " + err.Error())
	}
	b, _ := json.Marshal(sigResult{
		Status:   "Computed",
		SHA256:   hex.EncodeToString(h.Sum(nil)),
		Size:     info.Size(),
		SizeText: formatSize(info.Size()),
	})
	return string(b)
}

// formatSize 把字节数格式化为人类可读大小（MB/KB/GB）。
func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// HealthCheck 返回后端关键依赖状态汇总，供前端启动时探活与状态栏展示。
// 该方法直接转发到 runner.HealthCheck()，无副作用、可频繁调用。
func (s *IDEService) HealthCheck() runner.HealthReport {
	return runner.HealthCheck()
}

// ToggleFullscreen 切换 IDE 主窗口全屏/还原状态。
func (s *IDEService) ToggleFullscreen() {
	if s.win == nil {
		return
	}
	s.win.ToggleFullscreen()
}

// ToggleMaximize 切换 IDE 主窗口最大化/还原状态。
func (s *IDEService) ToggleMaximize() {
	if s.win == nil {
		return
	}
	s.win.ToggleMaximise()
}
