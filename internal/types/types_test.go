package types

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// ===== Stage 常量值测试 =====

func TestStageConstants(t *testing.T) {
	t.Run("engine:* 命名空间常量", func(t *testing.T) {
		cases := []struct {
			name string
			got  string
			want string
		}{
			{"StageEngineStart", StageEngineStart, "engine:start"},
			{"StageEngineStop", StageEngineStop, "engine:stop"},
			{"StageEngineAttached", StageEngineAttached, "engine:attached"},
			{"StageEngineDetached", StageEngineDetached, "engine:detached"},
			{"StageEngineError", StageEngineError, "engine:error"},
		}
		for _, c := range cases {
			if c.got != c.want {
				t.Fatalf("%s 应为 %q,实际 %q", c.name, c.want, c.got)
			}
		}
	})

	t.Run("node:* 命名空间常量", func(t *testing.T) {
		cases := []struct {
			name string
			got  string
			want string
		}{
			{"StageNodeEnter", StageNodeEnter, "node:enter"},
			{"StageNodeLeave", StageNodeLeave, "node:leave"},
			{"StageNodeError", StageNodeError, "node:error"},
			{"StageNodeOutput", StageNodeOutput, "node:output"},
		}
		for _, c := range cases {
			if c.got != c.want {
				t.Fatalf("%s 应为 %q,实际 %q", c.name, c.want, c.got)
			}
		}
	})

	t.Run("test:* 命名空间常量", func(t *testing.T) {
		cases := []struct {
			name string
			got  string
			want string
		}{
			{"StageTestSuiteStart", StageTestSuiteStart, "test:suite-start"},
			{"StageTestSuiteEnd", StageTestSuiteEnd, "test:suite-end"},
			{"StageTestCaseStart", StageTestCaseStart, "test:case-start"},
			{"StageTestCasePass", StageTestCasePass, "test:case-pass"},
			{"StageTestCaseFail", StageTestCaseFail, "test:case-fail"},
		}
		for _, c := range cases {
			if c.got != c.want {
				t.Fatalf("%s 应为 %q,实际 %q", c.name, c.want, c.got)
			}
		}
	})

	t.Run("action:* 命名空间常量", func(t *testing.T) {
		cases := []struct {
			name string
			got  string
			want string
		}{
			{"StageActionStepOver", StageActionStepOver, "action:step-over"},
			{"StageActionStepInto", StageActionStepInto, "action:step-into"},
			{"StageActionStepOut", StageActionStepOut, "action:step-out"},
			{"StageActionContinue", StageActionContinue, "action:continue"},
			{"StageActionPause", StageActionPause, "action:pause"},
			{"StageActionToggleBP", StageActionToggleBP, "action:toggle-bp"},
		}
		for _, c := range cases {
			if c.got != c.want {
				t.Fatalf("%s 应为 %q,实际 %q", c.name, c.want, c.got)
			}
		}
	})

	t.Run("runtime:* 命名空间常量", func(t *testing.T) {
		if StageRuntimeStdout != "runtime:stdout" {
			t.Fatalf("StageRuntimeStdout 应为 runtime:stdout,实际 %q", StageRuntimeStdout)
		}
		if StageRuntimeStderr != "runtime:stderr" {
			t.Fatalf("StageRuntimeStderr 应为 runtime:stderr,实际 %q", StageRuntimeStderr)
		}
	})

	t.Run("所有 Stage 常量使用 命名空间:子类型 格式", func(t *testing.T) {
		all := []string{
			StageEngineStart, StageEngineStop, StageEngineAttached, StageEngineDetached, StageEngineError,
			StageNodeEnter, StageNodeLeave, StageNodeError, StageNodeOutput,
			StageTestSuiteStart, StageTestSuiteEnd, StageTestCaseStart, StageTestCasePass, StageTestCaseFail,
			StageActionStepOver, StageActionStepInto, StageActionStepOut, StageActionContinue, StageActionPause, StageActionToggleBP,
			StageRuntimeStdout, StageRuntimeStderr,
		}
		for _, s := range all {
			if !strings.Contains(s, ":") {
				t.Fatalf("Stage 常量 %q 不符合 命名空间:子类型 格式", s)
			}
			// 不应以 : 开头或结尾
			if strings.HasPrefix(s, ":") {
				t.Fatalf("Stage 常量 %q 不应以 : 开头", s)
			}
			if strings.HasSuffix(s, ":") {
				t.Fatalf("Stage 常量 %q 不应以 : 结尾", s)
			}
		}
	})

	t.Run("各命名空间前缀互不冲突", func(t *testing.T) {
		namespaces := []string{
			"engine:", "node:", "test:", "action:", "runtime:",
		}
		constants := []string{
			StageEngineStart, StageNodeEnter, StageTestSuiteStart,
			StageActionStepOver, StageRuntimeStdout,
		}
		// 每个 namespace 至少有一个常量以它为前缀
		for i, ns := range namespaces {
			if !strings.HasPrefix(constants[i], ns) {
				t.Fatalf("常量 %q 应以 %q 为前缀", constants[i], ns)
			}
		}
	})
}

