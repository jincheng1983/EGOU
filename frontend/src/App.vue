<template>
  <n-config-provider :theme="theme" :theme-overrides="themeOverrides">
    <n-message-provider>
    <n-dialog-provider>
    <div class="app-shell">
      <TitleBar
        :is-dark="isDark"
        :theme-options="themeOptions"
        :current-theme="currentThemeName"
        @save="saveFile"
        @quick-save="quickSave"
        @undo="undo"
        @redo="redo"
        @run="runCode"
        @build="onBuild"
        @debug="debugCode"
        @about="showAbout"
        @select-theme="currentThemeName = $event"
        @settings="showSettings"
        @snippets="showSnippets"
      />

      <div class="main-container">
        <StartPage
          v-if="!projectOpen"
          :recent="recentProjects"
          @create-project="createProject"
          @open-project="openProject"
          @open-recent="openRecent"
        />

        <template v-else>
          <LeftMenu
            :active="leftPanelKey"
            :output-collapsed="outputCollapsed"
            :plugin-panels="pluginPanels"
            @select="leftPanelKey = $event"
            @toggle-output="outputCollapsed = !outputCollapsed"
            @search="onSearch"
            @user="output = '用户功能开发中'"
            @close-project="closeProject"
          />

          <div class="workspace">
            <div class="workspace-body">
              <aside class="left-panel" :style="{ width: leftPanelWidth + 'px' }">
                <FileExplorer
                  v-if="leftPanelKey === 'files'"
                  :files="projectTree"
                  :project-name="projectName"
                  :dim-lines="!!currentFunctionName"
                  @open-file="openProjectFile"
                  @refresh="loadProjectTree(projectPath)"
                  @delete-file="deleteFileByPath"
                />
                <ProjectExplorer
                  v-else-if="leftPanelKey === 'project'"
                  :files="projectTree"
                  :libs="projectLibsList"
                  :project-name="projectName"
                  :current-file-path="activeFile?.path || ''"
                  @open-file="openProjectFile"
                  @refresh="loadProjectTree(projectPath)"
                  @new-window="showNewWindowDialog"
                  @new-module="newModuleFile"
                  @new-class="newClassFile"
                  @new-code-file="newCodeFile"
                  @delete-file="deleteFileByPath"
                />
                <SupportPanel
                  v-else-if="leftPanelKey === 'support'"
                  @show-help="onShowHelp"
                  @open-file="openProjectFile"
                />
                <AIPanel v-show="leftPanelKey === 'ai'" :agents="aiCustomAgents" v-model:current-agent-id="aiCurrentAgent" :ai-config="aiConfig" :models="aiModels" :active-model-id="activeModelId" :get-current-file="getCurrentFileForAI" :project-path="projectPath" @open-settings="openAISettings" @switch-model="switchAIModel" />
                <!-- G7：插件自定义面板。leftPanelKey 形如 "plugin:<id>" -->
                <div
                  v-if="isPluginPanelActive"
                  ref="pluginPanelContainerRef"
                  class="plugin-panel-container"
                ></div>
              </aside>

              <div
                class="splitter splitter-v"
                @mousedown="onSplitterMouseDown($event, 'left')"
              />

              <main class="editor-stack">
                <div class="tabs-bar-wrapper">
                  <div class="file-tabs" @wheel="onTabsWheel" ref="tabsBarRef">
                    <div
                      v-for="(file, idx) in files"
                      :key="idx"
                      class="file-tab"
                      :class="{ active: activeFileIndex === idx, 'drag-over': dragOverIndex === idx, pinned: file.pinned }"
                      :title="file.path ? (file.pinned ? '📌 已固定\n' + file.path : file.path) : file.name"
                      draggable="true"
                      @click="switchFile(idx)"
                      @mousedown="onTabMouseDown($event, idx)"
                      @dblclick="onTabDblClick(idx)"
                      @contextmenu="onTabContextMenu($event, idx)"
                      @dragstart="onTabDragStart($event, idx)"
                      @dragover.prevent="onTabDragOver($event, idx)"
                      @dragleave="onTabDragLeave"
                      @drop="onTabDrop($event, idx)"
                      @dragend="onTabDragEnd"
                    >
                      <span v-if="file.pinned" class="tab-pin-icon" title="已固定">📌</span>
                      <span class="tab-dot" :class="{ dirty: file.source !== file.savedSource }"></span>
                      <span class="file-name" :class="{ 'file-dirty': file.source !== file.savedSource }">{{ file.name }}</span>
                      <span v-if="files.length > 1 && !file.pinned" class="close-btn" @click.stop="closeFile(idx)">×</span>
                    </div>
                  </div>
                  <n-dropdown
                    :options="newTabOptions"
                    trigger="click"
                    placement="bottom-start"
                    @select="onNewTabSelect"
                  >
                    <n-button size="tiny" text class="new-tab-btn">+</n-button>
                  </n-dropdown>
                  <n-dropdown
                    :show="tabContextMenuShow"
                    :options="tabContextMenuOptions"
                    :x="tabContextMenuX"
                    :y="tabContextMenuY"
                    placement="bottom-start"
                    trigger="manual"
                    @select="onTabContextMenuSelect"
                    @clickoutside="tabContextMenuShow = false"
                  />
                </div>
                <div class="editor-area" :class="{ 'code-layout': activeFile.view !== 'design' }">
                  <div v-if="isFormFile" class="view-tabs">
                    <div
                      class="view-tab"
                      :class="{ active: activeFile.view === 'design' }"
                      @click="switchToDesignView"
                    >设计</div>
                    <div
                      class="view-tab"
                      :class="{ active: activeFile.view === 'code' }"
                      @click="switchToCodeView"
                    >代码</div>
                    <template v-if="activeFile.view === 'design'">
                      <span class="view-tab-sep" />
                      <div
                        class="view-tab"
                        :class="{ active: designerSidePanel === 'templates' }"
                        @click="designerSidePanel = 'templates'"
                      >模板</div>
                      <div
                        class="view-tab"
                        :class="{ active: designerSidePanel === 'layers' }"
                        @click="designerSidePanel = 'layers'"
                      >层级</div>
                      <span class="view-tab-sep" />
                      <label class="view-tab-tool" title="切换网格线显示">
                        <n-checkbox v-model:checked="designerShowGrid" size="small" />
                        <span>显示网格</span>
                      </label>
                      <label class="view-tab-tool" title="切换网格吸附">
                        <n-checkbox v-model:checked="designerSnapEnabled" size="small" />
                        <span>网格吸附</span>
                      </label>
                      <select v-model.number="designerGridSize" class="view-tab-select" title="网格大小（像素）">
                        <option :value="1">关闭</option>
                        <option :value="4">4px</option>
                        <option :value="8">8px</option>
                        <option :value="10">10px</option>
                        <option :value="16">16px</option>
                        <option :value="20">20px</option>
                      </select>
                      <span class="view-tab-sep" />
                      <label class="view-tab-tool" title="切换 Tab 顺序编辑模式">
                        <n-checkbox v-model:checked="designerTabOrderMode" size="small" />
                        <span>Tab 顺序</span>
                      </label>
                      <button v-if="designerTabOrderMode" class="view-tab-btn" title="按位置自动排序" @click="designerRef?.autoSortTabOrder()">自动排序</button>
                      <button v-if="designerTabOrderMode" class="view-tab-btn" title="重置 Tab 顺序" @click="designerRef?.resetTabOrder()">重置</button>
                    </template>
                  </div>
                  <template v-if="activeFile.view === 'code' || !isFormFile">
                    <div class="editor-code-body">
                      <div class="editor-main">
                        <Editor
                          ref="editorRef"
                          v-model="activeFile.source"
                          :suggestions="editorSuggestions"
                          :is-dark="isDark"
                          :editor-theme="editorTheme"
                          :file-id="activeFile.path || activeFile.name"
                          :minimap-enabled="minimapEnabled"
                          :font-size="editorFontSize"
                          :font-family="editorFontFamily"
                          :auto-convert-symbols="editorAutoConvertSymbols"
                          :line-numbers-enabled="editorLineNumbers"
                          :line-height="editorLineHeight"
                          :tab-size="editorTabSize"
                          :word-wrap="editorWordWrap"
                          :render-whitespace="editorRenderWhitespace"
                          :cursor-blinking="editorCursorBlinking"
                          :cursor-smooth-caret-animation="editorCursorSmoothCaret"
                          :cursor-width="editorCursorWidth"
                          :bracket-pair-colorization="editorBracketPairColorization"
                          :guides-bracket-pairs="editorGuidesBracketPairs"
                          :font-ligatures="editorFontLigatures"
                          :line-numbers-min-chars="editorLineNumbersMinChars"
                          :render-final-newline="editorRenderFinalNewline"
                          :minimap-show-slider="editorMinimapShowSlider"
                          :minimap-render-characters="editorMinimapRenderCharacters"
                          :minimap-max-column="editorMinimapMaxColumn"
                          :project-path="projectPath"
                          @cursor-change="onCursorChange"
                          @show-help="onShowHelp"
                          @goto-def="onGotoDef"
                          @find-refs="onFindRefs"
                          @rename-symbol="onRenameSymbol"
                          @font-size-change="onFontSizeChange"
                          @open-file-at="onOpenFileAt"
                          @toggle-breakpoint="onToggleBreakpoint"
                        />
                      </div>
                      <div
                        v-if="!zenMode"
                        class="splitter splitter-v"
                        @mousedown="onSplitterMouseDown($event, 'right')"
                      />
                      <aside v-if="!zenMode" class="right-panel" :style="{ width: rightPanelWidth + 'px' }">
                        <PropertiesPanel
                          :parsed="parsed"
                          :current-function-name="currentFunctionName"
                          @goto-function="onOutlineGotoFunction"
                          @goto-line="gotoLine"
                        />
                      </aside>
                    </div>
                  </template>
                  <WindowDesigner
                    v-else
                    ref="designerRef"
                    class="designer-full"
                    :model-value="activeFile.design"
                    @update:model-value="onDesignChange"
                    @open-event="onOpenEvent"
                    v-model:show-grid="designerShowGrid"
                    v-model:snap-enabled="designerSnapEnabled"
                    v-model:tab-order-mode="designerTabOrderMode"
                    v-model:grid-size="designerGridSize"
                    :side-panel="designerSidePanel"
                  />
                </div>

                <div v-if="!outputCollapsed && !zenMode" class="output-panel" :style="{ height: outputPanelHeight + 'px' }">
                  <div
                    class="splitter splitter-h"
                    @mousedown="onSplitterMouseDown($event, 'output')"
                  />
                  <BuildProgress
                    ref="buildProgressRef"
                    :active="buildActive"
                    :step="buildStep"
                    :percent="buildPercent"
                  />
                  <div class="output-toolbar">
                    <n-tabs v-model:value="outputTabName" type="line" size="small" display-directive="show" style="flex: 1; height: 100%;">
                    <n-tab-pane name="output" :tab="t('output.output')" style="height: 100%;">
                      <pre ref="outputPreRef" class="output-pre" @scroll="onOutputScroll">{{ output }}</pre>
                    </n-tab-pane>
                    <n-tab-pane name="errors" :tab="outputTabLabel" style="height: 100%;">
                      <div ref="errorPreRef" class="output-pre error-text" @scroll="onErrorScroll">
                        <div
                          v-for="(entry, i) in errorEntries"
                          :key="i"
                          :class="{ 'error-line-clickable': entry.clickable }"
                          @click="entry.clickable && gotoErrorEntry(entry)"
                        >{{ entry.text || ' ' }}</div>
                      </div>
                    </n-tab-pane>
                    <n-tab-pane name="tips" :tab="t('output.tips')" style="height: 100%;">
                      <div v-if="refsResults.length > 0" class="output-pre refs-list">
                        <div class="refs-header">
                          <span>{{ t('output.findRefs', { query: refsQuery, total: refsResults.length, files: groupedRefs.length }) }}</span>
                          <span class="refs-header-actions">
                            <n-button size="tiny" quaternary @click="expandAllRefFiles">{{ t('output.expandAll') }}</n-button>
                            <n-button size="tiny" quaternary @click="collapseAllRefFiles">{{ t('output.collapseAll') }}</n-button>
                            <n-button size="tiny" quaternary @click="refsResults = []">{{ t('output.close') }}</n-button>
                          </span>
                        </div>
                        <div
                          v-for="g in groupedRefs"
                          :key="g.filePath"
                          class="ref-file"
                        >
                          <div class="ref-file-header" @click="toggleRefFile(g.filePath)">
                            <span class="ref-file-twisty" :class="{ collapsed: isRefFileCollapsed(g.filePath) }">▶</span>
                            <span class="ref-file-name">{{ g.file }}</span>
                            <span class="ref-file-count">{{ g.items.length }}</span>
                          </div>
                          <div v-if="!isRefFileCollapsed(g.filePath)" class="ref-file-items">
                            <div
                              v-for="(r, i) in g.items"
                              :key="i"
                              class="ref-item"
                              @click="gotoRefItem({ filePath: g.filePath, file: g.file, line: r.line, col: r.col, preview: r.preview })"
                            >
                              <span class="ref-line-no">{{ r.line }}</span>
                              <span class="ref-preview">{{ r.preview }}</span>
                            </div>
                          </div>
                        </div>
                      </div>
                      <pre v-else ref="tipPreRef" class="output-pre" @scroll="onTipScroll">{{ tipOutput }}</pre>
                    </n-tab-pane>
                    <n-tab-pane name="bookmarks" :tab="t('output.bookmarks')" style="height: 100%;">
                      <div ref="bookmarkListRef" class="output-pre refs-list">
                        <div class="refs-header">
                          <span>{{ t('output.bookmarkList', { count: bookmarkList.length }) }}</span>
                          <n-button size="tiny" quaternary @click="clearAllBookmarks">{{ t('output.clearAll') }}</n-button>
                        </div>
                        <div v-if="bookmarkList.length === 0" class="ref-item" style="cursor: default;">
                          <span class="ref-preview">{{ t('output.emptyBookmarks') }}</span>
                        </div>
                        <div
                          v-for="(b, i) in bookmarkList"
                          :key="i"
                          class="ref-item"
                          @click="gotoBookmark(b.line)"
                        >
                          <span class="ref-loc">{{ t('output.line', { line: b.line }) }}</span>
                          <span class="ref-preview">{{ b.preview }}</span>
                        </div>
                      </div>
                    </n-tab-pane>
                    <n-tab-pane name="history" :tab="t('output.history')" style="height: 100%;">
                      <div class="output-pre refs-list">
                        <div class="refs-header">
                          <span>{{ t('output.buildHistory', { count: buildHistory.length }) }}</span>
                          <n-button size="tiny" quaternary @click="clearBuildHistory">{{ t('output.clearAll') }}</n-button>
                        </div>
                        <div v-if="buildHistory.length === 0" class="ref-item" style="cursor: default;">
                          <span class="ref-preview">{{ t('output.emptyBuildHistory') }}</span>
                        </div>
                        <div v-for="(h, i) in buildHistory" :key="i" class="ref-item" style="flex-direction: column; align-items: flex-start; gap: 4px;">
                          <div style="width: 100%; display: flex; justify-content: space-between; align-items: center;">
                            <span class="ref-loc">{{ h.time }} · v{{ h.version || '?' }} · {{ h.mode }}</span>
                            <n-space size="tiny">
                              <n-button size="tiny" quaternary @click.stop="copyBuildHistoryPath(h.artifact)">{{ t('output.copyPath') }}</n-button>
                              <n-button size="tiny" quaternary @click.stop="openBuildHistoryFolder(h.artifact)">{{ t('output.open') }}</n-button>
                            </n-space>
                          </div>
                          <span class="ref-preview" style="word-break: break-all; width: 100%;">{{ h.artifact }}</span>
                        </div>
                      </div>
                    </n-tab-pane>
                    <n-tab-pane name="debug" :tab="t('output.debug')" style="height: 100%;">
                      <DebugPanel
                        ref="debugPanelRef"
                        :project-path="projectPath"
                        @jump-to="gotoDebugLocation"
                        @debug-log="onDebugLog"
                        @debug-started="onDebugStarted"
                      />
                    </n-tab-pane>
                  </n-tabs>
                  <div class="output-actions">
                    <n-button size="tiny" quaternary title="导出日志到文件" @click="exportLog">导出</n-button>
                    <n-button size="tiny" quaternary title="清空所有输出" @click="clearAllOutput">清空</n-button>
                  </div>
                  </div>
                </div>
              </main>
            </div>

            <div class="status-bar">
              <div class="status-bar-left">
                <span v-if="zenMode" class="status-bar-item status-bar-clickable sb-zen" @click="toggleZenMode" title="点击退出禅模式">禅模式</span>
                <span v-if="isDebugging" class="status-bar-item sb-debug" title="调试器正在运行">调试中</span>
                <span v-if="statusMessage" class="status-bar-msg">{{ statusMessage }}</span>
              </div>
              <div class="status-bar-right">
                <span
                  v-if="cursorText && activeFileIndex >= 0 && sbShowCursor"
                  class="status-bar-goto"
                  title="点击跳转到行 (Ctrl+G)"
                  @click="showGotoLine()"
                >{{ cursorText }}</span>
                <span v-if="activeFileIndex >= 0" class="status-bar-item status-bar-clickable sb-toggle" :class="{ active: editorWordWrap }" title="自动换行 (Alt+Z)" @click="toggleWordWrap">换</span>
                <span v-if="activeFileIndex >= 0" class="status-bar-item status-bar-clickable sb-toggle" :class="{ active: minimapEnabled }" title="缩略图" @click="toggleMinimap">图</span>
                <span v-if="activeFileIndex >= 0 && sbShowIndent" class="status-bar-item" title="缩进">Sp:{{ editorTabSize }}</span>
                <span v-if="activeFileIndex >= 0 && sbShowEncoding" class="status-bar-item" title="编码">UTF-8</span>
                <span v-if="activeFileIndex >= 0 && sbShowEol" class="status-bar-item" title="换行符">CRLF</span>
                <span v-if="activeFileIndex >= 0 && !isFormFile && sbShowLang" class="status-bar-item" title="语言">EGOU</span>
                <n-popover v-if="sbShowHealth && healthReport" trigger="hover" placement="topEnd" :width="300">
                  <template #trigger>
                    <button
                      class="health-indicator health-dot-only"
                      :class="{ ok: healthReport.ok, bad: !healthReport.ok }"
                      :title="healthReport.ok ? '后端正常（点击刷新）' : '后端异常（点击刷新）'"
                      @click="refreshHealth"
                    >
                      <span class="health-dot" />
                    </button>
                  </template>
                  <div class="health-detail">
                    <div class="health-detail-title">
                      <span class="health-dot" :class="{ ok: healthReport.ok, bad: !healthReport.ok }" />
                      <span>{{ healthReport.message }}</span>
                    </div>
                    <div v-for="row in healthDetailRows" :key="row.label" class="health-detail-row">
                      <span class="health-detail-label">{{ row.label }}</span>
                      <span class="health-detail-value" :class="{ ok: row.ok, bad: !row.ok }">{{ row.value }}</span>
                    </div>
                    <div v-if="healthReport.cacheDir" class="health-detail-path">
                      缓存目录：{{ healthReport.cacheDir }}
                  </div>
                  </div>
                </n-popover>
              </div>
            </div>
          </div>
        </template>
      </div>

      <n-modal
        v-model:show="settingsVisible"
        preset="card"
        title="系统设置"
        :bordered="false"
        style="width: 560px; max-width: 90vw;"
      >
        <SettingsPanel v-model="currentThemeName" :active-menu-key="settingsActiveMenu" v-model:minimap-enabled="minimapEnabled" v-model:font-size="editorFontSize" v-model:line-numbers-enabled="editorLineNumbers" v-model:line-height="editorLineHeight" v-model:auto-save-delay="autoSaveDelay" v-model:tab-size="editorTabSize" v-model:word-wrap="editorWordWrap"
  v-model:render-whitespace="editorRenderWhitespace"
  v-model:cursor-blinking="editorCursorBlinking"
  v-model:cursor-smooth-caret-animation="editorCursorSmoothCaret"
  v-model:cursor-width="editorCursorWidth"
  v-model:bracket-pair-colorization="editorBracketPairColorization"
  v-model:guides-bracket-pairs="editorGuidesBracketPairs"
  v-model:font-ligatures="editorFontLigatures"
  v-model:line-numbers-min-chars="editorLineNumbersMinChars"
  v-model:render-final-newline="editorRenderFinalNewline"
  v-model:minimap-show-slider="editorMinimapShowSlider"
  v-model:minimap-render-characters="editorMinimapRenderCharacters"
  v-model:minimap-max-column="editorMinimapMaxColumn"
  v-model:editor-theme="editorTheme"
  v-model:font-family="editorFontFamily"
  v-model:auto-convert-symbols="editorAutoConvertSymbols"
  v-model:grid-size="designerGridSize"
  v-model:show-grid="designerShowGrid"
  v-model:snap-grid="designerSnapEnabled"
  v-model:default-radius="designerDefaultRadius"
  v-model:default-border-width="designerDefaultBorderWidth"
  v-model:default-build-mode="buildDefaultMode"
  v-model:auto-open-folder="buildAutoOpenFolder"
  v-model:garble-level="buildGarbleLevel"
  v-model:show-build-history="buildShowHistory"
  v-model:output-dir="buildOutputDir"
  v-model:go-path="buildGoPath"
  v-model:delve-path="buildDelvePath"
  v-model:open-last-project="uiOpenLastProject"
  v-model:left-panel-width="leftPanelWidth"
  v-model:right-panel-width="rightPanelWidth"
  v-model:output-panel-height="outputPanelHeight"
  v-model:sb-show-cursor="sbShowCursor"
  v-model:sb-show-indent="sbShowIndent"
  v-model:sb-show-encoding="sbShowEncoding"
  v-model:sb-show-eol="sbShowEol"
  v-model:sb-show-lang="sbShowLang"
  v-model:sb-show-health="sbShowHealth"
  v-model:auto-switch-output-tab="uiAutoSwitchOutputTab"
  v-model:smart-scroll="uiSmartScroll"
  :models="aiModels"
  :active-model-id="activeModelId"
  :agents="aiCustomAgents"
  v-model:ai-stream="aiStream"
  v-model:ai-compress-threshold="aiCompressThreshold"
  v-model:ai-keep-recent="aiKeepRecent"
  @add-model="addAIModel"
  @update-model="updateAIModel"
  @delete-model="deleteAIModel"
  @switch-model="switchAIModel"
  :project-path="projectPath"
  :global-templates="globalTemplates"
  @refresh-templates="loadGlobalTemplates" />
      </n-modal>

      <n-modal v-model:show="searchVisible" preset="card" title="项目搜索" :bordered="false" style="width: 560px; max-width: 90vw;">
        <n-form label-placement="left" label-width="80">
          <n-form-item label="搜索词">
            <n-input v-model:value="searchQuery" placeholder="输入搜索内容（回车搜索）" @keyup.enter="doSearch" />
          </n-form-item>
          <n-form-item label="选项">
            <n-checkbox v-model:checked="searchUseRegex">正则表达式</n-checkbox>
          </n-form-item>
        </n-form>
        <template #footer>
          <n-space justify="end">
            <n-button @click="searchVisible = false">取消</n-button>
            <n-button type="primary" :loading="searching" @click="doSearch">搜索</n-button>
          </n-space>
        </template>
      </n-modal>

      <n-modal
        v-model:show="quickOpenVisible"
        :show-icon="false"
        :bordered="false"
        :mask-closable="true"
        preset="card"
        title=""
        style="width: 520px; max-width: 90vw; padding: 0;"
        :body-style="{ padding: '0' }"
        :header-style="{ display: 'none' }"
        :footer-style="{ display: 'none' }"
        @keydown="onQuickOpenKeydown"
      >
        <div class="quick-open">
          <div class="quick-open-input-wrap">
            <n-input
              ref="quickOpenInputRef"
              v-model:value="quickOpenQuery"
              placeholder="输入文件名快速打开 (Ctrl+P)"
              size="large"
              :bordered="false"
              autofocus
            />
          </div>
          <div v-if="quickOpenFiles.length > 0" class="quick-open-list">
            <div
              v-for="(f, i) in quickOpenFiles"
              :key="f.path"
              class="quick-open-item"
              :class="{ active: i === quickOpenSelected }"
              @click="quickOpenVisible = false; openProjectFile(f.path)"
              @mouseenter="quickOpenSelected = i"
            >
              <span class="qo-icon">📄</span>
              <span class="qo-name">{{ f.name }}</span>
              <span class="qo-path">{{ f.path.replace(projectPath + '\\', '').replace(/\\/g, '/') }}</span>
            </div>
          </div>
          <div v-else class="quick-open-empty">
            <n-text depth="3" style="font-size: var(--ide-font-size);">未找到匹配文件</n-text>
          </div>
        </div>
      </n-modal>

      <n-modal
        v-model:show="gotoLineVisible"
        :show-icon="false"
        :bordered="false"
        :mask-closable="true"
        preset="card"
        title=""
        style="width: 360px; max-width: 90vw; padding: 0;"
        :body-style="{ padding: '0' }"
        :header-style="{ display: 'none' }"
        :footer-style="{ display: 'none' }"
      >
        <div class="goto-line-box">
          <div class="goto-line-input">
            <n-input
              ref="gotoLineInputRef"
              v-model:value="gotoLineInput"
              placeholder="输入行号，按 Enter 跳转 (Ctrl+G)"
              size="large"
              :bordered="false"
              @keydown="onGotoLineKeydown"
            />
          </div>
          <div class="goto-line-hint">
            <n-text depth="3" style="font-size: var(--ide-font-size-sm);">当前文件共 {{ activeFile.source ? activeFile.source.split('\n').length : 0 }} 行</n-text>
          </div>
        </div>
      </n-modal>

      <n-modal
        v-model:show="gotoSymbolVisible"
        :show-icon="false"
        :bordered="false"
        :mask-closable="true"
        preset="card"
        title=""
        style="width: 520px; max-width: 90vw; padding: 0;"
        :body-style="{ padding: '0' }"
        :header-style="{ display: 'none' }"
        :footer-style="{ display: 'none' }"
        @keydown="onGotoSymbolKeydown"
      >
        <div class="quick-open">
          <div class="quick-open-input-wrap">
            <n-input
              ref="gotoSymbolInputRef"
              v-model:value="gotoSymbolQuery"
              placeholder="输入符号名称 (Ctrl+Shift+O) @符号"
              size="large"
              :bordered="false"
              autofocus
            />
          </div>
          <div v-if="gotoSymbolItems.length > 0" class="quick-open-list">
            <div
              v-for="(s, i) in gotoSymbolItems"
              :key="s.kind + '-' + s.name + '-' + s.line"
              class="quick-open-item"
              :class="{ active: i === gotoSymbolSelected }"
              @click="gotoSymbolVisible = false; gotoLine(s.line + 1)"
              @mouseenter="gotoSymbolSelected = i"
            >
              <span class="qo-icon" :class="'sym-icon-' + s.kind">{{ s.kind === 'function' ? 'ƒ' : s.kind === 'variable' ? 'V' : 'C' }}</span>
              <span class="qo-name">{{ s.name }}</span>
              <span class="qo-path" :class="'sym-kind-' + s.kind">{{ s.kind === 'function' ? '函数' : s.kind === 'variable' ? '变量' : '常量' }}{{ s.type ? ': ' + s.type : '' }} 行 {{ s.line + 1 }}</span>
            </div>
          </div>
          <div v-else class="quick-open-empty">
            <n-text depth="3" style="font-size: var(--ide-font-size);">未找到匹配符号</n-text>
          </div>
        </div>
      </n-modal>

      <n-modal
        v-model:show="confirmDialogVisible"
        :show-icon="false"
        :bordered="false"
        :mask-closable="false"
        preset="card"
        :title="confirmDialogTitle"
        style="width: 400px; max-width: 90vw;"
        :closable="false"
      >
        <div class="confirm-dialog-body">
          <n-text style="font-size: var(--ide-font-size-lg);">{{ confirmDialogMessage }}</n-text>
        </div>
        <template #footer>
          <n-space justify="end">
            <n-button @click="onConfirmCancel">取消</n-button>
            <n-button type="error" @click="onConfirmOk">确定关闭</n-button>
          </n-space>
        </template>
      </n-modal>

      <n-modal
        v-model:show="commandPaletteVisible"
        :show-icon="false"
        :bordered="false"
        :mask-closable="true"
        preset="card"
        title=""
        style="width: 520px; max-width: 90vw; padding: 0;"
        :body-style="{ padding: '0' }"
        :header-style="{ display: 'none' }"
        :footer-style="{ display: 'none' }"
        @keydown="onCommandPaletteKeydown"
      >
        <div class="command-palette">
          <div class="cp-input-wrap">
            <n-input
              ref="commandPaletteInputRef"
              v-model:value="commandPaletteQuery"
              placeholder="输入命令名称搜索 (Ctrl+Shift+P)"
              size="large"
              :bordered="false"
              autofocus
            />
          </div>
          <div v-if="filteredCommands.length > 0" class="cp-list">
            <div
              v-for="(c, i) in filteredCommands"
              :key="c.id"
              class="cp-item"
              :class="{ active: i === commandPaletteSelected }"
              @click="execCommand(c)"
              @mouseenter="commandPaletteSelected = i"
            >
              <span class="cp-icon">{{ c.category === '文件' ? '📄' : c.category === '运行' ? '▶' : c.category === '视图' ? '👁' : c.category === '导航' ? '🧭' : '✏️' }}</span>
              <span class="cp-label">{{ c.label }}</span>
              <span class="cp-cat">{{ c.category }}</span>
              <span v-if="c.shortcut" class="cp-shortcut">{{ c.shortcut }}</span>
            </div>
          </div>
          <div v-else class="cp-empty">
            <n-text depth="3" style="font-size: var(--ide-font-size);">未找到匹配命令</n-text>
          </div>
        </div>
      </n-modal>

      <n-modal
        v-model:show="keybindingsVisible"
        preset="card"
        title="键盘快捷键"
        :bordered="false"
        style="width: 600px; max-width: 90vw; max-height: 80vh;"
      >
        <div class="keybindings-list">
          <div v-for="group in shortcutGroups" :key="group.name" class="kb-group">
            <div class="kb-group-title">{{ group.name }}</div>
            <div v-for="item in group.items" :key="item.key" class="kb-row">
              <span class="kb-desc">{{ item.desc }}</span>
              <span class="kb-keys">
                <template v-for="(k, i) in item.keys" :key="i">
                  <kbd class="kb-key">{{ k }}</kbd>
                  <span v-if="i < item.keys.length - 1" class="kb-plus">+</span>
                </template>
              </span>
            </div>
          </div>
        </div>
      </n-modal>

      <n-modal
        v-model:show="snippetsVisible"
        preset="card"
        title="代码片段管理"
        :bordered="false"
        style="width: 640px; max-width: 90vw;"
      >
        <div style="display: flex; flex-direction: column; gap: 12px;">
          <n-space justify="space-between" align="center">
            <n-text depth="3" style="font-size: var(--ide-font-size);">用户自定义代码片段（Ctrl+Shift+P 触发补全）</n-text>
            <n-space size="small" align="center">
              <n-button size="small" @click="exportSnippets">导出</n-button>
              <n-button size="small" @click="importSnippetsClick">导入</n-button>
              <n-button size="small" quaternary type="error" @click="clearFilteredSnippets">清空</n-button>
            </n-space>
          </n-space>
          <!-- 分类统计 chips：点击即筛选 -->
          <n-space size="small">
            <n-tag
              v-for="opt in snippetFilterOptions"
              :key="String(opt.value)"
              :type="snippetFilterCategory === opt.value ? 'primary' : 'default'"
              :bordered="snippetFilterCategory !== opt.value"
              size="small"
              round
              checkable
              :checked="snippetFilterCategory === opt.value"
              @click="snippetFilterCategory = opt.value; selectedSnippetIndex = -1"
              style="cursor: pointer;"
            >
              {{ opt.label }}
              <template v-if="opt.value !== null">
                （{{ snippetStats[opt.value] || 0 }}）
              </template>
              <template v-else>（{{ snippetList.length }}）</template>
            </n-tag>
          </n-space>
          <!-- 片段列表 -->
          <div v-for="(s, i) in filteredSnippetList" :key="i" style="display: flex; gap: 8px; align-items: center;" :style="{ background: selectedSnippetIndex === i ? 'var(--bg-hover)' : 'transparent', padding: '4px', borderRadius: '4px' }" @click="selectedSnippetIndex = i">
            <n-input v-model:value="s.label" size="small" placeholder="触发词" style="width: 120px;" @click.stop />
            <n-input v-model:value="s.insertText" size="small" placeholder="代码内容（支持 $1 $0 占位符）" style="flex: 1;" @click.stop />
            <n-input v-model:value="s.documentation" size="small" placeholder="说明" style="width: 120px;" @click.stop />
            <n-select v-model:value="s.category" size="small" :options="snippetCategoryOptions" placeholder="分类" style="width: 110px;" @click.stop />
            <n-button size="small" quaternary type="error" @click.stop="removeSnippet(s)">删</n-button>
          </div>
          <!-- 片段预览 -->
          <div v-if="selectedSnippet" style="border: 1px solid var(--border-color); border-radius: 6px; padding: 8px; background: var(--bg-secondary);">
            <n-text depth="3" style="font-size: var(--ide-font-size-sm); display: block; margin-bottom: 4px;">预览：{{ selectedSnippet.label || '（未命名）' }} · {{ selectedSnippet.category || '通用' }}</n-text>
            <pre style="margin: 0; padding: 8px; font-family: var(--ide-code-font, monospace); font-size: var(--ide-font-size-sm); white-space: pre-wrap; word-break: break-all; max-height: 160px; overflow: auto;">{{ selectedSnippet.insertText || '（空）' }}</pre>
            <n-text v-if="selectedSnippet.documentation" depth="3" style="font-size: var(--ide-font-size-sm); display: block; margin-top: 4px;">说明：{{ selectedSnippet.documentation }}</n-text>
          </div>
          <n-space>
            <n-button size="small" type="primary" @click="snippetList.push({ label: '', insertText: '', documentation: '', category: snippetFilterCategory || '通用' }); selectedSnippetIndex = filteredSnippetList.length - 1">添加片段</n-button>
            <n-button size="small" type="primary" @click="saveSnippets">保存</n-button>
          </n-space>
          <input ref="importFileInput" type="file" accept=".json" style="display:none" @change="importSnippetsFile" />
        </div>
      </n-modal>

      <n-modal
        v-model:show="createProjectVisible"
        preset="card"
        title="创建项目"
        :bordered="false"
        style="width: 560px; max-width: 90vw;"
      >
        <div class="create-project-dialog">
          <!-- 模板选择卡片 -->
          <div class="template-section">
            <div class="template-label">选择模板</div>
            <div class="template-grid">
              <div
                v-for="opt in createProjectTemplateOptions"
                :key="opt.value"
                class="template-card"
                :class="{ active: createProjectTemplate === opt.value }"
                @click="createProjectTemplate = opt.value"
              >
                <div class="template-icon">{{ templateIcon(opt.value) }}</div>
                <div class="template-name">{{ opt.label }}</div>
                <div v-if="opt.description" class="template-desc">{{ opt.description }}</div>
              </div>
            </div>
          </div>
          <!-- 项目信息 -->
          <n-form label-placement="left" label-width="80" style="margin-top: 16px;">
            <n-form-item label="项目位置">
              <n-input-group>
                <n-input
                  v-model:value="createProjectParent"
                  readonly
                  placeholder="点击选择项目存放目录"
                  @click="selectProjectParent"
                  style="flex: 1"
                />
                <n-button @click="selectProjectParent" title="浏览选择目录">浏览...</n-button>
              </n-input-group>
            </n-form-item>
            <n-form-item label="项目名称">
              <n-input v-model:value="createProjectName" placeholder="未命名项目" />
            </n-form-item>
            <n-form-item v-if="createProjectParent" label="完整路径">
              <n-text depth="3" style="font-size: var(--ide-font-size-sm); word-break: break-all;">
                {{ createProjectParent }}\{{ createProjectName || '未命名项目' }}
              </n-text>
            </n-form-item>
          </n-form>
        </div>
        <template #footer>
          <n-space justify="end">
            <n-button @click="createProjectVisible = false">取消</n-button>
            <n-button type="primary" @click="confirmCreateProject">创建</n-button>
          </n-space>
        </template>
      </n-modal>

      <n-modal
        v-model:show="newWindowVisible"
        preset="card"
        title="新建窗口"
        :bordered="false"
        style="width: 400px; max-width: 90vw;"
      >
        <n-form label-placement="left" label-width="80">
          <n-form-item label="窗口名称">
            <n-input v-model:value="newWindowName" placeholder="窗口1" />
          </n-form-item>
        </n-form>
        <template #footer>
          <n-space justify="end">
            <n-button @click="newWindowVisible = false">取消</n-button>
            <n-button type="primary" @click="confirmNewWindow">创建</n-button>
          </n-space>
        </template>
      </n-modal>

      <n-modal
        v-model:show="buildConfigVisible"
        preset="card"
        title="编译选项"
        :bordered="false"
        style="width: 720px; max-width: 95vw;"
      >
        <n-form label-placement="left" :label-width="56" size="small" style="margin-top: -8px;">
          <div class="bc-cols">
            <!-- 左栏：项目 + 图标 -->
            <div class="bc-col">
              <div class="bc-section">项目</div>
              <n-form-item label="名称">
                <n-input v-model:value="buildConfig.projectName" placeholder="MyApp" />
              </n-form-item>
              <n-form-item label="版本">
                <n-input v-model:value="buildConfig.version" placeholder="1.0.0" />
              </n-form-item>
              <n-form-item label="作者">
                <n-input v-model:value="buildConfig.author" placeholder="作者" />
              </n-form-item>
              <n-form-item label="描述">
                <n-input
                  v-model:value="buildConfig.description"
                  placeholder="项目描述"
                  type="textarea"
                  :autosize="{ minRows: 2, maxRows: 4 }"
                />
              </n-form-item>
              <div class="bc-section">图标</div>
              <n-form-item label="文件">
                <n-input-group>
                  <n-input v-model:value="buildConfig.iconPath" placeholder="留空用默认（.ico）" style="flex: 1;" />
                  <n-button @click="browseIconPath">浏览…</n-button>
                  <n-button quaternary @click="buildConfig.iconPath = ''" title="清除">清</n-button>
                </n-input-group>
              </n-form-item>
              <div class="bc-section">编译</div>
              <n-form-item label="模式">
                <n-select v-model:value="buildConfig.mode" :options="buildModeOptions" />
              </n-form-item>
              <n-form-item label="完成后">
                <n-checkbox v-model:checked="buildConfig.autoOpenFolder">自动打开产物目录</n-checkbox>
              </n-form-item>
            </div>
            <!-- 右栏：版权信息 -->
            <div class="bc-col">
              <div class="bc-section">版权信息</div>
              <n-form-item label="公司">
                <n-input v-model:value="buildConfig.companyName" placeholder="公司名称" />
              </n-form-item>
              <n-form-item label="产品">
                <n-input v-model:value="buildConfig.productName" placeholder="产品名称" />
              </n-form-item>
              <n-form-item label="描述">
                <n-input v-model:value="buildConfig.fileDescription" placeholder="文件描述" />
              </n-form-item>
              <n-form-item label="版权">
                <n-input
                  v-model:value="buildConfig.legalCopyright"
                  placeholder="© 2026, Company"
                  type="textarea"
                  :autosize="{ minRows: 1, maxRows: 3 }"
                />
              </n-form-item>
              <n-form-item label="备注">
                <n-input
                  v-model:value="buildConfig.comments"
                  placeholder="备注"
                  type="textarea"
                  :autosize="{ minRows: 2, maxRows: 4 }"
                />
              </n-form-item>
            </div>
          </div>
        </n-form>
        <template #footer>
          <n-space justify="end">
            <n-button @click="buildConfigVisible = false">取消</n-button>
            <n-button @click="saveBuildOptions">保存</n-button>
            <n-button type="primary" @click="() => { saveBuildOptions(); buildExecutable() }">保存并构建</n-button>
          </n-space>
        </template>
      </n-modal>
    </div>
    </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup>
