package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Component 描述窗口中的一个组件。
// 坐标与尺寸使用 float64，兼容设计器可能输出的浮点数值。

// titlebarHeight 是前端自定义标题栏的像素高度。
// 创建窗口时需要把客户区高度加上此值，确保运行时客户区尺寸与设计器一致（WYSIWYG）。
// 必须与 runtime/wails-template/frontend/src/App.vue 中 .custom-titlebar 高度保持同步。
const titlebarHeight = 32

type Component struct {
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Text     string                 `json:"text"`
	Items    string                 `json:"items"`
	X        float64                `json:"x"`
	Y        float64                `json:"y"`
	Width    float64                `json:"width"`
	Height   float64                `json:"height"`
	Visible  bool                   `json:"visible"`
	Enabled  bool                   `json:"enabled"`
	FontSize int                    `json:"fontSize"`
	Color    string                 `json:"color"`
	BgColor  string                 `json:"bgColor"`
	Props    map[string]interface{} `json:"props"`
}

// WindowState 是运行时窗口的当前状态，会同步给前端渲染。
type WindowState struct {
	Title       string      `json:"title"`
	Icon        string      `json:"icon"`
	X           float64     `json:"x"`
	Y           float64     `json:"y"`
	Width       float64     `json:"width"`
	Height      float64     `json:"height"`
	MinWidth    float64     `json:"minWidth"`
	MinHeight   float64     `json:"minHeight"`
	MaxWidth    float64     `json:"maxWidth"`
	MaxHeight   float64     `json:"maxHeight"`
	Resizable   bool        `json:"resizable"`
	Minimizable bool        `json:"minimizable"`
	Maximizable bool        `json:"maximizable"`
	FullScreen  bool        `json:"fullScreen"`
	AlwaysOnTop bool        `json:"alwaysOnTop"`
	Frameless   bool        `json:"frameless"`
	Transparent bool        `json:"transparent"`
	Translucent bool        `json:"translucent"`
	Backdrop    string      `json:"backdrop"`
	Rounded     bool        `json:"rounded"`
	Shadow      bool        `json:"shadow"`
	Opacity     float64     `json:"opacity"`
	Centered    bool        `json:"centered"`
	BgColor     string      `json:"bgColor"`
	Components  []Component `json:"components"`
}

// UIService 提供运行时 UI 能力，通过 Wails bindings 与前端通信。
// H5：所有共享状态（window/state/handlers）通过 mu 互斥锁保护，
// 避免用户 goroutine 与 Wails 事件回调 goroutine 并发读写引发竞态。
type UIService struct {
	app                *application.App
	mu                 sync.Mutex
	window             *application.WebviewWindow
	state              WindowState
	handlers           map[string]func()
	hasWindow          bool
	windowCreated      chan struct{}
	windowCreatedOnce  sync.Once
	mainDone           chan struct{}
	mainFinished       bool
	messageLoopEntered bool
	appQuit            chan struct{}
	appQuitOnce        sync.Once
	registerFunc       func()
	mainFunc           func()
}

// NewUIService 创建 UI 服务实例。
func NewUIService() *UIService {
	return &UIService{
		handlers:      make(map[string]func()),
		windowCreated: make(chan struct{}),
		mainDone:      make(chan struct{}),
		appQuit:       make(chan struct{}),
	}
}

// SetMainFuncs 设置用户注入的主函数与事件注册函数。
func (u *UIService) SetMainFuncs(register func(), main func()) {
	u.registerFunc = register
	u.mainFunc = main
}

// ServiceStartup 是 Wails v3 服务启动回调，应用进入事件循环后调用。
func (u *UIService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	log.Println("[runtime] ServiceStartup 被调用")
	u.RunMainFunc()
	return nil
}

// RunMainFunc 在 Wails 应用启动后于后台 goroutine 中执行用户主函数。
func (u *UIService) RunMainFunc() {
	log.Println("[runtime] 执行用户主函数...")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[runtime] mainImpl panic: %v", r)
			}
			if !u.mainFinished {
				u.mainFinished = true
				close(u.mainDone)
			}
			if !u.messageLoopEntered && u.app != nil {
				log.Println("[runtime] 用户代码未进入消息循环，调用 Quit")
				u.app.Quit()
			}
			log.Println("[runtime] 用户主函数执行完毕")
		}()
		if u.registerFunc != nil {
			log.Println("[runtime] 注册事件处理函数...")
			u.registerFunc()
		}
		if u.mainFunc != nil {
			u.mainFunc()
		}
	}()
}

