<template>
  <div class="settings-panel">
    <!-- 左右分栏：左侧菜单 + 右侧内容 -->
    <div class="settings-layout">
      <aside class="settings-menu">
        <div
          v-for="item in menuItems"
          :key="item.key"
          class="settings-menu-item"
          :class="{ active: activeMenu === item.key }"
          @click="activeMenu = item.key"
        >
          <span class="settings-menu-icon">{{ item.icon }}</span>
          <span class="settings-menu-label">{{ item.label }}</span>
        </div>
      </aside>
      <section class="settings-content">
        <!-- 常规（语言切换） -->
        <div v-show="activeMenu === 'general'">
          <div class="section-title">
            <span>{{ t('settings.language') }}</span>
          </div>
          <n-form label-placement="left" label-width="100" size="small">
            <n-form-item :label="t('settings.language')">
              <n-select
                v-model:value="currentLocale"
                :options="localeOptions"
                size="small"
                style="max-width: 280px"
                @update:value="onLocaleChange"
              />
            </n-form-item>
          </n-form>
          <n-text depth="2" style="font-size: var(--ide-font-size-sm)">{{ t('settings.languageDesc') }}</n-text>
        </div>

        <!-- 主题 -->
        <div v-show="activeMenu === 'theme'">
          <div class="section-title">
            <span>{{ t('settings.themeSectionBuiltin') }}</span>
          </div>
          <div class="theme-cards">
            <div
              v-for="name in builtinThemeNames"
              :key="name"
              class="theme-card"
              :class="{ active: modelValue === name }"
              @click="onSelectTheme(name)"
            >
              <div class="theme-card-preview" :style="previewStyle(name)">
                <div class="preview-titlebar">
                  <span class="preview-dot" v-for="i in 3" :key="i"></span>
                  <span class="preview-title-text"></span>
                </div>
                <div class="preview-body">
                  <div class="preview-sidebar">
                    <div class="preview-menu-item" v-for="i in 3" :key="i"></div>
                  </div>
                  <div class="preview-editor">
                    <div class="preview-code-line" v-for="i in 3" :key="i"
                      :style="{ width: (60 + i * 10) + '%' }"></div>
                  </div>
                </div>
                <div class="preview-footer">
                  <span class="preview-accent-btn"></span>
                </div>
              </div>
              <div class="theme-card-label">{{ getTheme(name).label }}</div>
            </div>
          </div>

          <div class="section-title">
            <span>{{ t('settings.themeSectionCustom') }}</span>
            <n-button size="tiny" @click="copyCurrentAsCustom">{{ t('settings.themeBtnCopy') }}</n-button>
          </div>

          <n-space v-if="customNames.length" size="small" style="margin-bottom: 12px; flex-wrap: wrap;">
            <n-tag
              v-for="name in customNames"
              :key="name"
              closable
              :type="modelValue === name ? 'primary' : 'default'"
              @click="onSelectTheme(name)"
              @close="removeCustom(name)"
            >
              {{ getTheme(name).label }}
            </n-tag>
          </n-space>
          <n-empty v-else :description="t('settings.themeNoCustom')" size="small" style="margin-bottom: 12px;" />

          <template v-if="!isBuiltInTheme(modelValue)">
            <div class="section-title">{{ t('settings.themeSectionEdit') }}</div>
            <n-form label-placement="left" label-width="90" size="small">
              <n-form-item :label="t('settings.themeNameLabel')">
                <n-input v-model:value="current.label" size="small" />
              </n-form-item>
              <n-form-item :label="t('settings.themeDarkMode')">
                <n-switch v-model:value="current.isDark" size="small" />
              </n-form-item>
              <n-form-item v-for="(label, key) in varLabels" :key="key" :label="label">
                <n-color-picker
                  v-model:value="current.variables[key]"
                  size="small"
                  :modes="['hex', 'rgb']"
                />
              </n-form-item>
            </n-form>
            <n-button size="small" type="primary" block @click="saveCurrent">
              {{ t('settings.themeBtnSaveApply') }}
            </n-button>
          </template>
        </div>

        <!-- 编辑器 -->
        <div v-show="activeMenu === 'editor'">
          <!-- 编辑器主题（独立于 IDE 主题） -->
          <div class="section-title"><span>{{ t('settings.editorSectionTheme') }}</span></div>
          <div class="prop-grid">
            <div class="prop-cell" style="grid-column: 1 / -1;">
              <label class="prop-cell-label">{{ t('settings.editorMonacoTheme') }}</label>
              <n-select
                v-model:value="editorThemeLocal"
                size="small"
                :options="editorThemeOptions"
                style="width: 100%;"
              />
            </div>
          </div>
          <div class="prop-hint">{{ t('settings.editorThemeHint') }}</div>

          <!-- 外观分组 -->
          <div class="section-title"><span>{{ t('settings.editorSectionAppearance') }}</span></div>
          <div class="prop-grid">
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorFontSizeLabel') }}</label>
              <n-input-number v-model:value="fontSizeLocal" size="small" :min="10" :max="28" :step="1" style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorFontFamilyLabel') }}</label>
              <n-select v-model:value="fontFamilyLocal" size="small" :options="fontFamilyOptions" filterable style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorLineHeightLabel') }}</label>
              <n-input-number v-model:value="lineHeightLocal" size="small" :min="0" :max="40" :step="1" style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorTabSizeLabel') }}</label>
              <n-select v-model:value="tabSizeLocal" size="small" :options="tabSizeOptions" style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorLineNumbersMinChars') }}</label>
              <n-input-number v-model:value="lineNumbersMinCharsLocal" size="small" :min="1" :max="10" :step="1" style="width: 100%;" />
            </div>
          </div>
          <div class="prop-checks">
            <n-checkbox v-model:checked="lineNumbersLocal">{{ t('settings.editorShowLineNumbers') }}</n-checkbox>
            <n-checkbox v-model:checked="wordWrapLocal">{{ t('settings.editorWordWrap') }}</n-checkbox>
            <n-checkbox v-model:checked="fontLigaturesLocal">{{ t('settings.editorFontLigatures') }}</n-checkbox>
            <n-checkbox v-model:checked="renderFinalNewlineLocal">{{ t('settings.editorRenderFinalNewline') }}</n-checkbox>
            <n-checkbox v-model:checked="bracketPairColorizationLocal">{{ t('settings.editorBracketPairColorization') }}</n-checkbox>
            <n-checkbox v-model:checked="guidesBracketPairsLocal">{{ t('settings.editorGuidesBracketPairs') }}</n-checkbox>
            <n-checkbox v-model:checked="autoConvertSymbolsLocal">{{ t('settings.editorAutoConvertSymbols') }}</n-checkbox>
          </div>

          <!-- 小地图分组 -->
          <div class="section-title"><span>{{ t('settings.editorSectionMinimap') }}</span></div>
          <div class="prop-checks">
            <n-checkbox v-model:checked="minimapLocal">{{ t('settings.editorShowMinimap') }}</n-checkbox>
            <n-checkbox v-model:checked="minimapRenderCharactersLocal">{{ t('settings.editorMinimapRenderCharacters') }}</n-checkbox>
          </div>
          <div class="prop-grid">
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorMinimapShowSlider') }}</label>
              <n-select v-model:value="minimapShowSliderLocal" size="small" :options="minimapShowSliderOptions" style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorMinimapMaxColumn') }}</label>
              <n-input-number v-model:value="minimapMaxColumnLocal" size="small" :min="40" :max="300" :step="10" style="width: 100%;" />
            </div>
          </div>

          <!-- 光标分组 -->
          <div class="section-title"><span>{{ t('settings.editorSectionCursor') }}</span></div>
          <div class="prop-grid">
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorCursorBlinkingLabel') }}</label>
              <n-select v-model:value="cursorBlinkingLocal" size="small" :options="cursorBlinkingOptions" style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorCursorWidthLabel') }}</label>
              <n-input-number v-model:value="cursorWidthLocal" size="small" :min="0" :max="10" :step="1" style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorRenderWhitespaceLabel') }}</label>
              <n-select v-model:value="renderWhitespaceLocal" size="small" :options="renderWhitespaceOptions" style="width: 100%;" />
            </div>
          </div>
          <div class="prop-checks">
            <n-checkbox v-model:checked="cursorSmoothCaretAnimationLocal">{{ t('settings.editorCursorSmooth') }}</n-checkbox>
          </div>

          <!-- 行为分组 -->
          <div class="section-title"><span>{{ t('settings.editorSectionBehavior') }}</span></div>
          <div class="prop-grid">
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.editorAutoSaveLabel') }}</label>
              <n-input-number v-model:value="autoSaveLocal" size="small" :min="0" :max="30000" :step="500" style="width: 100%;" />
            </div>
          </div>
          <div class="prop-hint">{{ t('settings.editorAutoSaveHint') }}</div>
        </div>

        <!-- 设计器 -->
        <div v-show="activeMenu === 'designer'">
          <div class="section-title"><span>{{ t('settings.designerSectionGrid') }}</span></div>
          <div class="prop-grid">
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.designerGridSize') }}</label>
              <n-input-number v-model:value="gridSizeLocal" size="small" :min="4" :max="32" :step="2" style="width: 100%;" />
            </div>
          </div>
          <div class="prop-checks">
            <n-checkbox v-model:checked="showGridLocal">{{ t('settings.designerShowGrid') }}</n-checkbox>
            <n-checkbox v-model:checked="snapGridLocal">{{ t('settings.designerSnapGrid') }}</n-checkbox>
          </div>

          <div class="section-title"><span>{{ t('settings.designerSectionDefault') }}</span></div>
          <div class="prop-grid">
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.designerDefaultRadius') }}</label>
              <n-input-number v-model:value="defaultRadiusLocal" size="small" :min="0" :max="24" :step="1" style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.designerDefaultBorder') }}</label>
              <n-input-number v-model:value="defaultBorderWidthLocal" size="small" :min="0" :max="4" :step="1" style="width: 100%;" />
            </div>
          </div>
        </div>

        <!-- 编译 -->
        <div v-show="activeMenu === 'build'">
          <div class="section-title"><span>{{ t('settings.buildSectionOptions') }}</span></div>
          <div class="prop-grid">
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.buildDefaultMode') }}</label>
              <n-select v-model:value="defaultBuildModeLocal" size="small" :options="buildModeOptions" style="width: 100%;" />
            </div>
          </div>
          <div class="prop-checks">
            <n-checkbox v-model:checked="autoOpenFolderLocal">{{ t('settings.buildAutoOpenFolder') }}</n-checkbox>
            <n-checkbox v-model:checked="showBuildHistoryLocal">{{ t('settings.buildShowHistory') }}</n-checkbox>
          </div>

          <div class="section-title"><span>{{ t('settings.buildSectionOutput') }}</span></div>
          <div class="prop-grid">
            <div class="prop-cell" style="grid-column: 1 / -1;">
              <label class="prop-cell-label">{{ t('settings.buildOutputDirLabel') }}</label>
              <n-input v-model:value="outputDirLocal" size="small" :placeholder="t('settings.buildOutputDirPh')" />
            </div>
          </div>
          <div class="prop-hint">{{ t('settings.buildReleaseHint') }}</div>
        </div>

        <!-- 界面 -->
        <div v-show="activeMenu === 'ui'">
          <div class="section-title"><span>{{ t('settings.uiSectionStartup') }}</span></div>
          <div class="prop-checks">
            <n-checkbox v-model:checked="openLastProjectLocal">{{ t('settings.uiOpenLastProject') }}</n-checkbox>
          </div>

          <div class="section-title"><span>{{ t('settings.uiSectionFont') }}</span></div>
          <div class="prop-grid">
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.uiFontFamilyLabel') }}</label>
              <n-select v-model:value="uiFontFamilyLocal" size="small" :options="uiFontFamilyOptions" filterable style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.uiFontSizeLabel') }}</label>
              <n-input-number v-model:value="uiFontSizeLocal" size="small" :min="11" :max="18" :step="1" style="width: 100%;" />
            </div>
          </div>

          <div class="section-title"><span>{{ t('settings.uiSectionPanel') }}</span></div>
          <div class="prop-grid">
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.uiLeftPanelWidth') }}</label>
              <n-input-number v-model:value="leftWidthLocal" size="small" :min="180" :max="400" :step="10" style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.uiRightPanelWidth') }}</label>
              <n-input-number v-model:value="rightWidthLocal" size="small" :min="200" :max="500" :step="10" style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.uiOutputPanelHeight') }}</label>
              <n-input-number v-model:value="outputHeightLocal" size="small" :min="80" :max="400" :step="10" style="width: 100%;" />
            </div>
          </div>

          <div class="section-title"><span>{{ t('settings.uiSectionStatus') }}</span></div>
          <div class="prop-checks">
            <n-checkbox v-model:checked="sbShowCursorLocal">{{ t('settings.uiSbShowCursor') }}</n-checkbox>
            <n-checkbox v-model:checked="sbShowIndentLocal">{{ t('settings.uiSbShowIndent') }}</n-checkbox>
            <n-checkbox v-model:checked="sbShowEncodingLocal">{{ t('settings.uiSbShowEncoding') }}</n-checkbox>
            <n-checkbox v-model:checked="sbShowEolLocal">{{ t('settings.uiSbShowEol') }}</n-checkbox>
            <n-checkbox v-model:checked="sbShowLangLocal">{{ t('settings.uiSbShowLang') }}</n-checkbox>
            <n-checkbox v-model:checked="sbShowHealthLocal">{{ t('settings.uiSbShowHealth') }}</n-checkbox>
          </div>

          <div class="section-title"><span>{{ t('settings.uiSectionOutput') }}</span></div>
          <div class="prop-checks">
            <n-checkbox v-model:checked="autoSwitchOutputTabLocal">{{ t('settings.uiAutoSwitchOutput') }}</n-checkbox>
            <n-checkbox v-model:checked="smartScrollLocal">{{ t('settings.uiSmartScroll') }}</n-checkbox>
          </div>
        </div>

        <!-- AI -->
        <div v-show="activeMenu === 'ai'">
          <div class="section-title">
            <span>{{ t('settings.aiSectionModels') }}</span>
            <n-button size="tiny" type="primary" @click="startAddModel">{{ t('settings.aiBtnAddModel') }}</n-button>
          </div>
          <div class="model-settings-list">
            <div
              v-for="m in modelsLocal"
              :key="m.id"
              class="model-setting-card"
              :class="{ active: m.id === activeModelIdLocal }"
            >
              <div class="model-setting-header" @click="toggleModelEdit(m.id)">
                <div class="model-setting-info">
                  <span class="model-setting-name">{{ m.name || m.model }}</span>
                  <span v-if="m.id === activeModelIdLocal" class="model-setting-active">{{ t('settings.aiCurrentUsing') }}</span>
                </div>
                <div class="model-setting-caps">
                  <n-tag v-if="m.supportsVision" size="tiny" type="info" :bordered="false">{{ t('settings.aiVisionCap') }}</n-tag>
                  <n-tag v-if="m.supportsFiles" size="tiny" type="info" :bordered="false">{{ t('settings.aiFileCap') }}</n-tag>
                </div>
                <n-button text size="tiny" type="error" @click.stop="removeModel(m.id)" v-if="modelsLocal.length > 1">{{ t('settings.aiBtnDelete') }}</n-button>
                <n-button text size="tiny" @click.stop="toggleModelEdit(m.id)">{{ editingModelId === m.id ? t('settings.aiBtnCollapse') : t('settings.aiBtnEdit') }}</n-button>
              </div>
              <div v-if="editingModelId === m.id" class="model-setting-body">
                <div class="prop-grid">
                  <div class="prop-cell">
                    <label class="prop-cell-label">{{ t('settings.aiName') }}</label>
                    <n-input v-model:value="m.name" size="small" :placeholder="t('settings.aiNamePh')" @update:value="() => updateModel(m)" />
                  </div>
                  <div class="prop-cell">
                    <label class="prop-cell-label">{{ t('settings.aiModelId') }}</label>
                    <n-input v-model:value="m.model" size="small" :placeholder="t('settings.aiModelIdPh')" @update:value="() => updateModel(m)" />
                  </div>
                </div>
                <div class="prop-grid">
                  <div class="prop-cell" style="grid-column: 1 / -1;">
                    <label class="prop-cell-label">{{ t('settings.aiEndpoint') }}</label>
                    <n-input v-model:value="m.endpoint" size="small" :placeholder="t('settings.aiEndpointPh')" @update:value="() => updateModel(m)" />
                  </div>
                  <div class="prop-cell" style="grid-column: 1 / -1;">
                    <label class="prop-cell-label">{{ t('settings.aiApiKey') }}</label>
                    <n-input v-model:value="m.apiKey" size="small" type="password" show-password-on="click" :placeholder="t('settings.aiApiKeyPh')" @update:value="() => updateModel(m)" />
                  </div>
                </div>
                <div class="prop-grid">
                  <div class="prop-cell">
                    <label class="prop-cell-label">{{ t('settings.aiTemperature') }}</label>
                    <n-input-number v-model:value="m.temperature" size="small" :min="0" :max="2" :step="0.1" style="width: 100%;" @update:value="() => updateModel(m)" />
                  </div>
                  <div class="prop-cell">
                    <label class="prop-cell-label">{{ t('settings.aiContextWindow') }}</label>
                    <n-input-number v-model:value="m.contextWindow" size="small" :min="4096" :max="1048576" :step="4096" style="width: 100%;" @update:value="() => updateModel(m)" />
                  </div>
                  <div class="prop-cell">
                    <label class="prop-cell-label">{{ t('settings.aiMaxTokens') }}</label>
                    <n-input-number v-model:value="m.maxTokens" size="small" :min="256" :max="1048576" :step="256" style="width: 100%;" @update:value="() => updateModel(m)" />
                  </div>
                </div>
                <div class="prop-checks">
                  <n-checkbox v-model:checked="m.supportsVision" @update:checked="() => updateModel(m)">{{ t('settings.aiSupportsVision') }}</n-checkbox>
                  <n-checkbox v-model:checked="m.supportsFiles" @update:checked="() => updateModel(m)">{{ t('settings.aiSupportsFiles') }}</n-checkbox>
                  <n-button v-if="m.id !== activeModelIdLocal" size="tiny" @click="switchToModel(m.id)">{{ t('settings.aiBtnSetCurrent') }}</n-button>
                </div>
              </div>
            </div>
          </div>

          <div class="section-title"><span>{{ t('settings.aiSectionParams') }}</span></div>
          <div class="prop-checks">
            <n-checkbox v-model:checked="aiStreamLocal">{{ t('settings.aiStream') }}</n-checkbox>
          </div>
          <div class="prop-grid">
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.aiCompressThreshold') }}</label>
              <n-input-number v-model:value="aiCompressThresholdLocal" size="small" :min="2000" :max="20000" :step="500" style="width: 100%;" />
            </div>
            <div class="prop-cell">
              <label class="prop-cell-label">{{ t('settings.aiKeepRecent') }}</label>
              <n-input-number v-model:value="aiKeepRecentLocal" size="small" :min="2" :max="20" :step="1" style="width: 100%;" />
            </div>
          </div>
          <div class="prop-hint">{{ t('settings.aiCompressHint') }}</div>

          <div class="section-title"><span>{{ t('settings.aiSectionAgents') }}</span></div>
          <div class="prop-hint">{{ t('settings.aiAgentHint') }}</div>
          <div class="agent-list">
            <div v-for="a in allAgentsList" :key="a.id" class="agent-card">
              <div class="agent-card-header">
                <span class="agent-emoji">{{ a.emoji || '🤖' }}</span>
                <span class="agent-name">{{ a.name }}</span>
                <n-tag v-if="a.isBuiltin" size="tiny" type="info" :bordered="false">{{ t('settings.aiBuiltin') }}</n-tag>
                <n-tag v-else size="tiny" :bordered="false">{{ t('settings.aiCustom') }}</n-tag>
              </div>
              <div class="agent-desc">{{ a.desc }}</div>
              <div class="agent-keywords" v-if="a.autoSelect && a.autoSelect.length > 0">
                <span class="agent-kw-label">{{ t('settings.aiAutoTriggerKw') }}</span>
                <span v-for="kw in a.autoSelect.slice(0,8)" :key="kw" class="agent-kw">{{ kw }}</span>
                <span v-if="a.autoSelect.length > 8" class="agent-kw">...</span>
              </div>
            </div>
          </div>

          <div class="section-title"><span>{{ t('settings.aiSectionSkills') }}</span></div>
          <div class="prop-hint" v-html="t('settings.aiSkillHint')"></div>
          <div class="skill-list">
            <div v-for="s in allSkillsList" :key="s.id" class="skill-card">
              <div class="skill-card-header">
                <span class="skill-icon">{{ s.icon || '🔧' }}</span>
                <span class="skill-name">{{ s.name }}</span>
                <n-tag v-if="s.builtin" size="tiny" type="success" :bordered="false">{{ t('settings.aiBuiltin') }}</n-tag>
              </div>
              <div class="skill-desc">{{ s.desc }}</div>
            </div>
          </div>
        </div>

        <!-- 插件 -->
        <div v-show="activeMenu === 'plugins'">
          <div class="section-title">
            <span>{{ t('settings.pluginSectionManage') }}</span>
            <n-button size="tiny" @click="reloadPlugins">{{ t('settings.pluginBtnReload') }}</n-button>
          </div>
          <n-empty v-if="loadedPlugins.length === 0" :description="t('settings.pluginNoPlugins')" size="small" style="margin-bottom: 12px;" />
          <div v-else class="plugin-list">
            <div v-for="p in loadedPlugins" :key="p.dir" class="plugin-card">
              <div class="plugin-card-header">
                <span class="plugin-name">{{ p.name }}</span>
                <n-tag size="tiny" :bordered="false">v{{ p.version || '0.0.0' }}</n-tag>
              </div>
              <div class="plugin-meta">
                <span v-if="p.author">{{ t('settings.pluginAuthor') }}{{ p.author }}</span>
                <span>{{ t('settings.pluginDir') }}{{ p.dir }}</span>
              </div>
              <div v-if="p.description" class="plugin-desc">{{ p.description }}</div>
            </div>
          </div>
        </div>

        <!-- 模板 -->
        <div v-show="activeMenu === 'templates'">
          <div class="section-title">
            <span>{{ t('settings.templateSectionTitle') }}</span>
            <n-button size="tiny" @click="$emit('refresh-templates')">{{ t('settings.templateBtnRefresh') }}</n-button>
          </div>
          <div v-if="projectPath" class="template-save-row">
            <n-input v-model:value="newTemplateName" size="small" :placeholder="t('settings.templateNamePh')" style="flex: 1;" />
            <n-input v-model:value="newTemplateDesc" size="small" :placeholder="t('settings.templateDescPh')" style="flex: 1;" />
            <n-input v-model:value="newTemplateIcon" size="small" :placeholder="t('settings.templateIconPh')" style="width: 60px;" />
            <n-button size="small" type="primary" @click="saveAsTemplate">{{ t('settings.templateBtnSave') }}</n-button>
          </div>
          <n-empty v-if="globalTemplates.length === 0" :description="t('settings.templateNoTemplates')" size="small" style="margin-bottom: 12px;" />
          <div v-else class="plugin-list">
            <div v-for="t in globalTemplates" :key="t.dir" class="plugin-card">
              <div class="plugin-card-header">
                <span class="plugin-name">{{ t.icon ? t.icon + ' ' : '' }}{{ t.name }}</span>
                <n-button size="tiny" quaternary type="error" @click="deleteTemplate(t.dir)">{{ t('settings.templateBtnDelete') }}</n-button>
              </div>
              <div v-if="t.description" class="plugin-desc">{{ t.description }}</div>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import {
  NForm,
  NFormItem,
  NSelect,
  NInput,
  NInputGroup,
  NSwitch,
  NCheckbox,
  NInputNumber,
  NColorPicker,
  NButton,
  NSpace,
  NTag,
  NEmpty,
  NText,
  useMessage
} from 'naive-ui'
import {
  getThemes,
  getThemeNames,
  getTheme,
  saveCustomTheme,
  deleteCustomTheme,
  isBuiltInTheme,
  applyTheme
} from '../themes.js'
// G12：插件管理器 UI 需要读取已加载插件列表并支持重新加载
import { loadedPlugins, loadAllPlugins } from '../plugins/loader.js'
import { IDEService } from '../../bindings/egou/internal/app'
import { BUILTIN_AGENTS } from '../lib/aiAgents.js'
import { BUILTIN_SKILLS } from '../lib/aiSkills.js'
import { t, setLocale, getLocale, listLocales } from '../i18n/index.js'