// ===== 循环防死循环常量测试 =====

func TestLoopProtectionConstants(t *testing.T) {
	t.Run("MaxLoopIterations 默认值", func(t *testing.T) {
		if MaxLoopIterations != 10000 {
			t.Fatalf("MaxLoopIterations 应为 10000,实际 %d", MaxLoopIterations)
		}
	})

	t.Run("MaxNodeVisits 默认值", func(t *testing.T) {
		if MaxNodeVisits != 500 {
			t.Fatalf("MaxNodeVisits 应为 500,实际 %d", MaxNodeVisits)
		}
	})

	t.Run("MaxLoopIterations 大于 0", func(t *testing.T) {
		if MaxLoopIterations <= 0 {
			t.Fatalf("MaxLoopIterations 应大于 0,实际 %d", MaxLoopIterations)
		}
	})

	t.Run("MaxNodeVisits 大于 0", func(t *testing.T) {
		if MaxNodeVisits <= 0 {
			t.Fatalf("MaxNodeVisits 应大于 0,实际 %d", MaxNodeVisits)
		}
	})

	t.Run("MaxLoopIterations 大于 MaxNodeVisits", func(t *testing.T) {
		// 默认配置下循环迭代上限应大于单节点访问上限
		if MaxLoopIterations <= MaxNodeVisits {
			t.Fatalf("MaxLoopIterations(%d) 应大于 MaxNodeVisits(%d)", MaxLoopIterations, MaxNodeVisits)
		}
	})

	t.Run("EnvMaxLoopIterations 环境变量名", func(t *testing.T) {
		if EnvMaxLoopIterations != "NXG_MAX_LOOP_ITERATIONS" {
			t.Fatalf("EnvMaxLoopIterations 应为 NXG_MAX_LOOP_ITERATIONS,实际 %q", EnvMaxLoopIterations)
		}
	})
}

// ===== Event 结构体测试 =====

func TestEvent(t *testing.T) {
	t.Run("实例化并读取字段", func(t *testing.T) {
		ev := Event{
			Stage:    StageEngineStart,
			Output:   "调试器已启动",
			IsOutput: false,
		}
		if ev.Stage != StageEngineStart {
			t.Fatalf("Stage 应为 %q,实际 %q", StageEngineStart, ev.Stage)
		}
		if ev.Output != "调试器已启动" {
			t.Fatalf("Output 不匹配,实际 %q", ev.Output)
		}
		if ev.IsOutput != false {
			t.Fatal("IsOutput 应为 false")
		}
	})

	t.Run("IsOutput=true 标识运行时输出", func(t *testing.T) {
		ev := Event{
			Stage:    StageRuntimeStdout,
			Output:   "hello world",
			IsOutput: true,
		}
		if !ev.IsOutput {
			t.Fatal("IsOutput 应为 true")
		}
	})

	t.Run("JSON tag 正确(stage/output/isOutput)", func(t *testing.T) {
		ev := Event{
			Stage:    StageNodeEnter,
			Output:   "进入节点",
			IsOutput: true,
		}
		data, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("Marshal 失败: %v", err)
		}
		s := string(data)
		// JSON key 应为 stage(非 Stage)
		if !strings.Contains(s, `"stage"`) {
			t.Fatalf("JSON 应包含 \"stage\" key,实际 %s", s)
		}
		if !strings.Contains(s, `"output"`) {
			t.Fatalf("JSON 应包含 \"output\" key,实际 %s", s)
		}
		if !strings.Contains(s, `"isOutput"`) {
			t.Fatalf("JSON 应包含 \"isOutput\" key,实际 %s", s)
		}
		// 不应包含 Go 字段名
		if strings.Contains(s, `"Stage"`) {
			t.Fatalf("JSON 不应包含 \"Stage\",实际 %s", s)
		}
	})

	t.Run("JSON 反序列化回填字段", func(t *testing.T) {
		original := Event{
			Stage:    StageTestCasePass,
			Output:   "用例通过",
			IsOutput: true,
		}
		data, _ := json.Marshal(original)
		var restored Event
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal 失败: %v", err)
		}
		if !reflect.DeepEqual(original, restored) {
			t.Fatalf("往返不一致,原 %+v,还原 %+v", original, restored)
		}
	})

	t.Run("零值 Event 可正常序列化", func(t *testing.T) {
		var ev Event
		data, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("零值 Marshal 失败: %v", err)
		}
		s := string(data)
		if !strings.Contains(s, `"isOutput":false`) {
			t.Fatalf("零值 isOutput 应为 false,实际 %s", s)
		}
	})
}