// MessageLoop 阻塞当前 goroutine，直到窗口关闭或应用退出。
// 调用后用户主函数结束不会导致程序退出，窗口会保持显示。
func (u *UIService) MessageLoop() {
	u.messageLoopEntered = true
	log.Println("[runtime] 进入消息循环，等待窗口关闭...")
	<-u.appQuit
	log.Println("[runtime] 退出消息循环")
}

// NewWindow 根据名称创建新窗口（复用 .ew 设计文件加载逻辑）。
func (u *UIService) NewWindow(name string) *WindowState {
	return u.LoadWindow(name)
}

// LoadWindow 根据名称加载 .ew 设计文件并创建窗口。
// 优先从 embeddedWindows（导出 exe 时嵌入的资源）查找，找不到再回退到文件系统。
// H4：用 sync.Once 包裹 close(windowCreated)，避免多次调用 LoadWindow 时重复 close panic。
func (u *UIService) LoadWindow(name string) *WindowState {
	projectPath := os.Getenv("EG_PROJECT_PATH")
	log.Printf("[runtime] LoadWindow: name=%s projectPath=%s", name, projectPath)
	var data []byte
	var loadedPath string
	// 1. 优先查嵌入资源（单文件分发时使用）
	if embedded, ok := embeddedWindows[name]; ok {
		data = []byte(embedded)
		loadedPath = "<embedded>:" + name + ".ew"
	}
	// 2. 回退到文件系统
	if data == nil {
		candidates := []string{
			filepath.Join(projectPath, name+".ew"),
			filepath.Join(projectPath, "src", name+".ew"),
		}
		for _, p := range candidates {
			if b, err := os.ReadFile(p); err == nil {
				data = b
				loadedPath = p
				break
			}
		}
	}
	u.mu.Lock()
	if data == nil {
		log.Printf("[runtime] 未找到 .ew 文件（嵌入资源和文件系统均无），使用默认窗口: name=%s", name)
		u.state = WindowState{Title: name, Width: 538, Height: 350}
	} else {
		log.Printf("[runtime] 加载 .ew: %s", loadedPath)
		var design struct {
			Form       WindowState `json:"form"`
			Components []Component `json:"components"`
		}
		if err := json.Unmarshal(data, &design); err != nil {
			log.Printf("[runtime] 解析 .ew 失败: %v", err)
			u.state = WindowState{Title: name, Width: 538, Height: 350, Resizable: true, Minimizable: true, Maximizable: true, Shadow: true, Opacity: 100}
		} else {
			u.state = design.Form
			if u.state.Width == 0 {
				u.state.Width = 538
			}
			if u.state.Height == 0 {
				u.state.Height = 350
			}
			// Opacity 含义：背景效果强度 1~100。值越大，背景越透明，Mica/Acrylic 越明显。
			// 0 / 缺失都视为 100（完全不透明）。
			if u.state.Opacity <= 0 {
				u.state.Opacity = 100
			}
			if !u.state.Resizable && !design.Form.Resizable {
				u.state.Resizable = true
			}
			u.state.Components = design.Components
			log.Printf("[runtime] 解析 .ew 成功: 标题=%s 尺寸=%.0fx%.0f 背景效果=%s 强度=%d", u.state.Title, u.state.Width, u.state.Height, u.state.Backdrop, int(u.state.Opacity))
		}
	}

	u.hasWindow = true
	win := u.createWindowFromState()
	u.window = win
	u.mu.Unlock()
	log.Printf("[runtime] 窗口已创建，等待 app.Run 显示...")
	u.windowCreatedOnce.Do(func() { close(u.windowCreated) })
	u.emitState()
	return &u.state
}