import { ref, computed, nextTick, watch, watchEffect, onMounted, onUnmounted } from 'vue'
import { loadAllPlugins, getPluginCommands, executePluginCommand, pluginPanels, pluginComponents } from './plugins/loader.js'
import { darkTheme, lightTheme, NConfigProvider, NSpace, NButton, NText, NTabs, NTabPane, NEmpty, NModal, NForm, NFormItem, NInput, NSelect, NDropdown, NMessageProvider, NDialogProvider, NCheckbox, NInputGroup } from 'naive-ui'
import Editor from './components/Editor.vue'
import FileExplorer from './components/FileExplorer.vue'
import ProjectExplorer from './components/ProjectExplorer.vue'
import TitleBar from './components/TitleBar.vue'
import StartPage from './components/StartPage.vue'
import LeftMenu from './components/LeftMenu.vue'
import SupportPanel from './components/SupportPanel.vue'
import AIPanel from './components/AIPanel.vue'
import { parseEg } from './utils/egParser.js'
import { IDEService } from '../bindings/egou/internal/app'
import { Events } from '@wailsio/runtime'
import { applyTheme, getSavedThemeName, getThemeNames, getThemes, getTheme } from './themes.js'
import { loadProjectLibs, unloadProjectLibs, getProjectLibsSummary, getMergedTree, loadGlobalLibs, libVersion } from './utils/supportCommands.js'
import SettingsPanel from './components/SettingsPanel.vue'
import WindowDesigner from './components/WindowDesigner.vue'
import PropertiesPanel from './components/PropertiesPanel.vue'
import DebugPanel from './components/DebugPanel.vue'
import BuildProgress from './components/BuildProgress.vue'
import { t } from './i18n/index.js'

const themes = getThemes()
const themeNames = getThemeNames()
const themeOptions = computed(() =>
  getThemeNames().map(name => ({ label: getTheme(name).label, value: name }))
)
const currentThemeName = ref(getSavedThemeName())
applyTheme(currentThemeName.value)
const theme = computed(() => getTheme(currentThemeName.value).isDark ? darkTheme : lightTheme)
const isDark = computed(() => getTheme(currentThemeName.value).isDark)
// 界面字号（与 SettingsPanel 联动，同步 Naive UI themeOverrides.common.fontSize）
const uiFontSize = ref(parseInt(localStorage.getItem('eg-uifontsize'), 10) || 13)
if (typeof window !== 'undefined') {
  window.addEventListener('eg-uifontsize-change', (e) => {
    const v = parseInt(e.detail, 10)
    if (v >= 11 && v <= 18) uiFontSize.value = v
  })
}

const themeOverrides = computed(() => {
  const preset = getTheme(currentThemeName.value)
  const accent = preset.variables['--accent-color'] || '#63e2b7'
  const accentHover = preset.variables['--accent-hover'] || accent
  const fs = uiFontSize.value + 'px'
  // Naive UI 字号统一策略：
  // 1. common.fontSize* 全部设为界面字号 → 覆盖 Input/Button/Select/Checkbox/Radio/Dropdown/
  //    Empty/Table/Menu/Tree/List/Alert/Popover/Collapse/InternalSelection/InternalSelectMenu 等
  // 2. 各组件独立命名/硬编码的字号变量（不跟随 common）必须单独覆盖：
  //    Tabs(tabFontSize*)/Card(titleFontSize*)/Dialog(titleFontSize)/Drawer(titleFontSize)/
  //    Tag(fontSize*)/Badge(fontSize)/Message(fontSize)/Notification(title/meta/desc)/
  //    Form(feedback/label*)/Pagination(item/jumper*)/Typography(header*)/PageHeader(title)/
  //    Anchor(link)/Timeline(title*)/Steps(stepHeader/indicatorIndex*)/Statistic(value)/
  //    Progress(circle)/TimePicker(item)/Calendar(title)/Transfer(extra/title*)/Result(title/fontSize*)
  // 3. 保留视觉层级的特殊场景：Statistic 大数字、Progress 圆环、Typography 标题、Result 标题、
  //    Calendar/CalendarTitle 用稍大字号，不与正文完全一致
  const sm = (uiFontSize.value - 1) + 'px'  // 小一档（次要文字）
  const xs = (uiFontSize.value - 2) + 'px'  // 超小档（角标/元信息）
  const lg = (uiFontSize.value + 1) + 'px'  // 大一档（区域标题）
  const xl = (uiFontSize.value + 2) + 'px'  // 超大档（面板/对话框标题）
  return {
    common: {
      borderRadius: '8px',
      fontSize: fs,
      fontSizeMini: fs,
      fontSizeTiny: fs,
      fontSizeSmall: fs,
      fontSizeMedium: fs,
      fontSizeLarge: fs,
      fontSizeHuge: fs,
      fontFamily: 'var(--ide-font)',
      primaryColor: accent,
      primaryColorHover: accentHover,
      primaryColorPressed: accentHover,
      primaryColorSuppl: accent
    },
    // 标签页（输出栏/错误/提示/书签/历史/调试 标签）— 独立字号变量，必须单独覆盖
    Tabs: {
      tabFontSizeSmall: fs,
      tabFontSizeMedium: fs,
      tabFontSizeLarge: fs,
      tabFontSizeCard: fs
    },
    // 卡片标题
    Card: {
      titleFontSizeSmall: lg,
      titleFontSizeMedium: lg,
      titleFontSizeLarge: xl,
      titleFontSizeHuge: xl
    },
    // 对话框/抽屉标题
    Dialog: { titleFontSize: xl },
    Drawer: { titleFontSize: xl },
    PageHeader: { titleFontSize: xl },
    // 标签（能力徽章等）— 重新映射型，比 common 小一档，保持角标视觉
    Tag: {
      fontSizeTiny: xs,
      fontSizeSmall: xs,
      fontSizeMedium: sm,
      fontSizeLarge: sm
    },
    // 徽标数字
    Badge: { fontSize: xs },
    // 消息提示
    Message: { fontSize: fs },
    // 通知标题/元信息/描述
    Notification: {
      titleFontSize: lg,
      metaFontSize: xs,
      descriptionFontSize: sm
    },
    // 表单反馈/标签
    Form: {
      feedbackFontSizeSmall: sm,
      feedbackFontSizeMedium: fs,
      feedbackFontSizeLarge: fs,
      labelFontSizeLeftSmall: fs,
      labelFontSizeLeftMedium: fs,
      labelFontSizeLeftLarge: lg,
      labelFontSizeTopSmall: sm,
      labelFontSizeTopMedium: fs,
      labelFontSizeTopLarge: fs
    },
    // 分页
    Pagination: {
      itemFontSizeSmall: xs,
      itemFontSizeMedium: fs,
      itemFontSizeLarge: fs,
      jumperFontSizeSmall: xs,
      jumperFontSizeMedium: fs,
      jumperFontSizeLarge: fs
    },
    // 时间线/步骤
    Timeline: {
      titleFontSizeMedium: fs,
      titleFontSizeLarge: lg
    },
    Steps: {
      stepHeaderFontSizeSmall: fs,
      stepHeaderFontSizeMedium: lg,
      indicatorIndexFontSizeSmall: fs,
      indicatorIndexFontSizeMedium: lg
    },
    // 锚点链接
    Anchor: { linkFontSize: fs },
    // 时间选择器项
    TimePicker: { itemFontSize: sm },
    // 统计数值（保留大字号视觉层级）
    Statistic: {
      labelFontSize: fs,
      valueFontSize: (uiFontSize.value + 6) + 'px'
    },
    // 进度条圆环数字（保留大字号）
    Progress: { fontSizeCircle: (uiFontSize.value + 8) + 'px' },
    // 日历标题
    Calendar: { titleFontSize: lg },
    // 穿梭框
    Transfer: {
      extraFontSizeSmall: xs,
      extraFontSizeMedium: xs,
      extraFontSizeLarge: sm,
      titleFontSizeSmall: fs,
      titleFontSizeMedium: lg,
      titleFontSizeLarge: lg
    },
    // 结果页（保留大标题视觉层级）
    Result: {
      titleFontSizeSmall: xl,
      titleFontSizeMedium: (uiFontSize.value + 4) + 'px',
      titleFontSizeLarge: (uiFontSize.value + 6) + 'px',
      titleFontSizeHuge: (uiFontSize.value + 8) + 'px',
      fontSizeSmall: fs,
      fontSizeMedium: fs,
      fontSizeLarge: lg,
      fontSizeHuge: lg
    }
  }
})

watchEffect(() => {
  applyTheme(currentThemeName.value)
})

const projectOpen = ref(false)
const projectPath = ref('')
const projectTree = ref([])

const quickOpenVisible = ref(false)
const quickOpenQuery = ref('')
const quickOpenSelected = ref(0)
const quickOpenInputRef = ref(null)
const tabsBarRef = ref(null)

const gotoLineVisible = ref(false)
const gotoLineInput = ref('')
const gotoLineInputRef = ref(null)

const gotoSymbolVisible = ref(false)
const gotoSymbolQuery = ref('')
const gotoSymbolSelected = ref(0)
const gotoSymbolInputRef = ref(null)

// 提前声明 parsed，避免后续 computed/watch 引用时出现 TDZ（暂时性死区）错误
const parsed = ref(parseEg(''))
let parseDebounceTimer = null

const zenMode = ref(false)

const confirmDialogVisible = ref(false)
const confirmDialogTitle = ref('')
const confirmDialogMessage = ref('')
let confirmDialogCallback = null

// Ctrl+K 和弦快捷键状态
let chordKeyTimer = null
let pendingChord = null

function resetChordKey() {
  pendingChord = null
  if (chordKeyTimer) {
    clearTimeout(chordKeyTimer)
    chordKeyTimer = null
  }
}

// ===== 导航历史（Alt+Left/Right 前进后退） =====
const navHistory = ref([])
let navIndex = -1
let navSuppressing = false
const NAV_HISTORY_LIMIT = 100

// ===== 已关闭文件历史（Ctrl+Shift+T 重新打开） =====
const closedFiles = ref([])
const CLOSED_FILES_LIMIT = 20

function pushClosedFile(file) {
  if (!file || !file.path) return
  const list = closedFiles.value
  if (list.length >= CLOSED_FILES_LIMIT) {
    list.shift()
  }
  list.push({ path: file.path, name: file.name })
}

function reopenClosedFile() {
  const list = closedFiles.value
  if (list.length === 0) {
    setStatusMsg('没有可重新打开的文件', 2000)
    return
  }
  const entry = list.pop()
  if (entry && entry.path) {
    openProjectFile(entry.path)
  }
}

function recordNavigation(fileIdx, line) {
  if (navSuppressing) return
  if (!files.value[fileIdx]) return
  const entry = { fileIdx, line: line || 1 }
  // 如果在历史中间位置导航，截断前向历史
  if (navIndex < navHistory.value.length - 1) {
    navHistory.value = navHistory.value.slice(0, navIndex + 1)
  }
  // 避免连续重复记录
  const last = navHistory.value[navHistory.value.length - 1]
  if (last && last.fileIdx === fileIdx && Math.abs((last.line || 1) - (line || 1)) < 3) return
  navHistory.value.push(entry)
  if (navHistory.value.length > NAV_HISTORY_LIMIT) {
    navHistory.value.shift()
  }
  navIndex = navHistory.value.length - 1
}

function navigateBack() {
  while (navIndex > 0) {
    navIndex--
    const entry = navHistory.value[navIndex]
    if (entry && entry.fileIdx < files.value.length) {
      navSuppressing = true
      switchFile(entry.fileIdx)
      nextTick(() => {
        editorRef.value?.gotoLine?.(entry.line)
        navSuppressing = false
      })
      return
    }
  }
  navIndex = 0
  setStatusMsg('已到达导航历史起点', 1500)
}

function navigateForward() {
  while (navIndex < navHistory.value.length - 1) {
    navIndex++
    const entry = navHistory.value[navIndex]
    if (entry && entry.fileIdx < files.value.length) {
      navSuppressing = true
      switchFile(entry.fileIdx)
      nextTick(() => {
        editorRef.value?.gotoLine?.(entry.line)
        navSuppressing = false
      })
      return
    }
  }
  navIndex = navHistory.value.length - 1
  setStatusMsg('已到达导航历史终点', 1500)
}

function showQuickOpen() {
  quickOpenQuery.value = ''
  quickOpenSelected.value = 0
  quickOpenVisible.value = true
  nextTick(() => {
    const input = quickOpenInputRef.value
    if (input && input.focus) input.focus()
    else if (input && input.textareaRef) input.textareaRef.focus()
  })
}

watch(quickOpenVisible, (val) => {
  if (val) {
    nextTick(() => {
      const el = document.querySelector('.quick-open .n-input__input-el')
      if (el) el.focus()
    })
  }
})

function collectOpenableFiles(nodes, out) {
  if (!Array.isArray(nodes)) return
  for (const n of nodes) {
    if (n.IsDir) {
      collectOpenableFiles(n.Children, out)
    } else if (n.Path) {
      const ext = (n.Name || '').toLowerCase().split('.').pop()
      if (['eg', 'ew', 'json', 'txt', 'md'].includes(ext)) {
        out.push({ name: n.Name, path: n.Path })
      }
    }
  }
}

const quickOpenFiles = computed(() => {
  const all = []
  collectOpenableFiles(projectTree.value, all)
  const q = quickOpenQuery.value.trim().toLowerCase()
  if (!q) return all.slice(0, 50)
  return all.filter(f => f.name.toLowerCase().includes(q)).slice(0, 50)
})

function onQuickOpenKeydown(e) {
  const files = quickOpenFiles.value
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    quickOpenSelected.value = Math.min(quickOpenSelected.value + 1, files.length - 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    quickOpenSelected.value = Math.max(quickOpenSelected.value - 1, 0)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const f = files[quickOpenSelected.value]
    if (f) {
      quickOpenVisible.value = false
      openProjectFile(f.path)
    }
  } else if (e.key === 'Escape') {
    quickOpenVisible.value = false
  }
}

watch(quickOpenFiles, () => {
  quickOpenSelected.value = 0
})

// ===== 命令面板 (Ctrl+Shift+P) =====
const commandPaletteVisible = ref(false)
const commandPaletteQuery = ref('')
const commandPaletteSelected = ref(0)
const recentCommands = ref(JSON.parse(localStorage.getItem('eg-recent-cmds') || '[]'))

function recordCommandUsed(cmdId) {
  const list = recentCommands.value.filter(id => id !== cmdId)
  list.unshift(cmdId)
  if (list.length > 10) list.length = 10
  recentCommands.value = list
  localStorage.setItem('eg-recent-cmds', JSON.stringify(list))
}

function execCommand(c) {
  commandPaletteVisible.value = false
  if (c && c.id) recordCommandUsed(c.id)
  c?.action?.()
}
const commandPaletteInputRef = ref(null)

