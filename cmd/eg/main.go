// eg 是 EGOU 的命令行工具，目前支持 build 和 transpile 子命令。
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"egou/internal/transpiler"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		cmdBuild(os.Args[2:])
	case "transpile":
		cmdTranspile(os.Args[2:])
	case "run":
		cmdRun(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("用法: eg <命令> [选项]")
	fmt.Println("")
	fmt.Println("命令:")
	fmt.Println("  build      将 .eg 文件转译为 .go 并编译成可执行文件")
	fmt.Println("  run        将 .eg 文件转译为 .go 并直接运行")
	fmt.Println("  transpile  仅将 .eg 文件转译为 .go，输出到标准输出或文件")
}

func cmdBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	input := fs.String("i", "", "输入的 .eg 源文件路径")
	output := fs.String("o", "", "输出的可执行文件路径（可选）")
	fs.Parse(args)

	if *input == "" {
		fmt.Fprintln(os.Stderr, "错误: 必须指定 -i 输入文件")
		fs.Usage()
		os.Exit(1)
	}

	tmpDir, err := os.MkdirTemp("", "eg-build-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建临时目录失败: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	goFile := filepath.Join(tmpDir, "main.go")
	if err := transpileFile(*input, goFile); err != nil {
		fmt.Fprintf(os.Stderr, "转译失败: %v\n", err)
		os.Exit(1)
	}

	goMod := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module egruntime\n\ngo 1.25.0\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "写入 go.mod 失败: %v\n", err)
		os.Exit(1)
	}

	out := *output
	if out == "" {
		out = strings.TrimSuffix(*input, filepath.Ext(*input))
	}
	out, err = filepath.Abs(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析输出路径失败: %v\n", err)
		os.Exit(1)
	}

	buildCmd := exec.Command("go", "build", "-ldflags", "-H=windowsgui", "-o", out, goFile)
	buildCmd.Dir = tmpDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Env = append(os.Environ(), "EG_PROJECT_PATH="+filepath.Dir(*input))
	if err := buildCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "编译失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("构建成功: %s\n", out)
}

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	input := fs.String("i", "", "输入的 .eg 源文件路径")
	fs.Parse(args)

	if *input == "" {
		fmt.Fprintln(os.Stderr, "错误: 必须指定 -i 输入文件")
		fs.Usage()
		os.Exit(1)
	}

	tmpDir, err := os.MkdirTemp("", "eg-run-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建临时目录失败: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	goFile := filepath.Join(tmpDir, "main.go")
	if err := transpileFile(*input, goFile); err != nil {
		fmt.Fprintf(os.Stderr, "转译失败: %v\n", err)
		os.Exit(1)
	}

	goMod := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module egruntime\n\ngo 1.25.0\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "写入 go.mod 失败: %v\n", err)
		os.Exit(1)
	}

	runCmd := exec.Command("go", "run", "-ldflags", "-H=windowsgui", goFile)
	runCmd.Dir = tmpDir
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Env = append(os.Environ(), "EG_PROJECT_PATH="+filepath.Dir(*input))
	if err := runCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}

func cmdTranspile(args []string) {
	fs := flag.NewFlagSet("transpile", flag.ExitOnError)
	input := fs.String("i", "", "输入的 .eg 源文件路径")
	output := fs.String("o", "", "输出的 .go 文件路径（可选，默认输出到标准输出）")
	fs.Parse(args)

	if *input == "" {
		fmt.Fprintln(os.Stderr, "错误: 必须指定 -i 输入文件")
		fs.Usage()
		os.Exit(1)
	}

	src, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取文件失败: %v\n", err)
		os.Exit(1)
	}

	goSrc, err := transpiler.Transpile(string(src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "转译失败: %v\n", err)
		os.Exit(1)
	}

	if *output != "" {
		if err := os.WriteFile(*output, []byte(goSrc), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "写入文件失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已生成: %s\n", *output)
	} else {
		fmt.Println(goSrc)
	}
}

func transpileFile(input, output string) error {
	src, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	goSrc, err := transpiler.Transpile(string(src))
	if err != nil {
		return err
	}
	return os.WriteFile(output, []byte(goSrc), 0644)
}