// createWindowFromState 根据当前 WindowState 创建 Wails 窗口。
func (u *UIService) createWindowFromState() *application.WebviewWindow {
	// WYSIWYG：state.Width/Height 是客户区尺寸（设计器中网格区域的大小），
	// 但 Wails Frameless 窗口的 Width/Height 是整个 WebView 的尺寸（包含自定义标题栏）。
	// 因此创建窗口时 Height 需要加上标题栏高度，使运行时客户区与设计器一致。
	winW := int(u.state.Width)
	winH := int(u.state.Height) + titlebarHeight
	winMinW := int(u.state.MinWidth)
	winMinH := int(u.state.MinHeight)
	if winMinH > 0 {
		winMinH += titlebarHeight
	}
	winMaxW := int(u.state.MaxWidth)
	winMaxH := int(u.state.MaxHeight)
	if winMaxH > 0 {
		winMaxH += titlebarHeight
	}

	opts := application.WebviewWindowOptions{
		Title:            u.state.Title,
		Width:            winW,
		Height:           winH,
		X:                int(u.state.X),
		Y:                int(u.state.Y),
		MinWidth:         winMinW,
		MinHeight:        winMinH,
		MaxWidth:         winMaxW,
		MaxHeight:        winMaxH,
		DisableResize:    !u.state.Resizable,
		AlwaysOnTop:      u.state.AlwaysOnTop,
		Frameless:        true, // 始终使用 Frameless，前端自定义标题栏
		BackgroundType:   backgroundTypeFromState(u.state),
		BackgroundColour: bgColorWithIntensity(u.state),
		InitialPosition:  initialPositionFromState(u.state.Centered),
		URL:              "/",
		Windows: application.WindowsWindow{
			BackdropType:                      backdropTypeFromString(u.state.Backdrop),
			DisableFramelessWindowDecorations: false,
		},
		Mac: application.MacWindow{
			Backdrop:      macBackdropFromState(u.state),
			DisableShadow: !u.state.Shadow,
		},
		Linux: application.LinuxWindow{
			WindowIsTranslucent: u.state.Transparent || u.state.Translucent || (u.state.Backdrop != "" && u.state.Backdrop != "none"),
		},
	}

	if u.state.FullScreen {
		opts.StartState = application.WindowStateFullscreen
	}

	log.Printf("[runtime] 创建窗口对象: %s %dx%d (客户区 %dx%d, 标题栏 %dpx)", opts.Title, winW, winH, int(u.state.Width), int(u.state.Height), titlebarHeight)
	win := u.app.Window.NewWithOptions(opts)
	if win == nil {
		log.Printf("[runtime] NewWithOptions 返回 nil")
		return nil
	}
	log.Printf("[runtime] 窗口对象已创建")

	// Wails v3 的 Show() 在 impl 为 nil 时只会调用 Run() 创建原生窗口，
	// 不会真正显示。必须先显式 Run() 让 impl 初始化，再调用 Show()。
	win.Run()
	log.Printf("[runtime] 窗口原生实现已初始化")
	win.Show()
	log.Printf("[runtime] 窗口已调用 Show()")

	// 设置窗口图标（Win32 API），确保任务栏、Alt+Tab 和对话框继承正确图标
	hwnd := getWindowHWND(win.NativeWindow())
	if hwnd != 0 {
		setWindowIcon(hwnd)
	} else {
		log.Printf("[runtime] 警告: 无法获取窗口 HWND，将在 RuntimeReady 时重试")
	}

	// 注册窗口生命周期事件，便于排查显示问题。
	win.OnWindowEvent(events.Common.WindowRuntimeReady, func(event *application.WindowEvent) {
		log.Printf("[runtime] 窗口事件: WindowRuntimeReady")
		// 重试设置窗口图标（某些情况下 Run() 后 HWND 尚未就绪）
		if hwnd := getWindowHWND(win.NativeWindow()); hwnd != 0 {
			setWindowIcon(hwnd)
		}
		// 前端页面加载完成，重新发送状态确保组件等数据不丢失
		u.emitState()
		log.Printf("[runtime] 已重新发送窗口状态到前端，组件数=%d 背景效果=%s 强度=%d", len(u.state.Components), u.state.Backdrop, int(u.state.Opacity))
		// 禁用 WebView 右键菜单和开发者工具快捷键
		win.ExecJS(`
			document.addEventListener('contextmenu', function(e) { e.preventDefault(); });
			document.addEventListener('keydown', function(e) {
				if (e.key === 'F5' || (e.ctrlKey && (e.key === 'r' || e.key === 'R'))) {
					e.preventDefault();
				}
			});
		`)
	})
	win.OnWindowEvent(events.Common.WindowShow, func(event *application.WindowEvent) {
		log.Printf("[runtime] 窗口事件: WindowShow")
	})
	win.OnWindowEvent(events.Common.WindowFocus, func(event *application.WindowEvent) {
		log.Printf("[runtime] 窗口事件: WindowFocus")
	})
	win.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		log.Printf("[runtime] 窗口事件: WindowClosing")
		u.appQuitOnce.Do(func() { close(u.appQuit) })
	})

	if u.state.Centered {
		win.Center()
		log.Printf("[runtime] 窗口已居中")
	}
	win.Focus()
	log.Printf("[runtime] 窗口已请求焦点")
	// 透明度设置已移至 WindowRuntimeReady 事件中确保页面加载完成后生效

	// 延迟检查窗口是否真正显示，如未显示则再次尝试。
	go func() {
		time.Sleep(2 * time.Second)
		u.mu.Lock()
		win := u.window
		u.mu.Unlock()
		if win == nil {
			return
		}
		visible := false
		// 通过 InvokeSync 在主线程读取可见性，避免跨线程问题。
		application.InvokeSync(func() {
			visible = win.IsVisible()
		})
		log.Printf("[runtime] 窗口可见性检查: visible=%t", visible)
		if !visible {
			log.Printf("[runtime] 窗口未显示，尝试重新 Show/Focus")
			win.Show()
			win.Focus()
		}
	}()

	return win
}