function getCommandList() {
  const hasFile = activeFileIndex.value >= 0 && files.value.length > 0
  const hasProject = projectOpen.value
  const hasEditor = hasFile
  const dbg = isDebugging.value
  return [
    // 文件操作
    { id: 'newFile', label: '新建代码文件', shortcut: 'Ctrl+N', category: '文件', action: () => newCodeFile(), enabled: hasProject },
    { id: 'newWindow', label: '新建窗口', shortcut: '', category: '文件', action: () => { newWindowVisible.value = true; newWindowName.value = '窗口' + (files.value.filter(f => isFormFileName(f.name)).length + 1) }, enabled: hasProject },
    { id: 'newClass', label: '新建类', shortcut: '', category: '文件', action: () => newClassFile(), enabled: hasProject },
    { id: 'newModule', label: '新建模块', shortcut: '', category: '文件', action: () => newModuleFile(), enabled: hasProject },
    { id: 'openFile', label: '打开文件...', shortcut: 'Ctrl+O', category: '文件', action: () => openFile(), enabled: hasProject },
    { id: 'save', label: '保存', shortcut: 'Ctrl+S', category: '文件', action: () => quickSave(), enabled: hasFile },
    { id: 'saveAs', label: '另存为...', shortcut: 'Ctrl+Shift+S', category: '文件', action: () => saveFile(true), enabled: hasFile },
    { id: 'saveAll', label: '保存全部', shortcut: '', category: '文件', action: () => saveAllFiles(), enabled: files.value.length > 0 },
    { id: 'closeFile', label: '关闭文件', shortcut: 'Ctrl+W', category: '文件', action: () => closeFile(activeFileIndex.value), enabled: hasFile },
    { id: 'closeOtherFiles', label: '关闭其他文件', shortcut: '', category: '文件', action: () => closeOtherFiles(activeFileIndex.value), enabled: files.value.length > 1 },
    { id: 'closeLeftFiles', label: '关闭左侧文件', shortcut: '', category: '文件', action: () => closeLeftFiles(activeFileIndex.value), enabled: files.value.length > 1 && activeFileIndex.value > 0 },
    { id: 'closeRightFiles', label: '关闭右侧文件', shortcut: '', category: '文件', action: () => closeRightFiles(activeFileIndex.value), enabled: files.value.length > 1 && activeFileIndex.value < files.value.length - 1 },
    { id: 'closeAllFiles', label: '关闭所有文件', shortcut: '', category: '文件', action: () => closeAllFiles(), enabled: files.value.length > 0 },
    { id: 'reopenClosed', label: '重新打开已关闭的文件', shortcut: 'Ctrl+Shift+T', category: '文件', action: () => reopenClosedFile(), enabled: closedFiles.value.length > 0 || projectOpen.value },
    // 编译运行
    { id: 'run', label: '编译运行', shortcut: 'F5', category: '运行', action: () => runCode(), enabled: hasProject },
    { id: 'build', label: '生成可执行文件', shortcut: '', category: '运行', action: () => buildExecutable(), enabled: hasProject },
    { id: 'buildOptions', label: '编译选项...', shortcut: '', category: '运行', action: () => openBuildOptions(), enabled: hasProject },
    // 调试
    { id: 'startDebug', label: t('command.startDebug'), shortcut: '', category: t('command.categoryDebug'), action: () => debugPanelRef.value?.startDebug?.(), enabled: hasProject && !dbg },
    { id: 'stopDebug', label: t('command.stopDebug'), shortcut: '', category: t('command.categoryDebug'), action: () => debugPanelRef.value?.stopDebug?.(), enabled: dbg },
    { id: 'debugContinue', label: t('command.debugContinue'), shortcut: 'F5', category: t('command.categoryDebug'), action: () => IDEService.DebugContinue().catch(() => {}), enabled: dbg },
    { id: 'debugStepOver', label: t('command.debugStepOver'), shortcut: 'F10', category: t('command.categoryDebug'), action: () => IDEService.DebugNext().catch(() => {}), enabled: dbg },
    { id: 'debugStepInto', label: t('command.debugStepInto'), shortcut: 'F11', category: t('command.categoryDebug'), action: () => IDEService.DebugStep().catch(() => {}), enabled: dbg },
    { id: 'debugStepOut', label: t('command.debugStepOut'), shortcut: 'Shift+F11', category: t('command.categoryDebug'), action: () => IDEService.DebugStepOut().catch(() => {}), enabled: dbg },
    { id: 'toggleBreakpoint', label: t('command.toggleBreakpoint'), shortcut: 'F9', category: t('command.categoryDebug'), action: () => { const ln = editorRef.value?.getCurrentLine?.(); if (ln) onToggleBreakpoint(ln) }, enabled: hasEditor },
    // 视图
    { id: 'fullscreen', label: '切换全屏', shortcut: 'F11', category: '视图', action: () => IDEService.ToggleFullscreen(), enabled: true },
    { id: 'toggleOutput', label: '切换输出面板', shortcut: 'Ctrl+`', category: '视图', action: () => { outputCollapsed.value = !outputCollapsed.value }, enabled: true },
    { id: 'toggleWordWrap', label: '切换自动换行', shortcut: 'Alt+Z', category: '视图', action: () => toggleWordWrap(), enabled: hasEditor },
    { id: 'toggleMinimap', label: '切换缩略图', shortcut: '', category: '视图', action: () => toggleMinimap(), enabled: hasEditor },
    { id: 'zenMode', label: '切换禅模式', shortcut: 'Ctrl+K Z', category: '视图', action: () => toggleZenMode(), enabled: true },
    // 导航
    { id: 'quickOpen', label: '快速打开文件', shortcut: 'Ctrl+P', category: '导航', action: () => { commandPaletteVisible.value = false; nextTick(() => showQuickOpen()) }, enabled: hasProject },
    { id: 'gotoSymbol', label: '转到符号', shortcut: 'Ctrl+Shift+O', category: '导航', action: () => { commandPaletteVisible.value = false; nextTick(() => showGotoSymbol()) }, enabled: hasFile },
    { id: 'gotoLine', label: '转到行...', shortcut: 'Ctrl+G', category: '导航', action: () => { commandPaletteVisible.value = false; nextTick(() => showGotoLine()) }, enabled: hasFile },
    { id: 'nextTab', label: '下一个标签页', shortcut: 'Ctrl+Tab', category: '导航', action: () => { const n = activeFileIndex.value + 1; switchFile(n >= files.value.length ? 0 : n) }, enabled: files.value.length > 1 },
    { id: 'prevTab', label: '上一个标签页', shortcut: 'Ctrl+Shift+Tab', category: '导航', action: () => { const p = activeFileIndex.value - 1; switchFile(p < 0 ? files.value.length - 1 : p) }, enabled: files.value.length > 1 },
    { id: 'navBack', label: '后退', shortcut: 'Alt+←', category: '导航', action: () => navigateBack(), enabled: navIndex > 0 },
    { id: 'navForward', label: '前进', shortcut: 'Alt+→', category: '导航', action: () => navigateForward(), enabled: navIndex < navHistory.value.length - 1 },
    // 编辑器
    { id: 'toggleComment', label: '切换行注释', shortcut: 'Ctrl+/', category: '编辑器', action: () => runEditorAction('editor.action.commentLine'), enabled: hasEditor },
    { id: 'addSelectionNext', label: '将下一个匹配项添加到选区', shortcut: 'Ctrl+D', category: '编辑器', action: () => runEditorAction('editor.action.addSelectionToNextFindMatch'), enabled: hasEditor },
    { id: 'selectAllOccurrences', label: '选中所有匹配项', shortcut: 'Ctrl+Shift+L', category: '编辑器', action: () => runEditorAction('editor.action.selectHighlights'), enabled: hasEditor },
    { id: 'moveLineUp', label: '向上移动行', shortcut: 'Alt+↑', category: '编辑器', action: () => runEditorAction('editor.action.moveLinesUpAction'), enabled: hasEditor },
    { id: 'moveLineDown', label: '向下移动行', shortcut: 'Alt+↓', category: '编辑器', action: () => runEditorAction('editor.action.moveLinesDownAction'), enabled: hasEditor },
    { id: 'copyLineUp', label: '向上复制行', shortcut: 'Shift+Alt+↑', category: '编辑器', action: () => runEditorAction('editor.action.copyLinesUpAction'), enabled: hasEditor },
    { id: 'copyLineDown', label: '向下复制行', shortcut: 'Shift+Alt+↓', category: '编辑器', action: () => runEditorAction('editor.action.copyLinesDownAction'), enabled: hasEditor },
    { id: 'deleteLine', label: '删除行', shortcut: 'Ctrl+Shift+K', category: '编辑器', action: () => runEditorAction('editor.action.deleteLines'), enabled: hasEditor },
    { id: 'jumpToBracket', label: '跳转到匹配括号', shortcut: 'Ctrl+Shift+\\', category: '编辑器', action: () => { runEditorAction('editor.action.jumpToBracket') }, enabled: hasEditor },
    { id: 'insertCursorBelow', label: '向下添加光标', shortcut: 'Ctrl+Alt+↓', category: '编辑器', action: () => runEditorAction('editor.action.insertCursorBelow'), enabled: hasEditor },
    { id: 'insertCursorAbove', label: '向上添加光标', shortcut: 'Ctrl+Alt+↑', category: '编辑器', action: () => runEditorAction('editor.action.insertCursorAbove'), enabled: hasEditor },
    { id: 'foldAll', label: '全部折叠', shortcut: 'Ctrl+K Ctrl+0', category: '编辑器', action: () => runEditorAction('editor.foldAll'), enabled: hasEditor },
    { id: 'unfoldAll', label: '全部展开', shortcut: 'Ctrl+K Ctrl+J', category: '编辑器', action: () => runEditorAction('editor.unfoldAll'), enabled: hasEditor },
    { id: 'findHistory', label: '查找历史', shortcut: 'Alt+F', category: '编辑器', action: () => { editorRef.value?.toggleFindHistory?.() }, enabled: hasEditor },
    { id: 'formatDoc', label: '格式化文档', shortcut: 'Ctrl+Shift+F', category: '编辑器', action: () => runEditorAction('editor.action.formatDocument'), enabled: hasEditor },
    { id: 'trimTrailing', label: '裁剪尾随空格', shortcut: 'Ctrl+K Ctrl+X', category: '编辑器', action: () => runEditorAction('editor.action.trimTrailingWhitespace'), enabled: hasEditor },
    { id: 'addLineComment', label: '添加行注释', shortcut: 'Ctrl+K Ctrl+C', category: '编辑器', action: () => runEditorAction('editor.action.addCommentLine'), enabled: hasEditor },
    { id: 'removeLineComment', label: '取消行注释', shortcut: 'Ctrl+K Ctrl+U', category: '编辑器', action: () => runEditorAction('editor.action.removeCommentLine'), enabled: hasEditor },
    { id: 'insertLineBelow', label: '在下方插入行', shortcut: 'Ctrl+Enter', category: '编辑器', action: () => runEditorAction('editor.action.insertLineAfter'), enabled: hasEditor },
    { id: 'insertLineAbove', label: '在上方插入行', shortcut: 'Ctrl+Shift+Enter', category: '编辑器', action: () => runEditorAction('editor.action.insertLineBefore'), enabled: hasEditor },
    { id: 'transformToUppercase', label: '转换为大写', category: '编辑器', action: () => runEditorAction('editor.action.transformToUppercase'), enabled: hasEditor },
    { id: 'transformToLowercase', label: '转换为小写', category: '编辑器', action: () => runEditorAction('editor.action.transformToLowercase'), enabled: hasEditor },
    { id: 'expandSelection', label: '扩展选区', shortcut: 'Shift+Alt+→', category: '编辑器', action: () => runEditorAction('editor.action.smartSelect.expand'), enabled: hasEditor },
    { id: 'shrinkSelection', label: '收缩选区', shortcut: 'Shift+Alt+←', category: '编辑器', action: () => runEditorAction('editor.action.smartSelect.shrink'), enabled: hasEditor },
    { id: 'keybindings', label: '键盘快捷键', shortcut: 'Ctrl+K Ctrl+S', category: '视图', action: () => showKeybindings(), enabled: true },
  ].filter(c => c.enabled)
}

function runEditorAction(actionId) {
  editorRef.value?.runAction?.(actionId)
}

function showCommandPalette() {
  commandPaletteQuery.value = ''
  commandPaletteSelected.value = 0
  commandPaletteVisible.value = true
  nextTick(() => {
    const el = document.querySelector('.command-palette .n-input__input-el')
    if (el) el.focus()
  })
}

const filteredCommands = computed(() => {
  const all = getCommandList()
  const q = commandPaletteQuery.value.trim().toLowerCase()
  if (!q) {
    const recent = recentCommands.value
      .map(id => all.find(c => c.id === id))
      .filter(Boolean)
    const rest = all.filter(c => !recentCommands.value.includes(c.id))
    return [...recent, ...rest]
  }
  return all.filter(c =>
    c.label.toLowerCase().includes(q) ||
    c.category.toLowerCase().includes(q)
  )
})

function onCommandPaletteKeydown(e) {
  const cmds = filteredCommands.value
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    commandPaletteSelected.value = Math.min(commandPaletteSelected.value + 1, cmds.length - 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    commandPaletteSelected.value = Math.max(commandPaletteSelected.value - 1, 0)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const c = cmds[commandPaletteSelected.value]
    if (c) execCommand(c)
  } else if (e.key === 'Escape') {
    commandPaletteVisible.value = false
  }
}

// watch 延迟到 onMounted 注册，避免 setup 阶段 TDZ 错误（filteredCommands 依赖 activeFileIndex/files 等稍后声明的变量）

// ===== 快捷键速查表 =====
const keybindingsVisible = ref(false)

const shortcutGroups = [
  {
    name: '通用',
    items: [
      { key: 'cmd-palette', desc: '命令面板', keys: ['Ctrl', 'Shift', 'P'] },
      { key: 'quick-open', desc: '快速打开文件', keys: ['Ctrl', 'P'] },
      { key: 'goto-symbol', desc: '转到符号', keys: ['Ctrl', 'Shift', 'O'] },
      { key: 'goto-line', desc: '转到行', keys: ['Ctrl', 'G'] },
      { key: 'zen-mode', desc: '切换禅模式', keys: ['Ctrl', 'K', 'Z'] },
      { key: 'fullscreen', desc: '切换全屏', keys: ['F11'] },
      { key: 'keybindings', desc: '键盘快捷键', keys: ['Ctrl', 'K', 'Ctrl', 'S'] },
      { key: 'trim-trailing', desc: '裁剪尾随空格', keys: ['Ctrl', 'K', 'Ctrl', 'X'] },
      { key: 'add-comment', desc: '添加行注释', keys: ['Ctrl', 'K', 'Ctrl', 'C'] },
      { key: 'remove-comment', desc: '取消行注释', keys: ['Ctrl', 'K', 'Ctrl', 'U'] },
    ]
  },
  {
    name: '文件操作',
    items: [
      { key: 'new-file', desc: '新建代码文件', keys: ['Ctrl', 'N'] },
      { key: 'open-file', desc: '打开文件', keys: ['Ctrl', 'O'] },
      { key: 'save', desc: '保存', keys: ['Ctrl', 'S'] },
      { key: 'save-as', desc: '另存为', keys: ['Ctrl', 'Shift', 'S'] },
      { key: 'close-file', desc: '关闭文件', keys: ['Ctrl', 'W'] },
      { key: 'reopen-closed', desc: '重新打开已关闭文件', keys: ['Ctrl', 'Shift', 'T'] },
      { key: 'next-tab', desc: '下一个标签页', keys: ['Ctrl', 'Tab'] },
      { key: 'prev-tab', desc: '上一个标签页', keys: ['Ctrl', 'Shift', 'Tab'] },
    ]
  },
  {
    name: '编辑',
    items: [
      { key: 'undo', desc: '撤销', keys: ['Ctrl', 'Z'] },
      { key: 'redo', desc: '重做', keys: ['Ctrl', 'Y'] },
      { key: 'comment', desc: '切换行注释', keys: ['Ctrl', '/'] },
      { key: 'format', desc: '格式化文档', keys: ['Ctrl', 'Shift', 'F'] },
      { key: 'select-next', desc: '选中下一个匹配项', keys: ['Ctrl', 'D'] },
      { key: 'select-all-matches', desc: '选中所有匹配项', keys: ['Ctrl', 'Shift', 'L'] },
      { key: 'add-cursor-down', desc: '在下方添加光标', keys: ['Ctrl', 'Alt', '↓'] },
      { key: 'add-cursor-up', desc: '在上方添加光标', keys: ['Ctrl', 'Alt', '↑'] },
    ]
  },
  {
    name: '运行',
    items: [
      { key: 'run', desc: '编译运行', keys: ['F5'] },
    ]
  },
  {
    name: t('command.categoryDebug'),
    items: [
      { key: 'debug-continue', desc: t('command.debugContinueDesc'), keys: ['F5'] },
      { key: 'debug-step-over', desc: t('command.debugStepOver'), keys: ['F10'] },
      { key: 'debug-step-into', desc: t('command.debugStepInto'), keys: ['F11'] },
      { key: 'debug-step-out', desc: t('command.debugStepOut'), keys: ['Shift', 'F11'] },
      { key: 'toggle-bp', desc: t('command.toggleBreakpoint'), keys: ['F9'] },
      { key: 'toggle-bp-glyph', desc: t('command.toggleBreakpointGlyph'), keys: ['Shift', '点击行号栏'] },
    ]
  },
  {
    name: '导航',
    items: [
      { key: 'find', desc: '查找', keys: ['Ctrl', 'F'] },
      { key: 'replace', desc: '替换', keys: ['Ctrl', 'H'] },
      { key: 'find-history', desc: '查找历史', keys: ['Alt', 'F'] },
      { key: 'goto-def', desc: '转到定义', keys: ['F12'] },
      { key: 'find-refs', desc: '查找引用', keys: ['Shift', 'F12'] },
      { key: 'rename', desc: '重命名符号', keys: ['F2'] },
      { key: 'nav-back', desc: '后退', keys: ['Alt', '←'] },
      { key: 'nav-forward', desc: '前进', keys: ['Alt', '→'] },
    ]
  },
  {
    name: t('command.categoryBookmark'),
    items: [
      { key: 'toggle-bookmark', desc: t('command.toggleBookmark'), keys: ['Ctrl', 'F2'] },
      { key: 'next-bookmark', desc: t('command.nextBookmark'), keys: ['Alt', 'F2'] },
      { key: 'prev-bookmark', desc: t('command.prevBookmark'), keys: ['Alt', 'Shift', 'F2'] },
    ]
  },
  {
    name: '编辑器',
    items: [
      { key: 'multi-cursor-next', desc: '选中下一个匹配项', keys: ['Ctrl', 'D'] },
      { key: 'multi-cursor-all', desc: '选中所有匹配项', keys: ['Ctrl', 'Shift', 'L'] },
      { key: 'move-line-up', desc: '向上移动行', keys: ['Alt', '↑'] },
      { key: 'move-line-down', desc: '向下移动行', keys: ['Alt', '↓'] },
      { key: 'copy-line-up', desc: '向上复制行', keys: ['Shift', 'Alt', '↑'] },
      { key: 'copy-line-down', desc: '向下复制行', keys: ['Shift', 'Alt', '↓'] },
      { key: 'delete-line', desc: '删除行', keys: ['Ctrl', 'Shift', 'K'] },
      { key: 'toggle-comment', desc: '切换行注释', keys: ['Ctrl', '/'] },
      { key: 'format-doc', desc: '格式化文档', keys: ['Ctrl', 'Shift', 'F'] },
      { key: 'fold-all', desc: '全部折叠', keys: ['Ctrl', 'K', 'Ctrl', '0'] },
      { key: 'unfold-all', desc: '全部展开', keys: ['Ctrl', 'K', 'Ctrl', 'J'] },
      { key: 'bracket-jump', desc: '跳转到配对括号', keys: ['Ctrl', 'Shift', '\\'] },
      { key: 'zoom-in', desc: '放大字体', keys: ['Ctrl', '滚轮↑'] },
      { key: 'zoom-out', desc: '缩小字体', keys: ['Ctrl', '滚轮↓'] },
    ]
  },
  {
    name: '鼠标',
    items: [
      { key: 'middle-close', desc: '中键关闭标签页', keys: ['鼠标中键'] },
      { key: 'dblclick-close', desc: '双击关闭标签页', keys: ['双击标签'] },
      { key: 'ctrl-click', desc: 'Ctrl+点击跳转到定义', keys: ['Ctrl', '点击'] },
      { key: 'alt-click', desc: 'Alt+点击添加多光标', keys: ['Alt', '点击'] },
    ]
  }
]

function showKeybindings() {
  keybindingsVisible.value = true
}

// 递归统计项目内 .eg 文件数（用于状态栏显示）
const countEgFiles = computed(() => {
  let count = 0
  function walk(nodes) {
    if (!Array.isArray(nodes)) return
    for (const n of nodes) {
      if (n.IsDir) walk(n.Children)
      else if (n.Path && n.Path.toLowerCase().endsWith('.eg')) count++
    }
  }
  walk(projectTree.value)
  return count
})
const projectConfig = ref(null)
const settingsVisible = ref(false)
const settingsActiveMenu = ref('')
// L3：持久化 ref 工厂，合并 ref 声明与 localStorage watch，消除 20 个独立 watch 样板。
// persistedBoolTrue: getItem === 'true'（默认 false）
// persistedBoolNotFalse: getItem !== 'false'（默认 true）
// persistedInt: parseInt(getItem || def, 10)
// persistedStr: getItem || def
function persistedBoolTrue(key) {
  const r = ref(localStorage.getItem(key) === 'true')
  watch(r, (v) => localStorage.setItem(key, v ? 'true' : 'false'))
  return r
}
function persistedBoolNotFalse(key) {
  const r = ref(localStorage.getItem(key) !== 'false')
  watch(r, (v) => localStorage.setItem(key, v ? 'true' : 'false'))
  return r
}
function persistedInt(key, def) {
  const r = ref(parseInt(localStorage.getItem(key) || String(def), 10))
  watch(r, (v) => localStorage.setItem(key, String(v)))
  return r
}
function persistedStr(key, def) {
  const r = ref(localStorage.getItem(key) || def)
  watch(r, (v) => localStorage.setItem(key, v))
  return r
}
const minimapEnabled = persistedBoolTrue('eg-minimap')
const editorFontSize = persistedInt('eg-fontsize', 14)
const editorLineNumbers = persistedBoolNotFalse('eg-linenumbers')
const editorLineHeight = persistedInt('eg-lineheight', 0)
const autoSaveDelay = persistedInt('eg-autosave', 3000)
const editorTabSize = persistedInt('eg-tabsize', 4)
const editorWordWrap = persistedBoolTrue('eg-wordwrap')
const editorRenderWhitespace = persistedStr('eg-whitespace', 'selection')
const editorCursorBlinking = persistedStr('eg-cursorblink', 'blink')
const editorCursorSmoothCaret = persistedBoolNotFalse('eg-cursorsmooth')
const editorCursorWidth = persistedInt('eg-cursorwidth', 0)
const editorBracketPairColorization = persistedBoolNotFalse('eg-bracketcolor')
const editorGuidesBracketPairs = persistedBoolTrue('eg-bracketguides')
const editorFontLigatures = persistedBoolTrue('eg-fontligatures')
const editorLineNumbersMinChars = persistedInt('eg-linenumminchars', 3)
const editorRenderFinalNewline = persistedBoolNotFalse('eg-renderfinalnewline')
const editorMinimapShowSlider = persistedStr('eg-minimapslider', 'mouseover')
const editorMinimapRenderCharacters = persistedBoolNotFalse('eg-minimaprenderchars')
const editorMinimapMaxColumn = persistedInt('eg-minimapmaxcol', 120)
const editorTheme = persistedStr('eg-editor-theme', 'auto')
const editorFontFamily = persistedStr('eg-fontfamily', "'IdeFont', 'Consolas', 'Courier New', monospace")
// 编辑器字体族变化时，同步更新 --ide-code-font CSS 变量，让 AI 面板/代码块等跟随设置
watch(editorFontFamily, (v) => {
  document.documentElement.style.setProperty('--ide-code-font', v)
}, { immediate: true })
const editorAutoConvertSymbols = persistedBoolNotFalse('eg-autoconvert')
// 设计器补充设置（designerShowGrid/designerSnapEnabled/designerGridSize 已在下方定义）
const designerDefaultRadius = persistedInt('eg-designer-radius', 0)
const designerDefaultBorderWidth = persistedInt('eg-designer-borderwidth', 1)
// 编译
const buildDefaultMode = persistedStr('eg-build-mode', 'debug')
const buildAutoOpenFolder = persistedBoolNotFalse('eg-build-autofolder')
// v0.8.0 修订：Garble 混淆强度三档（off/basic/full），默认 basic（仅 -tiny，无杀软误报）
// 旧 localStorage 键 'eg-build-garble'（布尔）已废弃，新键 'eg-build-garble-level'（字符串）
const buildGarbleLevel = persistedStr('eg-build-garble-level', 'basic')
const buildShowHistory = persistedBoolNotFalse('eg-build-history')
const buildOutputDir = persistedStr('eg-build-outputdir', 'bin')
// v0.9.2：Go/dlv 路径可配置（企业锁版本或 dlv 版本不匹配时用户自行指定，留空则自动查找）
const buildGoPath = persistedStr('eg-build-go-path', '')
const buildDelvePath = persistedStr('eg-build-delve-path', '')
// 界面
const uiOpenLastProject = persistedBoolNotFalse('eg-ui-lastproject')
const sbShowCursor = persistedBoolNotFalse('eg-sb-cursor')
const sbShowIndent = persistedBoolNotFalse('eg-sb-indent')
const sbShowEncoding = persistedBoolNotFalse('eg-sb-encoding')
const sbShowEol = persistedBoolNotFalse('eg-sb-eol')
const sbShowLang = persistedBoolNotFalse('eg-sb-lang')
const sbShowHealth = persistedBoolNotFalse('eg-sb-health')
const uiAutoSwitchOutputTab = persistedBoolNotFalse('eg-ui-autoswitchtab')
const uiSmartScroll = persistedBoolNotFalse('eg-ui-smartscroll')
// AI
const aiProvider = persistedStr('eg-ai-provider', 'openai')
const aiModel = persistedStr('eg-ai-model', '')
const aiEndpoint = persistedStr('eg-ai-endpoint', '')
const aiApiKey = persistedStr('eg-ai-apikey', '')
const aiTemperature = ref(parseFloat(localStorage.getItem('eg-ai-temp') || '0.7'))
watch(aiTemperature, v => localStorage.setItem('eg-ai-temp', String(v)))
const aiMaxTokens = persistedInt('eg-ai-maxtokens', 4096)
const aiStream = persistedBoolNotFalse('eg-ai-stream')
const aiCompressThreshold = persistedInt('eg-ai-threshold', 6000)
const aiKeepRecent = persistedInt('eg-ai-keeprecent', 8)

// 旧智能体ID迁移映射
const OLD_AGENT_ID_MAP = {
  'coder': 'eg-coder',
  'debugger': 'eg-debug',
  'architect': 'eg-arch',
  'reviewer': 'eg-coder',
  'explainer': 'eg-teacher'
}
let savedAgent = localStorage.getItem('eg-ai-agent') || 'eg-coder'
if (OLD_AGENT_ID_MAP[savedAgent]) savedAgent = OLD_AGENT_ID_MAP[savedAgent]
const aiCurrentAgent = ref(savedAgent)
watch(aiCurrentAgent, v => localStorage.setItem('eg-ai-agent', v))
const aiCustomAgents = ref(JSON.parse(localStorage.getItem('eg-ai-custom-agents') || '[]'))
watch(aiCustomAgents, (v) => localStorage.setItem('eg-ai-custom-agents', JSON.stringify(v)), { deep: true })

// 多模型管理
const MODEL_CAPABILITIES = {
  'gpt-4o': { vision: true, files: true },
  'gpt-4o-mini': { vision: true, files: false },
  'gpt-4-turbo': { vision: true, files: true },
  'glm-4v': { vision: true, files: false },
  'glm-4v-plus': { vision: true, files: false },
  'glm-4.5v': { vision: true, files: false },
  'qwen-vl': { vision: true, files: false },
  'qwen2.5-vl': { vision: true, files: false },
  'qwen3-vl': { vision: true, files: false },
  'deepseek-vl': { vision: true, files: false },
}

function getModelCaps(modelName) {
  if (!modelName) return { vision: false, files: false }
  const lower = modelName.toLowerCase()
  for (const [key, caps] of Object.entries(MODEL_CAPABILITIES)) {
    if (lower.includes(key.toLowerCase())) return caps
  }
  // 通用规则：名字含 vision/vl/4v/4o/多模态 支持图片
  if (/vision|-vl$|4v|4o|多模态/.test(lower)) return { vision: true, files: false }
  return { vision: false, files: false }
}

function loadModels() {
  try {
    const saved = localStorage.getItem('eg-ai-models')
    if (saved) {
      const arr = JSON.parse(saved)
      if (Array.isArray(arr) && arr.length > 0) return arr
    }
  } catch (e) {}
  // 迁移旧的单模型配置，或预置智谱GLM-4.7-Flash为默认
  const oldEndpoint = aiEndpoint.value || 'https://open.bigmodel.cn/api/paas/v4'
  const oldModel = aiModel.value || 'glm-4.7-flash'
  const oldApiKey = aiApiKey.value || ''
  const caps = getModelCaps(oldModel)
  return [{
    id: 'default',
    name: oldModel === 'glm-4.7-flash' ? '智谱 GLM-4.7-Flash（免费）' : '默认模型',
    provider: aiProvider.value || 'zhipu',
    endpoint: oldEndpoint,
    apiKey: oldApiKey,
    model: oldModel,
    temperature: aiTemperature.value || 0.7,
    maxTokens: aiMaxTokens.value || 4096,
    supportsVision: caps.vision,
    supportsFiles: caps.files,
  }]
}

const aiModels = ref(loadModels())
const activeModelId = persistedStr('eg-ai-active-model', 'default')

watch(aiModels, (v) => localStorage.setItem('eg-ai-models', JSON.stringify(v)), { deep: true })

function ensureActiveModel() {
  if (!aiModels.value.find(m => m.id === activeModelId.value)) {
    if (aiModels.value.length > 0) activeModelId.value = aiModels.value[0].id
  }
}
ensureActiveModel()
watch(aiModels, ensureActiveModel)

const currentModel = computed(() => {
  return aiModels.value.find(m => m.id === activeModelId.value) || aiModels.value[0] || null
})