// ===== CompileError 结构体测试 =====

func TestCompileError(t *testing.T) {
	t.Run("实例化并读取字段", func(t *testing.T) {
		ce := CompileError{
			File:     "main.eg",
			Line:     42,
			Col:      10,
			Message:  "undefined: foo",
			Severity: "error",
		}
		if ce.File != "main.eg" {
			t.Fatalf("File 不匹配,实际 %q", ce.File)
		}
		if ce.Line != 42 {
			t.Fatalf("Line 应为 42,实际 %d", ce.Line)
		}
		if ce.Col != 10 {
			t.Fatalf("Col 应为 10,实际 %d", ce.Col)
		}
		if ce.Message != "undefined: foo" {
			t.Fatalf("Message 不匹配,实际 %q", ce.Message)
		}
		if ce.Severity != "error" {
			t.Fatalf("Severity 应为 error,实际 %q", ce.Severity)
		}
	})

	t.Run("JSON tag 正确(file/line/col/message/severity)", func(t *testing.T) {
		ce := CompileError{
			File:     "a.eg",
			Line:     1,
			Col:      2,
			Message:  "msg",
			Severity: "warning",
		}
		data, err := json.Marshal(ce)
		if err != nil {
			t.Fatalf("Marshal 失败: %v", err)
		}
		s := string(data)
		expectedKeys := []string{`"file"`, `"line"`, `"col"`, `"message"`, `"severity"`}
		for _, k := range expectedKeys {
			if !strings.Contains(s, k) {
				t.Fatalf("JSON 应包含 %s,实际 %s", k, s)
			}
		}
		// 不应包含 Go 字段名
		for _, bad := range []string{`"File"`, `"Line"`, `"Col"`, `"Message"`, `"Severity"`} {
			if strings.Contains(s, bad) {
				t.Fatalf("JSON 不应包含 %s,实际 %s", bad, s)
			}
		}
	})

	t.Run("JSON 往返一致", func(t *testing.T) {
		original := CompileError{
			File:     "types/types.go",
			Line:     99,
			Col:      5,
			Message:  "cannot use x as y",
			Severity: "error",
		}
		data, _ := json.Marshal(original)
		var restored CompileError
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal 失败: %v", err)
		}
		if !reflect.DeepEqual(original, restored) {
			t.Fatalf("往返不一致,原 %+v,还原 %+v", original, restored)
		}
	})

	t.Run("Severity 支持 error 和 warning", func(t *testing.T) {
		cases := []string{"error", "warning"}
		for _, sev := range cases {
			ce := CompileError{Severity: sev}
			data, _ := json.Marshal(ce)
			var restored CompileError
			_ = json.Unmarshal(data, &restored)
			if restored.Severity != sev {
				t.Fatalf("Severity 往返后应为 %q,实际 %q", sev, restored.Severity)
			}
		}
	})
}

// ===== HealthReport 结构体测试 =====