// GetState 返回当前窗口状态，供前端主动获取。
func (u *UIService) GetState() WindowState {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.state
}

// MessageBox 显示一个信息提示框。
// title 可选，缺省时使用空字符串。
// M6：检查 u.window 是否为 nil，避免 AttachToWindow(nil) panic。
func (u *UIService) MessageBox(content string, title ...string) bool {
	if u.app == nil {
		log.Println("[runtime] MessageBox (无 app):", content)
		return false
	}
	t := ""
	if len(title) > 0 {
		t = title[0]
	}
	u.mu.Lock()
	win := u.window
	u.mu.Unlock()
	dlg := u.app.Dialog.Info().
		SetTitle(t).
		SetMessage(content)
	if win != nil {
		dlg = dlg.AttachToWindow(win)
	}
	dlg.Show()
	return true
}

// QuestionBox 显示一个确认框，返回是否点击“是”。
// H6：移除 select 的 default 分支，让 select 阻塞等待用户点击，
// 原 default 会导致永远立即返回 false，确认框功能完全失效。
func (u *UIService) QuestionBox(content string, title ...string) bool {
	if u.app == nil {
		log.Println("[runtime] QuestionBox (无 app):", content)
		return false
	}
	t := ""
	if len(title) > 0 {
		t = title[0]
	}
	u.mu.Lock()
	win := u.window
	u.mu.Unlock()
	result := make(chan bool, 1)
	dlg := u.app.Dialog.Question().
		SetTitle(t).
		SetMessage(content)
	if win != nil {
		dlg = dlg.AttachToWindow(win)
	}
	dlg.AddButton("是").OnClick(func() { result <- true })
	dlg.AddButton("否").OnClick(func() { result <- false })
	dlg.Show()
	// 阻塞等待用户选择，无 default 分支
	r := <-result
	return r
}

// InputBox 显示一个输入框，返回用户输入的文本。
// M2：当前未实现前端输入页配合，直接返回空字符串，
// 不再创建窗口后丢弃（原实现会泄漏窗口对象且永远 100ms 超时返回空）。
func (u *UIService) InputBox(prompt string, title string) string {
	if u.app == nil {
		return ""
	}
	log.Printf("[runtime] InputBox 未实现（prompt=%s title=%s），返回空", prompt, title)
	return ""
}

// OpenFileDialog 弹出打开文件对话框，返回选中的文件路径。
func (u *UIService) OpenFileDialog(title string, filterName string, patterns string) string {
	if u.app == nil {
		return ""
	}
	dlg := u.app.Dialog.OpenFile().SetTitle(title)
	if patterns != "" {
		dlg.AddFilter(filterName, patterns)
	}
	u.mu.Lock()
	win := u.window
	u.mu.Unlock()
	if win != nil {
		dlg = dlg.AttachToWindow(win)
	}
	path, err := dlg.PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return path
}