const aiConfig = computed(() => {
  const m = currentModel.value
  return {
    endpoint: m?.endpoint || aiEndpoint.value,
    apiKey: m?.apiKey || aiApiKey.value,
    model: m?.model || aiModel.value,
    temperature: m?.temperature ?? aiTemperature.value,
    maxTokens: m?.maxTokens ?? aiMaxTokens.value,
    stream: aiStream.value,
    compressThreshold: aiCompressThreshold.value,
    keepRecent: aiKeepRecent.value,
    supportsVision: m?.supportsVision || false,
    supportsFiles: m?.supportsFiles || false,
    modelName: m?.name || m?.model || '未配置',
  }
})
function onFontSizeChange(fs) {
  editorFontSize.value = fs
}
const snippetsVisible = ref(false)
const snippetList = ref([])
const importFileInput = ref(null)
const snippetCategoryOptions = [
  { label: '通用', value: '通用' },
  { label: '控制结构', value: '控制结构' },
  { label: '函数', value: '函数' },
  { label: '类型', value: '类型' },
  { label: '窗口', value: '窗口' },
  { label: 'IO操作', value: 'IO操作' },
  { label: '数学', value: '数学' },
  { label: '字符串', value: '字符串' },
]
const snippetFilterCategory = ref(null)
const snippetFilterOptions = computed(() => [
  { label: '全部', value: null },
  ...snippetCategoryOptions
])
const filteredSnippetList = computed(() => {
  if (!snippetFilterCategory.value) return snippetList.value
  return snippetList.value.filter(s => (s.category || '通用') === snippetFilterCategory.value)
})
// 分类统计：每个分类的片段数量
const snippetStats = computed(() => {
  const stats = {}
  for (const c of snippetCategoryOptions) stats[c.value] = 0
  for (const s of snippetList.value) {
    const cat = s.category || '通用'
    stats[cat] = (stats[cat] || 0) + 1
  }
  return stats
})
// 当前选中的片段索引（用于预览）
const selectedSnippetIndex = ref(-1)
const selectedSnippet = computed(() => {
  if (selectedSnippetIndex.value < 0 || selectedSnippetIndex.value >= filteredSnippetList.value.length) return null
  return filteredSnippetList.value[selectedSnippetIndex.value]
})
function removeSnippet(s) {
  const idx = snippetList.value.findIndex(x => x === s)
  if (idx >= 0) snippetList.value.splice(idx, 1)
}

function showSnippets() {
  if (editorRef.value?.getUserSnippets) {
    snippetList.value = editorRef.value.getUserSnippets()
  }
  snippetsVisible.value = true
}
function saveSnippets() {
  if (!editorRef.value) return
  // 先清除所有，再重新添加
  const existing = editorRef.value.getUserSnippets()
  for (const s of existing) {
    editorRef.value.removeUserSnippet(s.label)
  }
  for (const s of snippetList.value) {
    if (s.label && s.insertText) {
      editorRef.value.addUserSnippet(s.label, s.insertText, s.documentation || '', s.category || '通用')
    }
  }
  setStatusMsg('代码片段已保存', 2000)
  snippetsVisible.value = false
}
function exportSnippets() {
  // 导出筛选后的片段（未筛选则导出全部）
  const data = JSON.stringify(filteredSnippetList.value, null, 2)
  const blob = new Blob([data], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'eg-snippets.json'
  a.click()
  URL.revokeObjectURL(url)
}
// 清空当前筛选分类下的所有片段
function clearFilteredSnippets() {
  const cat = snippetFilterCategory.value
  if (!cat) {
    if (snippetList.value.length === 0) return
    if (!confirm('确定要清空所有代码片段吗？此操作不可撤销。')) return
    snippetList.value = []
  } else {
    const count = snippetList.value.filter(s => (s.category || '通用') === cat).length
    if (count === 0) return
    if (!confirm(`确定要清空「${cat}」分类下的 ${count} 个片段吗？`)) return
    snippetList.value = snippetList.value.filter(s => (s.category || '通用') !== cat)
  }
  selectedSnippetIndex.value = -1
}
function importSnippetsClick() {
  importFileInput.value?.click()
}
async function importSnippetsFile(e) {
  const file = e.target.files?.[0]
  if (!file) return
  const text = await file.text()
  if (editorRef.value?.importUserSnippets) {
    const ok = editorRef.value.importUserSnippets(text)
    if (ok) {
      setStatusMsg('代码片段导入成功', 2000)
      snippetList.value = editorRef.value.getUserSnippets()
    } else {
      setStatusMsg('导入失败：格式不正确', 3000)
    }
  }
  e.target.value = ''
}
const leftPanelKey = ref('project')

// G7：插件自定义面板渲染支持
const pluginPanelContainerRef = ref(null)
// leftPanelKey 形如 "plugin:<id>" 时表示当前激活的是插件面板
const isPluginPanelActive = computed(() => leftPanelKey.value.startsWith('plugin:'))
const activePluginPanelId = computed(() => {
  if (!isPluginPanelActive.value) return ''
  return leftPanelKey.value.slice('plugin:'.length)
})
const activePluginPanel = computed(() => {
  const id = activePluginPanelId.value
  if (!id) return null
  return pluginPanels.value.find(p => p.id === id) || null
})
// 切换到插件面板时，等容器挂载后调用插件的 render(el)
watch(activePluginPanel, (panel) => {
  if (!panel) return
  nextTick(() => {
    const el = pluginPanelContainerRef.value
    if (!el) return
    // 清空容器，避免重复渲染
    el.innerHTML = ''
    try {
      panel.render(el)
    } catch (e) {
      el.textContent = '[plugin] 面板渲染失败: ' + (e && e.message ? e.message : String(e))
      console.error('[plugin] 面板渲染失败:', e)
    }
  })
})

// 项目关闭时清理：项目级支持库重新放回只含内置库的状态
watch(projectOpen, (open) => {
  if (!open) {
    try { unloadProjectLibs() } catch (e) { console.warn('[lib] unload 失败:', e) }
  }
})

const createProjectVisible = ref(false)
const createProjectParent = ref('')
const createProjectName = ref('未命名项目')
const createProjectTemplate = ref('builtin:window')
// 全局模板列表（exe 同级 templates/），启动时加载，合并到 createProjectTemplateOptions
const globalTemplates = ref([])
const newWindowVisible = ref(false)
const newWindowName = ref('窗口1')
// 构建配置面板
const buildConfigVisible = ref(false)
const buildConfig = ref({
  mode: 'release', // 'debug' | 'release'
  autoOpenFolder: true,
  garbleLevel: 'basic', // v0.8.0 修订：Garble 混淆强度三档（off/basic/full），默认 basic
  // 项目信息（写回 project.eg.json）
  projectName: '',
  version: '',
  description: '',
  author: '',
  // 图标设置
  iconPath: '',
  // 版权信息（嵌入 .syso 资源）
  companyName: '',
  fileDescription: '',
  legalCopyright: '',
  productName: '',
  comments: ''
})
const buildModeOptions = [
  { label: 'Release（发布，-s -w 去除调试信息，体积更小）', value: 'release' },
  { label: 'Debug（调试，保留符号表）', value: 'debug' }
]

// 打开编译选项对话框时，从后端读取当前项目配置回填
async function openBuildOptions() {
  buildConfigVisible.value = true
  if (!projectPath.value) return
  try {
    const cfg = await IDEService.ReadProjectConfig(projectPath.value)
    if (cfg) {
      buildConfig.value.projectName = cfg.name || ''
      buildConfig.value.version = cfg.version || ''
      buildConfig.value.description = cfg.description || ''
      buildConfig.value.author = cfg.author || ''
      buildConfig.value.iconPath = cfg.iconPath || ''
      buildConfig.value.companyName = cfg.companyName || ''
      buildConfig.value.fileDescription = cfg.fileDescription || ''
      buildConfig.value.legalCopyright = cfg.legalCopyright || ''
      buildConfig.value.productName = cfg.productName || ''
      buildConfig.value.comments = cfg.comments || ''
    }
  } catch (e) { /* 读取失败忽略，使用空值 */ }
}

// 浏览选择图标文件
async function browseIconPath() {
  try {
    const path = await IDEService.PickFilePath('选择图标文件', '图标文件|*.ico|所有文件|*.*')
    if (path) buildConfig.value.iconPath = path
  } catch (e) { /* 取消选择 */ }
}

// 保存编译选项（写回 project.eg.json）
async function saveBuildOptions() {
  if (!projectPath.value) {
    buildConfigVisible.value = false
    return
  }
  const cfg = {
    name: buildConfig.value.projectName,
    version: buildConfig.value.version,
    description: buildConfig.value.description,
    author: buildConfig.value.author,
    output: 'bin',
    sdk: '',
    dependencies: [],
    iconPath: buildConfig.value.iconPath,
    companyName: buildConfig.value.companyName,
    fileDescription: buildConfig.value.fileDescription,
    legalCopyright: buildConfig.value.legalCopyright,
    productName: buildConfig.value.productName,
    comments: buildConfig.value.comments
  }
  try {
    const err = await IDEService.SaveProjectConfig(projectPath.value, cfg)
    if (err) {
      setStatusMsg('保存配置失败: ' + err, 4000)
    } else {
      setStatusMsg('编译选项已保存', 2000)
    }
  } catch (e) {
    setStatusMsg('保存配置失败: ' + e.message, 4000)
  }
  buildConfigVisible.value = false
}

async function callIDEService(method, ...args) {
  if (window.IDEService && typeof window.IDEService[method] === 'function') {
    return await window.IDEService[method](...args)
  }
  throw new Error('IDEService.' + method + ' 不存在')
}
// 书签列表面板
const bookmarkList = ref([])
const bookmarkListRef = ref(null)
// 构建历史记录面板（按项目路径分键持久化到 localStorage）
// 跨文件搜索（项目内 .eg）
const searchVisible = ref(false)
const searchQuery = ref('')
const searchUseRegex = ref(false)
const searching = ref(false)

const buildHistory = ref([])
const MAX_BUILD_HISTORY = 20
const createProjectTemplateOptions = computed(() => {
  // 内置模板
  const builtin = [
    { label: '控制台程序', value: 'builtin:console' },
    { label: '窗口程序', value: 'builtin:window' },
    { label: '空白项目', value: 'builtin:empty' }
  ]
  // 全局模板（exe 同级 templates/，value 前缀 'global:' 区分）
  const global = globalTemplates.value.map(t => ({
    label: t.icon ? t.icon + ' ' + t.name : t.name,
    value: 'global:' + t.dir,
    description: t.description
  }))
  return [...builtin, ...global]
})

function templateIcon(value) {
  if (value === 'builtin:console') return '🖥️'
  if (value === 'builtin:window') return '🪟'
  if (value === 'builtin:empty') return '📄'
  if (value && value.startsWith('global:')) return '📦'
  return '📄'
}

const newTabOptions = [
  { label: '代码文件', key: 'code' },
  { label: '窗口', key: 'window' },
  { label: '类', key: 'class' },
  { label: '模块', key: 'module' }
]

const tabContextMenuShow = ref(false)
const tabContextMenuX = ref(0)
const tabContextMenuY = ref(0)
const tabContextMenuIdx = ref(-1)
// 标签页右键菜单 — 多级分组，减少视觉占用
const tabContextMenuOptions = computed(() => {
  const idx = tabContextMenuIdx.value
  const file = idx >= 0 ? files.value[idx] : null
  const isPinned = file?.pinned
  const pinLabel = isPinned ? '取消固定' : '固定标签页'
  return [
    {
      label: '关闭',
      key: 'close-group',
      children: [
        { label: '关闭当前', key: 'close', disabled: isPinned },
        { label: '关闭已保存', key: 'close-saved' },
        { label: '关闭其他', key: 'close-others' },
        { label: '关闭左侧', key: 'close-left' },
        { label: '关闭右侧', key: 'close-right' },
        { label: '关闭全部', key: 'close-all' }
      ]
    },
    {
      label: '操作',
      key: 'action-group',
      children: [
        { label: pinLabel, key: 'toggle-pin' },
        { label: '复制路径', key: 'copy-path' },
        { label: '在文件管理器中显示', key: 'reveal' }
      ]
    }
  ]
})

function onTabContextMenu(e, idx) {
  e.preventDefault()
  tabContextMenuIdx.value = idx
  tabContextMenuX.value = e.clientX
  tabContextMenuY.value = e.clientY
  tabContextMenuShow.value = true
}

function onTabContextMenuSelect(key) {
  const idx = tabContextMenuIdx.value
  tabContextMenuShow.value = false
  if (idx < 0) return
  const file = files.value[idx]
  if (key === 'close') {
    if (!file.pinned) closeFile(idx)
  } else if (key === 'close-others') {
    closeOtherFiles(idx)
  } else if (key === 'close-left') {
    closeLeftFiles(idx)
  } else if (key === 'close-right') {
    closeRightFiles(idx)
  } else if (key === 'close-all') {
    closeAllFiles()
  } else if (key === 'close-saved') {
    closeSavedFiles()
  } else if (key === 'toggle-pin') {
    togglePinTab(idx)
  } else if (key === 'copy-path') {
    if (file?.path) {
      navigator.clipboard?.writeText(file.path).catch(() => {})
      setStatusMsg('已复制路径', 2000)
    }
  } else if (key === 'reveal') {
    if (file?.path && window.IDEService?.OpenInExplorer) {
      const dir = file.path.substring(0, file.path.lastIndexOf('\\'))
      window.IDEService.OpenInExplorer(dir)
    }
  }
}

function togglePinTab(idx) {
  const file = files.value[idx]
  if (!file) return
  file.pinned = !file.pinned
  if (file.pinned) {
    const pinned = files.value.filter((f, i) => f.pinned)
    const unpinned = files.value.filter(f => !f.pinned)
    const wasActive = activeFileIndex.value === idx
    files.value = [...pinned, ...unpinned]
    if (wasActive) {
      activeFileIndex.value = pinned.findIndex(f => f === file)
    } else {
      activeFileIndex.value = files.value.indexOf(files.value[activeFileIndex.value])
    }
    setStatusMsg('已固定标签页', 1500)
  } else {
    setStatusMsg('已取消固定', 1500)
  }
}

function closeRightFiles(fromIdx) {
  if (fromIdx >= files.value.length - 1) return
  const toClose = []
  for (let i = fromIdx + 1; i < files.value.length; i++) {
    if (!files.value[i].pinned && isFileDirty(files.value[i])) {
      toClose.push(i)
    }
  }
  if (toClose.length > 0) {
    showConfirm('未保存的更改', `${toClose.length} 个文件有未保存的更改，确定要关闭吗？`, () => {
      doCloseRight(fromIdx)
    })
    return
  }
  doCloseRight(fromIdx)
}

function doCloseRight(fromIdx) {
  const keep = []
  for (let i = 0; i <= fromIdx; i++) keep.push(files.value[i])
  for (let i = fromIdx + 1; i < files.value.length; i++) {
    if (files.value[i].pinned) {
      keep.push(files.value[i])
    } else {
      pushClosedFile(files.value[i])
    }
  }
  files.value = keep
  if (activeFileIndex.value >= files.value.length) {
    activeFileIndex.value = files.value.length - 1
  }
  selectedFunctionIndex.value = null
}

function closeLeftFiles(fromIdx) {
  if (fromIdx <= 0) return
  const toClose = []
  for (let i = 0; i < fromIdx; i++) {
    if (!files.value[i].pinned && isFileDirty(files.value[i])) {
      toClose.push(i)
    }
  }
  if (toClose.length > 0) {
    showConfirm('未保存的更改', `${toClose.length} 个文件有未保存的更改，确定要关闭吗？`, () => {
      doCloseLeft(fromIdx)
    })
    return
  }
  doCloseLeft(fromIdx)
}

function doCloseLeft(fromIdx) {
  const keep = []
  for (let i = 0; i < fromIdx; i++) {
    if (files.value[i].pinned) {
      keep.push(files.value[i])
    } else {
      pushClosedFile(files.value[i])
    }
  }
  for (let i = fromIdx; i < files.value.length; i++) keep.push(files.value[i])
  files.value = keep
  const activeFile = files.value[activeFileIndex.value]
  activeFileIndex.value = keep.indexOf(activeFile)
  if (activeFileIndex.value < 0) activeFileIndex.value = keep.length - 1
  selectedFunctionIndex.value = null
}

function onTabDblClick(idx) {
  if (!files.value[idx]?.pinned) closeFile(idx)
}

function onNewTabSelect(key) {
  if (key === 'code') {
    newCodeFile()
  } else if (key === 'window') {
    showNewWindowDialog()
  } else if (key === 'class') {
    newClassFile()
  } else if (key === 'module') {
    newModuleFile()
  }
}

const projectName = computed(() => {
  if (!projectPath.value) return ''
  const parts = projectPath.value.split(/[\\/]/)
  return parts[parts.length - 1] || ''
})
const isFormFile = computed(() => isFormFileName(activeFile.value.name))
const outputCollapsed = ref(false)
const rightPanelVisible = ref(true)

// 面包屑显示的相对路径
const breadcrumbPath = computed(() => {
  const f = activeFile.value
  if (!f) return ''
  if (!f.path || !projectPath.value) return f.name
  const p = f.path.replace(/\\/g, '/')
  const base = projectPath.value.replace(/\\/g, '/').replace(/\/$/, '')
  if (p.startsWith(base + '/')) {
    const rel = p.slice(base.length + 1)
    return rel
  }
  return f.name
})

const leftPanelWidth = ref(parseInt(localStorage.getItem('eg-left-width') || '240', 10))
const rightPanelWidth = ref(parseInt(localStorage.getItem('eg-right-width') || '280', 10))
const outputPanelHeight = ref(parseInt(localStorage.getItem('eg-output-height') || '160', 10))

const MIN_LEFT = 180
const MAX_LEFT = 400
const MIN_RIGHT = 200
const MAX_RIGHT = 500
const MIN_OUTPUT = 80
const MAX_OUTPUT = 400

watch(leftPanelWidth, (v) => localStorage.setItem('eg-left-width', String(Math.max(MIN_LEFT, Math.min(MAX_LEFT, v)))))
watch(rightPanelWidth, (v) => localStorage.setItem('eg-right-width', String(Math.max(MIN_RIGHT, Math.min(MAX_RIGHT, v)))))
watch(outputPanelHeight, (v) => localStorage.setItem('eg-output-height', String(Math.max(MIN_OUTPUT, Math.min(MAX_OUTPUT, v)))))

let dragTarget = null
let dragStartX = 0
let dragStartY = 0
let dragStartLeft = 0
let dragStartRight = 0
let dragStartOutput = 0

function onSplitterMouseDown(e, target) {
  e.preventDefault()
  dragTarget = target
  dragStartX = e.clientX
  dragStartY = e.clientY
  dragStartLeft = leftPanelWidth.value
  dragStartRight = rightPanelWidth.value
  dragStartOutput = outputPanelHeight.value
  document.body.style.cursor = target === 'output' ? 'ns-resize' : 'ew-resize'
  document.body.style.userSelect = 'none'
  window.addEventListener('mousemove', onSplitterMouseMove)
  window.addEventListener('mouseup', onSplitterMouseUp)
}

function onSplitterMouseMove(e) {
  if (!dragTarget) return
  if (dragTarget === 'left') {
    const delta = e.clientX - dragStartX
    leftPanelWidth.value = Math.max(MIN_LEFT, Math.min(MAX_LEFT, dragStartLeft + delta))
  } else if (dragTarget === 'right') {
    const delta = dragStartX - e.clientX
    rightPanelWidth.value = Math.max(MIN_RIGHT, Math.min(MAX_RIGHT, dragStartRight + delta))
  } else if (dragTarget === 'output') {
    const delta = dragStartY - e.clientY
    outputPanelHeight.value = Math.max(MIN_OUTPUT, Math.min(MAX_OUTPUT, dragStartOutput + delta))
  }
}

function onSplitterMouseUp() {
  dragTarget = null
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
  window.removeEventListener('mousemove', onSplitterMouseMove)
  window.removeEventListener('mouseup', onSplitterMouseUp)
}

const recentProjects = ref([])
const RECENT_KEY = 'egou_recent_projects'

function loadRecentProjects() {
  try {
    const raw = localStorage.getItem(RECENT_KEY)
    recentProjects.value = raw ? JSON.parse(raw) : []
  } catch {
    recentProjects.value = []
  }
}

function saveRecentProjects() {
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(recentProjects.value.slice(0, 10)))
  } catch {}
}

function addRecentProject(path) {
  if (!path) return
  const name = path.split(/[\\/]/).pop() || path
  const list = recentProjects.value.filter(item => item.path !== path)
  list.unshift({ name, path })
  recentProjects.value = list.slice(0, 10)
  saveRecentProjects()
}

function isInputFocused() {
  const el = document.activeElement
  if (!el) return false
  const tag = el.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  if (el.isContentEditable) return true
  if (el.closest('.monaco-editor')) return true
  if (el.closest('.n-input')) return true
  return false
}

function onKeyDown(e) {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault()
    quickSave()
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 'p') {
    e.preventDefault()
    if (projectOpen.value) showQuickOpen()
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'P') {
    e.preventDefault()
    if (projectOpen.value) showCommandPalette()
    resetChordKey()
    return
  }
  // Ctrl+` 切换输出面板（VS Code 标准快捷键，用 e.code 兼容不同键盘布局）
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && (e.key === '`' || e.code === 'Backquote')) {
    e.preventDefault()
    outputCollapsed.value = !outputCollapsed.value
    if (!outputCollapsed.value) {
      nextTick(() => {
        const el = document.querySelector('.output-panel .n-tabs-tab--active')
        el?.scrollIntoView({ block: 'nearest' })
      })
    }
    resetChordKey()
    return
  }
  // Ctrl+K 和弦快捷键
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault()
    pendingChord = 'K'
    if (chordKeyTimer) clearTimeout(chordKeyTimer)
    chordKeyTimer = setTimeout(resetChordKey, 1500)
    setStatusMsg('(Ctrl+K) 等待第二个快捷键...', 1500)
    return
  }
  if (pendingChord === 'K' && (e.ctrlKey || e.metaKey) && !e.shiftKey && e.key === 's') {
    e.preventDefault()
    resetChordKey()
    showKeybindings()
    return
  }
  if (pendingChord === 'K' && (e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'x' || e.key === 'X')) {
    e.preventDefault()
    resetChordKey()
    runEditorAction('editor.action.trimTrailingWhitespace')
    setStatusMsg('已裁剪尾随空格', 2000)
    return
  }
  if (pendingChord === 'K' && (e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'c' || e.key === 'C')) {
    e.preventDefault()
    resetChordKey()
    runEditorAction('editor.action.addCommentLine')
    return
  }
  if (pendingChord === 'K' && (e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'u' || e.key === 'U')) {
    e.preventDefault()
    resetChordKey()
    runEditorAction('editor.action.removeCommentLine')
    return
  }
  if (pendingChord === 'K' && (e.ctrlKey || e.metaKey) && !e.shiftKey && e.key === '0') {
    e.preventDefault()
    resetChordKey()
    runEditorAction('editor.foldAll')
    setStatusMsg('已折叠所有区域', 2000)
    return
  }
  if (pendingChord === 'K' && (e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'j' || e.key === 'J')) {
    e.preventDefault()
    resetChordKey()
    runEditorAction('editor.unfoldAll')
    setStatusMsg('已展开所有区域', 2000)
    return
  }
  if (pendingChord === 'K' && !e.ctrlKey && !e.metaKey && !e.shiftKey && (e.key === 'z' || e.key === 'Z')) {
    e.preventDefault()
    resetChordKey()
    toggleZenMode()
    return
  }
  // 非和弦组合键，重置和弦状态
  if (e.key !== 'Control' && e.key !== 'Meta' && e.key !== 'Shift' && e.key !== 'Alt') {
    resetChordKey()
  }
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'o' || e.key === 'O')) {
    e.preventDefault()
    if (activeFileIndex.value >= 0) showGotoSymbol()
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 'g') {
    e.preventDefault()
    if (activeFileIndex.value >= 0) showGotoLine()
    return
  }
  if (e.key === 'F11') {
    e.preventDefault()
    // 调试中：F11 = 单步进入；否则 F11 = 全屏切换
    if (isDebugging.value) {
      IDEService.DebugStep().catch(() => {})
    } else {
      IDEService.ToggleFullscreen()
    }
    return
  }
  if (e.key === 'F10') {
    // 调试中：F10 = 单步跳过
    if (isDebugging.value) {
      e.preventDefault()
      IDEService.DebugNext().catch(() => {})
      return
    }
  }
  if (e.shiftKey && e.key === 'F11') {
    // 调试中：Shift+F11 = 单步跳出
    if (isDebugging.value) {
      e.preventDefault()
      IDEService.DebugStepOut().catch(() => {})
      return
    }
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 'n') {
    e.preventDefault()
    if (projectOpen.value) newCodeFile()
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 'o') {
    e.preventDefault()
    if (projectOpen.value) openFile()
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'S') {
    e.preventDefault()
    if (activeFileIndex.value >= 0) saveFile(true)
    return
  }
  if (e.key === 'F5') {
    e.preventDefault()
    // 调试中：F5 = 继续执行；否则 F5 = 编译运行
    if (isDebugging.value) {
      IDEService.DebugContinue().catch(() => {})
    } else if (projectOpen.value) {
      runCode()
    }
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 'w') {
    e.preventDefault()
    if (activeFileIndex.value >= 0 && files.value.length > 0) closeFile(activeFileIndex.value)
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 't' || e.key === 'T')) {
    e.preventDefault()
    reopenClosedFile()
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 'Tab') {
    e.preventDefault()
    if (files.value.length > 1) {
      if (e.shiftKey) {
        const prev = activeFileIndex.value - 1
        switchFile(prev < 0 ? files.value.length - 1 : prev)
      } else {
        const next = activeFileIndex.value + 1
        switchFile(next >= files.value.length ? 0 : next)
      }
    }
    return
  }
  // Alt+Left/Right 导航前进后退
  if (e.altKey && !e.ctrlKey && !e.metaKey && !e.shiftKey) {
    if (e.key === 'ArrowLeft') {
      e.preventDefault()
      navigateBack()
      return
    }
    if (e.key === 'ArrowRight') {
      e.preventDefault()
      navigateForward()
      return
    }
  }
  if (isInputFocused()) return
  // Alt+Z 切换自动换行
  if (e.altKey && !e.ctrlKey && !e.metaKey && !e.shiftKey && (e.key === 'z' || e.key === 'Z')) {
    e.preventDefault()
    toggleWordWrap()
    return
  }
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && e.key === 'z') {
    e.preventDefault()
    undo()
    return
  }
  if ((e.ctrlKey || e.metaKey) && (e.key === 'y' || (e.shiftKey && e.key === 'Z'))) {
    e.preventDefault()
    redo()
    return
  }
}

function onTabMouseDown(e, idx) {
  if (e.button === 1) {
    e.preventDefault()
    e.stopPropagation()
    if (!files.value[idx]?.pinned) closeFile(idx)
  }
}

function onTabsWheel(e) {
  const el = tabsBarRef.value
  if (!el) return
  if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
    e.preventDefault()
    el.scrollLeft += e.deltaY
  }
}

function showGotoLine() {
  gotoLineInput.value = ''
  gotoLineVisible.value = true
  nextTick(() => {
    const el = gotoLineInputRef.value
    if (el && el.focus) el.focus()
    else {
      const inputEl = document.querySelector('.goto-line-input .n-input__input-el')
      if (inputEl) inputEl.focus()
    }
  })
}

function onGotoLineConfirm() {
  const line = parseInt(gotoLineInput.value, 10)
  if (!isNaN(line) && line > 0) {
    gotoLineVisible.value = false
    gotoLine(line)
  }
}

function onGotoLineKeydown(e) {
  if (e.key === 'Enter') {
    e.preventDefault()
    onGotoLineConfirm()
  } else if (e.key === 'Escape') {
    gotoLineVisible.value = false
  }
}

// ===== 转到符号 (Ctrl+Shift+O) =====
function showGotoSymbol() {
  gotoSymbolQuery.value = ''
  gotoSymbolSelected.value = 0
  gotoSymbolVisible.value = true
  nextTick(() => {
    const el = document.querySelector('.goto-symbol-input .n-input__input-el') || document.querySelector('.quick-open .n-input__input-el')
    if (el) el.focus()
  })
}

const gotoSymbolItems = computed(() => {
  const items = []
  const p = parsed.value
  if (!p) return items
  for (const fn of (p.functions || [])) {
    items.push({ kind: 'function', name: fn.name, line: fn.startLine, type: fn.returnType || '' })
  }
  for (const v of (p.globalVars || [])) {
    items.push({ kind: 'variable', name: v.name, line: v.line, type: v.type || '' })
  }
  for (const c of (p.constants || [])) {
    items.push({ kind: 'constant', name: c.name, line: c.line, type: c.type || '' })
  }
  const q = gotoSymbolQuery.value.trim().toLowerCase()
  if (!q) return items
  return items.filter(s => s.name.toLowerCase().includes(q))
})

function onGotoSymbolKeydown(e) {
  const items = gotoSymbolItems.value
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    gotoSymbolSelected.value = Math.min(gotoSymbolSelected.value + 1, items.length - 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    gotoSymbolSelected.value = Math.max(gotoSymbolSelected.value - 1, 0)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const s = items[gotoSymbolSelected.value]
    if (s) {
      gotoSymbolVisible.value = false
      gotoLine(s.line + 1)
    }
  } else if (e.key === 'Escape') {
    gotoSymbolVisible.value = false
  }
}

watch(gotoSymbolVisible, (val) => {
  if (val) {
    nextTick(() => {
      const el = document.querySelector('.quick-open .n-input__input-el')
      if (el) el.focus()
    })
  }
})