const useMsg = useMessage()

// 左右分栏：左侧菜单当前选中项
const activeMenu = ref('theme')
const menuItems = [
  { key: 'general', icon: '⚙️', label: t('settings.general') },
  { key: 'theme', icon: '🎨', label: t('settings.theme') },
  { key: 'editor', icon: '📝', label: t('settings.editor') },
  { key: 'designer', icon: '🎯', label: t('settings.designer') },
  { key: 'build', icon: '🔨', label: t('settings.build') },
  { key: 'ui', icon: '🖥️', label: t('settings.ui') },
  { key: 'ai', icon: '🤖', label: t('settings.ai') },
  { key: 'plugins', icon: '🔌', label: t('settings.plugins') },
  { key: 'templates', icon: '📦', label: t('settings.templates') }
]

// ===== i18n 语言切换 =====
const currentLocale = ref(getLocale())
const localeOptions = computed(() => {
  const labels = { 'zh-CN': t('settings.generalLocaleZhCn'), 'en-US': 'English' }
  return listLocales().map(loc => ({ label: labels[loc] || loc, value: loc }))
})
function onLocaleChange(loc) {
  setLocale(loc)
  // 提示用户重启以完全生效（部分组件文案在初始化时已读取，运行时切换不更新）
  useMsg?.info?.(t('settings.languageDesc'))
}