// SaveFileDialog 弹出保存文件对话框，返回保存的文件路径。
func (u *UIService) SaveFileDialog(title string, filterName string, patterns string, filename string) string {
	if u.app == nil {
		return ""
	}
	dlg := u.app.Dialog.SaveFile()
	dlg.SetOptions(&application.SaveFileDialogOptions{
		Title:    title,
		Filename: filename,
	})
	if patterns != "" {
		dlg.AddFilter(filterName, patterns)
	}
	u.mu.Lock()
	win := u.window
	u.mu.Unlock()
	if win != nil {
		dlg = dlg.AttachToWindow(win)
	}
	path, err := dlg.PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return path
}

// SetWindowPosition 设置窗口位置。
func (u *UIService) SetWindowPosition(x, y int) {
	if u.window != nil {
		u.window.SetPosition(x, y)
	}
}

// SetWindowSize 设置窗口尺寸。
func (u *UIService) SetWindowSize(width, height int) {
	if u.window != nil {
		u.window.SetSize(width, height)
	}
}

// SetWindowTitle 设置窗口标题。
func (u *UIService) SetWindowTitle(title string) {
	u.mu.Lock()
	win := u.window
	u.state.Title = title
	u.mu.Unlock()
	if win != nil {
		win.SetTitle(title)
	}
	u.emitState()
}

// SetWindowOpacity 设置背景效果强度（1~100 整数，值越大背景越透明）。
func (u *UIService) SetWindowOpacity(intensity float64) {
	u.mu.Lock()
	win := u.window
	if win == nil {
		u.mu.Unlock()
		return
	}
	if intensity < 1 {
		intensity = 1
	}
	if intensity > 100 {
		intensity = 100
	}
	u.state.Opacity = intensity
	bgColor := bgColorWithIntensity(u.state)
	u.mu.Unlock()
	if u.app != nil {
		_ = win.SetBackgroundColour(bgColor)
	}
	u.emitState()
}

// SetAlwaysOnTop 设置窗口置顶。
func (u *UIService) SetAlwaysOnTop(on bool) {
	u.mu.Lock()
	win := u.window
	u.state.AlwaysOnTop = on
	u.mu.Unlock()
	if win != nil {
		win.SetAlwaysOnTop(on)
	}
	u.emitState()
}

// CenterWindow 窗口居中。
func (u *UIService) CenterWindow() {
	if u.window != nil {
		u.window.Center()
	}
}

// MinimizeWindow 最小化窗口。
func (u *UIService) MinimizeWindow() {
	if u.window != nil {
		u.window.Minimise()
	}
}

// MaximizeWindow 最大化窗口。
func (u *UIService) MaximizeWindow() {
	if u.window != nil {
		u.window.Maximise()
	}
}

// ToggleMaximizeWindow 切换窗口最大化/还原。
func (u *UIService) ToggleMaximizeWindow() {
	if u.window != nil {
		u.window.ToggleMaximise()
	}
}

// RestoreWindow 恢复窗口。
func (u *UIService) RestoreWindow() {
	if u.window != nil {
		u.window.Restore()
	}
}

// FullScreenWindow 全屏窗口。
func (u *UIService) FullScreenWindow() {
	if u.window != nil {
		u.window.Fullscreen()
	}
}

// CloseWindow 关闭窗口。
func (u *UIService) CloseWindow() {
	if u.window != nil {
		u.window.Close()
	}
}

// HideWindow 隐藏窗口。
func (u *UIService) HideWindow() {
	if u.window != nil {
		u.window.Hide()
	}
}

// ShowWindow 显示窗口。
func (u *UIService) ShowWindow() {
	if u.window != nil {
		u.window.Show()
	}
}

// ScreenWidth 返回主屏幕宽度。
func (u *UIService) ScreenWidth() int {
	if u.app == nil {
		return 0
	}
	screen := u.app.Screen.GetPrimary()
	if screen == nil {
		return 0
	}
	return screen.Size.Width
}

// ScreenHeight 返回主屏幕高度。
func (u *UIService) ScreenHeight() int {
	if u.app == nil {
		return 0
	}
	screen := u.app.Screen.GetPrimary()
	if screen == nil {
		return 0
	}
	return screen.Size.Height
}

// ClipboardGetText 获取剪贴板文本。
func (u *UIService) ClipboardGetText() string {
	if u.app == nil {
		return ""
	}
	text, _ := u.app.Clipboard.Text()
	return text
}

// ClipboardSetText 设置剪贴板文本。
func (u *UIService) ClipboardSetText(text string) bool {
	if u.app == nil {
		return false
	}
	return u.app.Clipboard.SetText(text)
}