func TestHealthReport(t *testing.T) {
	t.Run("实例化并读取字段", func(t *testing.T) {
		hr := HealthReport{
			OK:          true,
			Message:     "全部正常",
			GoCompiler:  "/usr/local/go/bin/go",
			GoVersion:   "go1.23.4",
			OS:          "windows",
			Arch:        "amd64",
			TemplateDir: "(embedded)",
			TemplateOK:  true,
			CacheDir:    "/tmp/cache",
			CacheReady:  true,
			NPM:         "/usr/bin/npm",
			Wails3CLI:   "/usr/local/bin/wails3",
			CCompiler:   "gcc",
			CGOVersion:  "gcc 13.0",
			Windres:     "windres",
		}
		if !hr.OK {
			t.Fatal("OK 应为 true")
		}
		if hr.Message != "全部正常" {
			t.Fatalf("Message 不匹配,实际 %q", hr.Message)
		}
		if hr.GoCompiler != "/usr/local/go/bin/go" {
			t.Fatalf("GoCompiler 不匹配,实际 %q", hr.GoCompiler)
		}
		if hr.GoVersion != "go1.23.4" {
			t.Fatalf("GoVersion 不匹配,实际 %q", hr.GoVersion)
		}
		if hr.OS != "windows" {
			t.Fatalf("OS 不匹配,实际 %q", hr.OS)
		}
		if hr.Arch != "amd64" {
			t.Fatalf("Arch 不匹配,实际 %q", hr.Arch)
		}
		if hr.TemplateDir != "(embedded)" {
			t.Fatalf("TemplateDir 不匹配,实际 %q", hr.TemplateDir)
		}
		if !hr.TemplateOK {
			t.Fatal("TemplateOK 应为 true")
		}
		if hr.CacheDir != "/tmp/cache" {
			t.Fatalf("CacheDir 不匹配,实际 %q", hr.CacheDir)
		}
		if !hr.CacheReady {
			t.Fatal("CacheReady 应为 true")
		}
		if hr.NPM != "/usr/bin/npm" {
			t.Fatalf("NPM 不匹配,实际 %q", hr.NPM)
		}
		if hr.Wails3CLI != "/usr/local/bin/wails3" {
			t.Fatalf("Wails3CLI 不匹配,实际 %q", hr.Wails3CLI)
		}
		if hr.CCompiler != "gcc" {
			t.Fatalf("CCompiler 不匹配,实际 %q", hr.CCompiler)
		}
		if hr.CGOVersion != "gcc 13.0" {
			t.Fatalf("CGOVersion 不匹配,实际 %q", hr.CGOVersion)
		}
		if hr.Windres != "windres" {
			t.Fatalf("Windres 不匹配,实际 %q", hr.Windres)
		}
	})

	t.Run("JSON tag 使用 camelCase", func(t *testing.T) {
		hr := HealthReport{
			OK:          true,
			GoCompiler:  "go",
			GoVersion:   "v1",
			OS:          "linux",
			Arch:        "arm64",
			TemplateDir: "tpl",
			TemplateOK:  true,
			CacheDir:    "cache",
			CacheReady:  true,
			NPM:         "npm",
			Wails3CLI:   "wails3",
			CCompiler:   "gcc",
			CGOVersion:  "cgo",
			Windres:     "windres",
		}
		data, err := json.Marshal(hr)
		if err != nil {
			t.Fatalf("Marshal 失败: %v", err)
		}
		s := string(data)
		expectedKeys := []string{
			`"ok"`, `"message"`, `"goCompiler"`, `"goVersion"`,
			`"os"`, `"arch"`, `"templateDir"`, `"templateOk"`,
			`"cacheDir"`, `"cacheReady"`, `"npm"`, `"wails3Cli"`,
			`"cCompiler"`, `"cgoVersion"`, `"windres"`,
		}
		for _, k := range expectedKeys {
			if !strings.Contains(s, k) {
				t.Fatalf("JSON 应包含 %s,实际 %s", k, s)
			}
		}
	})

	t.Run("JSON 往返一致", func(t *testing.T) {
		original := HealthReport{
			OK:          false,
			Message:     "Go 未找到",
			GoCompiler:  "",
			GoVersion:   "",
			OS:          "windows",
			Arch:        "amd64",
			TemplateDir: "(embedded)",
			TemplateOK:  true,
			CacheDir:    "",
			CacheReady:  false,
			NPM:         "",
			Wails3CLI:   "",
			CCompiler:   "",
			CGOVersion:  "",
			Windres:     "",
		}
		data, _ := json.Marshal(original)
		var restored HealthReport
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal 失败: %v", err)
		}
		if !reflect.DeepEqual(original, restored) {
			t.Fatalf("往返不一致,原 %+v,还原 %+v", original, restored)
		}
	})

	t.Run("零值 OK=false 表示不健康", func(t *testing.T) {
		var hr HealthReport
		if hr.OK {
			t.Fatal("零值 OK 应为 false(不健康)")
		}
		if hr.TemplateOK {
			t.Fatal("零值 TemplateOK 应为 false")
		}
		if hr.CacheReady {
			t.Fatal("零值 CacheReady 应为 false")
		}
	})
}

// ===== EventSink 类型测试 =====

func TestEventSink(t *testing.T) {
	t.Run("可赋值函数并调用", func(t *testing.T) {
		var received Event
		var sink EventSink = func(ev Event) {
			received = ev
		}
		input := Event{Stage: StageEngineStop, Output: "停止", IsOutput: false}
		sink(input)
		if received != input {
			t.Fatalf("sink 应接收到 %+v,实际 %+v", input, received)
		}
	})

	t.Run("nil sink 可安全忽略(调用方需判空)", func(t *testing.T) {
		// EventSink 为 nil 时静默忽略是契约,调用方应判空
		// 这里仅验证类型本身可声明为 nil
		var sink EventSink
		if sink != nil {
			t.Fatal("未赋值的 sink 应为 nil")
		}
		// 模拟调用方判空逻辑
		emit := func(sink EventSink, ev Event) {
			if sink != nil {
				sink(ev)
			}
		}
		// 不应 panic
		emit(nil, Event{Stage: StageNodeEnter})
	})
}