const props = defineProps({
  activeMenuKey: { type: String, default: '' },
  modelValue: { type: String, default: 'dark' },
  // P6：当前项目路径（为空时不显示"保存为模板"）
  projectPath: { type: String, default: '' },
  // P6：全局模板列表
  globalTemplates: { type: Array, default: () => [] },
  minimapEnabled: { type: Boolean, default: false },
  fontSize: { type: Number, default: 14 },
  lineNumbersEnabled: { type: Boolean, default: true },
  lineHeight: { type: Number, default: 0 },
  autoSaveDelay: { type: Number, default: 3000 },
  tabSize: { type: Number, default: 4 },
  wordWrap: { type: Boolean, default: false },
  renderWhitespace: { type: String, default: 'selection' },
  cursorBlinking: { type: String, default: 'blink' },
  cursorSmoothCaretAnimation: { type: Boolean, default: true },
  cursorWidth: { type: Number, default: 0 },
  bracketPairColorization: { type: Boolean, default: true },
  guidesBracketPairs: { type: Boolean, default: false },
  fontLigatures: { type: Boolean, default: false },
  lineNumbersMinChars: { type: Number, default: 3 },
  renderFinalNewline: { type: Boolean, default: true },
  minimapShowSlider: { type: String, default: 'mouseover' },
  minimapRenderCharacters: { type: Boolean, default: true },
  minimapMaxColumn: { type: Number, default: 120 },
  // 编辑器主题（独立于 IDE 主题）
  editorTheme: { type: String, default: 'auto' },
  fontFamily: { type: String, default: 'Consolas, "Courier New", monospace' },
  autoConvertSymbols: { type: Boolean, default: true },
  // 设计器
  gridSize: { type: Number, default: 8 },
  showGrid: { type: Boolean, default: true },
  snapGrid: { type: Boolean, default: true },
  defaultRadius: { type: Number, default: 0 },
  defaultBorderWidth: { type: Number, default: 1 },
  // 编译
  defaultBuildMode: { type: String, default: 'debug' },
  autoOpenFolder: { type: Boolean, default: true },
  showBuildHistory: { type: Boolean, default: true },
  outputDir: { type: String, default: 'bin' },
  // 界面
  openLastProject: { type: Boolean, default: true },
  leftPanelWidth: { type: Number, default: 240 },
  rightPanelWidth: { type: Number, default: 200 },
  outputPanelHeight: { type: Number, default: 200 },
  sbShowCursor: { type: Boolean, default: true },
  sbShowIndent: { type: Boolean, default: true },
  sbShowEncoding: { type: Boolean, default: true },
  sbShowEol: { type: Boolean, default: true },
  sbShowLang: { type: Boolean, default: true },
  sbShowHealth: { type: Boolean, default: true },
  autoSwitchOutputTab: { type: Boolean, default: true },
  smartScroll: { type: Boolean, default: true },
  // AI - 多模型
  models: { type: Array, default: () => [] },
  activeModelId: { type: String, default: '' },
  agents: { type: Array, default: () => [] },
  aiStream: { type: Boolean, default: true },
  aiCompressThreshold: { type: Number, default: 6000 },
  aiKeepRecent: { type: Number, default: 8 }
})
const emit = defineEmits(['update:modelValue', 'update:minimapEnabled', 'update:fontSize', 'update:lineNumbersEnabled', 'update:lineHeight', 'update:autoSaveDelay', 'update:tabSize', 'update:wordWrap', 'update:renderWhitespace', 'update:cursorBlinking', 'update:cursorSmoothCaretAnimation', 'update:cursorWidth', 'update:bracketPairColorization', 'update:guidesBracketPairs', 'update:fontLigatures', 'update:lineNumbersMinChars', 'update:renderFinalNewline', 'update:minimapShowSlider', 'update:minimapRenderCharacters', 'update:minimapMaxColumn', 'update:editorTheme', 'update:fontFamily', 'update:autoConvertSymbols',
  'update:gridSize', 'update:showGrid', 'update:snapGrid', 'update:defaultRadius', 'update:defaultBorderWidth',
  'update:defaultBuildMode', 'update:autoOpenFolder', 'update:showBuildHistory', 'update:outputDir',
  'update:openLastProject', 'update:leftPanelWidth', 'update:rightPanelWidth', 'update:outputPanelHeight',
  'update:sbShowCursor', 'update:sbShowIndent', 'update:sbShowEncoding', 'update:sbShowEol', 'update:sbShowLang', 'update:sbShowHealth',
  'update:autoSwitchOutputTab', 'update:smartScroll',
  'add-model', 'update-model', 'delete-model', 'switch-model',
  'update:aiStream', 'update:aiCompressThreshold', 'update:aiKeepRecent',
  'refresh-templates'])