// ===== 禅模式 =====
let zenModeSavedState = null
function toggleZenMode() {
  if (!zenMode.value) {
    zenModeSavedState = {
      outputCollapsed: outputCollapsed.value,
    }
    outputCollapsed.value = true
    zenMode.value = true
    setStatusMsg('禅模式已开启 (Ctrl+K Z 退出)', 2500)
  } else {
    zenMode.value = false
    if (zenModeSavedState) {
      outputCollapsed.value = zenModeSavedState.outputCollapsed
    }
    setStatusMsg('禅模式已退出', 2000)
  }
}

// ===== 后端健康检查 =====
// 启动时探活一次，之后每 60s 刷新；状态栏右侧展示整体健康度，hover 显示详情。
const healthReport = ref(null)
let healthTimer = null
async function refreshHealth() {
  try {
    healthReport.value = await IDEService.HealthCheck()
  } catch (e) {
    // 探活失败（例如后端未就绪）也写入 report，标记为异常
    healthReport.value = { ok: false, message: '后端探活失败：' + String(e) }
  }
}
const healthDetailRows = computed(() => {
  const r = healthReport.value
  if (!r) return []
  const rows = []
  rows.push({ label: 'Go 编译器', value: r.goCompiler || '未找到', ok: !!r.goCompiler })
  rows.push({ label: 'Go 版本', value: r.goVersion || '—', ok: !!r.goVersion })
  rows.push({ label: '运行时模板', value: r.templateOk ? '存在' : '缺失', ok: r.templateOk })
  rows.push({ label: '前端缓存', value: r.cacheReady ? '就绪' : '未就绪', ok: r.cacheReady })
  rows.push({ label: 'npm', value: r.npm || '未找到', ok: !!r.npm })
  rows.push({ label: 'Wails3 CLI', value: r.wails3Cli || '未配置', ok: !!r.wails3Cli })
  rows.push({ label: 'Delve 调试器', value: r.delve || '未安装（go install github.com/go-delve/delve/cmd/dlv@latest）', ok: !!r.delve })
  rows.push({ label: '系统', value: `${r.os}/${r.arch}`, ok: true })
  return rows
})

onMounted(() => {
  loadRecentProjects()
  // G1：IDE 启动时扫描 exe 同级 libs/ 全局扩展包，所有项目共享
  loadGlobalLibs().catch(e => console.warn('[lib] 全局库加载失败:', e))
  // G10：IDE 启动时扫描 exe 同级 templates/ 全局项目模板，合并到新建项目对话框
  loadGlobalTemplates().catch(e => console.warn('[template] 全局模板加载失败:', e))
  // 同步"编译选项"到后端（Garble 混淆强度等），让 buildRuntime 根据强度决定混淆方式
  if (window.IDEService && window.IDEService.SetBuildOptions) {
    try { window.IDEService.SetBuildOptions(buildGarbleLevel.value) } catch (e) {
      console.warn('[build] SetBuildOptions 初始同步失败:', e)
    }
  }
  // watch buildGarbleLevel 变化时实时同步到后端
  watch(buildGarbleLevel, (v) => {
    if (window.IDEService && window.IDEService.SetBuildOptions) {
      try { window.IDEService.SetBuildOptions(v) } catch (e) {
        console.warn('[build] SetBuildOptions 同步失败:', e)
      }
    }
  })
  // v0.9.2：同步 Go/dlv 路径到后端（用户可指定自安装的 SDK 路径，用于版本不匹配场景）
  if (window.IDEService && window.IDEService.SetGoPath) {
    try { window.IDEService.SetGoPath(buildGoPath.value) } catch (e) {
      console.warn('[build] SetGoPath 初始同步失败:', e)
    }
  }
  if (window.IDEService && window.IDEService.SetDelvePath) {
    try { window.IDEService.SetDelvePath(buildDelvePath.value) } catch (e) {
      console.warn('[build] SetDelvePath 初始同步失败:', e)
    }
  }
  watch(buildGoPath, (v) => {
    if (window.IDEService && window.IDEService.SetGoPath) {
      try { window.IDEService.SetGoPath(v) } catch (e) {
        console.warn('[build] SetGoPath 同步失败:', e)
      }
    }
  })
  watch(buildDelvePath, (v) => {
    if (window.IDEService && window.IDEService.SetDelvePath) {
      try { window.IDEService.SetDelvePath(v) } catch (e) {
        console.warn('[build] SetDelvePath 同步失败:', e)
      }
    }
  })
  // G5/G6/G8：IDE 启动时扫描 exe 同级 plugins/ 加载插件
  loadAllPlugins({
    output: (text) => { appendOutput(text) },
    getStatus: () => statusMessage.value,
    setStatus: (msg) => { statusMessage.value = msg },
    getActiveFile: () => activeFile.value ? { name: activeFile.value.name, path: activeFile.value.path, source: activeFile.value.source } : null,
    openFile: (path) => { openProjectFile(path) },
    getProjectPath: () => projectPath.value,
    callBackend: async (method, ...args) => {
      if (window.IDEService && typeof window.IDEService[method] === 'function') {
        return await window.IDEService[method](...args)
      }
      throw new Error('IDEService.' + method + ' 不存在')
    }
  }).then(result => {
    if (result.ok && result.count > 0) {
      output.value += `[plugin] 已加载 ${result.count} 个插件\n`
    }
  }).catch(e => console.warn('[plugin] 插件加载失败:', e))
  // P3：加载外置组件包（exe 同级 components/ 目录），注册到 WindowDesigner 工具箱
  loadExternalComponents()
  window.addEventListener('keydown', onKeyDown)
  refreshHealth()
  healthTimer = setInterval(refreshHealth, 60000)
  // 命令面板选中项重置 watch（延迟到 mounted 注册，避免 setup 阶段 TDZ）
  watch(filteredCommands, () => {
    commandPaletteSelected.value = 0
  })
  // 转到符号列表选中项重置（延迟注册，避免 gotoSymbolItems computed 中访问 parsed 触发 TDZ）
  watch(gotoSymbolItems, () => {
    gotoSymbolSelected.value = 0
  })
  // P2 调试器：监听 debug:exit 清除编辑器的当前执行行高亮
  offDebugExit = Events.On('debug:exit', () => {
    isDebugging.value = false
    editorRef.value?.clearDebugState?.()
  })
  // P2 调试器：监听 debug:halt 同步调试状态
  offDebugHalt = Events.On('debug:halt', () => {
    isDebugging.value = true
  })
  // v0.9.5：监听 debug:error 同步到全局状态栏（之前仅 DebugPanel 内部显示，用户切 tab 后看不到）
  offDebugError = Events.On('debug:error', (ev) => {
    const errMsg = ev?.data?.error || '调试器发生未知错误'
    setStatusMsg('调试错误: ' + errMsg, 5000)
  })
  // v0.11.7：全局监听编译进度事件，驱动 BuildProgress 进度条（覆盖 run/build/debug 三种场景）
  offBuildProgress = Events.On('ide:run-event', (ev) => {
    const data = ev?.data || {}
    const stage = data.stage
    const text = data.output || ''
    if (stage === 'progress') {
      const colon = text.indexOf(':')
      if (colon > 0) {
        const step = text.substring(0, colon)
        const pct = parseInt(text.substring(colon + 1), 10)
        if (!isNaN(pct)) {
          buildActive.value = true
          buildStep.value = step
          buildPercent.value = pct
        }
      }
    } else if (stage === 'error') {
      buildActive.value = false
      buildProgressRef.value?.reset()
    }
  })
})

// G10：加载 exe 同级 templates/ 全局项目模板
async function loadGlobalTemplates() {
  if (!window.IDEService || !window.IDEService.ScanGlobalTemplates) return
  try {
    const list = await window.IDEService.ScanGlobalTemplates()
    if (Array.isArray(list)) {
      globalTemplates.value = list
    }
  } catch (e) {
    console.warn('[template] ScanGlobalTemplates 失败:', e)
  }
}

onUnmounted(() => {
  window.removeEventListener('keydown', onKeyDown)
  if (healthTimer) { clearInterval(healthTimer); healthTimer = null }
  // F8：清理防抖/节流定时器，避免组件卸载后仍触发回调
  if (autoSaveTimer) { clearTimeout(autoSaveTimer); autoSaveTimer = null }
  if (parseDebounceTimer) { clearTimeout(parseDebounceTimer); parseDebounceTimer = null }
  for (const slot in scrollTimers) {
    if (scrollTimers[slot]) { clearTimeout(scrollTimers[slot]); scrollTimers[slot] = null }
  }
  // P2 调试器：清理事件订阅
  if (offDebugExit) { offDebugExit(); offDebugExit = null }
  if (offDebugHalt) { offDebugHalt(); offDebugHalt = null }
  if (offDebugError) { offDebugError(); offDebugError = null }
  if (offBuildProgress) { offBuildProgress(); offBuildProgress = null }
})

const DEFAULT_SOURCE = `# 程序集 main

导入 (
    "fmt"
)

函数 主函数()
    fmt.Println("你好，世界！")
结束函数
`

function isFormFileName(name) {
  return name.toLowerCase().endsWith('.ew')
}

function boundCodeFileName(name) {
  return name.replace(/\.ew$/i, '.eg')
}

function makeFile(name, source, path = '') {
  const isForm = isFormFileName(name)
  return {
    name,
    source,
    path,
    savedSource: source,
    view: isForm ? 'design' : 'code',
    design: isForm ? defaultFormDesign() : null,
    codeFileName: isForm ? boundCodeFileName(name) : null,
    pinned: false
  }
}

// 默认窗口设计数据：与 WindowDesigner 的 defaultDesign() 保持一致，
// 确保 form 字段完整，避免 formSurfaceStyle/formClientStyle 生成 'undefinedpx'
function defaultFormDesign() {
  return {
    form: {
      title: '窗口', width: 538, height: 350, bgColor: '#f0f0f0',
      icon: '',
      x: 0, y: 0,
      minWidth: 120, minHeight: 80, maxWidth: 0, maxHeight: 0,
      resizable: true, minimizable: true, maximizable: true, closable: true,
      fullScreen: false, alwaysOnTop: false,
      frameless: false, transparent: false, translucent: false,
      backdrop: 'auto', rounded: true, shadow: true,
      opacity: 100, centered: true
    },
    components: []
  }
}

const files = ref([makeFile('main.eg', DEFAULT_SOURCE)])
const activeFileIndex = ref(0)
const activeFile = computed(() => files.value[activeFileIndex.value] || files.value[0])

// 文件操作撤销/重做栈
const fileUndoStack = ref([])
const fileRedoStack = ref([])

function pushFileOp(op) {
  fileUndoStack.value.push(op)
  fileRedoStack.value = []
}

function cloneFile(f) {
  return {
    name: f.name,
    source: f.source,
    path: f.path,
    savedSource: f.savedSource,
    view: f.view,
    design: f.design ? JSON.parse(JSON.stringify(f.design)) : null,
    codeFileName: f.codeFileName,
    pinned: f.pinned || false
  }
}

const output = ref('')
const errorOutput = ref('')
const tipOutput = ref('')
const refsResults = ref([])
const refsQuery = ref('')
// 查找引用：按文件分组展示，支持折叠/展开
// collapsedRefFiles 存被折叠的 filePath，默认全部展开
const collapsedRefFiles = ref(new Set())
const groupedRefs = computed(() => {
  const groups = []
  const indexByPath = new Map()
  for (const r of refsResults.value) {
    let g = indexByPath.get(r.filePath)
    if (!g) {
      g = { filePath: r.filePath, file: r.file, items: [] }
      indexByPath.set(r.filePath, g)
      groups.push(g)
    }
    g.items.push({ line: r.line, col: r.col, preview: r.preview })
  }
  // 当前文件排最前（与 refsResults 中出现顺序一致，已是先当前后其它）
  return groups
})
function toggleRefFile(filePath) {
  const s = new Set(collapsedRefFiles.value)
  if (s.has(filePath)) s.delete(filePath)
  else s.add(filePath)
  collapsedRefFiles.value = s
}
function isRefFileCollapsed(filePath) {
  return collapsedRefFiles.value.has(filePath)
}
function expandAllRefFiles() {
  collapsedRefFiles.value = new Set()
}
function collapseAllRefFiles() {
  collapsedRefFiles.value = new Set(groupedRefs.value.map(g => g.filePath))
}

// 解析错误输出为结构化条目，支持点击跳转
const errorEntries = computed(() => {
  if (!errorOutput.value) return []
  return errorOutput.value.split('\n').map(line => {
    const trimmed = line.trim()
    if (!trimmed) return { text: line, clickable: false, file: null, line: null }
    // .eg:N:col: 格式（Go 编译错误，//line 指令生效后）
    const eg = trimmed.match(/(\S+\.eg):(\d+):/)
    if (eg) return { text: line, clickable: true, file: eg[1], line: parseInt(eg[2], 10) }
    // 第 N 行 格式（转译错误）
    const m = trimmed.match(/第\s*(\d+)\s*行/)
    if (m) return { text: line, clickable: true, file: null, line: parseInt(m[1], 10) }
    return { text: line, clickable: false, file: null, line: null }
  })
})
const outputTabName = ref('output')
// P2-17：错误数量徽标 — 计算 errorEntries 中的非空行数
const errorCount = computed(() => errorEntries.value.filter(e => e.text && e.text.trim()).length)
// P2-17：动态标签文本（带数量徽标）
const outputTabLabel = computed(() => errorCount.value > 0 ? t('output.errorsWithCount', { count: errorCount.value }) : t('output.errors'))
// P2-17：输出条数上限 2000 条防内存膨胀
const OUTPUT_MAX_LINES = 2000
function appendOutput(text) {
  const lines = (output.value + text + '\n').split('\n')
  if (lines.length > OUTPUT_MAX_LINES) {
    output.value = lines.slice(lines.length - OUTPUT_MAX_LINES).join('\n')
  } else {
    output.value += text + '\n'
  }
}
const selectedFunctionIndex = ref(null)
const statusMessage = ref('')
let statusMsgTimer = null
function setStatusMsg(msg, duration = 3000) {
  statusMessage.value = msg
  if (statusMsgTimer) clearTimeout(statusMsgTimer)
  if (msg && duration > 0) {
    statusMsgTimer = setTimeout(() => {
      statusMessage.value = ''
      statusMsgTimer = null
    }, duration)
  }
}
const cursorText = ref('')
const editorRef = ref(null)
const designerRef = ref(null)
const debugPanelRef = ref(null)
const isDebugging = ref(false)
let offDebugExit = null
let offDebugHalt = null
let offDebugError = null
let offBuildProgress = null
// G9：插件组件注册后同步到 WindowDesigner（通过 defineExpose 的方法）
watch(pluginComponents, (list) => {
  const d = designerRef.value
  if (!d) return
  // 先清空再重新注册，避免重复
  if (typeof d.clearPluginComponents === 'function') d.clearPluginComponents()
  for (const def of list) {
    if (typeof d.registerPluginComponent === 'function') d.registerPluginComponent(def)
  }
}, { deep: true })

// P3：外置组件包列表（从 components/ 目录加载）
const externalComponents = ref([])
// 加载外置组件包：调用后端 ScanComponents 获取组件包列表，转换为设计器可用的组件定义
async function loadExternalComponents() {
  if (!window.IDEService || !window.IDEService.ScanComponents) return
  try {
    const packages = await window.IDEService.ScanComponents()
    if (!Array.isArray(packages) || packages.length === 0) return
    const defs = []
    for (const pkg of packages) {
      if (!pkg.components || !pkg.components.length) continue
      for (const comp of pkg.components) {
        // G9 完善：预加载 icon SVG（comp.icon 指向相对组件目录的文件名）
        let iconData = null
        if (comp.icon && window.IDEService.ReadComponentFile) {
          try {
            const svg = await window.IDEService.ReadComponentFile(pkg.dir, 'components/' + comp.type + '/' + comp.icon)
            if (svg) iconData = svg
          } catch {}
        }
        defs.push({
          type: comp.type,
          label: comp.label || comp.type,
          icon: iconData, // G9：传 SVG 字符串，toolbox computed 会处理
          iconIsSvg: !!iconData,
          width: comp.width || 80,
          height: comp.height || 24,
          text: comp.text || '',
          props: comp.props || [],
          events: comp.events || [],
          preview: comp.preview || null, // G9：预览 HTML 模板
          packageDir: pkg.dir,
          isExternal: true
        })
      }
    }
    externalComponents.value = defs
    // 同步到 WindowDesigner
    const d = designerRef.value
    if (!d) return
    // 注意：不清空 pluginComponents（插件组件），只追加外置组件
    for (const def of defs) {
      if (typeof d.registerPluginComponent === 'function') d.registerPluginComponent(def)
    }
    if (defs.length > 0) {
      output.value += `[components] 已加载 ${defs.length} 个外置组件\n`
    }
  } catch (e) {
    console.warn('[components] 外置组件加载失败:', e)
  }
}
// 设计器网格/吸附状态（localStorage 持久化）
const designerShowGrid = persistedBoolNotFalse('eg-designer-showgrid')
const designerSnapEnabled = persistedBoolNotFalse('eg-designer-snap')
const designerGridSize = persistedInt('eg-designer-gridsize', 8)
// 设计器侧边栏切换：layers（层级） / templates（模板），默认层级
const designerSidePanel = ref('layers')
const designerTabOrderMode = ref(false) // Tab 顺序模式不持久化，每次打开默认关闭
const outputPreRef = ref(null)
const errorPreRef = ref(null)
const tipPreRef = ref(null)
const outputAutoScroll = ref(true)
const errorAutoScroll = ref(true)
const tipAutoScroll = ref(true)
const buildProgressRef = ref(null)
const buildActive = ref(false)
const buildStep = ref('')
const buildPercent = ref(0)

function handleBuildProgress(stage, output) {
  if (stage === 'progress') {
    const colon = output.indexOf(':')
    if (colon > 0) {
      const step = output.substring(0, colon)
      const pct = parseInt(output.substring(colon + 1), 10)
      if (!isNaN(pct)) {
        buildActive.value = true
        buildStep.value = step
        buildPercent.value = pct
      }
    }
    return true
  }
  if (stage === 'error') {
    buildActive.value = false
    buildProgressRef.value?.reset()
    return false
  }
  if (stage === 'done') {
    buildStep.value = 'done'
    buildPercent.value = 100
    return false
  }
  return false
}

function onOutputScroll(e) {
  const el = e.target
  if (!el) return
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 20
  outputAutoScroll.value = atBottom
}
function onErrorScroll(e) {
  const el = e.target
  if (!el) return
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 20
  errorAutoScroll.value = atBottom
}
function onTipScroll(e) {
  const el = e.target
  if (!el) return
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 20
  tipAutoScroll.value = atBottom
}

// 光标当前所在函数名（null 表示在全局区）。用于在编辑时关闭左栏彩虹流程线。
const currentFunctionName = ref(null)

// 滚动一个 ref 指向的 <pre> 到底部；在内容变化时通过 watcher 触发。
// 智能滚动：仅当用户之前在底部时才自动滚动，用户上滚查看历史时不强制跳转。
function scrollPreToBottom(refInstance, force = false) {
  let el = refInstance ? (refInstance.$el || refInstance) : null
  // 兜底：ref 为 null 时（如 tab-pane 刚切换），用 querySelector 查找当前可见的 pre
  if (!el) {
    const selector = force ? '.output-pre' : null
    if (selector) el = document.querySelector(selector)
  }
  if (el && typeof el.scrollHeight === 'number') {
    el.scrollTop = el.scrollHeight
  }
}

// L8：输出面板滚动节流，运行时快速输出多行（如循环打印）时
// 50ms 窗口内合并多次滚动请求，避免频繁 nextTick + DOM 读写。
const scrollTimers = { output: null, error: null, tip: null }
// 每次调用都更新最新的 ref，避免闭包捕获陈旧的 null ref
const pendingScrollRef = { output: null, error: null, tip: null }
function scheduleScroll(slot, refInstance) {
  pendingScrollRef[slot] = refInstance
  if (scrollTimers[slot]) return
  scrollTimers[slot] = setTimeout(() => {
    scrollTimers[slot] = null
    const shouldScroll = slot === 'output' ? outputAutoScroll.value
      : slot === 'error' ? errorAutoScroll.value
      : tipAutoScroll.value
    if (shouldScroll) scrollPreToBottom(pendingScrollRef[slot])
    pendingScrollRef[slot] = null
  }, 50)
}

watch(output, () => {
  scheduleScroll('output', outputPreRef.value)
})

watch(errorOutput, () => {
  scheduleScroll('error', errorPreRef.value)
})

watch(tipOutput, () => {
  scheduleScroll('tip', tipPreRef.value)
})

// 新的引用结果进来时，重置折叠状态（默认全部展开）
watch(refsResults, () => {
  collapsedRefFiles.value = new Set()
})

watch(outputTabName, () => {
  // 切到任意输出 tab 时，滚到最新一行并恢复自动滚动
  nextTick(() => {
    if (outputTabName.value === 'output') {
      outputAutoScroll.value = true
      scrollPreToBottom(outputPreRef.value)
    } else if (outputTabName.value === 'errors') {
      errorAutoScroll.value = true
      scrollPreToBottom(errorPreRef.value)
    } else if (outputTabName.value === 'tips') {
      tipAutoScroll.value = true
      scrollPreToBottom(tipPreRef.value)
    } else if (outputTabName.value === 'bookmarks') {
      refreshBookmarkList()
    }
  })
})

watch(output, (val, oldVal) => {
  if (val && !oldVal && !outputCollapsed.value) {
    outputTabName.value = 'output'
  }
})
watch(errorOutput, (val, oldVal) => {
  if (val && !oldVal && !outputCollapsed.value) {
    outputTabName.value = 'errors'
  }
})
watch(tipOutput, (val, oldVal) => {
  if (val && !oldVal && !outputCollapsed.value) {
    outputTabName.value = 'tips'
  }
})

// 自动保存指示器：当前文件内容与已保存内容是否一致
const isDirty = computed(() => {
  const f = activeFile.value
  if (!f) return false
  if (!f.path) return !!f.source // 新建文件有内容即未保存
  return f.source !== f.savedSource
})

// 自动保存：防抖 3s 无操作后保存到已有路径（仅对有 path 的文件生效）
let autoSaveTimer = null
watch(() => activeFile.value.source, () => {
  if (!activeFile.value.path) return // 新建未保存的文件不自动保存
  if (!autoSaveDelay.value) return // 0 = 禁用自动保存
  if (autoSaveTimer) clearTimeout(autoSaveTimer)
  autoSaveTimer = setTimeout(async () => {
    const file = activeFile.value
    if (!file.path) return
    const content = getFileSaveContent(file)
    if (content === file.savedSource) return // 无修改
    try {
      const data = await IDEService.QuickSave(file.path, content)
      if (!data.error) {
        file.savedSource = content
      }
    } catch {}
  }, autoSaveDelay.value)
})

// H2：parsed 改为 debounced ref，避免逐键触发整文件重解析。
// 250ms 节流：输入停止 250ms 后才重新解析，期间用上一次结果。
// parsed 已在文件前部提前声名为 ref(parseEg(''))，此处通过 watch + immediate 初始化当前文件。
function scheduleParse(src) {
  if (parseDebounceTimer) clearTimeout(parseDebounceTimer)
  parseDebounceTimer = setTimeout(() => {
    parsed.value = parseEg(src || '')
  }, 250)
}
watch(() => activeFile.value ? activeFile.value.source : '', (src) => {
  scheduleParse(src)
})
parsed.value = parseEg(activeFile.value ? activeFile.value.source : '')

// 文件切换后延时刷新书签列表（等待 Editor 组件完成书签恢复）
watch(activeFileIndex, () => {
  setTimeout(() => {
    if (outputTabName.value === 'bookmarks') refreshBookmarkList()
  }, 300)
  // v0.9.13：切换文件后同步断点（断点按文件隔离，Editor 单实例复用）
  nextTick(() => {
    const fileName = activeFile.value?.name
    if (!fileName) return
    const bps = debugPanelRef.value?.getBreakpoints?.() || []
    const lines = bps.filter(bp => bp.file === fileName).map(bp => bp.line)
    editorRef.value?.setBreakpoints?.(lines)
  })
})

// 项目模块引用表（.elib），libVersion 变化时自动刷新
const projectLibsList = computed(() => {
  libVersion.value
  return getProjectLibsSummary()
})

const editorSuggestions = computed(() => {
  const list = []
  for (const fn of parsed.value.functions) {
    const params = fn.params.map(p => p.name).join(', ')
    list.push({
      label: fn.name,
      kind: 'function',
      detail: `${fn.kind} ${fn.name}(${params})`,
      insertText: fn.name
    })
    for (const p of fn.params) {
      list.push({
        label: p.name,
        kind: 'param',
        detail: `${fn.name} 的参数`,
        insertText: p.name
      })
    }
  }
  return list
})

function switchFile(idx) {
  selectedFunctionIndex.value = null
  currentFunctionName.value = null

  const prevIdx = activeFileIndex.value
  activeFileIndex.value = idx
  if (prevIdx !== idx) {
    recordNavigation(idx, 1)
  }
  nextTick(() => {
    const bar = tabsBarRef.value
    if (!bar) return
    const activeTab = bar.querySelector('.file-tab.active')
    if (!activeTab) return
    const barRect = bar.getBoundingClientRect()
    const tabRect = activeTab.getBoundingClientRect()
    if (tabRect.left < barRect.left) {
      bar.scrollBy({ left: tabRect.left - barRect.left - 8, behavior: 'smooth' })
    } else if (tabRect.right > barRect.right) {
      bar.scrollBy({ left: tabRect.right - barRect.right + 8, behavior: 'smooth' })
    }
  })
}

// ===== 文件 tab 拖拽排序 =====
const dragTabIndex = ref(null)
const dragOverIndex = ref(null)

function onTabDragStart(e, idx) {
  dragTabIndex.value = idx
  e.dataTransfer.effectAllowed = 'move'
}
function onTabDragOver(e, idx) {
  if (dragTabIndex.value === null || dragTabIndex.value === idx) return
  dragOverIndex.value = idx
  e.dataTransfer.dropEffect = 'move'
}
function onTabDragLeave() {
  dragOverIndex.value = null
}
function onTabDrop(e, idx) {
  e.preventDefault()
  const from = dragTabIndex.value
  dragOverIndex.value = null
  dragTabIndex.value = null
  if (from === null || from === idx) return
  const movedFile = files.value[from]
  const targetFile = files.value[idx]
  if (movedFile.pinned !== targetFile.pinned) return
  const arr = [...files.value]
  const [moved] = arr.splice(from, 1)
  arr.splice(idx, 0, moved)
  if (activeFileIndex.value === from) {
    activeFileIndex.value = idx
  } else if (from < activeFileIndex.value && idx >= activeFileIndex.value) {
    activeFileIndex.value--
  } else if (from > activeFileIndex.value && idx <= activeFileIndex.value) {
    activeFileIndex.value++
  }
  files.value = arr
}
function onTabDragEnd() {
  dragTabIndex.value = null
  dragOverIndex.value = null
}

function newCodeFile() {
  const count = files.value.filter(f => f.name.startsWith('未命名')).length + 1
  const name = count === 1 ? '未命名.eg' : `未命名${count}.eg`
  const source = `# 程序集 main

函数 主函数()
结束函数
`
  const f = makeFile(name, source)
  files.value.push(f)
  switchFile(files.value.length - 1)
  pushFileOp({ type: 'add', file: cloneFile(f), index: files.value.length - 1, onDisk: false })
}

function newClassFile() {
  const count = files.value.filter(f => f.name.startsWith('类_')).length + 1
  const name = `类_${count}.eg`
  const source = `# 程序集 main

类型 类_${count} 类
    // 类成员
结束类型
`
  const f = makeFile(name, source)
  files.value.push(f)
  switchFile(files.value.length - 1)
  pushFileOp({ type: 'add', file: cloneFile(f), index: files.value.length - 1, onDisk: false })
}

function newModuleFile() {
  const count = files.value.filter(f => f.name.startsWith('模块_')).length + 1
  const name = `模块_${count}.eg`
  const source = `# 程序集 main

// 模块级函数

函数 初始化()
结束函数
`
  const f = makeFile(name, source)
  files.value.push(f)
  switchFile(files.value.length - 1)
  pushFileOp({ type: 'add', file: cloneFile(f), index: files.value.length - 1, onDisk: false })
}

async function ensureCodeFileForForm(formFile) {
  if (!formFile.codeFileName) return formFile
  const idx = files.value.findIndex(f => f.name === formFile.codeFileName)
  if (idx >= 0) return files.value[idx]
  const codeName = formFile.codeFileName
  // 尝试从磁盘加载同名 .eg，避免丢失用户已写好的代码
  if (formFile.path) {
    const codePath = formFile.path.replace(/\.ew$/i, '.eg')
    try {
      const codeData = await IDEService.ReadProjectFile(codePath)
      if (!codeData.error && typeof codeData.content === 'string') {
        files.value.push(makeFile(codeName, codeData.content, codePath))
        return files.value[files.value.length - 1]
      }
    } catch {}
  }
  const codeSource = `# 程序集 main\n\n函数 主函数()\n结束函数\n`
  files.value.push(makeFile(codeName, codeSource))
  return files.value[files.value.length - 1]
}