// Println 输出到运行时控制台。
func (u *UIService) Println(args ...interface{}) {
	fmt.Println(args...)
}

// CreateButton 创建按钮。
func (u *UIService) CreateButton(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "button", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateEdit 创建编辑框。
func (u *UIService) CreateEdit(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "edit", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateLabel 创建标签。
func (u *UIService) CreateLabel(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "label", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateCheckbox 创建复选框。
func (u *UIService) CreateCheckbox(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "checkbox", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateRadio 创建单选框。
func (u *UIService) CreateRadio(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "radio", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateListbox 创建列表框。
func (u *UIService) CreateListbox(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "listbox", Name: name, Text: text, Items: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateCombobox 创建组合框。
func (u *UIService) CreateCombobox(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "combobox", Name: name, Text: text, Items: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateSwitch 创建开关。
func (u *UIService) CreateSwitch(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "switch", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateSlider 创建滑动条。
func (u *UIService) CreateSlider(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "slider", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateProgress 创建进度条。
func (u *UIService) CreateProgress(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "progress", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateImage 创建图片。
func (u *UIService) CreateImage(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "image", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateTextarea 创建多行编辑框。
func (u *UIService) CreateTextarea(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "textarea", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateTabs 创建标签页。
func (u *UIService) CreateTabs(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "tabs", Name: name, Text: text, Items: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateCard 创建卡片。
func (u *UIService) CreateCard(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "card", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// CreateDivider 创建分割线。
func (u *UIService) CreateDivider(name string, text string, x float64, y float64, width float64, height float64) {
	u.addComponent(Component{Type: "divider", Name: name, Text: text, X: x, Y: y, Width: width, Height: height, Visible: true, Enabled: true})
}

// SetText 修改组件文本。
func (u *UIService) SetText(name string, text string) {
	u.mu.Lock()
	for i := range u.state.Components {
		if u.state.Components[i].Name == name {
			u.state.Components[i].Text = text
			u.mu.Unlock()
			u.emitState()
			return
		}
	}
	u.mu.Unlock()
}

// SetItems 修改列表类组件的选项。
func (u *UIService) SetItems(name string, items string) {
	u.mu.Lock()
	for i := range u.state.Components {
		if u.state.Components[i].Name == name {
			u.state.Components[i].Items = items
			u.mu.Unlock()
			u.emitState()
			return
		}
	}
	u.mu.Unlock()
}

// RegisterEvent 注册组件事件处理函数。
func (u *UIService) RegisterEvent(component string, event string, handler func()) {
	key := component + "_" + event
	u.mu.Lock()
	u.handlers[key] = handler
	u.mu.Unlock()
	log.Printf("[runtime] 注册事件: %s -> %p", key, handler)
}

// HandleEvent 由前端调用，触发已注册的事件处理函数。
func (u *UIService) HandleEvent(component string, event string) {
	key := component + "_" + event
	u.mu.Lock()
	h, ok := u.handlers[key]
	u.mu.Unlock()
	log.Printf("[runtime] HandleEvent: key=%s found=%v", key, ok)
	if ok {
		h()
	}
}

func (u *UIService) addComponent(c Component) {
	u.mu.Lock()
	u.state.Components = append(u.state.Components, c)
	u.mu.Unlock()
	u.emitState()
}

func (u *UIService) emitState() {
	u.mu.Lock()
	state := u.state
	win := u.window
	u.mu.Unlock()
	if u.app != nil {
		u.app.Event.Emit("ui:update", state)
	}
	// 双保险：通过 ExecJS 直接调用前端全局函数设置状态。
	// wails v3 Events.On 回调参数格式可能与预期不符，直接注入避免组件丢失。
	if win != nil {
		if data, err := json.Marshal(state); err == nil {
			win.ExecJS(fmt.Sprintf("window.__setEgState__ && window.__setEgState__(%s)", string(data)))
		}
	}
}

func parseColor(s string) application.RGBA {
	if s == "" {
		return application.NewRGBA(255, 255, 255, 255)
	}
	// #RRGGBBAA 格式
	if len(s) == 9 && s[0] == '#' {
		var r, g, b, a uint8
		fmt.Sscanf(s, "#%02x%02x%02x%02x", &r, &g, &b, &a)
		return application.NewRGBA(r, g, b, a)
	}
	// #RRGGBB 格式
	if len(s) == 7 && s[0] == '#' {
		var r, g, b uint8
		fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
		return application.NewRGBA(r, g, b, 255)
	}
	// rgba(r, g, b, a) 格式（Naive UI n-color-picker show-alpha 默认返回格式）
	if strings.HasPrefix(s, "rgba(") && strings.HasSuffix(s, ")") {
		inner := strings.TrimPrefix(s, "rgba(")
		inner = strings.TrimSuffix(inner, ")")
		parts := strings.Split(inner, ",")
		if len(parts) == 4 {
			var r, g, b uint8
			var a float64
			fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &r)
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &g)
			fmt.Sscanf(strings.TrimSpace(parts[2]), "%d", &b)
			fmt.Sscanf(strings.TrimSpace(parts[3]), "%f", &a)
			alpha := uint8(a * 255)
			if a >= 1.0 {
				alpha = 255
			}
			return application.NewRGBA(r, g, b, alpha)
		}
	}
	// rgb(r, g, b) 格式
	if strings.HasPrefix(s, "rgb(") && strings.HasSuffix(s, ")") {
		inner := strings.TrimPrefix(s, "rgb(")
		inner = strings.TrimSuffix(inner, ")")
		parts := strings.Split(inner, ",")
		if len(parts) == 3 {
			var r, g, b uint8
			fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &r)
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &g)
			fmt.Sscanf(strings.TrimSpace(parts[2]), "%d", &b)
			return application.NewRGBA(r, g, b, 255)
		}
	}
	return application.NewRGBA(255, 255, 255, 255)
}

func buttonStateFromBool(enabled bool) application.ButtonState {
	if enabled {
		return application.ButtonEnabled
	}
	return application.ButtonDisabled
}

func backgroundTypeFromState(state WindowState) application.BackgroundType {
	// 完全透明窗口：穿透显示桌面内容
	if state.Transparent {
		return application.BackgroundTypeTransparent
	}
	// 毛玻璃半透明窗口：Mica/Acrylic 效果，配合 Windows.BackdropType
	// 只要指定了非 none 的 backdrop（mica/acrylic/tabbed）就需要 Translucent，
	// 否则 WebView2 不透明背景会盖住毛玻璃层，背景效果无效。
	if state.Translucent || (state.Backdrop != "" && state.Backdrop != "none") {
		return application.BackgroundTypeTranslucent
	}
	// 默认：不透明窗口，WebView 背景正常可见，组件清晰显示
	return application.BackgroundTypeSolid
}

// bgColorWithIntensity 根据背景效果强度（state.Opacity 1~100 整数）调整 BgColor 的 alpha 通道。
// 强度=100 → alpha=255（完全不透明，遮住 Mica/Acrylic），
// 强度=1   → alpha≈3  （几乎全透明，背景效果最明显）。
// 仅在 Translucent 模式（Mica/Acrylic/Tabbed）下有意义；Solid 模式 WebView 永远不透明。
func bgColorWithIntensity(state WindowState) application.RGBA {
	rgba := parseColor(state.BgColor)
	intensity := state.Opacity
	if intensity < 1 {
		intensity = 1
	}
	if intensity > 100 {
		intensity = 100
	}
	// 1~100 → alpha 3~255（线形映射）；1 时不完全透明是为了避免 WebView 完全无背景
	alpha := uint8((intensity * 252 / 100) + 3)
	return application.NewRGBA(rgba.Red, rgba.Green, rgba.Blue, alpha)
}

func backdropTypeFromString(s string) application.BackdropType {
	switch s {
	case "none":
		return application.None
	case "mica":
		return application.Mica
	case "acrylic":
		return application.Acrylic
	case "tabbed":
		return application.Tabbed
	default:
		return application.Auto
	}
}

func macBackdropFromState(state WindowState) application.MacBackdrop {
	if state.Transparent {
		return application.MacBackdropTransparent
	}
	if state.Translucent || (state.Backdrop != "" && state.Backdrop != "none") {
		return application.MacBackdropTranslucent
	}
	// 默认：不透明背景
	return application.MacBackdropNormal
}

func initialPositionFromState(centered bool) application.WindowStartPosition {
	if centered {
		return application.WindowCentered
	}
	return application.WindowXY
}