watch(() => props.activeMenuKey, (v) => {
  if (v && menuItems.some(m => m.key === v)) {
    activeMenu.value = v
  }
})

// 编辑器主题选项
const editorThemeOptions = computed(() => [
  { label: t('settings.editorThemeOptAuto'), value: 'auto' },
  { label: t('settings.editorThemeOptDark'), value: 'dark' },
  { label: t('settings.editorThemeOptLight'), value: 'light' },
  { label: t('settings.editorThemeOptHcDark'), value: 'hc-black' },
  { label: t('settings.editorThemeOptHcLight'), value: 'hc-light' }
])
const editorThemeLocal = ref(props.editorTheme)
watch(() => props.editorTheme, (v) => { editorThemeLocal.value = v })
watch(editorThemeLocal, (v) => emit('update:editorTheme', v))

const fontFamilyOptions = computed(() => [
  { label: t('settings.fontFamilyOptBuiltin'), value: "'IdeFont', 'Consolas', 'Courier New', monospace" },
  { label: t('settings.fontFamilyOptConsolas'), value: 'Consolas, "Courier New", monospace' },
  { label: t('settings.fontFamilyOptCascadia'), value: '"Cascadia Code", Consolas, monospace' },
  { label: t('settings.fontFamilyOptJetBrains'), value: '"JetBrains Mono", Consolas, monospace' },
  { label: t('settings.fontFamilyOptFiraCode'), value: '"Fira Code", Consolas, monospace' },
  { label: t('settings.fontFamilyOptSourceCodePro'), value: '"Source Code Pro", Consolas, monospace' },
  { label: t('settings.fontFamilyOptMenlo'), value: 'Menlo, Monaco, Consolas, monospace' },
  { label: t('settings.fontFamilyOptMonaco'), value: 'Monaco, Menlo, Consolas, monospace' },
  { label: t('settings.fontFamilyOptSarasa'), value: '"Sarasa Mono SC", Consolas, monospace' },
  { label: t('settings.fontFamilyOptSystemDefault'), value: 'monospace' }
])
const fontFamilyLocal = ref(props.fontFamily)
watch(() => props.fontFamily, (v) => { fontFamilyLocal.value = v })
watch(fontFamilyLocal, (v) => emit('update:fontFamily', v))