async function onOpenEvent({ component, event }) {
  const codeFile = isFormFile.value ? await ensureCodeFileForForm(activeFile.value) : activeFile.value
  const codeIdx = files.value.indexOf(codeFile)
  if (codeIdx >= 0) {
    switchFile(codeIdx)
  }
  codeFile.view = 'code'

  nextTick(() => {
    const funcName = `${component}_${event}`
    const lines = codeFile.source.split('\n')
    let lineIdx = lines.findIndex(l => {
      const trimmed = l.trim()
      return trimmed.startsWith(`函数 ${funcName}(`)
    })

    if (lineIdx === -1) {
      const lastEnd = lines.map((l, i) => ({ l: l.trim(), i }))
        .filter(item => item.l === '结束函数')
        .pop()
      const insertIdx = lastEnd ? lastEnd.i + 1 : lines.length
      const indent = '    '
      lines.splice(insertIdx, 0, '', `函数 ${funcName}()`, `${indent}// 事件处理`, '结束函数')
      codeFile.source = lines.join('\n')
      lineIdx = insertIdx + 1
    }

    nextTick(() => {
      editorRef.value?.gotoLine(lineIdx + 1)
    })
  })
}

function switchToDesignView() {
  const file = activeFile.value
  if (!file || file.view === 'design') return
  const design = parseDesignSource(file.source)
  if (design) {
    file.design = design
  }
  file.view = 'design'
}

function switchToCodeView() {
  const file = activeFile.value
  if (!file || file.view === 'code') return
  file.view = 'code'
}

function onDesignChange(design) {
  const file = activeFile.value
  if (!file || !isFormFileName(file.name)) return
  file.design = design
  file.source = JSON.stringify(design, null, 2)
}

async function openFile() {
  try {
    const data = await IDEService.OpenFile()
    if (data.error) {
      output.value = '打开文件失败: ' + data.error
      setStatusMsg('打开失败', 3000)
      return
    }
    if (data.name) {
      files.value.push(makeFile(data.name, data.content, data.path))
      switchFile(files.value.length - 1)
      setStatusMsg('已打开 ' + data.name, 2000)
    }
  } catch (e) {
    output.value = '打开文件失败: ' + e.message
    setStatusMsg('打开失败', 3000)
  }
}

function getFileSaveContent(file) {
  // .ew 窗口文件的 source 已与 design 保持同步
  return file.source
}

async function quickSave() {
  if (!activeFile.value.path) {
    await saveFile(false)
    return
  }
  try {
    const content = getFileSaveContent(activeFile.value)
    const data = await IDEService.QuickSave(activeFile.value.path, content)
    if (data.error) {
      output.value = '快速保存失败: ' + data.error
      setStatusMsg('保存失败', 5000)
      return
    }
    activeFile.value.savedSource = content
    setStatusMsg('已保存')
    await maybeRefreshProjectLibs(data.path)
  } catch (e) {
    output.value = '快速保存失败: ' + e.message
    setStatusMsg('保存失败', 5000)
  }
}

// 导出当前输出面板内容到文件（保存对话框）
async function exportLog() {
  let content = ''
  if (outputTabName.value === 'output') content = output.value
  else if (outputTabName.value === 'errors') content = errorOutput.value
  else content = tipOutput.value
  if (!content) {
    setStatusMsg('当前面板无内容', 2000)
    return
  }
  const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
  const defaultName = `eg-${outputTabName.value}-${ts}.log`
  try {
    const data = await IDEService.SaveFile(defaultName, content, '导出日志')
    if (data.error) {
      setStatusMsg('导出失败: ' + data.error, 4000)
      return
    }
    setStatusMsg('已导出')
  } catch (e) {
    setStatusMsg('导出失败: ' + e.message, 4000)
  }
}

// 清空所有输出面板内容
function clearAllOutput() {
  output.value = ''
  errorOutput.value = ''
  tipOutput.value = ''
  refsResults.value = []
  setStatusMsg('已清空输出', 2000)
}

// 构建历史记录：加载/保存/追加/清除
function loadBuildHistory() {
  if (!projectPath.value) { buildHistory.value = []; return }
  try {
    const raw = localStorage.getItem('eg-build-history-' + projectPath.value)
    buildHistory.value = raw ? JSON.parse(raw) : []
  } catch { buildHistory.value = [] }
}
function saveBuildHistory() {
  if (!projectPath.value) return
  try { localStorage.setItem('eg-build-history-' + projectPath.value, JSON.stringify(buildHistory.value)) } catch {}
}
function addBuildHistory(entry) {
  buildHistory.value.unshift(entry)
  if (buildHistory.value.length > MAX_BUILD_HISTORY) {
    buildHistory.value = buildHistory.value.slice(0, MAX_BUILD_HISTORY)
  }
  saveBuildHistory()
}
function onSearch() { searchQuery.value = ''; searchVisible.value = true }

async function doSearch() {
  const q = searchQuery.value.trim()
  if (!q) { setStatusMsg('请输入搜索内容', 2000); return }
  if (!projectPath.value) { setStatusMsg('未打开项目', 2000); return }
  searching.value = true
  try {
    let re = null
    if (searchUseRegex.value) { try { re = new RegExp(q, 'g') } catch (e) { setStatusMsg('正则无效: ' + e.message, 3000); searching.value = false; return } }
    const results = []
    // H1：ListProjectDir 已递归返回整棵树，直接遍历 Children，不再逐层 RPC
    const tree = await IDEService.ListProjectDir(projectPath.value)
    const egFiles = collectEgFilesFromTree(tree)
    for (const node of egFiles) {
      let data
      try { data = await IDEService.ReadProjectFile(node.Path) } catch { continue }
      if (!data || data.error || typeof data.content !== 'string') continue
      const flines = data.content.split('\n')
      const rel = node.Path.replace(projectPath.value, '').replace(/^[\\/]+/, '')
      for (let i = 0; i < flines.length; i++) {
        const line = flines[i]
        let col = -1
        if (re) { re.lastIndex = 0; const m = re.exec(line); if (m) col = m.index + 1 }
        else { col = line.indexOf(q) + 1 }
        if (col > 0) results.push({ file: rel, filePath: node.Path, line: i + 1, col, preview: line.trim() })
      }
    }
    refsQuery.value = '搜索: ' + q
    refsResults.value = results
    outputTabName.value = 'tips'
    searchVisible.value = false
    setStatusMsg(`找到 ${results.length} 处匹配`, 3000)
  } catch (e) { setStatusMsg('搜索失败: ' + e.message, 4000) }
  finally { searching.value = false }
}

function clearBuildHistory() {
  buildHistory.value = []
  saveBuildHistory()
  setStatusMsg('已清空构建历史', 2000)
}
async function copyBuildHistoryPath(p) {
  try { await navigator.clipboard.writeText(p); setStatusMsg('已复制路径', 2000) } catch {}
}
function openBuildHistoryFolder(p) {
  IDEService.OpenInExplorer(p)
}
// 项目切换时自动加载构建历史
watch(projectPath, () => { loadBuildHistory() })

// 检查构建产物文件指纹（SHA256 + 大小）并输出到面板
async function checkSignatureAndReport(filePath) {
  try {
    const result = await IDEService.CheckSignature(filePath)
    let info
    try { info = JSON.parse(result) } catch { info = null }
    if (info && info.status === 'Computed') {
      output.value += `[指纹] SHA256: ${info.sha256}\n`
      output.value += `[大小] ${info.sizeText} (${info.size} 字节)\n`
    } else {
      output.value += `[指纹] 计算失败: ${info?.error || result}\n`
    }
  } catch (e) {
    output.value += `[指纹] 检查异常: ${e.message}\n`
  }
}

// 刷新书签列表面板：从 Editor 组件获取书签行号和预览文本
function refreshBookmarkList() {
  if (!editorRef.value) return
  const bookmarks = editorRef.value.getBookmarks?.()
  if (!bookmarks || bookmarks.length === 0) {
    bookmarkList.value = []
    return
  }
  // 为每个书签行获取预览文本（当前文件对应行内容）
  const source = activeFile.value.source || ''
  const lines = source.split('\n')
  bookmarkList.value = bookmarks.map(line => ({
    line,
    preview: lines[line - 1]?.trim() || '(空行)'
  }))
}

// 跳转到书签行
function gotoBookmark(line) {
  editorRef.value?.gotoLine?.(line)
}

// 清除所有书签
function clearAllBookmarks() {
  editorRef.value?.clearBookmarks?.()
  bookmarkList.value = []
  setStatusMsg('已清除所有书签', 2000)
}

async function saveFile(saveAs = false) {
  const title = saveAs ? '另存为' : '保存文件'
  try {
    const content = getFileSaveContent(activeFile.value)
    const data = await IDEService.SaveFile(activeFile.value.name, content, title)
    if (data.error) {
      output.value = (saveAs ? '另存为失败: ' : '保存失败: ') + data.error
      setStatusMsg(saveAs ? '另存为失败' : '保存失败', 5000)
      return
    }
    if (data.path) {
      activeFile.value.path = data.path
      activeFile.value.savedSource = content
      setStatusMsg(saveAs ? '已另存为' : '已保存')
      await maybeRefreshProjectLibs(data.path)
    }
  } catch (e) {
    output.value = (saveAs ? '另存为失败: ' : '保存失败: ') + e.message
    setStatusMsg(saveAs ? '另存为失败' : '保存失败', 5000)
  }
}

// 如果保存的是 <项目>/libs/<扩展包>/ 下的 source.eg / commands.json / package.json，
// 自动重新加载项目扩展包，让支持库面板和补全立刻反映修改。
async function maybeRefreshProjectLibs(savedPath) {
  if (!savedPath || !projectPath.value) return
  const root = projectPath.value.replace(/[\\/]+$/, '')
  const norm = savedPath.replace(/[\\/]+/g, '/')
  const rootNorm = root.replace(/[\\/]+/g, '/')
  if (!norm.startsWith(rootNorm + '/libs/')) return
  const tail = norm.slice((rootNorm + '/libs/').length)
  // 必须是 <name>/<file> 形式，且 file 是这三种之一
  const parts = tail.split('/')
  if (parts.length < 2) return
  const file = parts[parts.length - 1]
  if (file !== 'source.eg' && file !== 'commands.json' && file !== 'package.json') return
  try {
    const r = await loadProjectLibs(root)
    if (r && r.count > 0) {
      setStatusMsg(`已重新加载 ${r.count} 个扩展包`, 2500)
    }
  } catch (e) {
    console.warn('[lib] 保存后刷新扩展包失败:', e)
  }
}

function showConfirm(title, message, onOk) {
  confirmDialogTitle.value = title
  confirmDialogMessage.value = message
  confirmDialogCallback = onOk
  confirmDialogVisible.value = true
}

function onConfirmOk() {
  confirmDialogVisible.value = false
  if (confirmDialogCallback) {
    const cb = confirmDialogCallback
    confirmDialogCallback = null
    cb()
  }
}

function onConfirmCancel() {
  confirmDialogVisible.value = false
  confirmDialogCallback = null
}

function isFileDirty(file) {
  return file && file.source !== file.savedSource
}

function closeFile(idx) {
  const file = files.value[idx]
  if (file && isFileDirty(file)) {
    showConfirm('未保存的更改', `文件 "${file.name}" 有未保存的更改，确定要关闭吗？`, () => {
      doCloseFile(idx)
    })
    return
  }
  doCloseFile(idx)
}

function doCloseFile(idx) {
  if (files.value.length <= 1) {
    files.value[0] = makeFile('main.eg', DEFAULT_SOURCE)
    switchFile(0)
    return
  }
  pushClosedFile(files.value[idx])
  const wasActive = activeFileIndex.value === idx
  files.value.splice(idx, 1)
  if (wasActive) {
    activeFileIndex.value = Math.min(idx, files.value.length - 1)
  } else if (activeFileIndex.value > idx) {
    activeFileIndex.value = activeFileIndex.value - 1
  }
  selectedFunctionIndex.value = null
}

function closeOtherFiles(keepIdx) {
  const keep = files.value[keepIdx]
  if (!keep) return
  const toClose = []
  for (let i = 0; i < files.value.length; i++) {
    if (i !== keepIdx && !files.value[i].pinned && isFileDirty(files.value[i])) {
      toClose.push(i)
    }
  }
  if (toClose.length > 0) {
    showConfirm('未保存的更改', `${toClose.length} 个文件有未保存的更改，确定要关闭吗？`, () => {
      doCloseOthers(keepIdx)
    })
    return
  }
  doCloseOthers(keepIdx)
}

function doCloseOthers(keepIdx) {
  const keepFile = files.value[keepIdx]
  const newFiles = []
  for (let i = 0; i < files.value.length; i++) {
    const f = files.value[i]
    if (i === keepIdx || f.pinned) {
      newFiles.push(f)
    } else {
      pushClosedFile(f)
    }
  }
  files.value = newFiles
  activeFileIndex.value = newFiles.indexOf(keepFile)
  if (activeFileIndex.value < 0) activeFileIndex.value = 0
  selectedFunctionIndex.value = null
}

function closeAllFiles() {
  const dirtyCount = files.value.filter(f => !f.pinned && isFileDirty(f) && f.path).length
  const hasPinned = files.value.some(f => f.pinned)
  if (dirtyCount > 0) {
    showConfirm('未保存的更改', `${dirtyCount} 个文件有未保存的更改，确定要关闭${hasPinned ? '其他' : '所有'}文件吗？`, () => {
      doCloseAll()
    })
    return
  }
  doCloseAll()
}

function doCloseAll() {
  const pinned = files.value.filter(f => f.pinned)
  for (let i = files.value.length - 1; i >= 0; i--) {
    if (!files.value[i].pinned) {
      pushClosedFile(files.value[i])
    }
  }
  if (pinned.length > 0) {
    files.value = pinned
    activeFileIndex.value = 0
  } else {
    files.value = [makeFile('main.eg', DEFAULT_SOURCE)]
    activeFileIndex.value = 0
  }
  selectedFunctionIndex.value = null
}

function closeSavedFiles() {
  const savedFiles = files.value.filter(f => !f.pinned && !isFileDirty(f))
  if (savedFiles.length === 0) {
    setStatusMsg('没有已保存且可关闭的文件', 2000)
    return
  }
  const currentFile = files.value[activeFileIndex.value]
  const dirtyOrPinned = files.value.filter(f => f.pinned || isFileDirty(f))
  for (const f of savedFiles) {
    pushClosedFile(f)
  }
  if (dirtyOrPinned.length > 0) {
    files.value = dirtyOrPinned
    const curIdx = dirtyOrPinned.indexOf(currentFile)
    activeFileIndex.value = curIdx >= 0 ? curIdx : 0
  } else {
    files.value = [makeFile('main.eg', DEFAULT_SOURCE)]
    activeFileIndex.value = 0
  }
  selectedFunctionIndex.value = null
  setStatusMsg(`已关闭 ${savedFiles.length} 个已保存文件`, 2000)
}

async function deleteFile(idx) {
  const file = files.value[idx]
  if (!file) return

  const isForm = isFormFileName(file.name)
  const codeIdx = isForm && file.codeFileName
    ? files.value.findIndex(f => f.name === file.codeFileName)
    : -1
  const codeFile = codeIdx >= 0 ? files.value[codeIdx] : null

  const confirmMsg = file.path
    ? `确定要删除文件 "${file.name}" 吗？\n此操作将同时删除磁盘文件，可通过撤销恢复。`
    : `确定要删除 "${file.name}" 吗？\n可通过撤销恢复。`
  if (!confirm(confirmMsg)) return

  let formContent = file.source
  let codeContent = codeFile ? codeFile.source : null
  let formPath = file.path
  let codePath = codeFile ? codeFile.path : null

  if (file.path) {
    try {
      const r = await IDEService.ReadProjectFile(file.path)
      if (!r.error) formContent = r.content
      if (isForm && codeFile && codeFile.path) {
        const cr = await IDEService.ReadProjectFile(codeFile.path)
        if (!cr.error) codeContent = cr.content
      }
    } catch (e) {
      console.warn('删除前读取文件内容失败:', e)
    }
  }

  const deletedFormFile = cloneFile(file)
  const deletedCodeFile = codeFile ? cloneFile(codeFile) : null

  const indicesToDelete = []
  if (codeIdx >= 0) indicesToDelete.push(codeIdx)
  indicesToDelete.push(idx)
  indicesToDelete.sort((a, b) => b - a)
  for (const i of indicesToDelete) {
    files.value.splice(i, 1)
  }

  if (file.path) {
    try {
      await IDEService.DeleteFile(file.path)
      if (isForm && codeFile && codeFile.path) {
        await IDEService.DeleteFile(codeFile.path)
      }
    } catch (e) {
      console.warn('删除磁盘文件失败:', e)
    }
    if (projectPath.value) {
      await loadProjectTree(projectPath.value)
    }
  }

  if (files.value.length === 0) {
    files.value.push(makeFile('main.eg', DEFAULT_SOURCE))
    activeFileIndex.value = 0
  } else {
    const numDeleted = indicesToDelete.length
    let newActive = activeFileIndex.value
    for (const i of indicesToDelete) {
      if (newActive >= i) newActive = Math.max(0, newActive - 1)
    }
    activeFileIndex.value = Math.min(newActive, files.value.length - 1)
  }
  selectedFunctionIndex.value = null

  pushFileOp({
    type: isForm ? 'delete-window' : 'delete',
    formFile: deletedFormFile,
    formPath,
    formDiskContent: formContent,
    formWasOnDisk: !!file.path,
    codeFile: deletedCodeFile,
    codePath,
    codeDiskContent: codeContent,
    codeWasOnDisk: !!(codeFile && codeFile.path)
  })
  setStatusMsg('已删除: ' + file.name, 2500)
}

async function deleteFileByPath(filePath) {
  const idx = files.value.findIndex(f => f.path === filePath)
  if (idx >= 0) {
    await deleteFile(idx)
    return
  }

  const name = filePath.split(/[\\/]/).pop() || filePath
  const isForm = /\.ew$/i.test(name)
  const codePath = isForm ? filePath.replace(/\.ew$/i, '.eg') : null

  if (!confirm(`确定要删除文件 "${name}" 吗？\n此操作将同时删除磁盘文件，可通过撤销恢复。`)) return

  let formContent = null
  let codeContent = null
  let codeFileObj = null

  try {
    const r = await IDEService.ReadProjectFile(filePath)
    if (!r.error) formContent = r.content
    if (isForm && codePath) {
      const cr = await IDEService.ReadProjectFile(codePath)
      if (!cr.error) {
        codeContent = cr.content
        const codeName = codePath.split(/[\\/]/).pop()
        codeFileObj = makeFile(codeName, codeContent, codePath)
        codeFileObj.savedSource = codeContent
      }
    }
  } catch (e) {
    console.warn('删除前读取文件失败:', e)
  }

  try {
    await IDEService.DeleteFile(filePath)
    if (isForm && codePath) {
      try { await IDEService.DeleteFile(codePath) } catch (e) {}
    }
  } catch (e) {
    console.warn('删除磁盘文件失败:', e)
  }

  if (projectPath.value) await loadProjectTree(projectPath.value)

  const formFileObj = makeFile(name, formContent || '', filePath)
  formFileObj.savedSource = formContent || ''
  if (isForm) {
    formFileObj.design = formContent ? JSON.parse(formContent) : null
    formFileObj.view = 'design'
    formFileObj.codeFileName = codeFileObj ? codeFileObj.name : null
  }

  pushFileOp({
    type: isForm ? 'delete-window' : 'delete',
    formFile: cloneFile(formFileObj),
    formPath: filePath,
    formDiskContent: formContent,
    formWasOnDisk: true,
    codeFile: codeFileObj ? cloneFile(codeFileObj) : null,
    codePath,
    codeDiskContent: codeContent,
    codeWasOnDisk: !!(codeFileObj && codePath)
  })

  setStatusMsg('已删除: ' + name, 2500)
}

function selectFunction(idx) {
  if (idx === null) {
    selectedFunctionIndex.value = null
    return
  }
  selectedFunctionIndex.value = idx
  const fn = parsed.value.functions[idx]
  if (fn && editorRef.value) {
    editorRef.value.gotoLine(fn.startLine + 1)
  }
}

function onOutlineGotoFunction(fn) {
  const idx = parsed.value.functions.findIndex(f => f.name === fn.name)
  if (idx >= 0) selectFunction(idx)
}

function gotoLine(line) {
  selectedFunctionIndex.value = null
  recordNavigation(activeFileIndex.value, line)
  nextTick(() => {
    editorRef.value?.gotoLine(line)
  })
}

function onCursorChange(pos) {
  let text = `行 ${pos.line}, 列 ${pos.column}`
  if (pos.selection) {
    const s = pos.selection
    if (s.lineCount > 1) {
      text += ` (已选择 ${s.lineCount} 行, ${s.charCount} 字符)`
    } else if (s.charCount > 0) {
      text += ` (已选择 ${s.charCount} 字符)`
    }
  }
  cursorText.value = text
  // 1-based lineNumber -> 0-based 与 parsed.functions 对齐
  const line = pos.line - 1
  const fn = (parsed.value.functions || []).find(f => line >= f.startLine && line <= f.endLine)
  currentFunctionName.value = fn ? fn.name : null
}

function gotoCurrentFunction() {
  if (!currentFunctionName.value) return
  const fn = (parsed.value.functions || []).find(f => f.name === currentFunctionName.value)
  if (fn && editorRef.value) {
    const line = fn.startLine + 1
    recordNavigation(activeFileIndex.value, line)
    editorRef.value.gotoLine(line)
  }
}

function toggleWordWrap() {
  editorWordWrap.value = !editorWordWrap.value
  localStorage.setItem('eg-wordwrap', String(editorWordWrap.value))
  setStatusMsg('自动换行：' + (editorWordWrap.value ? '开启' : '关闭'), 1500)
}

function toggleMinimap() {
  minimapEnabled.value = !minimapEnabled.value
  localStorage.setItem('eg-minimap', String(minimapEnabled.value))
  setStatusMsg('缩略图：' + (minimapEnabled.value ? '显示' : '隐藏'), 1500)
}

function bcRevealProject() {
  if (projectPath.value && window.IDEService?.OpenInExplorer) {
    window.IDEService.OpenInExplorer(projectPath.value)
  }
}

function bcRevealFile() {
  const f = activeFile.value
  if (f?.path && window.IDEService?.OpenInExplorer) {
    const dir = f.path.substring(0, f.path.lastIndexOf('\\'))
    window.IDEService.OpenInExplorer(dir)
  }
}

function clearErrorMarkers() {
  editorRef.value?.setMarkers([])
}

function extractErrorLine(err) {
  // 转译错误：第 N 行
  const m = err.match(/第\s*(\d+)\s*行/)
  if (m) return parseInt(m[1], 10)
  // Go 编译错误（带 //line 指令）：filename.eg:N:col:
  const eg = err.match(/(\S+\.eg):(\d+):/)
  if (eg) return parseInt(eg[2], 10)
  // 兜底：usercode.go / main.go 行号
  const gm = err.match(/(?:usercode|main)\.go:(\d+):/)
  if (gm) return parseInt(gm[1], 10)
  return null
}

function gotoError(line, message) {
  selectedFunctionIndex.value = null

  nextTick(() => {
    editorRef.value?.setMarkers([{ line, message }])
    editorRef.value?.gotoLine(line)
    editorRef.value?.flashLine?.(line)
  })
}

// 点击错误面板中的可点击行，跳转到对应源码位置
async function gotoErrorEntry(entry) {
  if (!entry.clickable || !entry.line) return
  const file = entry.file
  if (file && file !== '源码.eg' && projectPath.value) {
    // 构造候选路径：相对路径直接拼接，纯文件名尝试 src/ 和根目录
    const candidates = []
    if (file.includes('/')) {
      candidates.push(projectPath.value + '/' + file)
    } else {
      candidates.push(projectPath.value + '/src/' + file)
      candidates.push(projectPath.value + '/' + file)
    }
    for (const p of candidates) {
      try {
        const data = await IDEService.ReadProjectFile(p)
        if (!data.error) {
          await openProjectFile(p)
          break
        }
      } catch {}
    }
  }
  nextTick(() => {
    editorRef.value?.setMarkers([{ line: entry.line, message: entry.text }])
    editorRef.value?.gotoLine(entry.line)
    editorRef.value?.flashLine?.(entry.line)
  })
}

// 编辑器「转到定义」：当用户 Ctrl+Click 一个 .elib 命令时，
// 由 Editor.vue 的 DefinitionProvider 触发 emit('goto-def', { word, key, meta })。
// 查找顺序：
//   1. 项目扩展包（.elib）的 source.eg 中找 `函数 <englishName>(`，找到则打开并跳转
//   2. 内置/项目支持库命令：在「提示」tab 显示完整命令文档（参数表、返回值、调用示例）
//   3. 都未命中：状态栏提示未找到定义
// 跨文件跳转：Editor.vue 调后端 FindDefCrossFile 命中后触发
// payload: { file, line, col, word, source, pkgName }
async function onOpenFileAt(payload) {
  if (!payload || !payload.file) return
  try {
    await openProjectFile(payload.file)
    nextTick(() => {
      editorRef.value?.gotoLine(payload.line || 1)
      const src = payload.source === 'global-lib' ? '全局库'
                 : payload.source === 'project-lib' ? '项目库'
                 : '项目'
      const pkg = payload.pkgName ? `:${payload.pkgName}` : ''
      setStatusMsg(`已转到定义: ${payload.word} → ${src}${pkg}:${payload.line}`, 3000)
    })
  } catch (e) {
    setStatusMsg(`打开文件失败: ${e.message || e}`, 3000)
  }
}

async function onGotoDef({ word, key, meta }) {
  if (!key) return

  // 1. 在项目 .elib 的 source.eg 中查找定义
  const libs = getProjectLibsSummary()
  if (libs && libs.length > 0) {
    const fnRe = new RegExp('^\\s*函数\\s+' + key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*\\(')
    for (const lib of libs) {
      const sep = lib.path.includes('\\') ? '\\' : '/'
      const srcPath = lib.path + sep + 'source.eg'
      let data
      try {
        data = await IDEService.ReadProjectFile(srcPath)
      } catch (e) { continue }
      if (!data || data.error || typeof data.content !== 'string') continue
      const lines = data.content.split('\n')
      let lineNo = -1
      for (let i = 0; i < lines.length; i++) {
        if (fnRe.test(lines[i])) { lineNo = i + 1; break }
      }
      if (lineNo > 0) {
        await openProjectFile(srcPath)
        nextTick(() => {
          editorRef.value?.gotoLine(lineNo)
          setStatusMsg(`已转到定义: ${word} → ${lib.name}:${lineNo}`, 3000)
        })
        return
      }
    }
  }

  // 2. 回退：支持库命令文档展示
  if (meta) {
    const doc = formatCommandDoc(word, key, meta)
    if (doc) {
      outputTabName.value = 'tips'
      tipOutput.value = doc
      setStatusMsg(`已显示命令文档: ${word}`, 2500)
      return
    }
  }

  setStatusMsg(`未找到 ${word} 的定义`, 2500)
}

// 格式化支持库命令为完整文档，用于「转到定义」回退展示。
// 返回多行文本；meta 无效时返回 null。
function formatCommandDoc(word, key, meta) {
  if (!meta) return null
  const lines = []
  const title = meta.displayName ? `${meta.displayName}（${key}）` : key
  lines.push(`===== 命令：${title} =====`)
  // 反查所属库：meta 自带 library 优先（项目库），否则通过 supportTree 遍历查找
  let libName = meta.library
  if (!libName) {
    const tree = getMergedTree()
    for (const lib of tree) {
      if (lib.children && lib.children.some(c => c.key === key)) {
        libName = `${lib.label}（${lib.key}）`
        break
      }
    }
  }
  if (libName) lines.push(`所属库：${libName}`)
  if (meta.category) lines.push(`分类：${meta.category}`)
  if (meta.callSyntax) lines.push(`调用：${meta.callSyntax}`)
  lines.push('')
  // 参数表
  const params = meta.params || []
  if (params.length > 0) {
    lines.push('参数：')
    for (const p of params) {
      const opt = p.optional ? '可选' : '必填'
      const desc = p.description ? ` — ${p.description}` : ''
      lines.push(`  - ${p.name}（${p.type || '未知'}，${opt}）${desc}`)
    }
  } else {
    lines.push('参数：无')
  }
  lines.push('')
  lines.push(`返回：${meta.returnType || '无返回值'}`)
  if (meta.summary) {
    lines.push('')
    lines.push(`说明：${meta.summary}`)
  }
  return lines.join('\n')
}

// 查找所有引用（Shift+F12）：
// Editor.vue 的 ReferenceProvider 在当前文件已扫了一遍（localRefs），
// 这里再扫描项目内所有其它 .eg 文件与 .elib 的 source.eg，
// 把所有引用位置汇总输出到「提示」页，便于快速浏览。
async function onFindRefs({ word, modelUri, localRefs }) {
  if (!word) return
  outputTabName.value = 'tips'
  refsQuery.value = word
  const results = []

  // 1) 当前文件
  if (localRefs && localRefs.length > 0) {
    const curName = activeFile.value.name
    const curPath = activeFile.value.path || curName
    const src = activeFile.value.source || ''
    const srcLines = src.split('\n')
    for (const r of localRefs) {
      const ln = r.range.startLineNumber
      const preview = (srcLines[ln - 1] || '').trim()
      results.push({ file: curName, filePath: curPath, line: ln, col: r.range.startColumn, preview })
    }
  }

  // 2) 项目内其它 .eg 文件
  if (projectPath.value) {
    try {
      const others = await scanReferencesInProjectEg(word, activeFile.value.path)
      for (const item of others) {
        for (const r of item.refs) {
          results.push({ file: item.relPath, filePath: item.absPath, line: r.line, col: r.col, preview: r.preview })
        }
      }
    } catch (e) {
      tipOutput.value = `扫描项目 .eg 失败: ${e.message}`
      return
    }
  }

  // 3) 项目 .elib 的 source.eg
  const libs = getProjectLibsSummary()
  for (const lib of libs) {
    const sep = lib.path.includes('\\') ? '\\' : '/'
    const srcPath = lib.path + sep + 'source.eg'
    let data
    try { data = await IDEService.ReadProjectFile(srcPath) } catch { continue }
    if (!data || data.error || typeof data.content !== 'string') continue
    const refs = scanReferencesInText(data.content, word)
    const libLines = data.content.split('\n')
    for (const r of refs) {
      const preview = (libLines[r.line - 1] || '').trim()
      results.push({ file: `${lib.name}/source.eg`, filePath: srcPath, line: r.line, col: r.col, preview })
    }
  }

  refsResults.value = results
  tipOutput.value = ''
  setStatusMsg(`已查找 ${word} 的引用：${results.length} 处`, 3000)
}

async function gotoRefItem(r) {
  if (!r || !r.filePath) return
  await openProjectFile(r.filePath)
  nextTick(() => {
    editorRef.value?.gotoLine(r.line)
  })
}

// ===== P2 调试器：前端集成 =====

// gotoDebugLocation: 点击调用栈/断点 → 跳转到编辑器对应行
// file 可能是 basename（dlv //line 指令返回的），需要匹配项目中的 .eg 文件
async function gotoDebugLocation(file, line) {
  if (!file || !line) return
  // file 是 basename（如 main.eg），在已打开文件中找匹配
  const fileName = file.includes('/') || file.includes('\\') ? fileNameFromPath(file) : file
  const idx = files.value.findIndex(f => f.name === fileName)
  if (idx >= 0) {
    switchFile(idx)
    nextTick(() => {
      editorRef.value?.gotoLine(line)
      editorRef.value?.setCurrentLine?.(line)
    })
  } else if (projectPath.value) {
    // 尝试从项目目录打开
    const candidates = [
      projectPath.value + '/' + fileName,
      projectPath.value + '/src/' + fileName
    ]
    for (const p of candidates) {
      try {
        await openProjectFile(p)
        nextTick(() => {
          editorRef.value?.gotoLine(line)
          editorRef.value?.setCurrentLine?.(line)
        })
        return
      } catch {}
    }
  }
}

// onDebugLog: 调试输出转发到"输出"tab（已有 watch(output) 自动滚动）
function onDebugLog(line) {
  output.value += line + '\n'
  // 调试输出总是切到输出tab（不切到调试tab，避免打断用户查看变量）
  if (outputTabName.value !== 'debug' && outputTabName.value !== 'output') {
    outputTabName.value = 'output'
  }
}

// onToggleBreakpoint: 编辑器 F9/Shift+点击 → 通知 DebugPanel
function onToggleBreakpoint(line) {
  const f = activeFile.value
  if (!f) return
  const fileName = f.name
  // v0.9.13：先切换 editor 装饰，根据返回值决定 add/remove DebugPanel 列表
  const isAdded = editorRef.value?.toggleBreakpointLine?.(line)
  if (isAdded) {
    // addBreakpoint 内部会在 isDebugging 时调用 dlv，这里不重复调用
    debugPanelRef.value?.addBreakpoint?.(fileName, line)
  } else {
    debugPanelRef.value?.removeBreakpointByFileLine?.(fileName, line)
    // removeBreakpointByFileLine 只更新列表，dlv 调用在此统一处理
    if (isDebugging.value) {
      IDEService.DebugToggleBreakpoint(fileName, line).catch(() => {})
    }
  }
}

// onDebugStarted: 调试器启动成功 → 展开输出面板 + 切换到"调试"tab
function onDebugStarted() {
  outputCollapsed.value = false
  outputTabName.value = 'debug'
}


// 在文本中扫描 `<word>(` 的调用位置（排除定义行），返回 [{line, col}]
function scanReferencesInText(text, word) {
  if (!word) return []
  const escaped = word.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const callRe = new RegExp('(^|[^\\u4e00-\\u9fa5A-Za-z_])(' + escaped + ')\\s*\\(', 'g')
  const fnDefRe = new RegExp('^\\s*函数\\s+' + escaped + '\\s*\\(')
  const mthDefRe = new RegExp('^\\s*方法\\s+\\([^)]*\\)\\s*' + escaped + '\\s*\\(')
  const refs = []
  const linesArr = text.split('\n')
  for (let i = 0; i < linesArr.length; i++) {
    const line = linesArr[i]
    if (fnDefRe.test(line) || mthDefRe.test(line)) continue
    callRe.lastIndex = 0
    let m
    while ((m = callRe.exec(line)) !== null) {
      refs.push({ line: i + 1, col: m.index + m[1].length + 1 })
    }
  }
  return refs
}

// H1：从已有目录树收集所有 .eg 文件节点（跳过 assets/bin/libs/node_modules 等无关目录）。
// ListProjectDir 已递归返回整棵树，这里直接遍历 Children，不再逐层 RPC。
function collectEgFilesFromTree(tree) {
  const result = []
  const skipDirs = new Set(['assets', '资源', 'bin', 'build', 'dist', 'node_modules', '.git', 'libs'])
  const stack = Array.isArray(tree) ? tree.slice() : []
  while (stack.length) {
    const node = stack.shift()
    if (!node || !node.Path) continue
    if (node.IsDir) {
      const name = (node.Name || '').toLowerCase()
      if (skipDirs.has(name)) continue
      if (Array.isArray(node.Children)) for (const c of node.Children) stack.push(c)
      continue
    }
    if (node.Path.toLowerCase().endsWith('.eg')) result.push(node)
  }
  return result
}

// 遍历项目内所有 .eg 文件（排除当前文件），返回 [{relPath, refs}]
async function scanReferencesInProjectEg(word, excludePath) {
  if (!projectPath.value) return []
  const tree = await IDEService.ListProjectDir(projectPath.value)
  const result = []
  // H1：直接遍历已有树，不再逐层 RPC
  const egFiles = collectEgFilesFromTree(tree)
  for (const node of egFiles) {
    if (excludePath && node.Path === excludePath) continue
    let data
    try { data = await IDEService.ReadProjectFile(node.Path) } catch { continue }
    if (!data || data.error || typeof data.content !== 'string') continue
    const refs = scanReferencesInText(data.content, word)
    const srcLines = data.content.split('\n')
    const refsFull = refs.map(r => ({ ...r, preview: (srcLines[r.line - 1] || '').trim() }))
    const rel = node.Path.replace(projectPath.value, '').replace(/^[\\/]+/, '')
    result.push({ relPath: rel, absPath: node.Path, refs: refsFull })
  }
  return result
}

// 重命名符号（F2）：
// Editor.vue 已在当前文件同步 rename 完成（含定义行 + 调用点），
// 这里负责跨文件：项目内其它 .eg + .elib source.eg 中的所有调用点，
// 直接重写文件。备注：定义行只发生在当前文件或 .elib 中，跨文件 rename 只需处理调用点。
async function onRenameSymbol({ oldName, newName, modelUri }) {
  if (!oldName || !newName || oldName === newName) return
  let totalChanged = 0
  const escaped = oldName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const callRe = new RegExp('(^|[^\\u4e00-\\u9fa5A-Za-z_])(' + escaped + ')\\s*\\(', 'g')
  const fnDefRe = new RegExp('(^|[^\\u4e00-\\u9fa5A-Za-z_])(函数\\s+)(' + escaped + ')(\\s*\\()')
  const mthDefRe = new RegExp('(^|[^\\u4e00-\\u9fa5A-Za-z_])(方法\\s+\\([^)]*\\)\\s*)(' + escaped + ')(\\s*\\()')

  // 替换文本中所有调用点与定义行
  function replaceInText(text) {
    let changed = 0
    // 先处理定义行（前缀 + 函数 + 名字 + 括号）
    text = text.replace(fnDefRe, (m, pre, kw, name, paren) => {
      changed++
      return pre + kw + newName + paren
    })
    text = text.replace(mthDefRe, (m, pre, kw, name, paren) => {
      changed++
      return pre + kw + newName + paren
    })
    // 再处理调用点（不匹配函数/方法定义行）
    text = text.replace(callRe, (m, pre, name) => {
      changed++
      return pre + newName + '('
    })
    return { text, changed }
  }

  // 1) 项目内其它 .eg 文件
  if (projectPath.value) {
    const tree = await IDEService.ListProjectDir(projectPath.value)
    // H1：直接遍历已有树，不再逐层 RPC
    const egFiles = collectEgFilesFromTree(tree)
    for (const node of egFiles) {
      // 跳过当前文件（已被 Editor 同步 rename）
      const isActive = activeFile.value && node.Path === activeFile.value.path
      let data
      try { data = await IDEService.ReadProjectFile(node.Path) } catch { continue }
      if (!data || data.error || typeof data.content !== 'string') continue
      const { text, changed } = replaceInText(data.content)
      if (changed > 0) {
        // 当前打开的文件直接更新内存（QuickSave 会落盘）
        if (isActive) {
          activeFile.value.source = text
          activeFile.value.savedSource = text
        } else {
          await IDEService.QuickSave(node.Path, text)
        }
        totalChanged += changed
      }
    }
  }

  // 2) 项目 .elib source.eg
  const libs = getProjectLibsSummary()
  for (const lib of libs) {
    const sep = lib.path.includes('\\') ? '\\' : '/'
    const srcPath = lib.path + sep + 'source.eg'
    let data
    try { data = await IDEService.ReadProjectFile(srcPath) } catch { continue }
    if (!data || data.error || typeof data.content !== 'string') continue
    const { text, changed } = replaceInText(data.content)
    if (changed > 0) {
      await IDEService.QuickSave(srcPath, text)
      totalChanged += changed
      // 如果当前编辑器正好打开了这个 source.eg，同步内存
      if (activeFile.value && activeFile.value.path === srcPath) {
        activeFile.value.source = text
        activeFile.value.savedSource = text
      }
    }
  }

  if (totalChanged > 0) {
    setStatusMsg(`已重命名 ${oldName} → ${newName}（${totalChanged} 处）`, 3000)
    await maybeRefreshProjectLibs(activeFile.value?.path)
  }
}

async function transpileCode() {
  clearErrorMarkers()
  output.value = ''
  errorOutput.value = ''
  tipOutput.value = '转译中...'
  outputTabName.value = 'tips'
  setStatusMsg('转译中...', 0)
  try {
    const data = await IDEService.Transpile(activeFile.value.source)
    if (data.error) {
      tipOutput.value = ''
      errorOutput.value = data.error
      setStatusMsg('转译失败', 4000)
      const line = extractErrorLine(data.error)
      if (line) gotoError(line, data.error)
    } else {
      tipOutput.value = '转译成功'
      errorOutput.value = ''
      setStatusMsg('转译成功', 2000)
    }
  } catch (e) {
    tipOutput.value = ''
    errorOutput.value = '调用失败: ' + e.message
    setStatusMsg('调用失败', 4000)
  }
}

async function saveAllFiles() {
  let savedCount = 0
  let skippedNoPath = 0
  // M2：收集需要保存的文件，用 Promise.all 并行保存，避免串行 await 叠加延迟
  const tasks = []
  for (const file of files.value) {
    const content = getFileSaveContent(file)
    if (content === file.savedSource) continue
    if (!file.path) {
      skippedNoPath++
      continue
    }
    tasks.push(
      IDEService.QuickSave(file.path, content).then(data => {
        if (!data.error) {
          file.savedSource = content
          return true
        }
        return false
      }).catch(() => false)
    )
  }
  if (tasks.length > 0) {
    const results = await Promise.all(tasks)
    savedCount = results.filter(ok => ok).length
  }
  if (savedCount > 0) {
    setStatusMsg(`已保存 ${savedCount} 个文件`, 1500)
  }
  if (skippedNoPath > 0) {
    // 有未指定路径的新文件，输出面板提示用户先另存为
    output.value += `[提示] ${skippedNoPath} 个新建文件未保存路径，请使用「另存为」保存后再编译\n`
  }
}

async function runCode() {
  clearErrorMarkers()
  await saveAllFiles()
  output.value = '[1/5] 准备编译运行…\n'
  errorOutput.value = ''
  tipOutput.value = ''
  setStatusMsg('运行中…', 0)
  outputTabName.value = 'output'
  buildActive.value = true
  buildStep.value = 'prepare'
  buildPercent.value = 0
  const offEvent = Events.On('ide:run-event', (ev) => {
    const data = ev?.data || {}
    const stage = data.stage || 'run'
    const text = data.output || ''
    const isOutput = !!data.isOutput
    if (handleBuildProgress(stage, text)) return
    if (stage === 'error') {
      errorOutput.value = (errorOutput.value ? errorOutput.value + '\n' : '') + text
      outputTabName.value = 'errors'
      outputCollapsed.value = false
      setStatusMsg('运行失败', 5000)
      const line = extractErrorLine(text)
      if (line) gotoError(line, text)
      return
    }
    if (stage === 'done') {
      setStatusMsg('运行完成', 3000)
      return
    }
    let prefix = ''
    switch (stage) {
      case 'transpile': prefix = '[转译] '; break
      case 'stage':     prefix = '[准备] '; break
      case 'frontend':  prefix = '[前端] '; break
      case 'build':     prefix = '[编译] '; break
      case 'run':       prefix = isOutput ? '[输出] ' : '[运行] '; break
      default:          prefix = `[${stage}] `
    }
    output.value += prefix + text + '\n'
    outputAutoScroll.value = true
  })
  try {
    const data = await IDEService.RunProject(projectPath.value)
    if (data.error) {
      buildActive.value = false
      buildProgressRef.value?.reset()
      errorOutput.value = (errorOutput.value ? errorOutput.value + '\n' : '') + data.error
      outputTabName.value = 'errors'
      outputCollapsed.value = false
      setStatusMsg('运行失败', 5000)
      const line = extractErrorLine(data.error)
      if (line) gotoError(line, data.error)
    } else if (data.output) {
      output.value += data.output + '\n'
    }
  } catch (e) {
    buildActive.value = false
    buildProgressRef.value?.reset()
    errorOutput.value = (errorOutput.value ? errorOutput.value + '\n' : '') + '调用失败: ' + e.message
    outputTabName.value = 'errors'
    outputCollapsed.value = false
    setStatusMsg('调用失败', 5000)
  } finally {
    offEvent && offEvent()
  }
}

function createProject() {
  createProjectVisible.value = true
  createProjectParent.value = ''
  createProjectName.value = '未命名项目'
  createProjectTemplate.value = 'builtin:window'
}

function showNewWindowDialog() {
  if (!projectOpen.value || !projectPath.value) {
    output.value = '请先打开或创建一个项目'
    setStatusMsg('未打开项目', 2500)
    return
  }
  newWindowVisible.value = true
  newWindowName.value = '窗口' + (files.value.filter(f => isFormFileName(f.name)).length + 1)
}

async function confirmNewWindow() {
  const name = newWindowName.value.trim() || '窗口1'
  const safeName = name.replace(/[\\/:*?"<>|]/g, '_')
  const formName = safeName.endsWith('.ew') ? safeName : safeName + '.ew'
  const codeName = formName.replace(/\.ew$/i, '.eg')
  const srcDir = projectPath.value + '\\src'
  const formPath = srcDir + '\\' + formName
  const codePath = srcDir + '\\' + codeName

  const design = { form: { title: name, width: 538, height: 350, bgColor: '#f0f0f0' }, components: [] }
  const codeSource = `# 程序集 main

函数 主函数()
结束函数
`

  try {
    const formRes = await IDEService.QuickSave(formPath, JSON.stringify(design, null, 2))
    if (formRes.error) throw new Error(formRes.error)
    const codeRes = await IDEService.QuickSave(codePath, codeSource)
    if (codeRes.error) throw new Error(codeRes.error)

    newWindowVisible.value = false
    await loadProjectTree(projectPath.value)

    const formFile = makeFile(formName, JSON.stringify(design, null, 2), formPath)
    formFile.design = design
    files.value.push(formFile)
    const codeFile = makeFile(codeName, codeSource, codePath)
    files.value.push(codeFile)
    const formIndex = files.value.length - 2
    const codeIndex = files.value.length - 1
    switchFile(formIndex)
    pushFileOp({
      type: 'add-window',
      formFile: cloneFile(formFile),
      codeFile: cloneFile(codeFile),
      formIndex,
      codeIndex,
      formPath,
      codePath
    })
    setStatusMsg('已创建窗口: ' + formName, 2500)
  } catch (e) {
    output.value = '创建窗口失败: ' + e.message
    setStatusMsg('创建失败', 3000)
  }
}

async function selectProjectParent() {
  try {
    const path = await IDEService.OpenProject()
    if (path) createProjectParent.value = path
  } catch (e) {}
}

async function confirmCreateProject() {
  const parent = createProjectParent.value
  const name = createProjectName.value.trim() || '未命名项目'
  if (!parent) {
    output.value = '请先选择项目存放目录'
    setStatusMsg('创建失败', 3000)
    return
  }
  try {
    const tmpl = createProjectTemplate.value
    let err
    if (tmpl.startsWith('global:')) {
      // 全局模板：用 CreateProjectFromTemplate 从模板目录复制
      const tmplDir = tmpl.slice('global:'.length)
      err = await IDEService.CreateProjectFromTemplate(tmplDir, parent, name)
    } else {
      // 内置模板：用 CreateProject 按 template key 生成
      const builtinKey = tmpl.startsWith('builtin:') ? tmpl.slice('builtin:'.length) : tmpl
      err = await IDEService.CreateProject(parent, name, builtinKey)
    }
    if (err) {
      output.value = '创建项目失败: ' + err
      setStatusMsg('创建失败', 3000)
      return
    }
    const fullPath = parent + '\\' + name
    createProjectVisible.value = false
    await openProjectAt(fullPath)
    setStatusMsg('项目已创建', 2500)
  } catch (e) {
    output.value = '创建项目失败: ' + e.message
    setStatusMsg('创建失败', 4000)
  }
}

async function loadProjectTree(path) {
  try {
    projectTree.value = await IDEService.ListProjectDir(path) || []
  } catch (e) {
    projectTree.value = []
  }
}

function findFileIndex(name) {
  return files.value.findIndex(f => f.name === name)
}

function fileNameFromPath(path) {
  const parts = path.split(/[\\/]/)
  return parts[parts.length - 1] || path
}

function parseDesignSource(content) {
  try {
    const parsed = JSON.parse(content)
    if (parsed && parsed.form) {
      // 确保 components 字段存在，避免 WindowDesigner 的 layersList computed
      // 对 undefined 调用 slice() 导致渲染崩溃
      if (!Array.isArray(parsed.components)) parsed.components = []
      // 用默认 form 补全缺失字段，避免 formSurfaceStyle/formClientStyle 生成 'undefinedpx'
      parsed.form = { ...defaultFormDesign().form, ...parsed.form }
      return parsed
    }
  } catch {}
  return null
}

async function openProjectFile(path) {
  try {
    const data = await IDEService.ReadProjectFile(path)
    if (data.error) {
      output.value = '读取文件失败: ' + data.error
      return
    }
    const name = data.name || fileNameFromPath(path)
    const idx = findFileIndex(name)
    if (idx >= 0) {
      files.value[idx].source = data.content
      files.value[idx].savedSource = data.content
      files.value[idx].path = path
      if (isFormFileName(name) && files.value[idx].design) {
        const design = parseDesignSource(data.content)
        if (design) files.value[idx].design = design
      }
      switchFile(idx)
    } else {
      const file = makeFile(name, data.content, path)
      if (isFormFileName(name) && file.design) {
        const design = parseDesignSource(data.content)
        if (design) file.design = design
      }
      files.value.push(file)
      switchFile(files.value.length - 1)
    }
    // 打开窗口设计文件时，自动加载同名的代码文件，避免点击事件时创建空文件导致代码丢失
    if (isFormFileName(name)) {
      const codePath = path.replace(/\.ew$/i, '.eg')
      const codeName = fileNameFromPath(codePath)
      if (findFileIndex(codeName) === -1) {
        try {
          const codeData = await IDEService.ReadProjectFile(codePath)
          if (!codeData.error && typeof codeData.content === 'string') {
            files.value.push(makeFile(codeName, codeData.content, codePath))
          }
        } catch {}
      }
    }
    setStatusMsg('已打开 ' + name, 2000)
  } catch (e) {
    output.value = '打开文件失败: ' + e.message
  }
}

function closeProject() {
  projectOpen.value = false
  projectPath.value = ''
  projectTree.value = []
  projectConfig.value = null
  files.value = [makeFile('main.eg', DEFAULT_SOURCE)]
  activeFileIndex.value = 0
  selectedFunctionIndex.value = null
  currentFunctionName.value = null
  output.value = ''
  errorOutput.value = ''
  tipOutput.value = ''
  refsResults.value = []
  refsQuery.value = ''
  setStatusMsg('已关闭项目', 2000)
}

async function openProjectAt(path) {
  projectPath.value = path
  projectOpen.value = true
  try {
    output.value = '已打开项目目录: ' + path
    setStatusMsg('项目已打开', 2000)
    addRecentProject(path)
    projectConfig.value = await IDEService.ReadProjectConfig(path)
    await loadProjectTree(path)

    // 扫描项目内的 .elib 用户扩展包，写入支持库面板 + Monaco 补全。
    // 找不到 libs/ 也属于正常情况（用户没写扩展），不影响主流程。
    try {
      const r = await loadProjectLibs(path)
      if (r && r.count > 0) {
        const names = getProjectLibsSummary().map(s => s.displayName || s.name).join('、')
        output.value = `已加载 ${r.count} 个扩展包：${names}`
      }
    } catch (e) {
      console.warn('[lib] 加载项目扩展包失败:', e)
    }

    const mainCandidates = []
    const entry = projectConfig.value?.entry
    if (entry) {
      mainCandidates.push(path + '\\' + entry.replace(/\//g, '\\'))
      mainCandidates.push(path + '/' + entry)
    }
    mainCandidates.push(
      path + '\\src\\main.eg',
      path + '/src/main.eg',
      path + '\\main.eg',
      path + '/main.eg'
    )
    for (const candidate of mainCandidates) {
      try {
        const data = await IDEService.ReadProjectFile(candidate)
        if (!data.error && data.content) {
          const name = data.name || 'main.eg'
          const file = makeFile(name, data.content, candidate)
          // .ew 主文件：从内容解析 design，避免属性面板显示默认值而非已保存值
          if (isFormFileName(name) && file.design) {
            const design = parseDesignSource(data.content)
            if (design) file.design = design
          }
          files.value = [file]
          activeFileIndex.value = 0
          selectedFunctionIndex.value = null
          output.value = '已打开项目: ' + path
          break
        }
      } catch {}
    }
  } catch (e) {
    // F7：加载失败时回滚 projectOpen/projectPath，避免半开状态导致 UI 误判
    projectOpen.value = false
    projectPath.value = ''
    output.value = '打开项目失败: ' + e.message
    setStatusMsg('打开项目失败', 3000)
  }
}

async function openProject() {
  try {
    const path = await IDEService.OpenProject()
    if (!path) return
    await openProjectAt(path)
  } catch (e) {
    output.value = '打开项目失败: ' + e.message
  }
}

async function openRecent(item) {
  await openProjectAt(item.path)
}

function findFileIndexByName(name) {
  return files.value.findIndex(f => f.name === name)
}

async function performUndo(op) {
  switch (op.type) {
    case 'add': {
      const idx = findFileIndexByName(op.file.name)
      if (idx >= 0) {
        files.value.splice(idx, 1)
        if (activeFileIndex.value >= idx && files.value.length > 0) {
          activeFileIndex.value = Math.min(activeFileIndex.value, files.value.length - 1)
        }
        if (files.value.length === 0) {
          files.value.push(makeFile('main.eg', DEFAULT_SOURCE))
          activeFileIndex.value = 0
        }
      }
      setStatusMsg('已撤销新建文件: ' + op.file.name, 2000)
      break
    }
    case 'add-window': {
      const formIdx = findFileIndexByName(op.formFile.name)
      const codeIdx = findFileIndexByName(op.codeFile.name)
      const toRemove = []
      if (codeIdx >= 0) toRemove.push(codeIdx)
      if (formIdx >= 0) toRemove.push(formIdx)
      toRemove.sort((a, b) => b - a)
      for (const i of toRemove) files.value.splice(i, 1)
      try {
        if (op.formPath) await IDEService.DeleteFile(op.formPath)
        if (op.codePath) await IDEService.DeleteFile(op.codePath)
      } catch (e) {
        console.warn('撤销新建窗口时删除磁盘文件失败:', e)
      }
      if (projectPath.value) await loadProjectTree(projectPath.value)
      if (files.value.length === 0) {
        files.value.push(makeFile('main.eg', DEFAULT_SOURCE))
        activeFileIndex.value = 0
      }
      setStatusMsg('已撤销新建窗口: ' + op.formFile.name, 2000)
      break
    }
    case 'delete': {
      const restored = cloneFile(op.formFile)
      if (op.formWasOnDisk && op.formPath) {
        try {
          await IDEService.QuickSave(op.formPath, op.formDiskContent || restored.source)
          restored.savedSource = op.formDiskContent || restored.source
        } catch (e) {
          console.warn('撤销删除时写回磁盘失败:', e)
        }
      }
      files.value.push(restored)
      activeFileIndex.value = files.value.length - 1
      if (projectPath.value && op.formWasOnDisk) await loadProjectTree(projectPath.value)
      setStatusMsg('已恢复文件: ' + op.formFile.name, 2000)
      break
    }
    case 'delete-window': {
      if (op.codeFile) {
        const restoredCode = cloneFile(op.codeFile)
        if (op.codeWasOnDisk && op.codePath) {
          try {
            await IDEService.QuickSave(op.codePath, op.codeDiskContent || restoredCode.source)
            restoredCode.savedSource = op.codeDiskContent || restoredCode.source
          } catch (e) {
            console.warn('撤销删除窗口时写回代码文件失败:', e)
          }
        }
        files.value.push(restoredCode)
      }
      const restoredForm = cloneFile(op.formFile)
      if (op.formWasOnDisk && op.formPath) {
        try {
          await IDEService.QuickSave(op.formPath, op.formDiskContent || restoredForm.source)
          restoredForm.savedSource = op.formDiskContent || restoredForm.source
        } catch (e) {
          console.warn('撤销删除窗口时写回窗口文件失败:', e)
        }
      }
      files.value.push(restoredForm)
      activeFileIndex.value = files.value.length - 1
      if (projectPath.value && (op.formWasOnDisk || op.codeWasOnDisk)) {
        await loadProjectTree(projectPath.value)
      }
      setStatusMsg('已恢复窗口: ' + op.formFile.name, 2000)
      break
    }
  }
  selectedFunctionIndex.value = null
}

async function performRedo(op) {
  switch (op.type) {
    case 'add': {
      const restored = cloneFile(op.file)
      files.value.push(restored)
      activeFileIndex.value = files.value.length - 1
      setStatusMsg('已重做新建文件: ' + op.file.name, 2000)
      break
    }
    case 'add-window': {
      if (op.formPath) {
        try {
          await IDEService.QuickSave(op.formPath, JSON.stringify(op.formFile.design, null, 2))
          await IDEService.QuickSave(op.codePath, op.codeFile.source)
        } catch (e) {
          console.warn('重做新建窗口时写回磁盘失败:', e)
        }
      }
      files.value.push(cloneFile(op.formFile))
      files.value.push(cloneFile(op.codeFile))
      activeFileIndex.value = files.value.length - 2
      if (projectPath.value) await loadProjectTree(projectPath.value)
      setStatusMsg('已重做新建窗口: ' + op.formFile.name, 2000)
      break
    }
    case 'delete': {
      const idx = findFileIndexByName(op.formFile.name)
      if (idx >= 0) {
        files.value.splice(idx, 1)
        if (op.formWasOnDisk && op.formPath) {
          try { await IDEService.DeleteFile(op.formPath) } catch (e) { console.warn(e) }
        }
        if (files.value.length === 0) {
          files.value.push(makeFile('main.eg', DEFAULT_SOURCE))
          activeFileIndex.value = 0
        } else if (activeFileIndex.value >= idx) {
          activeFileIndex.value = Math.max(0, activeFileIndex.value - 1)
        }
      }
      if (projectPath.value && op.formWasOnDisk) await loadProjectTree(projectPath.value)
      setStatusMsg('已重做删除: ' + op.formFile.name, 2000)
      break
    }
    case 'delete-window': {
      const formIdx = findFileIndexByName(op.formFile.name)
      const codeIdx = op.codeFile ? findFileIndexByName(op.codeFile.name) : -1
      const toRemove = []
      if (codeIdx >= 0) toRemove.push(codeIdx)
      if (formIdx >= 0) toRemove.push(formIdx)
      toRemove.sort((a, b) => b - a)
      for (const i of toRemove) files.value.splice(i, 1)
      if (op.formWasOnDisk && op.formPath) {
        try { await IDEService.DeleteFile(op.formPath) } catch (e) { console.warn(e) }
      }
      if (op.codeWasOnDisk && op.codePath) {
        try { await IDEService.DeleteFile(op.codePath) } catch (e) { console.warn(e) }
      }
      if (files.value.length === 0) {
        files.value.push(makeFile('main.eg', DEFAULT_SOURCE))
        activeFileIndex.value = 0
      }
      if (projectPath.value && (op.formWasOnDisk || op.codeWasOnDisk)) {
        await loadProjectTree(projectPath.value)
      }
      setStatusMsg('已重做删除窗口: ' + op.formFile.name, 2000)
      break
    }
  }
  selectedFunctionIndex.value = null
}

async function undo() {
  if (fileUndoStack.value.length === 0) {
    setStatusMsg('没有可撤销的文件操作', 2000)
    return
  }
  const op = fileUndoStack.value.pop()
  await performUndo(op)
  fileRedoStack.value.push(op)
}

async function redo() {
  if (fileRedoStack.value.length === 0) {
    setStatusMsg('没有可重做的文件操作', 2000)
    return
  }
  const op = fileRedoStack.value.pop()
  await performRedo(op)
  fileUndoStack.value.push(op)
}
// v0.9.5：TitleBar 调试按钮连接到真实调试器功能（之前是 STUB）
// 调试中 → 继续执行（F5 行为）；非调试中 → 开始调试
function debugCode() {
  if (isDebugging.value) {
    debugPanelRef.value?.continueDebug?.()
  } else {
    // 先展开输出面板并切到"调试"tab，让用户看到编译进度和反馈
    outputCollapsed.value = false
    outputTabName.value = 'debug'
    debugPanelRef.value?.startDebug?.()
  }
}
function onBuild(key) {
  if (key === 'build-all') {
    buildExecutable()
  } else if (key === 'build-options') {
    openBuildOptions()
  }
}

async function buildExecutable() {
  clearErrorMarkers()
  await saveAllFiles()
  const mode = buildConfig.value.mode
  output.value = `[1/5] 准备构建可执行文件（${mode}）…\n`
  errorOutput.value = ''
  setStatusMsg('构建中…', 0)
  outputTabName.value = 'output'
  buildActive.value = true
  buildStep.value = 'prepare'
  buildPercent.value = 0
  let artifactPath = ''
  const offEvent = Events.On('ide:run-event', (ev) => {
    const data = ev?.data || {}
    const stage = data.stage || 'build'
    const text = data.output || ''
    if (handleBuildProgress(stage, text)) return
    if (stage === 'error') {
      errorOutput.value = (errorOutput.value ? errorOutput.value + '\n' : '') + text
      outputTabName.value = 'errors'
      outputCollapsed.value = false
      setStatusMsg('构建失败', 4000)
      const line = extractErrorLine(text)
      if (line) gotoError(line, text)
      return
    }
    if (stage === 'artifact') {
      artifactPath = text
      return
    }
    if (stage === 'done') {
      setStatusMsg('构建完成', 3000)
      if (projectPath.value) {
        IDEService.ReadProjectConfig(projectPath.value).then(cfg => { projectConfig.value = cfg })
        if (buildConfig.value.autoOpenFolder) {
          IDEService.OpenInExplorer(projectPath.value)
        }
      }
      if (artifactPath) {
        checkSignatureAndReport(artifactPath)
        navigator.clipboard?.writeText(artifactPath).catch(() => {})
        output.value += `[产物] ${artifactPath}（路径已复制到剪贴板）\n`
        addBuildHistory({
          time: new Date().toLocaleString('zh-CN'),
          version: projectConfig.value?.version || '',
          mode: buildConfig.value.mode,
          artifact: artifactPath
        })
      }
      return
    }
    let prefix = ''
    switch (stage) {
      case 'transpile': prefix = '[转译] '; break
      case 'stage':     prefix = '[准备] '; break
      case 'frontend':  prefix = '[前端] '; break
      case 'build':     prefix = '[编译] '; break
      case 'run':       prefix = '[运行] '; break
      default:          prefix = `[${stage}] `
    }
    output.value += prefix + text + '\n'
  })
  try {
    if (!projectPath.value) {
      buildActive.value = false
      buildProgressRef.value?.reset()
      errorOutput.value = '未打开项目'
      setStatusMsg('构建失败', 4000)
      return
    }
    const entry = projectConfig.value?.entry || 'main.eg'
    const mainPath = projectPath.value + '\\' + entry
    let data
    if (mode === 'release') {
      data = await IDEService.BuildProjectRelease(projectPath.value)
    } else {
      data = await IDEService.BuildProject(mainPath, projectPath.value)
    }
    if (data.error) {
      buildActive.value = false
      buildProgressRef.value?.reset()
      errorOutput.value = (errorOutput.value ? errorOutput.value + '\n' : '') + data.error
      outputTabName.value = 'errors'
      outputCollapsed.value = false
      setStatusMsg('构建失败', 5000)
      const line = extractErrorLine(data.error)
      if (line) gotoError(line, data.error)
    } else if (data.output) {
      output.value += data.output + '\n'
    }
  } catch (e) {
    buildActive.value = false
    buildProgressRef.value?.reset()
    errorOutput.value = (errorOutput.value ? errorOutput.value + '\n' : '') + '调用失败: ' + e.message
    outputTabName.value = 'errors'
    outputCollapsed.value = false
    setStatusMsg('构建失败', 5000)
  } finally {
    offEvent && offEvent()
  }
}
function showAbout() {
  output.value = '易狗 IDE (EGOU) - 类易语言中文 Go IDE'
  outputTabName.value = 'output'
}
function toggleTheme() {
  const names = getThemeNames()
  const idx = names.indexOf(currentThemeName.value)
  currentThemeName.value = names[(idx + 1) % names.length]
}
function showSettings() {
  settingsActiveMenu.value = ''
  settingsVisible.value = true
}
function openAISettings() {
  settingsActiveMenu.value = 'ai'
  settingsVisible.value = true
}

function getCurrentFileForAI() {
  if (activeFileIndex.value < 0 || !openFiles.value[activeFileIndex.value]) return null
  const f = openFiles.value[activeFileIndex.value]
  const content = editorRef.value?.getValue?.() || f.source || ''
  return { name: f.name || 'untitled.eg', content }
}

function addAIModel(model) {
  const id = 'm_' + Date.now()
  aiModels.value.push({ id, ...model })
  activeModelId.value = id
}

function updateAIModel(id, updates) {
  const idx = aiModels.value.findIndex(m => m.id === id)
  if (idx >= 0) Object.assign(aiModels.value[idx], updates)
}

function deleteAIModel(id) {
  if (aiModels.value.length <= 1) return
  const idx = aiModels.value.findIndex(m => m.id === id)
  if (idx >= 0) {
    aiModels.value.splice(idx, 1)
    if (activeModelId.value === id && aiModels.value.length > 0) {
      activeModelId.value = aiModels.value[0].id
    }
  }
}

function switchAIModel(id) {
  activeModelId.value = id
}

function onShowHelp(info) {
  tipOutput.value = `[${info.name}] ${info.help}`
}
</script>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--bg-primary);
  backdrop-filter: blur(20px) saturate(1.3);
  -webkit-backdrop-filter: blur(20px) saturate(1.3);
  color: var(--text-primary);
}
.main-container {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}
.workspace {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.workspace-body {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}
.left-panel {
  flex-shrink: 0;
  background: var(--bg-sidebar);
  backdrop-filter: blur(16px) saturate(1.2);
  -webkit-backdrop-filter: blur(16px) saturate(1.2);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}
.right-panel {
  flex-shrink: 0;
  background: var(--bg-secondary);
  overflow: hidden;
  border-left: 1px solid var(--border-color);
}
.editor-main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
.editor-code-body {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}
.designer-full {
  flex: 1;
  min-height: 0;
}
/* G7：插件自定义面板容器，占满左侧面板区域 */
.plugin-panel-container {
  width: 100%;
  height: 100%;
  min-height: 200px;
  padding: 8px;
  overflow: auto;
  color: var(--text-primary);
  font-size: var(--ide-font-size);
}
.plugin-panel-container input,
.plugin-panel-container button,
.plugin-panel-container textarea {
  font-family: inherit;
}
.editor-stack {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 8px;
  overflow: hidden;
}
.splitter {
  flex-shrink: 0;
  position: relative;
  background: transparent;
  transition: background 0.15s;
  z-index: 10;
}
.splitter-v {
  width: 0px;
  cursor: ew-resize;
}
.splitter-v::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: -4px;
  right: -4px;
}
.splitter:hover,
.splitter:active {
  background: var(--accent-color);
  opacity: 0.7;
}
.output-panel {
  flex-shrink: 0;
  position: relative;
  display: flex;
  flex-direction: column;
  background: var(--bg-output);
  backdrop-filter: blur(16px) saturate(1.2);
  -webkit-backdrop-filter: blur(16px) saturate(1.2);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 8px;
  padding-top: 6px;
}
.splitter-h {
  position: absolute;
  top: -5px;
  left: 8px;
  right: 8px;
  height: 7px;
  cursor: ns-resize;
  z-index: 20;
}
.output-toolbar {
  display: flex;
  align-items: stretch;
  flex: 1;
  min-height: 0;
  gap: 4px;
  overflow: hidden;
}
.output-actions {
  display: flex;
  flex-direction: column;
  gap: 2px;
  justify-content: flex-start;
  padding-top: 2px;
}
.output-pre {
  margin: 0;
  height: 100%;
  overflow: auto;
  font-family: var(--ide-font), monospace;
  font-size: var(--ide-font-size);
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text-primary);
}
/* 确保 n-tabs 内部 flex 容器正确传递高度约束，避免 tab 内容溢出面板 */
.output-toolbar .n-tabs {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}
.output-toolbar .n-tabs .n-tabs-content {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.output-toolbar .n-tabs .n-tab-pane {
  height: 100%;
  min-height: 0;
  overflow: hidden;
}
.error-text {
  color: var(--color-error);
}
.error-line-clickable {
  cursor: pointer;
  color: var(--text-primary);
}
.error-line-clickable:hover {
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  border-radius: 3px;
}
.refs-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  height: 100%;
  overflow: auto;
  min-height: 0;
}
.refs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 6px;
  font-weight: 600;
  border-bottom: 1px solid var(--border-color);
  margin-bottom: 4px;
  position: sticky;
  top: 0;
  background: var(--bg-secondary);
  z-index: 1;
}
.refs-header-actions {
  display: inline-flex;
  gap: 2px;
}
.ref-file {
  margin-bottom: 2px;
  border-radius: 4px;
  overflow: hidden;
}
.ref-file-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  cursor: pointer;
  background: color-mix(in srgb, var(--accent-color) 6%, transparent);
  border-radius: 4px;
  user-select: none;
  transition: background 0.15s;
}
.ref-file-header:hover {
  background: color-mix(in srgb, var(--accent-color) 14%, transparent);
}
.ref-file-twisty {
  display: inline-block;
  font-size: 10px;
  color: var(--text-secondary);
  transition: transform 0.15s;
  transform: rotate(90deg);
  width: 10px;
  text-align: center;
  flex-shrink: 0;
}
.ref-file-twisty.collapsed {
  transform: rotate(0deg);
}
.ref-file-name {
  font-size: var(--ide-font-size-sm);
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--ide-font), monospace;
}
.ref-file-count {
  display: inline-block;
  min-width: 18px;
  padding: 0 6px;
  height: 16px;
  line-height: 16px;
  text-align: center;
  font-size: var(--ide-font-size-xs);
  color: var(--accent-color);
  background: color-mix(in srgb, var(--accent-color) 14%, transparent);
  border-radius: 8px;
  flex-shrink: 0;
}
.ref-file-items {
  display: flex;
  flex-direction: column;
  gap: 1px;
  padding: 2px 0 2px 20px;
}
.ref-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 3px 6px;
  cursor: pointer;
  border-radius: 3px;
  transition: background 0.15s;
}
.ref-item:hover {
  background: color-mix(in srgb, var(--accent-color) 12%, transparent);
}
.ref-line-no {
  flex-shrink: 0;
  min-width: 36px;
  font-size: var(--ide-font-size-xs);
  color: var(--accent-color);
  font-family: var(--ide-font), monospace;
  text-align: right;
  padding-top: 1px;
}
.ref-preview {
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-word;
  flex: 1;
  min-width: 0;
}
.tabs-bar-wrapper {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 0 4px;
  min-width: 0;
}
.file-tabs {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 4px;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: thin;
}
.file-tabs::-webkit-scrollbar {
  height: 3px;
}
.file-tabs::-webkit-scrollbar-thumb {
  background: var(--border-color);
  border-radius: 2px;
}
.file-tabs::-webkit-scrollbar-track {
  background: transparent;
}
.file-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 6px;
  background: var(--bg-tertiary);
  cursor: pointer;
  user-select: none;
  flex-shrink: 0;
  white-space: nowrap;
  font-size: var(--ide-font-size-sm);
  transition: background 0.2s;
}
.file-tab.active {
  background: var(--accent-bg);
}
.tab-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-muted);
  flex-shrink: 0;
  transition: background 0.15s;
}
.tab-dot.dirty {
  background: var(--color-warning);
  width: 8px;
  height: 8px;
}
.file-name {
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: font-style 0.15s;
}
.file-name.file-dirty {
  font-style: italic;
  font-weight: 600;
}
.file-tab.drag-over {
  border-left: 2px solid var(--accent-color);
}
.file-tab.pinned {
  border-left: 2px solid var(--accent-color);
  background: color-mix(in srgb, var(--accent-color) 8%, var(--bg-tertiary));
}
.tab-pin-icon {
  font-size: 10px;
  flex-shrink: 0;
  opacity: 0.8;
}
.close-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  font-size: var(--ide-font-size-sm);
  line-height: 1;
  opacity: 0;
  transition: opacity 0.15s, background 0.15s;
  flex-shrink: 0;
}
.file-tab:hover .close-btn,
.file-tab.active .close-btn {
  opacity: 1;
}
.close-btn:hover {
  background: color-mix(in srgb, var(--color-error) 25%, transparent);
}
.new-tab-btn {
  color: var(--text-secondary);
  flex-shrink: 0;
}
.editor-area {
  flex: 1;
  min-height: 0;
  background: var(--bg-tertiary);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
  box-shadow: var(--shadow-1);
  display: flex;
  flex-direction: column;
}
/* 编译选项对话框分组样式 */
.cfg-section {
  font-size: var(--ide-font-size);
  font-weight: 600;
  color: var(--text-primary);
  padding: 8px 0 4px;
  margin-top: 4px;
  border-bottom: 1px solid var(--border-color);
}
.cfg-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 12px;
}
.cfg-grid .n-form-item {
  grid-column: span 1;
}
/* 单项（如备注）跨两列 */
.cfg-grid .n-form-item--full {
  grid-column: span 2;
}
.view-tabs {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 4px;
  padding: 0 8px;
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-secondary);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}
.view-tab {
  padding: 6px 16px;
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
  cursor: pointer;
  border-bottom: 2px solid transparent;
  border-radius: var(--radius-sm) var(--radius-sm) 0 0;
  transition: all 0.15s ease;
}
.view-tab:hover {
  color: var(--text-primary);
  background: var(--bg-tertiary);
}
.view-tab.active {
  color: var(--accent-color);
  border-bottom-color: var(--accent-color);
  font-weight: 600;
}
/* 设计/代码标签行内的工具控件 */
.view-tab-sep {
  width: 1px;
  height: 16px;
  background: var(--border-color);
  margin: 0 4px;
  flex-shrink: 0;
}
.view-tab-tool {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
  cursor: pointer;
  padding: 4px 6px;
  border-radius: 4px;
  white-space: nowrap;
}
.view-tab-tool:hover {
  background: var(--bg-tertiary);
}
.view-tab-select {
  font-size: var(--ide-font-size-sm);
  padding: 2px 4px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--bg-primary);
  color: var(--text-primary);
  outline: none;
  cursor: pointer;
}
.view-tab-btn {
  font-size: var(--ide-font-size-sm);
  padding: 2px 8px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--bg-primary);
  color: var(--text-secondary);
  cursor: pointer;
  white-space: nowrap;
}
.view-tab-btn:hover {
  color: var(--accent-color);
  border-color: var(--accent-color);
}
.breadcrumb-bar {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 4px;
  padding: 4px 12px;
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
  background: var(--bg-primary);
  border-bottom: 1px solid var(--border-color);
  min-height: 28px;
}
.bc-segment {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 300px;
}
.bc-clickable {
  cursor: pointer;
  transition: background 0.12s;
}
.bc-clickable:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}
.bc-file {
  color: var(--text-primary);
  font-weight: 500;
}
.bc-path {
  color: var(--text-secondary);
}
.bc-fn {
  color: var(--accent-color);
  cursor: pointer;
  transition: background 0.15s;
}
.bc-fn:hover {
  background: var(--bg-tertiary);
}
.bc-cursor {
  color: var(--text-muted);
  font-family: var(--ide-code-font);
  font-size: var(--ide-font-size-xs);
}
.bc-status {
  cursor: pointer;
}
.bc-badge {
  font-size: 10px;
  padding: 1px 6px;
  background: var(--accent-color);
  color: var(--bg-primary);
  border-radius: 3px;
  font-weight: 600;
  letter-spacing: 0.5px;
}
.bc-icon {
  font-size: var(--ide-font-size-sm);
  opacity: 0.8;
}
.bc-sep {
  color: var(--text-dim);
  font-size: var(--ide-font-size-lg);
  line-height: 1;
  user-select: none;
}
.bc-spacer {
  flex: 1;
}
.bc-nav-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  border-radius: 4px;
  cursor: pointer;
  font-size: var(--ide-font-size-lg);
  line-height: 1;
  transition: background 0.12s, color 0.12s;
}
.bc-nav-btn:hover:not(.disabled) {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}
.bc-nav-btn.disabled {
  color: var(--text-dim);
  cursor: default;
  opacity: 0.4;
}
.bc-nav-icon {
  font-weight: bold;
  font-family: var(--ide-code-font);
}
.status-bar {
  height: 26px;
  flex-shrink: 0;
  padding: 0 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--bg-tertiary);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-top: 1px solid var(--border-color);
  font-size: var(--ide-font-size-sm);
  overflow: hidden;
  white-space: nowrap;
}
.status-bar-left {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  overflow: hidden;
}
.status-bar-right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.status-bar-msg {
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 0 6px;
}
.sb-zen {
  color: var(--accent-color) !important;
}
.sb-debug {
  color: var(--accent-color);
  font-weight: 600;
  position: relative;
  padding-left: 14px;
}
.sb-debug::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 50%;
  transform: translateY(-50%);
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--accent-color);
  animation: sb-debug-pulse 1.2s ease-in-out infinite;
}
@keyframes sb-debug-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}
.sb-letter {
  font-size: var(--ide-font-size-xs);
  opacity: 0.7;
  margin-right: 3px;
}
.sb-toggle {
  width: 20px;
  text-align: center;
  padding: 2px 0;
  opacity: 0.6;
}
.sb-toggle.active {
  opacity: 1;
  color: var(--accent-color);
}
.status-bar-goto {
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
  cursor: pointer;
  padding: 2px 8px;
  border-radius: 4px;
  transition: background 0.15s;
}
.status-bar-goto:hover {
  background: var(--bg-hover);
  color: var(--accent-color);
}
.status-bar-item {
  font-size: var(--ide-font-size-sm);
  color: var(--text-muted);
  padding: 2px 6px;
  border-radius: 3px;
  cursor: default;
  user-select: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 180px;
}
.status-bar-clickable {
  cursor: pointer;
  transition: background 0.15s;
}
.status-bar-clickable:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-secondary);
}
.sym-icon-function {
  background: color-mix(in srgb, var(--accent-color) 15%, transparent) !important;
  color: var(--accent-color) !important;
}
.sym-icon-variable {
  background: color-mix(in srgb, var(--color-info, #179fff) 15%, transparent) !important;
  color: var(--color-info, #179fff) !important;
}
.sym-icon-constant {
  background: color-mix(in srgb, var(--color-warning) 15%, transparent) !important;
  color: var(--color-warning) !important;
}
.sym-kind-function { color: var(--accent-color) !important; }
.sym-kind-variable { color: var(--color-info, #179fff) !important; }
.sym-kind-constant { color: var(--color-warning) !important; }
.health-indicator {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 8px;
  font-size: var(--ide-font-size-sm);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}
.health-indicator:hover {
  border-color: var(--accent-color);
  color: var(--accent-color);
}
.health-indicator:active {
  transform: translateY(1px);
}
.health-indicator.health-dot-only {
  padding: 2px 6px;
  gap: 0;
  border-radius: 50%;
  width: 20px;
  height: 20px;
  justify-content: center;
  align-items: center;
}
.health-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  display: inline-block;
  background: var(--text-dim);
  flex-shrink: 0;
}
.health-dot.ok,
.health-indicator.ok .health-dot {
  background: var(--color-success);
  box-shadow: 0 0 4px color-mix(in srgb, var(--color-success) 50%, transparent);
}
.health-dot.bad,
.health-indicator.bad .health-dot {
  background: var(--color-error);
  box-shadow: 0 0 4px color-mix(in srgb, var(--color-error) 50%, transparent);
}
.health-indicator.ok .health-text { color: var(--color-success); }
.health-indicator.bad .health-text { color: var(--color-error); }
.health-detail {
  font-size: var(--ide-font-size-sm);
  line-height: 1.6;
}
.health-detail-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  margin-bottom: 8px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border-color);
}
.health-detail-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 2px 0;
}
.health-detail-label {
  color: var(--text-secondary);
  flex-shrink: 0;
}
.health-detail-value {
  font-family: var(--ide-font, monospace);
  text-align: right;
  word-break: break-all;
  margin-left: 12px;
}
.health-detail-value.ok { color: var(--color-success); }
.health-detail-value.bad { color: var(--color-error); }
.health-detail-path {
  margin-top: 8px;
  padding-top: 6px;
  border-top: 1px solid var(--border-color);
  color: var(--text-tertiary);
  font-size: var(--ide-font-size-xs);
  word-break: break-all;
}
.panel-header {
  padding: 10px 12px;
  font-size: var(--ide-font-size);
  font-weight: 600;
  border-bottom: 1px solid var(--border-color);
}

/* ===== 创建项目对话框 — 模板卡片选择 ===== */
.create-project-dialog {
  display: flex;
  flex-direction: column;
}
.template-section {
  margin-bottom: 4px;
}
.template-label {
  font-size: var(--ide-font-size);
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: 8px;
}
.template-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 8px;
}
.template-card {
  padding: 12px 8px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  background: var(--bg-tertiary);
  cursor: pointer;
  text-align: center;
  transition: all 0.15s ease;
  position: relative;
  overflow: hidden;
}
.template-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: transparent;
  border-radius: var(--radius-sm);
  transition: background 0.15s ease;
}
.template-card:hover {
  border-color: var(--accent-color);
  background: var(--bg-hover);
  transform: translateY(-1px);
  box-shadow: var(--shadow-1);
}
.template-card.active {
  border-color: var(--accent-color);
  background: var(--accent-bg);
}
.template-card.active::before {
  background: linear-gradient(90deg, transparent, var(--accent-color), transparent);
}
.template-icon {
  font-size: 28px;
  margin-bottom: 6px;
  line-height: 1;
}
.template-name {
  font-size: var(--ide-font-size);
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 2px;
}
.template-desc {
  font-size: var(--ide-font-size-xs);
  color: var(--text-muted);
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
.quick-open {
  display: flex;
  flex-direction: column;
}
.quick-open-input-wrap {
  border-bottom: 1px solid var(--border-color);
  padding: 8px 12px;
}
.quick-open-input-wrap :deep(.n-input) {
  font-size: var(--ide-font-size-lg);
}
.quick-open-list {
  max-height: 320px;
  overflow-y: auto;
  padding: 4px 0;
}
.quick-open-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  cursor: pointer;
  font-size: var(--ide-font-size);
  transition: background 0.1s;
}
.quick-open-item:hover,
.quick-open-item.active {
  background: var(--accent-bg);
}
.quick-open-item.active {
  color: var(--accent-color);
}
.qo-icon {
  font-size: var(--ide-font-size-lg);
  flex-shrink: 0;
}
.qo-name {
  font-weight: 500;
  flex-shrink: 0;
}
.qo-path {
  font-size: var(--ide-font-size-xs);
  color: var(--text-muted);
  margin-left: auto;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 220px;
}
.quick-open-empty {
  padding: 20px;
  text-align: center;
}