// IDE 界面字体（动态修改 --ide-font CSS 变量，持久化到 localStorage）
const uiFontFamilyOptions = computed(() => [
  { label: t('settings.uiFontOptBuiltin'), value: "'IdeFont', system-ui, sans-serif" },
  { label: t('settings.uiFontOptYh'), value: "'Microsoft YaHei', system-ui, sans-serif" },
  { label: t('settings.uiFontOptSegoe'), value: "'Segoe UI', system-ui, sans-serif" },
  { label: t('settings.uiFontOptPingfang'), value: "'PingFang SC', system-ui, sans-serif" },
  { label: t('settings.uiFontOptSourceHan'), value: "'Source Han Sans SC', system-ui, sans-serif" },
  { label: t('settings.uiFontOptSimhei'), value: "'SimHei', system-ui, sans-serif" },
  { label: t('settings.uiFontOptSimsun'), value: "'SimSun', system-ui, sans-serif" },
  { label: t('settings.uiFontOptDefault'), value: 'system-ui, -apple-system, sans-serif' },
])
const uiFontFamilyLocal = ref(localStorage.getItem('eg-uifont') || "'IdeFont', system-ui, sans-serif")
watch(uiFontFamilyLocal, (v) => {
  localStorage.setItem('eg-uifont', v)
  document.documentElement.style.setProperty('--ide-font', v)
}, { immediate: true })

// IDE 界面字号（统一控制 UI 文字大小，写入 --ide-font-size 及衍生层级 sm/xs/lg/xl）
// 编辑器字号独立由 eg-fontsize 控制，与此处互不影响。
const uiFontSizeLocal = ref(parseInt(localStorage.getItem('eg-uifontsize'), 10) || 13)
watch(uiFontSizeLocal, (v) => {
  const size = Math.max(11, Math.min(18, parseInt(v, 10) || 13))
  localStorage.setItem('eg-uifontsize', String(size))
  const root = document.documentElement.style
  root.setProperty('--ide-font-size', size + 'px')
  root.setProperty('--ide-font-size-sm', (size - 1) + 'px')
  root.setProperty('--ide-font-size-xs', (size - 2) + 'px')
  root.setProperty('--ide-font-size-lg', (size + 1) + 'px')
  root.setProperty('--ide-font-size-xl', (size + 2) + 'px')
  // 通知 App.vue 同步 Naive UI themeOverrides.common.fontSize
  window.dispatchEvent(new CustomEvent('eg-uifontsize-change', { detail: size }))
}, { immediate: true })
const autoConvertSymbolsLocal = ref(props.autoConvertSymbols)
watch(() => props.autoConvertSymbols, (v) => { autoConvertSymbolsLocal.value = v })
watch(autoConvertSymbolsLocal, (v) => emit('update:autoConvertSymbols', v))

const minimapLocal = ref(props.minimapEnabled)
watch(() => props.minimapEnabled, (v) => { minimapLocal.value = v })
watch(minimapLocal, (v) => emit('update:minimapEnabled', v))

const fontSizeLocal = ref(props.fontSize)
watch(() => props.fontSize, (v) => { fontSizeLocal.value = v })
watch(fontSizeLocal, (v) => emit('update:fontSize', v))
const lineNumbersLocal = ref(props.lineNumbersEnabled)
watch(() => props.lineNumbersEnabled, (v) => { lineNumbersLocal.value = v })
watch(lineNumbersLocal, (v) => emit('update:lineNumbersEnabled', v))
const lineHeightLocal = ref(props.lineHeight)
watch(() => props.lineHeight, (v) => { lineHeightLocal.value = v })
watch(lineHeightLocal, (v) => emit('update:lineHeight', v))
const autoSaveLocal = ref(props.autoSaveDelay)
watch(() => props.autoSaveDelay, (v) => { autoSaveLocal.value = v })
watch(autoSaveLocal, (v) => emit('update:autoSaveDelay', v))
const tabSizeLocal = ref(props.tabSize)
watch(() => props.tabSize, (v) => { tabSizeLocal.value = v })
watch(tabSizeLocal, (v) => emit('update:tabSize', v))
// 规约 §4：缩进用 NSelect 提供 2/4 空格选项（替代 n-input-number）
const tabSizeOptions = computed(() => [
  { label: t('settings.tabSizeOpt2'), value: 2 },
  { label: t('settings.tabSizeOpt4'), value: 4 },
  { label: t('settings.tabSizeOpt6'), value: 6 },
  { label: t('settings.tabSizeOpt8'), value: 8 },
])
const wordWrapLocal = ref(props.wordWrap)
watch(() => props.wordWrap, (v) => { wordWrapLocal.value = v })
watch(wordWrapLocal, (v) => emit('update:wordWrap', v))
const renderWhitespaceLocal = ref(props.renderWhitespace)
watch(() => props.renderWhitespace, (v) => { renderWhitespaceLocal.value = v })
watch(renderWhitespaceLocal, (v) => emit('update:renderWhitespace', v))
const renderWhitespaceOptions = computed(() => [
  { label: t('settings.renderWhitespaceOptNone'), value: 'none' },
  { label: t('settings.renderWhitespaceOptSelection'), value: 'selection' },
  { label: t('settings.renderWhitespaceOptAll'), value: 'all' }
])
const cursorBlinkingLocal = ref(props.cursorBlinking)
watch(() => props.cursorBlinking, (v) => { cursorBlinkingLocal.value = v })
watch(cursorBlinkingLocal, (v) => emit('update:cursorBlinking', v))
const cursorBlinkingOptions = computed(() => [
  { label: t('settings.cursorBlinkingOptBlink'), value: 'blink' },
  { label: t('settings.cursorBlinkingOptSmooth'), value: 'smooth' },
  { label: t('settings.cursorBlinkingOptPhase'), value: 'phase' },
  { label: t('settings.cursorBlinkingOptExpand'), value: 'expand' },
  { label: t('settings.cursorBlinkingOptSolid'), value: 'solid' }
])
const cursorSmoothCaretAnimationLocal = ref(props.cursorSmoothCaretAnimation)
watch(() => props.cursorSmoothCaretAnimation, (v) => { cursorSmoothCaretAnimationLocal.value = v })
watch(cursorSmoothCaretAnimationLocal, (v) => emit('update:cursorSmoothCaretAnimation', v))
const cursorWidthLocal = ref(props.cursorWidth)
watch(() => props.cursorWidth, (v) => { cursorWidthLocal.value = v })
watch(cursorWidthLocal, (v) => emit('update:cursorWidth', v))
const bracketPairColorizationLocal = ref(props.bracketPairColorization)
watch(() => props.bracketPairColorization, (v) => { bracketPairColorizationLocal.value = v })
watch(bracketPairColorizationLocal, (v) => emit('update:bracketPairColorization', v))
const guidesBracketPairsLocal = ref(props.guidesBracketPairs)
watch(() => props.guidesBracketPairs, (v) => { guidesBracketPairsLocal.value = v })
watch(guidesBracketPairsLocal, (v) => emit('update:guidesBracketPairs', v))
const fontLigaturesLocal = ref(props.fontLigatures)
watch(() => props.fontLigatures, (v) => { fontLigaturesLocal.value = v })
watch(fontLigaturesLocal, (v) => emit('update:fontLigatures', v))
const lineNumbersMinCharsLocal = ref(props.lineNumbersMinChars)
watch(() => props.lineNumbersMinChars, (v) => { lineNumbersMinCharsLocal.value = v })
watch(lineNumbersMinCharsLocal, (v) => emit('update:lineNumbersMinChars', v))
const renderFinalNewlineLocal = ref(props.renderFinalNewline)
watch(() => props.renderFinalNewline, (v) => { renderFinalNewlineLocal.value = v })
watch(renderFinalNewlineLocal, (v) => emit('update:renderFinalNewline', v))
const minimapShowSliderLocal = ref(props.minimapShowSlider)
watch(() => props.minimapShowSlider, (v) => { minimapShowSliderLocal.value = v })
watch(minimapShowSliderLocal, (v) => emit('update:minimapShowSlider', v))
const minimapRenderCharactersLocal = ref(props.minimapRenderCharacters)
watch(() => props.minimapRenderCharacters, (v) => { minimapRenderCharactersLocal.value = v })
watch(minimapRenderCharactersLocal, (v) => emit('update:minimapRenderCharacters', v))
const minimapMaxColumnLocal = ref(props.minimapMaxColumn)
watch(() => props.minimapMaxColumn, (v) => { minimapMaxColumnLocal.value = v })
watch(minimapMaxColumnLocal, (v) => emit('update:minimapMaxColumn', v))
const minimapShowSliderOptions = computed(() => [
  { label: t('settings.minimapShowSliderOptAlways'), value: 'always' },
  { label: t('settings.minimapShowSliderOptHover'), value: 'mouseover' }
])

// 设计器
const gridSizeLocal = ref(props.gridSize)
watch(() => props.gridSize, (v) => { gridSizeLocal.value = v })
watch(gridSizeLocal, (v) => emit('update:gridSize', v))
const showGridLocal = ref(props.showGrid)
watch(() => props.showGrid, (v) => { showGridLocal.value = v })
watch(showGridLocal, (v) => emit('update:showGrid', v))
const snapGridLocal = ref(props.snapGrid)
watch(() => props.snapGrid, (v) => { snapGridLocal.value = v })
watch(snapGridLocal, (v) => emit('update:snapGrid', v))
const defaultRadiusLocal = ref(props.defaultRadius)
watch(() => props.defaultRadius, (v) => { defaultRadiusLocal.value = v })
watch(defaultRadiusLocal, (v) => emit('update:defaultRadius', v))
const defaultBorderWidthLocal = ref(props.defaultBorderWidth)
watch(() => props.defaultBorderWidth, (v) => { defaultBorderWidthLocal.value = v })
watch(defaultBorderWidthLocal, (v) => emit('update:defaultBorderWidth', v))

// 编译
const buildModeOptions = computed(() => [
  { label: t('settings.buildModeOptDebug'), value: 'debug' },
  { label: t('settings.buildModeOptRelease'), value: 'release' }
])
const defaultBuildModeLocal = ref(props.defaultBuildMode)
watch(() => props.defaultBuildMode, (v) => { defaultBuildModeLocal.value = v })
watch(defaultBuildModeLocal, (v) => emit('update:defaultBuildMode', v))
const autoOpenFolderLocal = ref(props.autoOpenFolder)
watch(() => props.autoOpenFolder, (v) => { autoOpenFolderLocal.value = v })
watch(autoOpenFolderLocal, (v) => emit('update:autoOpenFolder', v))
const showBuildHistoryLocal = ref(props.showBuildHistory)
watch(() => props.showBuildHistory, (v) => { showBuildHistoryLocal.value = v })
watch(showBuildHistoryLocal, (v) => emit('update:showBuildHistory', v))
const outputDirLocal = ref(props.outputDir)
watch(() => props.outputDir, (v) => { outputDirLocal.value = v })
watch(outputDirLocal, (v) => emit('update:outputDir', v))

// 界面
const openLastProjectLocal = ref(props.openLastProject)
watch(() => props.openLastProject, (v) => { openLastProjectLocal.value = v })
watch(openLastProjectLocal, (v) => emit('update:openLastProject', v))
const leftWidthLocal = ref(props.leftPanelWidth)
watch(() => props.leftPanelWidth, (v) => { leftWidthLocal.value = v })
watch(leftWidthLocal, (v) => emit('update:leftPanelWidth', v))
const rightWidthLocal = ref(props.rightPanelWidth)
watch(() => props.rightPanelWidth, (v) => { rightWidthLocal.value = v })
watch(rightWidthLocal, (v) => emit('update:rightPanelWidth', v))
const outputHeightLocal = ref(props.outputPanelHeight)
watch(() => props.outputPanelHeight, (v) => { outputHeightLocal.value = v })
watch(outputHeightLocal, (v) => emit('update:outputPanelHeight', v))
const sbShowCursorLocal = ref(props.sbShowCursor)
watch(() => props.sbShowCursor, (v) => { sbShowCursorLocal.value = v })
watch(sbShowCursorLocal, (v) => emit('update:sbShowCursor', v))
const sbShowIndentLocal = ref(props.sbShowIndent)
watch(() => props.sbShowIndent, (v) => { sbShowIndentLocal.value = v })
watch(sbShowIndentLocal, (v) => emit('update:sbShowIndent', v))
const sbShowEncodingLocal = ref(props.sbShowEncoding)
watch(() => props.sbShowEncoding, (v) => { sbShowEncodingLocal.value = v })
watch(sbShowEncodingLocal, (v) => emit('update:sbShowEncoding', v))
const sbShowEolLocal = ref(props.sbShowEol)
watch(() => props.sbShowEol, (v) => { sbShowEolLocal.value = v })
watch(sbShowEolLocal, (v) => emit('update:sbShowEol', v))
const sbShowLangLocal = ref(props.sbShowLang)
watch(() => props.sbShowLang, (v) => { sbShowLangLocal.value = v })
watch(sbShowLangLocal, (v) => emit('update:sbShowLang', v))
const sbShowHealthLocal = ref(props.sbShowHealth)
watch(() => props.sbShowHealth, (v) => { sbShowHealthLocal.value = v })
watch(sbShowHealthLocal, (v) => emit('update:sbShowHealth', v))
const autoSwitchOutputTabLocal = ref(props.autoSwitchOutputTab)
watch(() => props.autoSwitchOutputTab, (v) => { autoSwitchOutputTabLocal.value = v })
watch(autoSwitchOutputTabLocal, (v) => emit('update:autoSwitchOutputTab', v))
const smartScrollLocal = ref(props.smartScroll)
watch(() => props.smartScroll, (v) => { smartScrollLocal.value = v })
watch(smartScrollLocal, (v) => emit('update:smartScroll', v))