.command-palette {
  display: flex;
  flex-direction: column;
}
.cp-input-wrap {
  border-bottom: 1px solid var(--border-color);
  padding: 8px 12px;
}
.cp-input-wrap :deep(.n-input) {
  font-size: var(--ide-font-size-lg);
}
.cp-list {
  max-height: 380px;
  overflow-y: auto;
  padding: 4px 0;
}
.cp-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  cursor: pointer;
  font-size: var(--ide-font-size);
  transition: background 0.1s;
}
.cp-item:hover,
.cp-item.active {
  background: var(--accent-bg);
}
.cp-item.active {
  color: var(--accent-color);
}
.cp-icon {
  font-size: var(--ide-font-size);
  flex-shrink: 0;
  width: 20px;
  text-align: center;
}
.cp-label {
  font-weight: 500;
  flex-shrink: 0;
}
.cp-cat {
  font-size: var(--ide-font-size-xs);
  color: var(--text-muted);
  margin-left: 8px;
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--bg-quaternary);
}
.cp-shortcut {
  font-size: var(--ide-font-size-xs);
  color: var(--text-muted);
  margin-left: auto;
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  font-family: var(--ide-code-font);
}
.cp-empty {
  padding: 20px;
  text-align: center;
}

/* ===== 快捷键速查表 ===== */
.keybindings-list {
  max-height: 60vh;
  overflow-y: auto;
  padding-right: 4px;
}
.kb-group {
  margin-bottom: 16px;
}
.kb-group:last-child {
  margin-bottom: 0;
}
.kb-group-title {
  font-size: var(--ide-font-size-sm);
  font-weight: 600;
  color: var(--accent-color);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 4px 8px;
  border-bottom: 1px solid var(--border-color);
  margin-bottom: 4px;
}
.kb-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 5px 8px;
  border-radius: 4px;
  transition: background 0.1s;
}
.kb-row:hover {
  background: var(--bg-tertiary);
}
.kb-desc {
  font-size: var(--ide-font-size);
  color: var(--text-primary);
}
.kb-keys {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}
.kb-key {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 24px;
  height: 22px;
  padding: 0 6px;
  font-size: var(--ide-font-size-xs);
  font-family: var(--ide-font), monospace;
  color: var(--text-primary);
  background: var(--bg-tertiary);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  box-shadow: 0 1px 0 var(--border-color);
  user-select: none;
}
.kb-plus {
  font-size: var(--ide-font-size-xs);
  color: var(--text-dim);
  margin: 0 2px;
}

.goto-line-box {
  background: var(--bg-tertiary);
  border-radius: 8px;
  overflow: hidden;
}

.goto-line-input {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-primary);
}

.goto-line-hint {
  padding: 8px 16px;
  text-align: center;
  background: var(--bg-quaternary);
}

.confirm-dialog-body {
  padding: 8px 0 16px;
}
</style>