// AI - 多模型管理
const modelsLocal = ref(JSON.parse(JSON.stringify(props.models || [])))
watch(() => props.models, (v) => { modelsLocal.value = JSON.parse(JSON.stringify(v || [])) }, { deep: true })
const activeModelIdLocal = ref(props.activeModelId)
watch(() => props.activeModelId, (v) => { activeModelIdLocal.value = v })

const editingModelId = ref(null)

function toggleModelEdit(id) {
  editingModelId.value = editingModelId.value === id ? null : id
}

function startAddModel() {
  const newModel = {
    id: 'm_' + Date.now(),
    name: t('settings.aiNewModel'),
    endpoint: '',
    apiKey: '',
    model: '',
    temperature: 0.7,
    contextWindow: 200000,
    maxTokens: 8192,
    supportsVision: false,
    supportsFiles: false,
  }
  modelsLocal.value.push(newModel)
  emit('add-model', newModel)
  editingModelId.value = newModel.id
  useMsg.success(t('settings.aiMsgAdded'))
}

function updateModel(m) {
  emit('update-model', m.id, {
    name: m.name,
    endpoint: m.endpoint,
    apiKey: m.apiKey,
    model: m.model,
    temperature: m.temperature,
    contextWindow: m.contextWindow,
    maxTokens: m.maxTokens,
    supportsVision: m.supportsVision,
    supportsFiles: m.supportsFiles,
  })
}

function removeModel(id) {
  if (modelsLocal.value.length <= 1) {
    useMsg.warning(t('settings.aiMsgKeepOne'))
    return
  }
  const idx = modelsLocal.value.findIndex(m => m.id === id)
  if (idx >= 0) {
    modelsLocal.value.splice(idx, 1)
    emit('delete-model', id)
    if (editingModelId.value === id) editingModelId.value = null
    useMsg.success(t('settings.aiMsgDeleted'))
  }
}

function switchToModel(id) {
  activeModelIdLocal.value = id
  emit('switch-model', id)
  useMsg.success(t('settings.aiMsgSwitched'))
}

const aiStreamLocal = ref(props.aiStream)
watch(() => props.aiStream, (v) => { aiStreamLocal.value = v })
watch(aiStreamLocal, (v) => emit('update:aiStream', v))
const aiCompressThresholdLocal = ref(props.aiCompressThreshold)
watch(() => props.aiCompressThreshold, (v) => { aiCompressThresholdLocal.value = v })
watch(aiCompressThresholdLocal, (v) => emit('update:aiCompressThreshold', v))
const aiKeepRecentLocal = ref(props.aiKeepRecent)
watch(() => props.aiKeepRecent, (v) => { aiKeepRecentLocal.value = v })
watch(aiKeepRecentLocal, (v) => emit('update:aiKeepRecent', v))

// 智能体和技能列表
const allAgentsList = computed(() => {
  const custom = props.agents || []
  return [...BUILTIN_AGENTS, ...custom.filter(a => !BUILTIN_AGENTS.some(b => b.id === a.id))]
})
const allSkillsList = computed(() => BUILTIN_SKILLS)

const themes = getThemes()

// P2-1：内置主题名（dark/light/ocean/sunrise），自定义主题不显示预览卡
const builtinThemeNames = computed(() => getThemeNames().filter(name => isBuiltInTheme(name)))

const customNames = computed(() => getThemeNames().filter(name => !isBuiltInTheme(name)))

// P2-1：主题预览卡内联样式，用主题变量还原迷你 IDE 预览
function previewStyle(name) {
  const t = getTheme(name)
  const v = t.variables
  return {
    '--pv-bg': v['--bg-primary'],
    '--pv-sidebar': v['--bg-sidebar'],
    '--pv-editor': v['--bg-secondary'],
    '--pv-titlebar': v['--toolbar-gradient'],
    '--pv-text': v['--text-secondary'],
    '--pv-accent': v['--accent-color'],
    '--pv-border': v['--border-color'],
    '--pv-menu-active': v['--bg-active']
  }
}

const varLabels = computed(() => ({
  '--bg-primary': t('settings.themeVarBgPrimary'),
  '--bg-secondary': t('settings.themeVarBgSecondary'),
  '--bg-tertiary': t('settings.themeVarBgTertiary'),
  '--bg-hover': t('settings.themeVarBgHover'),
  '--bg-active': t('settings.themeVarBgSelected'),
  '--bg-sidebar': t('settings.themeVarBgSidebar'),
  '--bg-output': t('settings.themeVarBgOutput'),
  '--bg-input': t('settings.themeVarBgInput'),
  '--border-color': t('settings.themeVarBorder'),
  '--border-light': t('settings.themeVarBorderLight'),
  '--text-primary': t('settings.themeVarTextPrimary'),
  '--text-secondary': t('settings.themeVarTextSecondary'),
  '--text-muted': t('settings.themeVarTextLabel'),
  '--text-dim': t('settings.themeVarTextNote'),
  '--text-faint': t('settings.themeVarTextPlaceholder'),
  '--text-darker': t('settings.themeVarTextContrast'),
  '--accent-color': t('settings.themeVarAccent'),
  '--accent-hover': t('settings.themeVarAccentHover'),
  '--accent-light': t('settings.themeVarAccentHighlight'),
  '--accent-bg': t('settings.themeVarAccentBg')
}))

// 编辑自定义主题时使用本地副本
const current = reactive({
  label: '',
  isDark: true,
  variables: {}
})

function syncCurrent(name) {
  const t = getTheme(name)
  current.label = t.label
  current.isDark = t.isDark
  current.variables = { ...t.variables }
}

watch(() => props.modelValue, syncCurrent, { immediate: true })

function onSelectTheme(name) {
  emit('update:modelValue', name)
  applyTheme(name)
}

function makeCustomName() {
  let i = 1
  while (getThemeNames().includes(`custom${i}`)) i++
  return `custom${i}`
}

function copyCurrentAsCustom() {
  const source = getTheme(props.modelValue)
  const name = makeCustomName()
  const theme = {
    label: source.label + t('settings.themeCopySuffix'),
    isDark: source.isDark,
    variables: { ...source.variables }
  }
  saveCustomTheme(name, theme)
  emit('update:modelValue', name)
  applyTheme(name)
}

function saveCurrent() {
  const theme = {
    label: current.label,
    isDark: current.isDark,
    variables: { ...current.variables }
  }
  saveCustomTheme(props.modelValue, theme)
  applyTheme(props.modelValue)
}

function removeCustom(name) {
  if (props.modelValue === name) {
    emit('update:modelValue', 'dark')
    applyTheme('dark')
  }
  deleteCustomTheme(name)
}

// G12：重新加载插件（重新扫描 exe 同级 plugins/ 并加载）
// 注意：hooks 与 App.vue onMounted 中保持一致，避免重复定义导致状态错乱
async function reloadPlugins() {
  try {
    await loadAllPlugins({
      output: (text) => { /* 设置面板内无 output 引用，静默 */ },
      getStatus: () => '',
      setStatus: () => {},
      getActiveFile: () => null,
      openFile: () => {},
      getProjectPath: () => '',
      callBackend: async (method, ...args) => {
        if (window.IDEService && typeof window.IDEService[method] === 'function') {
          return await window.IDEService[method](...args)
        }
        throw new Error('IDEService.' + method + ' 不存在')
      }
    })
  } catch (e) {
    console.warn(t('settings.pluginReloadFailed') + ':', e)
  }
}

// P6：项目模板自定义 - 保存当前项目为全局模板
const newTemplateName = ref('')
const newTemplateDesc = ref('')
const newTemplateIcon = ref('📦')

async function saveAsTemplate() {
  if (!newTemplateName.value.trim()) {
    useMsg.warning(t('settings.templateMsgNameEmpty'))
    return
  }
  if (!props.projectPath) {
    useMsg.warning(t('settings.templateMsgNoProject'))
    return
  }
  try {
    const err = await IDEService.SaveProjectAsTemplate(
      props.projectPath,
      newTemplateName.value.trim(),
      newTemplateDesc.value.trim(),
      newTemplateIcon.value.trim() || '📦'
    )
    if (err) {
      useMsg.error(t('settings.templateMsgSaveFailed') + ': ' + err)
    } else {
      useMsg.success(t('settings.templateMsgSaved') + ': ' + newTemplateName.value.trim())
      newTemplateName.value = ''
      newTemplateDesc.value = ''
      emit('refresh-templates')
    }
  } catch (e) {
    useMsg.error(t('settings.templateMsgSaveError') + ': ' + (e && e.message ? e.message : String(e)))
  }
}

async function deleteTemplate(dir) {
  try {
    const err = await IDEService.DeleteGlobalTemplate(dir)
    if (err) {
      useMsg.error(t('settings.templateMsgDeleteFailed') + ': ' + err)
    } else {
      useMsg.success(t('settings.templateMsgDeleted') + ': ' + dir)
      emit('refresh-templates')
    }
  } catch (e) {
    useMsg.error(t('settings.templateMsgDeleteError') + ': ' + (e && e.message ? e.message : String(e)))
  }
}
</script>

<style scoped>
.settings-panel {
  max-height: 70vh;
  overflow: hidden;
  background: transparent;
}
/* 左右分栏布局 */
.settings-layout {
  display: flex;
  height: 480px;
  gap: 12px;
}
.settings-menu {
  width: 140px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  background: var(--bg-tertiary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  overflow-y: auto;
}
.settings-menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: var(--radius-md);
  cursor: pointer;
  font-size: var(--ide-font-size);
  color: var(--text-secondary);
  transition: all 0.15s ease;
  user-select: none;
}
.settings-menu-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}
.settings-menu-item.active {
  background: var(--accent-bg);
  color: var(--accent-color);
  font-weight: 600;
  position: relative;
}
/* 蓝色高光线 — 选中态左侧 */
.settings-menu-item.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 4px;
  bottom: 4px;
  width: 3px;
  background: var(--accent-color);
  border-radius: 0 2px 2px 0;
}
.settings-menu-icon {
  font-size: 16px;
  line-height: 1;
}
.settings-menu-label {
  flex: 1;
}
.settings-content {
  flex: 1;
  overflow-y: auto;
  padding: 4px 8px;
  min-width: 0;
}
/* 编辑器设置分组网格 */
.prop-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 10px 12px;
  margin: 8px 0 12px;
}
.prop-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.prop-cell-label {
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
  font-weight: 500;
}
.prop-checks {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 16px;
  margin: 6px 0 12px;
}
.prop-checks :deep(.n-checkbox) {
  margin: 0;
}
.prop-hint {
  font-size: var(--ide-font-size-xs);
  color: var(--text-muted);
  margin: 4px 0 12px;
  padding: 6px 10px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  border-left: 2px solid var(--accent-color);
}
.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: var(--ide-font-size);
  font-weight: 600;
  margin: 16px 0 8px;
  padding: 6px 8px;
  color: var(--text-primary);
  background: var(--card-gradient);
  border-radius: 6px;
  border-left: 3px solid var(--accent-color);
}
/* G12：插件卡片列表样式 */
.plugin-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
}
.plugin-card {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--card-gradient);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  transition: all 0.15s ease;
}
.plugin-card:hover {
  border-color: var(--accent-color);
  box-shadow: var(--shadow-1);
}
.plugin-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.plugin-name {
  font-size: var(--ide-font-size);
  font-weight: 600;
  color: var(--text-primary);
}
.plugin-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 4px;
  font-size: var(--ide-font-size-xs);
  color: var(--text-secondary);
}
.plugin-desc {
  margin-top: 4px;
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
}
/* AI 模型管理卡片 */
.model-settings-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
}
.model-setting-card {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--card-gradient);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  transition: all 0.15s ease;
  overflow: hidden;
}
.model-setting-card.active {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 1px var(--accent-color);
}
.model-setting-card:hover {
  border-color: var(--accent-color);
}
.model-setting-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  cursor: pointer;
  user-select: none;
}
.model-setting-info {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}
.model-setting-name {
  font-size: var(--ide-font-size);
  font-weight: 600;
  color: var(--text-primary);
}
.model-setting-active {
  font-size: var(--ide-font-size-xs);
  color: var(--accent-color);
  font-weight: 500;
}
.model-setting-caps {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}
.model-setting-body {
  padding: 0 12px 12px;
  border-top: 1px solid var(--border-color);
  padding-top: 12px;
}
/* 智能体卡片 */
.agent-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
}
.agent-card {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--card-gradient);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}
.agent-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.agent-emoji {
  font-size: 18px;
}
.agent-name {
  font-size: var(--ide-font-size);
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
}
.agent-desc {
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
  margin-bottom: 6px;
}
.agent-keywords {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}
.agent-kw-label {
  font-size: var(--ide-font-size-xs);
  color: var(--text-dim);
}
.agent-kw {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--bg-tertiary);
  color: var(--text-dim);
}
/* 技能卡片 */
.skill-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 8px;
  margin-bottom: 12px;
}
.skill-card {
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--card-gradient);
}
.skill-card-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
}
.skill-icon {
  font-size: 16px;
}
.skill-name {
  font-size: var(--ide-font-size-sm);
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
}
.skill-desc {
  font-size: var(--ide-font-size-xs);
  color: var(--text-secondary);
}
/* P6：保存为模板输入行 */
.template-save-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}
/* P2-1：主题预览卡 — 吸取 NxEGO1，每张卡用纯 CSS 还原菜单条+侧边栏+编辑器行 */
.theme-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 10px;
  margin-bottom: 16px;
}
.theme-card {
  cursor: pointer;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  overflow: hidden;
  transition: all 0.15s ease;
  position: relative;
  background: var(--bg-tertiary);
}
.theme-card:hover {
  border-color: var(--accent-color);
  transform: translateY(-1px);
  box-shadow: var(--shadow-1);
}
.theme-card.active {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px var(--accent-bg);
}
/* 蓝色高光线（NxEGO4 精髓）— 选中态顶部 */
.theme-card.active::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: linear-gradient(90deg, transparent, var(--accent-color), transparent);
  z-index: 2;
}
.theme-card-preview {
  height: 80px;
  display: flex;
  flex-direction: column;
  background: var(--pv-bg);
  border-bottom: 1px solid var(--pv-border);
}
.preview-titlebar {
  height: 14px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 6px;
  background: var(--pv-titlebar);
  border-bottom: 1px solid var(--pv-border);
  flex-shrink: 0;
}
.preview-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--pv-text);
  opacity: 0.5;
}
.preview-title-text {
  flex: 1;
  height: 5px;
  margin-left: 6px;
  background: var(--pv-text);
  opacity: 0.2;
  border-radius: 2px;
}
.preview-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}
.preview-sidebar {
  width: 24px;
  background: var(--pv-sidebar);
  padding: 6px 4px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  border-right: 1px solid var(--pv-border);
  flex-shrink: 0;
}
.preview-menu-item {
  height: 4px;
  background: var(--pv-text);
  opacity: 0.3;
  border-radius: 2px;
}
.preview-menu-item:first-child {
  opacity: 0.6;
  background: var(--pv-accent);
}
.preview-editor {
  flex: 1;
  background: var(--pv-editor);
  padding: 8px 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.preview-code-line {
  height: 3px;
  background: var(--pv-text);
  opacity: 0.35;
  border-radius: 2px;
}
.preview-footer {
  height: 12px;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 0 6px;
  background: var(--pv-bg);
  border-top: 1px solid var(--pv-border);
  flex-shrink: 0;
}
.preview-accent-btn {
  width: 16px;
  height: 5px;
  border-radius: 2px;
  background: var(--pv-accent);
}
.theme-card-label {
  padding: 6px 8px;
  font-size: var(--ide-font-size-sm);
  font-weight: 500;
  text-align: center;
  color: var(--text-primary);
  background: var(--bg-tertiary);
}
</style>
