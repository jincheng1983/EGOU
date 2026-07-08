<template>
  <div class="window-designer">
    <aside class="designer-toolbox">
      <!-- 组件箱视图 -->
      <div v-if="sidePanel === 'toolbox'" class="toolbox-list">
        <div
          v-for="item in toolbox"
          :key="item.type"
          class="toolbox-item"
          draggable="true"
          @dragstart="onDragStart($event, item.type)"
        >
          <span v-if="item.iconSvg" class="toolbox-icon-svg" v-html="item.iconSvg"></span>
          <n-icon v-else :component="item.icon" />
          <span>{{ item.label }}</span>
        </div>
      </div>

      <!-- 模板视图 -->
      <template v-else-if="sidePanel === 'templates'">
        <div class="template-header">
          <div class="template-actions">
            <button class="tpl-action-btn" :title="t('designer.exportTemplate')" @click="exportTemplates">
              <n-icon :component="DownloadOutline" size="14" />
            </button>
            <button class="tpl-action-btn" :title="t('designer.importTemplate')" @click="importTemplates">
              <n-icon :component="CloudUploadOutline" size="14" />
            </button>
          </div>
        </div>
        <div class="toolbox-list template-list">
          <div
            v-for="(tpl, idx) in allTemplates"
            :key="tpl.name"
            class="toolbox-item template-item"
            :class="{ 'custom-template': tpl.custom }"
            draggable="true"
            :title="t('designer.dragHint', { name: tpl.name + `（${tpl.components.length}）` + (tpl.custom ? '（右键删除）' : '') })"
            @dragstart="onTemplateDragStart($event, idx)"
            @contextmenu.stop="onTemplateContextMenu($event, idx)"
          >
            <n-icon :component="tpl.icon" />
            <span>{{ tpl.name }}</span>
          </div>
        </div>
        <input ref="importInputRef" type="file" accept=".json" style="display:none" @change="onImportFileChange" />
      </template>

      <!-- 层级视图 -->
      <template v-else>
        <div class="layer-list">
          <div
            v-for="comp in layersList"
            :key="comp.id"
            class="layer-item"
            :class="{ selected: isSelected(comp.id), hidden: comp.visible === false }"
            draggable="true"
            :title="`${labelForType(comp.type)} · ${comp.name}`"
            @click="onLayerClick($event, comp)"
            @dragstart="onLayerDragStart($event, comp)"
            @dragover="onLayerDragOver($event, comp)"
            @drop="onLayerDrop($event, comp)"
            @contextmenu.stop="onComponentContextMenu($event, comp)"
          >
            <span v-if="iconSvgForType(comp.type)" class="layer-icon-svg" v-html="iconSvgForType(comp.type)"></span>
            <n-icon v-else :component="iconForType(comp.type)" size="14" />
            <span class="layer-name">{{ comp.name || labelForType(comp.type) }}</span>
            <span v-if="comp.locked" class="layer-flag locked">{{ t('designer.lock') }}</span>
            <span v-if="comp.visible === false" class="layer-flag">{{ t('designer.hide') }}</span>
            <span v-if="comp.enabled === false" class="layer-flag">{{ t('designer.disable') }}</span>
          </div>
          <div v-if="layersList.length === 0" class="layer-empty">{{ t('designer.emptyLayers') }}</div>
        </div>
      </template>
    </aside>

    <main class="designer-canvas" @click="selectForm">
      <div v-if="selectedIds.size >= 2" class="align-toolbar">
        <button class="align-btn" :title="t('designer.alignLeft')" @click="alignComponents('left')">⤛</button>
        <button class="align-btn" :title="t('designer.alignHCenter')" @click="alignComponents('hcenter')">↔</button>
        <button class="align-btn" :title="t('designer.alignRight')" @click="alignComponents('right')">⤜</button>
        <span class="align-sep" />
        <button class="align-btn" :title="t('designer.alignTop')" @click="alignComponents('top')">⤈</button>
        <button class="align-btn" :title="t('designer.alignVCenter')" @click="alignComponents('vcenter')">↕</button>
        <button class="align-btn" :title="t('designer.alignBottom')" @click="alignComponents('bottom')">⤉</button>
        <span class="align-sep" />
        <button class="align-btn" :title="t('designer.distH')" :disabled="selectedIds.size < 3" @click="distributeComponents('horizontal')">⇔</button>
        <button class="align-btn" :title="t('designer.distV')" :disabled="selectedIds.size < 3" @click="distributeComponents('vertical')">⇕</button>
      </div>
      <div
        ref="formRef"
        class="form-surface"
        :class="{ selected: selectedType === 'form' }"
        :style="formSurfaceStyle"
        @click.stop
      >
        <div class="form-titlebar" :class="{ 'no-controls': !showControls }" @mousedown.stop="startDragForm">
          <img v-if="form.icon" :src="formIconSrc" class="form-title-icon" alt="icon" @error="onFormIconError" />
          <img v-else src="/appicon.png" class="form-title-icon" alt="icon" />
          <span class="form-title">{{ form.title }}</span>
          <div v-if="showControls" class="form-titlebar-controls">
            <button
              v-if="form.minimizable"
              type="button"
              class="ctrl-btn ctrl-min"
              :title="t('designer.minimize')"
              @mousedown.stop
            >
              <svg width="10" height="10" viewBox="0 0 10 10"><rect x="1" y="7" width="8" height="1" fill="currentColor"/></svg>
            </button>
            <button
              v-if="form.maximizable"
              type="button"
              class="ctrl-btn ctrl-max"
              :title="t('designer.maximize')"
              @mousedown.stop
            >
              <svg width="10" height="10" viewBox="0 0 10 10"><rect x="1" y="1" width="8" height="8" fill="none" stroke="currentColor"/></svg>
            </button>
            <button
              v-if="form.closable !== false"
              type="button"
              class="ctrl-btn ctrl-close"
              :title="t('designer.close')"
              @mousedown.stop
            >
              <svg width="10" height="10" viewBox="0 0 10 10"><path d="M1 1 L9 9 M9 1 L1 9" stroke="currentColor" stroke-width="1" fill="none"/></svg>
            </button>
          </div>
        </div>
        <div
          ref="clientRef"
          class="form-client"
          :style="formClientStyle"
          @dragover.prevent
          @drop="onDrop"
          @mousedown="onClientMouseDown"
          @contextmenu.stop="onContextMenu"
        >
          <div
            v-for="comp in visibleComponents"
            :key="comp.id"
            :data-id="comp.id"
            class="designer-component"
            :class="{ selected: isSelected(comp.id), disabled: !comp.enabled, 'tab-order-mode': tabOrderMode, 'grouped': comp.groupId }"
            :style="[componentStyle(comp), groupStyle(comp)]"
            @mousedown.stop="startDragComponent($event, comp)"
            @click.stop="onComponentClick($event, comp)"
            @contextmenu.stop="onComponentContextMenu($event, comp)"
          >
            <component-preview :comp="comp" :external-preview="getExternalPreview(comp.type)" />
            <template v-if="isSelected(comp.id)">
              <div
                v-for="h in resizeHandles"
                :key="h.dir"
                class="resize-handle"
                :class="h.dir"
                @mousedown.stop="startResizeComponent($event, comp, h.dir)"
              />
            </template>
            <div
              v-if="tabOrderMode"
              class="tab-order-badge"
              draggable="true"
              @click.stop="setNextTabOrder(comp)"
              @dragstart="onTabOrderDragStart($event, comp)"
              @dragover.prevent="onTabOrderDragOver($event, comp)"
              @drop.stop="onTabOrderDrop($event, comp)"
            >{{ comp.tabOrder || 0 }}</div>
            <div v-if="comp.locked" class="lock-badge" :title="t('designer.locked')"><n-icon :component="LockClosedOutline" size="10" /></div>
          </div>

          <div
            v-if="marqueeVisible"
            class="marquee"
            :style="marqueeStyle"
          />

          <!-- 对齐辅助线 -->
          <div
            v-if="alignGuides.x != null"
            class="align-guide align-guide-v"
            :style="{ left: alignGuides.x + 'px' }"
          />
          <div
            v-if="alignGuides.y != null"
            class="align-guide align-guide-h"
            :style="{ top: alignGuides.y + 'px' }"
          />
          <div
            v-if="dragHint.visible"
            class="drag-hint"
            :style="{ left: dragHint.left + 'px', top: dragHint.top + 'px' }"
          >{{ dragHint.text }}</div>
          <div
            v-if="selectionBBox"
            class="selection-bbox"
            :style="{ left: selectionBBox.x + 'px', top: selectionBBox.y + 'px', width: selectionBBox.width + 'px', height: selectionBBox.height + 'px' }"
          >
            <span class="bbox-size">{{ Math.round(selectionBBox.width) }} × {{ Math.round(selectionBBox.height) }}</span>
          </div>
          <!-- 等距分布提示线：水平方向左右间距 -->
          <template v-if="distribGuides.horizontal">
            <div
              class="distrib-guide distrib-h-left"
              :style="{ left: distribGuides.horizontal.leftStart + 'px', top: distribGuides.horizontal.labelY + 'px', width: (distribGuides.horizontal.leftEnd - distribGuides.horizontal.leftStart) + 'px' }"
            />
            <div
              class="distrib-guide distrib-h-right"
              :style="{ left: distribGuides.horizontal.rightStart + 'px', top: distribGuides.horizontal.labelY + 'px', width: (distribGuides.horizontal.rightEnd - distribGuides.horizontal.rightStart) + 'px' }"
            />
            <div
              class="distrib-label"
              :style="{ left: ((distribGuides.horizontal.leftStart + distribGuides.horizontal.leftEnd) / 2) + 'px', top: distribGuides.horizontal.labelY + 'px' }"
            >{{ distribGuides.horizontal.gap }}</div>
            <div
              class="distrib-label"
              :style="{ left: ((distribGuides.horizontal.rightStart + distribGuides.horizontal.rightEnd) / 2) + 'px', top: distribGuides.horizontal.labelY + 'px' }"
            >{{ distribGuides.horizontal.gap }}</div>
          </template>
          <!-- 等距分布提示线：垂直方向上下间距 -->
          <template v-if="distribGuides.vertical">
            <div
              class="distrib-guide distrib-v-top"
              :style="{ top: distribGuides.vertical.topStart + 'px', left: distribGuides.vertical.labelX + 'px', height: (distribGuides.vertical.topEnd - distribGuides.vertical.topStart) + 'px' }"
            />
            <div
              class="distrib-guide distrib-v-bottom"
              :style="{ top: distribGuides.vertical.bottomStart + 'px', left: distribGuides.vertical.labelX + 'px', height: (distribGuides.vertical.bottomEnd - distribGuides.vertical.bottomStart) + 'px' }"
            />
            <div
              class="distrib-label"
              :style="{ top: ((distribGuides.vertical.topStart + distribGuides.vertical.topEnd) / 2) + 'px', left: distribGuides.vertical.labelX + 'px' }"
            >{{ distribGuides.vertical.gap }}</div>
            <div
              class="distrib-label"
              :style="{ top: ((distribGuides.vertical.bottomStart + distribGuides.vertical.bottomEnd) / 2) + 'px', left: distribGuides.vertical.labelX + 'px' }"
            >{{ distribGuides.vertical.gap }}</div>
          </template>
          <!-- 间距测量标签：水平方向（左/右） -->
          <template v-if="measureGuides.horizontal">
            <div
              v-if="measureGuides.horizontal.leftGap != null"
              class="measure-label"
              :style="{ left: ((measureGuides.horizontal.leftEdge + measureGuides.horizontal.firstLeft) / 2) + 'px', top: measureGuides.horizontal.labelY + 'px' }"
            >{{ Math.round(measureGuides.horizontal.leftGap) }}</div>
            <div
              v-if="measureGuides.horizontal.rightGap != null"
              class="measure-label"
              :style="{ left: ((measureGuides.horizontal.firstRight + measureGuides.horizontal.rightEdge) / 2) + 'px', top: measureGuides.horizontal.labelY + 'px' }"
            >{{ Math.round(measureGuides.horizontal.rightGap) }}</div>
          </template>
          <!-- 间距测量标签：垂直方向（上/下） -->
          <template v-if="measureGuides.vertical">
            <div
              v-if="measureGuides.vertical.topGap != null"
              class="measure-label"
              :style="{ top: ((measureGuides.vertical.topEdge + measureGuides.vertical.firstTop) / 2) + 'px', left: measureGuides.vertical.labelX + 'px' }"
            >{{ Math.round(measureGuides.vertical.topGap) }}</div>
            <div
              v-if="measureGuides.vertical.bottomGap != null"
              class="measure-label"
              :style="{ top: ((measureGuides.vertical.firstBottom + measureGuides.vertical.bottomEdge) / 2) + 'px', left: measureGuides.vertical.labelX + 'px' }"
            >{{ Math.round(measureGuides.vertical.bottomGap) }}</div>
          </template>
        </div>

        <div class="form-resize form-resize-e" @mousedown.stop="startResizeForm('e', $event)" />
        <div class="form-resize form-resize-s" @mousedown.stop="startResizeForm('s', $event)" />
        <div class="form-resize form-resize-se" @mousedown.stop="startResizeForm('se', $event)" />
      </div>
    </main>

    <aside class="designer-props">
      <div class="props-form">
        <template v-if="selectedType === 'form'">
          <div class="prop-section">{{ t('designer.secBasic') }}</div>
          <div class="prop-row prop-row-full">
            <span class="prop-label">{{ t('designer.propTitle') }}</span>
            <n-input v-model:value="form.title" size="small" @update:value="emitChange" />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propWidth') }}</span>
            <n-input-number v-model:value="form.width" size="small" :min="120" :show-button="false" @update:value="emitChange" />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propHeight') }}</span>
            <n-input-number v-model:value="form.height" size="small" :min="80" :show-button="false" @update:value="emitChange" />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propX') }}</span>
            <n-input-number v-model:value="form.x" size="small" :show-button="false" @update:value="emitChange" />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propY') }}</span>
            <n-input-number v-model:value="form.y" size="small" :show-button="false" @update:value="emitChange" />
          </div>
          <div class="prop-row">
            <span class="prop-label prop-label-wide">{{ t('designer.propMinW') }}</span>
            <n-input-number v-model:value="form.minWidth" size="small" :min="0" :show-button="false" @update:value="emitChange" />
          </div>
          <div class="prop-row">
            <span class="prop-label prop-label-wide">{{ t('designer.propMinH') }}</span>
            <n-input-number v-model:value="form.minHeight" size="small" :min="0" :show-button="false" @update:value="emitChange" />
          </div>
          <div class="prop-row">
            <span class="prop-label prop-label-wide">{{ t('designer.propMaxW') }}</span>
            <n-input-number v-model:value="form.maxWidth" size="small" :min="0" :show-button="false" @update:value="emitChange" />
          </div>
          <div class="prop-row">
            <span class="prop-label prop-label-wide">{{ t('designer.propMaxH') }}</span>
            <n-input-number v-model:value="form.maxHeight" size="small" :min="0" :show-button="false" @update:value="emitChange" />
          </div>
          <div class="prop-section">{{ t('designer.secAppearance') }}</div>
          <div class="prop-row prop-row-full">
            <span class="prop-label">{{ t('designer.propIcon') }}</span>
            <n-input-group style="width: 100%;">
              <n-input v-model:value="form.icon" size="small" :placeholder="t('designer.iconPlaceholder')" @update:value="emitChange" style="flex: 1;" />
              <n-button size="small" @click="browseFormIcon" :title="t('designer.browseIcon')">...</n-button>
              <n-button size="small" quaternary @click="form.icon = ''; emitChange()" :title="t('designer.clearIcon')">{{ t('designer.clearIconShort') }}</n-button>
            </n-input-group>
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propBgColor') }}</span>
            <n-color-picker v-model:value="form.bgColor" :show-alpha="true" :show-preview="true" size="small" class="color-block-only" @update:value="emitChange" />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propEffect') }}</span>
            <n-input-number v-model:value="form.opacity" size="small" :min="1" :max="100" :step="1" :show-button="false" @update:value="emitChange" />
          </div>
          <div class="prop-row prop-row-full">
            <span class="prop-label prop-label-wide">{{ t('designer.propBgEffect') }}</span>
            <n-select v-model:value="form.backdrop" size="small" :options="backdropOptions" @update:value="emitChange" />
          </div>
          <div class="prop-section">{{ t('designer.secBehavior') }}</div>
          <div class="prop-row row-checks row-checks-3">
            <n-checkbox v-model:checked="form.resizable" @update:checked="emitChange">{{ t('designer.propResizable') }}</n-checkbox>
            <n-checkbox v-model:checked="form.centered" @update:checked="emitChange">{{ t('designer.propCenter') }}</n-checkbox>
            <n-checkbox v-model:checked="form.minimizable" @update:checked="emitChange">{{ t('designer.propMinBtn') }}</n-checkbox>
            <n-checkbox v-model:checked="form.maximizable" @update:checked="emitChange">{{ t('designer.propMaxBtn') }}</n-checkbox>
            <n-checkbox :checked="form.closable !== false" @update:checked="v => { form.closable = v; emitChange() }">{{ t('designer.propCloseBtn') }}</n-checkbox>
            <n-checkbox v-model:checked="form.fullScreen" @update:checked="emitChange">{{ t('designer.propFullscreen') }}</n-checkbox>
            <n-checkbox v-model:checked="form.alwaysOnTop" @update:checked="emitChange">{{ t('designer.propTopmost') }}</n-checkbox>
          </div>
          <div class="prop-section">{{ t('designer.secWindowEffect') }}</div>
          <div class="prop-row row-checks row-checks-3">
            <n-checkbox v-model:checked="form.rounded" @update:checked="emitChange">{{ t('designer.propRadius') }}</n-checkbox>
            <n-checkbox v-model:checked="form.shadow" @update:checked="emitChange">{{ t('designer.propShadow') }}</n-checkbox>
          </div>
        </template>

        <template v-else-if="firstSelectedComponent">
          <div class="prop-section">{{ t('designer.secComponent') }}</div>
          <div v-if="selectedIds.size === 1" class="prop-row prop-row-full">
            <span class="prop-label">{{ t('designer.propName') }}</span>
            <n-input v-model:value="firstSelectedComponent.name" size="small" @update:value="emitChange" />
          </div>
          <div v-if="selectedIds.size > 1" class="prop-row prop-row-full">
            <span class="prop-label">{{ t('designer.propSelected') }}</span>
            <span class="prop-value">{{ t('designer.componentsCount', { count: selectedIds.size }) }}</span>
          </div>
          <div v-if="showTextProp" class="prop-row prop-row-full">
            <span class="prop-label">{{ textPropLabel }}</span>
            <n-input v-model:value="firstSelectedComponent.text" size="small" @update:value="emitChange" />
          </div>
          <div v-if="showItemsProp" class="prop-row prop-row-full">
            <span class="prop-label">{{ t('designer.propOptions') }}</span>
            <n-input
              v-model:value="firstSelectedComponent.items"
              size="small"
              type="textarea"
              :rows="3"
              :placeholder="t('designer.optionsPlaceholder')"
              @update:value="emitChange"
            />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propX') }}</span>
            <n-input-number :value="multiValue('x')" size="small" :placeholder="multiPlaceholder('x')" :show-button="false" @update:value="v => setMulti('x', v)" />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propY') }}</span>
            <n-input-number :value="multiValue('y')" size="small" :placeholder="multiPlaceholder('y')" :show-button="false" @update:value="v => setMulti('y', v)" />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propWidth') }}</span>
            <n-input-number :value="multiValue('width')" size="small" :min="10" :placeholder="multiPlaceholder('width')" :show-button="false" @update:value="v => setMulti('width', v)" />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propHeight') }}</span>
            <n-input-number :value="multiValue('height')" size="small" :min="10" :placeholder="multiPlaceholder('height')" :show-button="false" @update:value="v => setMulti('height', v)" />
          </div>
          <div v-if="selectedIds.size === 1" class="prop-row">
            <span class="prop-label prop-label-wide">{{ t('designer.propTabOrder') }}</span>
            <n-input-number v-model:value="firstSelectedComponent.tabOrder" size="small" :min="0" :show-button="false" @update:value="emitChange" />
          </div>
          <div class="prop-section">{{ t('designer.secStyle') }}</div>
          <div class="prop-row">
            <span class="prop-label prop-label-wide">{{ t('designer.propFont') }}</span>
            <n-input-number :value="multiValue('fontSize')" size="small" :min="8" :placeholder="multiPlaceholder('fontSize')" :show-button="false" @update:value="v => setMulti('fontSize', v)" />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propColor') }}</span>
            <n-color-picker :value="multiValue('color')" :show-alpha="true" :show-preview="true" size="small" class="color-block-only" @update:value="v => setMulti('color', v)" />
          </div>
          <div class="prop-row">
            <span class="prop-label">{{ t('designer.propBg') }}</span>
            <n-color-picker :value="multiValue('bgColor')" :show-alpha="true" :show-preview="true" size="small" class="color-block-only" @update:value="v => setMulti('bgColor', v)" />
          </div>
          <div class="prop-row row-checks">
            <n-checkbox :checked="multiValue('visible') === true" :indeterminate="multiValue('visible') === null" @update:checked="v => setMulti('visible', v)">{{ t('designer.propVisible') }}</n-checkbox>
            <n-checkbox :checked="multiValue('enabled') === true" :indeterminate="multiValue('enabled') === null" @update:checked="v => setMulti('enabled', v)">{{ t('designer.propEnabled') }}</n-checkbox>
          </div>

          <template v-if="selectedIds.size === 1 && currentSchema.length">
            <div class="prop-section">{{ t('designer.secAdvanced') }}</div>
            <div
              v-for="s in currentSchema"
              :key="s.key"
              class="prop-row"
              :class="{ 'prop-row-full': s.type === 'text' || s.type === 'image' }"
            >
              <span class="prop-label">{{ s.label }}</span>
              <n-select
                v-if="s.type === 'select'"
                :value="propValue(s.key, s.default)"
                size="small"
                :options="s.options"
                @update:value="v => setProp(s.key, v)"
              />
              <n-input-number
                v-else-if="s.type === 'number'"
                :value="propValue(s.key, s.default)"
                size="small"
                :min="s.min"
                :max="s.max"
                :step="s.step"
                :show-button="false"
                @update:value="v => setProp(s.key, v)"
              />
              <n-input
                v-else-if="s.type === 'text'"
                :value="propValue(s.key, s.default)"
                size="small"
                :type="s.inputType || 'text'"
                :rows="s.rows"
                @update:value="v => setProp(s.key, v)"
              />
              <n-checkbox
                v-else-if="s.type === 'bool'"
                :checked="propValue(s.key, s.default)"
                @update:checked="v => setProp(s.key, v)"
              >
                {{ s.label }}
              </n-checkbox>
              <!-- 颜色选择器：直接用颜色块表示，不显示色号 -->
              <n-color-picker
                v-else-if="s.type === 'color'"
                :value="propValue(s.key, s.default) || ''"
                :show-alpha="true"
                :show-preview="true"
                size="small"
                class="color-block-only"
                @update:value="v => setProp(s.key, v)"
              />
              <!-- 字体选择器：下拉选择常用中文字体 -->
              <n-select
                v-else-if="s.type === 'font'"
                :value="propValue(s.key, s.default)"
                size="small"
                :options="fontOptions"
                filterable
                clearable
                :placeholder="t('designer.defaultFont')"
                @update:value="v => setProp(s.key, v)"
              />
              <!-- 图片选择器：输入框 + 浏览按钮，可粘贴路径或选择本地文件 -->
              <n-input-group
                v-else-if="s.type === 'image'"
                style="width: 100%;"
              >
                <n-input
                  :value="propValue(s.key, s.default)"
                  size="small"
                  :placeholder="t('designer.imagePath')"
                  @update:value="v => setProp(s.key, v)"
                  style="flex: 1;"
                />
                <n-button size="small" @click="browseImage(s.key)" :title="t('designer.browseImage')">...</n-button>
              </n-input-group>
            </div>
          </template>

          <div v-if="selectedIds.size === 1 && compEvents[firstSelectedComponent.type]?.length" class="prop-row events-row">
            <span class="prop-label">{{ t('designer.propEvent') }}</span>
            <n-select
              size="small"
              :placeholder="t('designer.selectEvent')"
              :options="eventOptions"
              @update:value="openEvent"
            />
          </div>
        </template>

        <div v-else class="props-empty">
          <n-empty :description="t('designer.selectHint')" size="small" />
        </div>
      </div>
    </aside>

    <n-dropdown
      :show="contextMenu.visible"
      :options="contextMenuOptions"
      :x="contextMenu.x"
      :y="contextMenu.y"
      trigger="manual"
      placement="bottom-start"
      @clickoutside="hideContextMenu"
      @select="onContextMenuSelect"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { NIcon, NInput, NInputNumber, NEmpty, NButton, NCheckbox, NDropdown, NSelect, NColorPicker, NInputGroup } from 'naive-ui'
import { IDEService } from '../../bindings/egou/internal/app'
import {
  SquareOutline,
  TextOutline,
  CheckboxOutline,
  RadioButtonOffOutline,
  ListOutline,
  ChevronDownCircleOutline,
  ToggleOutline,
  OptionsOutline,
  PulseOutline,
  ImageOutline,
  CodeSlashOutline,
  AlbumsOutline,
  CardOutline,
  RemoveOutline,
  InformationCircleOutline,
  LockClosedOutline,
  PersonOutline,
  SearchOutline,
  ChatboxOutline,
  CubeOutline,
  DownloadOutline,
  CloudUploadOutline
} from '@vicons/ionicons5'
import { GROUP_COLORS } from '../utils/colors.js'
import ComponentPreview from './ComponentPreview.vue'
import { t } from '../i18n/index.js'

const props = defineProps({
  modelValue: { type: Object, default: () => null },
  showGrid: { type: Boolean, default: true },
  snapEnabled: { type: Boolean, default: true },
  tabOrderMode: { type: Boolean, default: false },
  gridSize: { type: Number, default: 8 },
  sidePanel: { type: String, default: 'toolbox' }
})
const emit = defineEmits(['update:modelValue', 'open-event', 'update:showGrid', 'update:snapEnabled', 'update:tabOrderMode', 'update:gridSize', 'update:sidePanel'])

const TITLE_HEIGHT = 32

const backdropOptions = computed(() => [
  { label: t('designer.backdropAuto'), value: 'auto' },
  { label: t('designer.backdropNone'), value: 'none' },
  { label: t('designer.backdropMica'), value: 'mica' },
  { label: t('designer.backdropAcrylic'), value: 'acrylic' },
  { label: t('designer.backdropTabbed'), value: 'tabbed' }
])

// 字体选择器常用中文字体（可搜索过滤）
const fontOptions = computed(() => [
  { label: t('designer.optDefault'), value: '' },
  { label: '微软雅黑', value: 'Microsoft YaHei' },
  { label: '宋体', value: 'SimSun' },
  { label: '黑体', value: 'SimHei' },
  { label: '楷体', value: 'KaiTi' },
  { label: '仿宋', value: 'FangSong' },
  { label: '等线', value: 'DengXian' },
  { label: 'EGOU 圆体', value: 'IdeFont' },
  { label: 'Consolas', value: 'Consolas' },
  { label: 'Courier New', value: 'Courier New' },
  { label: 'Arial', value: 'Arial' },
  { label: 'Times New Roman', value: 'Times New Roman' },
  { label: 'Segoe UI', value: 'Segoe UI' },
  { label: '苹方', value: 'PingFang SC' },
  { label: '华文细黑', value: 'STXihei' }
])

// 图片浏览：通过 IDEService.PickFilePath 选择本地图片文件
async function browseImage(propKey) {
  try {
    if (window.IDEService && typeof window.IDEService.PickFilePath === 'function') {
      const path = await window.IDEService.PickFilePath(
        t('designer.selectImage'),
        '图片文件|*.png;*.jpg;*.jpeg;*.gif;*.bmp;*.svg;*.webp|所有文件|*.*'
      )
      if (path) setProp(propKey, path)
    }
  } catch (e) {
    console.warn('[image] 浏览图片失败:', e)
  }
}

// 窗口图标浏览：通过 IDEService.PickFilePath 选择 .ico/.png 图标文件
async function browseFormIcon() {
  try {
    const path = await IDEService.PickFilePath(
      t('designer.selectIcon'),
      '图标文件|*.ico;*.png;*.bmp|所有文件|*.*'
    )
    if (path) {
      form.value.icon = path
      emitChange()
    }
  } catch (e) {
    console.warn('[icon] 浏览图标失败:', e)
  }
}

const builtinToolbox = computed(() => [
  { type: 'button', label: t('designer.compButton'), icon: SquareOutline, width: 80, height: 28, text: t('designer.compButton') },
  { type: 'edit', label: t('designer.compEdit'), icon: TextOutline, width: 120, height: 24, text: '' },
  { type: 'textarea', label: t('designer.compTextarea'), icon: CodeSlashOutline, width: 160, height: 80, text: '' },
  { type: 'label', label: t('designer.compLabel'), icon: TextOutline, width: 80, height: 20, text: t('designer.compLabel') },
  { type: 'checkbox', label: t('designer.compCheckbox'), icon: CheckboxOutline, width: 80, height: 24, text: t('designer.compCheckbox') },
  { type: 'radio', label: t('designer.compRadio'), icon: RadioButtonOffOutline, width: 80, height: 24, text: t('designer.compRadio') },
  { type: 'switch', label: t('designer.compSwitch'), icon: ToggleOutline, width: 56, height: 24, text: '' },
  { type: 'slider', label: t('designer.compSlider'), icon: OptionsOutline, width: 120, height: 28, text: '' },
  { type: 'progress', label: t('designer.compProgress'), icon: PulseOutline, width: 120, height: 20, text: '' },
  { type: 'image', label: t('designer.compImage'), icon: ImageOutline, width: 100, height: 80, text: '' },
  { type: 'listbox', label: t('designer.compListbox'), icon: ListOutline, width: 120, height: 80, text: t('designer.compListbox') },
  { type: 'combobox', label: t('designer.compCombobox'), icon: ChevronDownCircleOutline, width: 120, height: 24, text: t('designer.compCombobox') },
  { type: 'tabs', label: t('designer.compTabs'), icon: AlbumsOutline, width: 200, height: 120, text: '标签1\n标签2' },
  { type: 'card', label: t('designer.compCard'), icon: CardOutline, width: 200, height: 120, text: t('designer.compCard') },
  { type: 'divider', label: t('designer.compDivider'), icon: RemoveOutline, width: 120, height: 2, text: '' }
])

// G9：合并内置组件 + 插件注册的组件 + 外置组件包（components/ 目录）
// 外置组件结构：{ type, label, icon(SVG字符串或null), width, height, text, props, events, preview, packageDir, isExternal }
const pluginComponents = ref([])
const toolbox = computed(() => {
  const pluginItems = pluginComponents.value.map(p => ({
    type: p.type,
    label: p.label || p.type,
    icon: (p.icon && typeof p.icon === 'object') ? p.icon : CubeOutline, // 内置图标用 n-icon component
    iconSvg: (typeof p.icon === 'string') ? p.icon : null, // G9：外置 SVG 字符串图标
    width: p.width || 80,
    height: p.height || 28,
    text: p.text || p.label || p.type,
    preview: p.preview || null // G9：预览 HTML 模板
  }))
  return [...builtinToolbox.value, ...pluginItems]
})

// G9：注册插件/外置组件（供 loader.js 和 App.vue 调用）
// def 结构：
//   - 插件组件：{ type, label, icon, width, height, text, pluginName }
//   - 外置组件：{ type, label, icon, width, height, text, props, events, packageDir, isExternal }
// 外置组件的 props（schema 数组）和 events 会动态注册到 propSchemasExternal / compEventsExternal
function registerPluginComponent(def) {
  if (!def || !def.type) return
  // 避免重复注册同 type
  const idx = pluginComponents.value.findIndex(p => p.type === def.type)
  if (idx >= 0) {
    pluginComponents.value.splice(idx, 1, def)
  } else {
    pluginComponents.value.push(def)
  }
  // 外置组件：注册属性 schema 和事件
  if (def.isExternal) {
    if (Array.isArray(def.props)) {
      propSchemasExternal.value[def.type] = def.props
    }
    if (Array.isArray(def.events)) {
      compEventsExternal.value[def.type] = def.events
    }
  }
}
function clearPluginComponents() {
  pluginComponents.value = []
  propSchemasExternal.value = {}
  compEventsExternal.value = {}
}

// P3：外置组件的属性 schema 和事件（从 components/ 目录加载）
// 与内置的 propSchemas / compEvents 合并使用，外置优先级高（允许覆盖内置）
const propSchemasExternal = ref({})
const compEventsExternal = ref({})

// ===== 组件模板库 =====
// 预设常用 UI 组合，一键拖入即可使用。每个模板定义一组组件及其相对位置/尺寸。
// 拖入时以鼠标位置为左上角原点，按相对偏移批量创建组件。
const componentTemplates = computed(() => [
  {
    name: t('designer.tplLogin'),
    icon: PersonOutline,
    components: [
      { type: 'label', text: '用户名：', dx: 0, dy: 0, width: 60, height: 20 },
      { type: 'edit', text: '', dx: 64, dy: -2, width: 140, height: 24 },
      { type: 'label', text: '密码：', dx: 0, dy: 30, width: 60, height: 20 },
      { type: 'edit', text: '', dx: 64, dy: 28, width: 140, height: 24 },
      { type: 'button', text: '登录', dx: 64, dy: 60, width: 60, height: 28 },
      { type: 'button', text: '取消', dx: 132, dy: 60, width: 60, height: 28 }
    ]
  },
  {
    name: t('designer.tplSearch'),
    icon: SearchOutline,
    components: [
      { type: 'edit', text: '', dx: 0, dy: 0, width: 200, height: 24 },
      { type: 'button', text: '搜索', dx: 206, dy: 0, width: 60, height: 24 }
    ]
  },
  {
    name: t('designer.tplConfirmDlg'),
    icon: ChatboxOutline,
    components: [
      { type: 'label', text: '提示内容', dx: 0, dy: 0, width: 200, height: 20 },
      { type: 'button', text: '确定', dx: 40, dy: 30, width: 60, height: 28 },
      { type: 'button', text: '取消', dx: 110, dy: 30, width: 60, height: 28 }
    ]
  },
  {
    name: t('designer.tplFormRow'),
    icon: TextOutline,
    components: [
      { type: 'label', text: '标签：', dx: 0, dy: 0, width: 60, height: 20 },
      { type: 'edit', text: '', dx: 64, dy: -2, width: 160, height: 24 }
    ]
  },
  {
    name: t('designer.tplCheckboxGroup'),
    icon: CheckboxOutline,
    components: [
      { type: 'checkbox', text: '选项1', dx: 0, dy: 0, width: 70, height: 24 },
      { type: 'checkbox', text: '选项2', dx: 74, dy: 0, width: 70, height: 24 },
      { type: 'checkbox', text: '选项3', dx: 148, dy: 0, width: 70, height: 24 }
    ]
  },
  {
    name: t('designer.tplRadioGroup'),
    icon: RadioButtonOffOutline,
    components: [
      { type: 'radio', text: '选项1', dx: 0, dy: 0, width: 70, height: 24 },
      { type: 'radio', text: '选项2', dx: 74, dy: 0, width: 70, height: 24 },
      { type: 'radio', text: '选项3', dx: 148, dy: 0, width: 70, height: 24 }
    ]
  },
  {
    name: t('designer.tplProgressLabel'),
    icon: PulseOutline,
    components: [
      { type: 'label', text: '进度：', dx: 0, dy: 0, width: 50, height: 20 },
      { type: 'progress', text: '', dx: 54, dy: 0, width: 160, height: 20 }
    ]
  },
  {
    name: t('designer.tplToolbar'),
    icon: OptionsOutline,
    components: [
      { type: 'button', text: '新建', dx: 0, dy: 0, width: 50, height: 24 },
      { type: 'button', text: '编辑', dx: 54, dy: 0, width: 50, height: 24 },
      { type: 'button', text: '删除', dx: 108, dy: 0, width: 50, height: 24 }
    ]
  },
  {
    name: t('designer.tplInputGroup'),
    icon: CardOutline,
    components: [
      { type: 'label', text: '宽度：', dx: 0, dy: 0, width: 50, height: 20 },
      { type: 'edit', text: '100', dx: 54, dy: -2, width: 80, height: 24 },
      { type: 'label', text: 'px', dx: 138, dy: 0, width: 24, height: 20 }
    ]
  },
  {
    name: t('designer.tplStatusbar'),
    icon: InformationCircleOutline,
    components: [
      { type: 'label', text: '就绪', dx: 0, dy: 0, width: 100, height: 20 },
      { type: 'label', text: '行 1 / 列 1', dx: 200, dy: 0, width: 80, height: 20 }
    ]
  },
  {
    name: t('designer.tplListTitle'),
    icon: ListOutline,
    components: [
      { type: 'label', text: '项目列表：', dx: 0, dy: 0, width: 80, height: 20 },
      { type: 'listbox', text: '项目1\n项目2\n项目3', dx: 0, dy: 24, width: 160, height: 80 }
    ]
  },
  {
    name: t('designer.tplImageDesc'),
    icon: ImageOutline,
    components: [
      { type: 'image', text: '', dx: 0, dy: 0, width: 80, height: 80 },
      { type: 'label', text: '图片说明', dx: 0, dy: 84, width: 80, height: 20 }
    ]
  }
])

// ===== 自定义模板（localStorage 持久化）=====
// 用户可右键选中组件 → 「保存为模板…」→ 输入名称 → 存入 localStorage。
// 自定义模板与内置 componentTemplates 合并展示，支持右键删除。
const CUSTOM_TPL_KEY = 'eg-designer-custom-templates'
const customTemplates = ref(loadCustomTemplates())
// 合并后的模板列表：内置 + 自定义，供模板分区渲染
const allTemplates = computed(() => [...componentTemplates.value, ...customTemplates.value])

function loadCustomTemplates() {
  try {
    const raw = localStorage.getItem(CUSTOM_TPL_KEY)
    return raw ? JSON.parse(raw) : []
  } catch { return [] }
}
function saveCustomTemplates() {
  try {
    localStorage.setItem(CUSTOM_TPL_KEY, JSON.stringify(customTemplates.value))
  } catch {}
}
// 把当前选中的组件保存为新模板：以选中组件的最小 x/y 为原点，计算各组件相对偏移
function saveSelectionAsTemplate() {
  const sel = selectedList.value
  if (sel.length === 0) return
  const name = window.prompt(t('designer.tplPromptName'), t('designer.tplPromptDefault', { n: customTemplates.value.length + 1 }))
  if (!name || !name.trim()) return
  const minX = Math.min(...sel.map(c => c.x || 0))
  const minY = Math.min(...sel.map(c => c.y || 0))
  const comps = sel.map(c => ({
    type: c.type,
    text: c.text || '',
    dx: (c.x || 0) - minX,
    dy: (c.y || 0) - minY,
    width: c.width || 80,
    height: c.height || 24
  }))
  customTemplates.value.push({
    name: name.trim(),
    icon: CubeOutline, // 自定义模板统一用 Cube 图标
    custom: true,
    components: comps
  })
  saveCustomTemplates()
}
function deleteCustomTemplate(idx) {
  // idx 是在 allTemplates 中的索引，需转为 customTemplates 中的索引
  const customIdx = idx - componentTemplates.value.length
  if (customIdx < 0 || customIdx >= customTemplates.value.length) return
  if (!window.confirm(t('designer.tplConfirmDelete', { name: customTemplates.value[customIdx].name }))) return
  customTemplates.value.splice(customIdx, 1)
  saveCustomTemplates()
}

// ===== 模板导入导出 =====
const importInputRef = ref(null)

function exportTemplates() {
  if (customTemplates.value.length === 0) {
    window.alert(t('designer.tplNoExport'))
    return
  }
  // 仅导出可序列化字段（去掉 icon 组件引用）
  const data = customTemplates.value.map(t => ({
    name: t.name,
    custom: true,
    components: t.components
  }))
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  const d = new Date()
  const stamp = `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`
  a.download = `eg-templates-${stamp}.json`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

function importTemplates() {
  importInputRef.value?.click()
}

function onImportFileChange(e) {
  const file = e.target.files && e.target.files[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    try {
      const data = JSON.parse(reader.result)
      if (!Array.isArray(data)) {
        window.alert(t('designer.tplInvalidFormat'))
        return
      }
      let added = 0
      let skipped = 0
      for (const tpl of data) {
        if (!tpl || !tpl.name || !Array.isArray(tpl.components)) { skipped++; continue }
        // 去重：同名跳过
        if (customTemplates.value.some(t => t.name === tpl.name)) { skipped++; continue }
        customTemplates.value.push({
          name: tpl.name,
          icon: CubeOutline,
          custom: true,
          components: tpl.components
        })
        added++
      }
      saveCustomTemplates()
      if (added > 0) {
        window.alert(t('designer.tplImported', { count: added, skipped: skipped > 0 ? t('designer.tplImportedSkipped', { count: skipped }) : '' }))
      } else {
        window.alert(t('designer.tplNoneImported', { count: skipped }))
      }
    } catch (err) {
      window.alert(t('designer.tplImportFailed'))
    }
    // 重置 input 以便重复导入同一文件
    e.target.value = ''
  }
  reader.readAsText(file)
}

const compEvents = computed(() => ({
  button: [t('designer.eventClick')],
  edit: [t('designer.eventChange'), t('designer.eventFocus'), t('designer.eventBlur')],
  textarea: [t('designer.eventChange'), t('designer.eventFocus'), t('designer.eventBlur')],
  checkbox: [t('designer.eventStateChange')],
  radio: [t('designer.eventStateChange')],
  switch: [t('designer.eventStateChange')],
  slider: [t('designer.eventValueChange')],
  image: [t('designer.eventClick')],
  listbox: [t('designer.eventOptionChange')],
  combobox: [t('designer.eventOptionChange')],
  tabs: [t('designer.eventOptionChange')],
  card: [t('designer.eventClick')]
}))

const resizeHandles = [
  { dir: 'nw' }, { dir: 'n' }, { dir: 'ne' },
  { dir: 'w' }, { dir: 'e' },
  { dir: 'sw' }, { dir: 's' }, { dir: 'se' }
]

const propSchemas = computed(() => ({
  button: [
    { key: 'type', label: t('designer.propSType'), type: 'select', default: 'default', options: [
      { label: t('designer.optDefault'), value: 'default' }, { label: t('designer.optPrimary'), value: 'primary' },
      { label: t('designer.optSuccess'), value: 'success' }, { label: t('designer.optWarning'), value: 'warning' },
      { label: t('designer.optError'), value: 'error' }, { label: t('designer.optInfo'), value: 'info' }
    ] },
    { key: 'size', label: t('designer.propSSize'), type: 'select', default: 'medium', options: [
      { label: t('designer.optTiny'), value: 'tiny' }, { label: t('designer.optSmall'), value: 'small' },
      { label: t('designer.optMedium'), value: 'medium' }, { label: t('designer.optLarge'), value: 'large' }
    ] },
    { key: 'ghost', label: t('designer.propSGhost'), type: 'bool', default: false },
    { key: 'dashed', label: t('designer.propSDashed'), type: 'bool', default: false },
    { key: 'round', label: t('designer.propSRound'), type: 'bool', default: false },
    { key: 'circle', label: t('designer.propSCircle'), type: 'bool', default: false },
    { key: 'borderRadius', label: t('designer.propSBorderRadius'), type: 'number', default: 0, min: 0 },
    { key: 'borderWidth', label: t('designer.propSBorderWidth'), type: 'number', default: 1, min: 0 },
    { key: 'fontWeight', label: t('designer.propSFontWeight'), type: 'select', default: 'normal', options: [
      { label: t('designer.optNormal'), value: 'normal' }, { label: t('designer.optBold'), value: 'bold' }, { label: t('designer.optLighter'), value: 'lighter' }
    ] }
  ],
  edit: [
    { key: 'placeholder', label: t('designer.propSPlaceholder'), type: 'text', default: '' },
    { key: 'inputType', label: t('designer.propSInputType'), type: 'select', default: 'text', options: [
      { label: t('designer.optText'), value: 'text' }, { label: t('designer.optPassword'), value: 'password' }, { label: t('designer.optTextarea'), value: 'textarea' }
    ] },
    { key: 'readonly', label: t('designer.propSReadonly'), type: 'bool', default: false },
    { key: 'clearable', label: t('designer.propSClearable'), type: 'bool', default: false },
    { key: 'maxlength', label: t('designer.propSMaxlength'), type: 'number', default: undefined }
  ],
  textarea: [
    { key: 'placeholder', label: t('designer.propSPlaceholder'), type: 'text', default: '' },
    { key: 'readonly', label: t('designer.propSReadonly'), type: 'bool', default: false },
    { key: 'maxlength', label: t('designer.propSMaxlength'), type: 'number', default: undefined },
    { key: 'rows', label: t('designer.propSRows'), type: 'number', default: 3, min: 1 }
  ],
  label: [
    { key: 'textAlign', label: t('designer.propSTextAlign'), type: 'select', default: 'left', options: [
      { label: t('designer.optAlignLeft'), value: 'left' }, { label: t('designer.optAlignCenter'), value: 'center' }, { label: t('designer.optAlignRight'), value: 'right' }
    ] },
    { key: 'fontWeight', label: t('designer.propSFontWeight'), type: 'select', default: 'normal', options: [
      { label: t('designer.optNormal'), value: 'normal' }, { label: t('designer.optBold'), value: 'bold' }, { label: t('designer.optLighter'), value: 'lighter' }
    ] },
    { key: 'fontFamily', label: t('designer.propSFontFamily'), type: 'font', default: '' },
    { key: 'lineHeight', label: t('designer.propSLineHeight'), type: 'text', default: '' }
  ],
  checkbox: [
    { key: 'checked', label: t('designer.propSChecked'), type: 'bool', default: false },
    { key: 'size', label: t('designer.propSSize'), type: 'select', default: 'medium', options: [
      { label: t('designer.optSmall'), value: 'small' }, { label: t('designer.optMedium'), value: 'medium' }, { label: t('designer.optLarge'), value: 'large' }
    ] }
  ],
  radio: [
    { key: 'checked', label: t('designer.propSChecked'), type: 'bool', default: false },
    { key: 'size', label: t('designer.propSSize'), type: 'select', default: 'medium', options: [
      { label: t('designer.optSmall'), value: 'small' }, { label: t('designer.optMedium'), value: 'medium' }, { label: t('designer.optLarge'), value: 'large' }
    ] }
  ],
  listbox: [
    { key: 'bordered', label: t('designer.propSBordered'), type: 'bool', default: true },
    { key: 'multiple', label: t('designer.propSMultiple'), type: 'bool', default: false }
  ],
  combobox: [
    { key: 'filterable', label: t('designer.propSFilterable'), type: 'bool', default: false },
    { key: 'clearable', label: t('designer.propSClearable'), type: 'bool', default: false },
    { key: 'multiple', label: t('designer.propSMultiple'), type: 'bool', default: false },
    { key: 'bordered', label: t('designer.propSBordered'), type: 'bool', default: true }
  ],
  switch: [
    { key: 'checked', label: t('designer.propSChecked'), type: 'bool', default: false },
    { key: 'size', label: t('designer.propSSize'), type: 'select', default: 'medium', options: [
      { label: t('designer.optSmall'), value: 'small' }, { label: t('designer.optMedium'), value: 'medium' }, { label: t('designer.optLarge'), value: 'large' }
    ] },
    { key: 'round', label: t('designer.propSRound'), type: 'bool', default: true }
  ],
  slider: [
    { key: 'value', label: t('designer.propSValue'), type: 'number', default: 0 },
    { key: 'min', label: t('designer.propSMin'), type: 'number', default: 0 },
    { key: 'max', label: t('designer.propSMax'), type: 'number', default: 100 },
    { key: 'step', label: t('designer.propSStep'), type: 'number', default: 1 },
    { key: 'vertical', label: t('designer.propSVertical'), type: 'bool', default: false }
  ],
  progress: [
    { key: 'percentage', label: t('designer.propSPercentage'), type: 'number', default: 0, min: 0, max: 100 },
    { key: 'progressType', label: t('designer.propSProgressType'), type: 'select', default: 'line', options: [
      { label: t('designer.optProgressLine'), value: 'line' }, { label: t('designer.optProgressCircle'), value: 'circle' }, { label: t('designer.optProgressDashboard'), value: 'dashboard' }
    ] },
    { key: 'status', label: t('designer.propSStatus'), type: 'select', default: 'default', options: [
      { label: t('designer.optDefault'), value: 'default' }, { label: t('designer.optSuccess'), value: 'success' },
      { label: t('designer.optWarning'), value: 'warning' }, { label: t('designer.optError'), value: 'error' }
    ] },
    { key: 'color', label: t('designer.propColor'), type: 'color', default: '' }
  ],
  image: [
    { key: 'src', label: t('designer.propSSrc'), type: 'image', default: '' },
    { key: 'objectFit', label: t('designer.propSObjectFit'), type: 'select', default: 'contain', options: [
      { label: t('designer.optFitFill'), value: 'fill' }, { label: t('designer.optFitContain'), value: 'contain' },
      { label: t('designer.optFitCover'), value: 'cover' }, { label: t('designer.optFitNone'), value: 'none' }, { label: t('designer.optFitScaleDown'), value: 'scale-down' }
    ] },
    { key: 'borderRadius', label: t('designer.propSBorderRadius'), type: 'number', default: 0, min: 0 }
  ],
  tabs: [
    { key: 'tabsType', label: t('designer.propSTabsType'), type: 'select', default: 'line', options: [
      { label: t('designer.optTabsLine'), value: 'line' }, { label: t('designer.optTabsCard'), value: 'card' }, { label: t('designer.optTabsSegment'), value: 'segment' }
    ] },
    { key: 'animated', label: t('designer.propSAnimated'), type: 'bool', default: true }
  ],
  card: [
    { key: 'bordered', label: t('designer.propSBordered'), type: 'bool', default: true },
    { key: 'hoverable', label: t('designer.propSHoverable'), type: 'bool', default: false },
    { key: 'size', label: t('designer.propSSize'), type: 'select', default: 'medium', options: [
      { label: t('designer.optSmall'), value: 'small' }, { label: t('designer.optMedium'), value: 'medium' },
      { label: t('designer.optLarge'), value: 'large' }, { label: t('designer.optHuge'), value: 'huge' }
    ] },
    { key: 'headerExtra', label: t('designer.propSHeaderExtra'), type: 'text', default: '' }
  ],
  divider: [
    { key: 'dashed', label: t('designer.propSDashed'), type: 'bool', default: false },
    { key: 'vertical', label: t('designer.propSVertical'), type: 'bool', default: false }
  ]
}))

function defaultProps(type) {
  // 外置组件 schema 优先，回退内置 schema
  const schema = propSchemasExternal.value[type] || propSchemas.value[type] || []
  const props = {}
  for (const s of schema) {
    if (s.default !== undefined) {
      props[s.key] = s.default
    }
  }
  return props
}

function defaultDesign() {
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

const design = ref(defaultDesign())
const form = computed(() => design.value.form || {})
const components = computed(() => design.value.components || [])

// 网格吸附配置
// 网格/吸附/Tab顺序状态从 props 初始化，双向同步到父组件
const gridSize = ref(props.gridSize)       // 网格大小（像素）
const snapEnabled = ref(props.snapEnabled) // 是否启用网格吸附
const showGrid = ref(props.showGrid)       // 是否显示网格线（独立于吸附开关）
const tabOrderMode = ref(props.tabOrderMode) // Tab 顺序编辑模式
watch(gridSize, (v) => emit('update:gridSize', v))
watch(snapEnabled, (v) => emit('update:snapEnabled', v))
watch(showGrid, (v) => emit('update:showGrid', v))
watch(tabOrderMode, (v) => emit('update:tabOrderMode', v))
watch(() => props.gridSize, (v) => { if (v !== gridSize.value) gridSize.value = v })
watch(() => props.snapEnabled, (v) => { if (v !== snapEnabled.value) snapEnabled.value = v })
watch(() => props.showGrid, (v) => { if (v !== showGrid.value) showGrid.value = v })
watch(() => props.tabOrderMode, (v) => { if (v !== tabOrderMode.value) tabOrderMode.value = v })

// 将坐标吸附到网格
function snapToGrid(v) {
  if (!snapEnabled.value || gridSize.value <= 1) return v
  return Math.round(v / gridSize.value) * gridSize.value
}

// 设置下一个 Tab 顺序：点击组件时，将其 tabOrder 设为当前最大值 + 1
function setNextTabOrder(comp) {
  const maxOrder = components.value.reduce((m, c) => Math.max(m, c.tabOrder || 0), 0)
  comp.tabOrder = maxOrder + 1
  pushHistory()
  emitChange()
}

// 重置所有组件的 Tab 顺序为 0
function resetTabOrder() {
  for (const c of components.value) {
    c.tabOrder = 0
  }
  pushHistory()
  emitChange()
}

// 自动按位置排序：先按 y（从上到下），同 y 按 x（从左到右），赋值 1..N
function autoSortTabOrder() {
  const visible = components.value.filter(c => c.visible !== false)
  // 复制一份避免排序过程中引用变化
  const sorted = visible.slice().sort((a, b) => {
    const dy = (a.y || 0) - (b.y || 0)
    if (Math.abs(dy) > 5) return dy // 行高阈值 5px，同行视为 y 相等
    return (a.x || 0) - (b.x || 0)
  })
  sorted.forEach((c, i) => { c.tabOrder = i + 1 })
  pushHistory()
  emitChange()
}

// ===== Tab 顺序拖拽排序 =====
// 拖拽 tab-order-badge 可重新排序：把源组件的 tabOrder 与目标组件的 tabOrder 交换，
// 然后按 tabOrder 重新连续编号（1..N），消除空缺。
let tabOrderDragSource = null
function onTabOrderDragStart(e, comp) {
  tabOrderDragSource = comp
  e.dataTransfer.effectAllowed = 'move'
  // Firefox 需要设置 data 才能触发 dragstart
  try { e.dataTransfer.setData('text/plain', String(comp.id)) } catch (_) {}
}
function onTabOrderDragOver(e, comp) {
  if (!tabOrderDragSource || tabOrderDragSource.id === comp.id) return
  e.dataTransfer.dropEffect = 'move'
}
function onTabOrderDrop(e, comp) {
  if (!tabOrderDragSource || tabOrderDragSource.id === comp.id) {
    tabOrderDragSource = null
    return
  }
  // 交换两者的 tabOrder
  const srcOrder = tabOrderDragSource.tabOrder || 0
  const dstOrder = comp.tabOrder || 0
  tabOrderDragSource.tabOrder = dstOrder
  comp.tabOrder = srcOrder
  // 重新连续编号：按 tabOrder 升序排列后赋值 1..N
  const visible = components.value.filter(c => c.visible !== false)
  const sorted = visible.slice().sort((a, b) => (a.tabOrder || 0) - (b.tabOrder || 0))
  sorted.forEach((c, i) => { c.tabOrder = i + 1 })
  pushHistory()
  emitChange()
  tabOrderDragSource = null
}
const selectedType = ref('form')
const selectedIds = ref(new Set())
let idCounter = 1

// ===== 撤销/重做 =====
// undoStack / redoStack 存储的是 design 的深拷贝快照。
// pushHistory 在「会改变 design 的操作」之前调用，把当前状态入栈作为可回滚点。
const undoStack = []
const redoStack = []
const MAX_HISTORY = 50
let historySuspended = false

function snapshot() {
  return JSON.parse(JSON.stringify(design.value))
}

function pushHistory() {
  if (historySuspended) return
  undoStack.push(snapshot())
  if (undoStack.length > MAX_HISTORY) undoStack.shift()
  redoStack.length = 0
}

function canUndo() { return undoStack.length > 0 }
function canRedo() { return redoStack.length > 0 }

function undo() {
  if (undoStack.length === 0) return
  redoStack.push(snapshot())
  const prev = undoStack.pop()
  historySuspended = true
  design.value = prev
  // 恢复 idCounter，避免新组件 id 冲突
  idCounter = components.value.reduce((m, c) => Math.max(m, c.id || 0), 0) + 1
  // 清理已不存在的选中项，避免残留高亮
  const validIds = new Set(components.value.map(c => c.id))
  for (const id of [...selectedIds.value]) {
    if (!validIds.has(id)) selectedIds.value.delete(id)
  }
  // 清除对齐辅助线残留
  alignGuides.value = { x: null, y: null }
  historySuspended = false
  emitChange()
}

function redo() {
  if (redoStack.length === 0) return
  undoStack.push(snapshot())
  const next = redoStack.pop()
  historySuspended = true
  design.value = next
  idCounter = components.value.reduce((m, c) => Math.max(m, c.id || 0), 0) + 1
  const validIds = new Set(components.value.map(c => c.id))
  for (const id of [...selectedIds.value]) {
    if (!validIds.has(id)) selectedIds.value.delete(id)
  }
  alignGuides.value = { x: null, y: null }
  historySuspended = false
  emitChange()
}

defineExpose({ undo, redo, canUndo, canRedo, autoSortTabOrder, resetTabOrder, registerPluginComponent, clearPluginComponents })

watch(() => props.modelValue, (val) => {
  if (val) {
    // 用 defaultDesign() 合并缺字段，避免上游传进来的 form 字段不全
    // 导致 formSurfaceStyle/formClientStyle 生成 'undefinedpx' 非法宽高
    const defaults = defaultDesign()
    design.value = {
      form: { ...defaults.form, ...(val.form || {}) },
      components: Array.isArray(val.components) ? JSON.parse(JSON.stringify(val.components)) : []
    }
    idCounter = components.value.reduce((m, c) => Math.max(m, c.id || 0), 0) + 1
    for (const c of components.value) {
      if (!c.props) c.props = defaultProps(c.type)
    }
  }
}, { immediate: true, deep: true })

function emitChange() {
  emit('update:modelValue', JSON.parse(JSON.stringify(design.value)))
}

const visibleComponents = computed(() => components.value.filter(c => c.visible !== false))
const selectedList = computed(() => components.value.filter(c => selectedIds.value.has(c.id)))
// 选中多组件时的包围盒（用于显示选中范围尺寸）
const selectionBBox = computed(() => {
  const sel = selectedList.value
  if (sel.length < 2) return null
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
  for (const c of sel) {
    const x = c.x || 0, y = c.y || 0, w = c.width || 0, h = c.height || 0
    if (x < minX) minX = x
    if (y < minY) minY = y
    if (x + w > maxX) maxX = x + w
    if (y + h > maxY) maxY = y + h
  }
  return { x: minX, y: minY, width: maxX - minX, height: maxY - minY }
})
const firstSelectedComponent = computed(() =>
  selectedType.value === 'component' ? selectedList.value[0] : null
)

// 多选批量编辑：获取公共属性值（一致返回该值，不一致返回 null）
function multiValue(key) {
  const sel = selectedList.value
  if (sel.length === 0) return null
  const first = sel[0][key]
  for (let i = 1; i < sel.length; i++) {
    if (sel[i][key] !== first) return null
  }
  return first
}
function multiPlaceholder(key) {
  return multiValue(key) === null ? '多值' : ''
}
// 多选批量编辑：设置公共属性值（应用到所有选中组件，记录撤销点）
function setMulti(key, val) {
  pushHistory()
  for (const c of selectedList.value) {
    c[key] = val
  }
  emitChange()
}

// 是否显示标题栏的最小化/最大化/关闭按钮（任一启用即显示控制器区）
const showControls = computed(() => {
  if (selectedType.value !== 'form') return true
  return form.value.minimizable || form.value.maximizable || form.value.closable !== false
})

// 窗口图标预览源：form.icon 非空时转为可访问的 src（绝对路径用 file://，相对路径用 file://+项目路径）
// 实际编译产物的标题栏图标来自 .syso 资源（编译选项中的图标设置）
const formIconSrc = computed(() => {
  const p = form.value.icon
  if (!p) return ''
  // 绝对路径：Windows 路径转 file:// URL
  if (/^[A-Za-z]:[\\/]/.test(p)) {
    return 'file:///' + p.replace(/\\/g, '/')
  }
  // 相对路径：相对项目根目录（由后端提供，前端无项目路径则原样）
  return 'file://./' + p
})

// 图标加载失败时回退到 IDE 默认图标
function onFormIconError(e) {
  e.target.src = '/appicon.png'
}

const currentSchema = computed(() => {
  if (!firstSelectedComponent.value) return []
  const t = firstSelectedComponent.value.type
  // 外置组件 schema 优先（允许覆盖内置同名组件的 schema）
  return propSchemasExternal.value[t] || propSchemas.value[t] || []
})

const eventOptions = computed(() => {
  if (!firstSelectedComponent.value) return []
  const t = firstSelectedComponent.value.type
  const events = compEventsExternal.value[t] || compEvents.value[t] || []
  return events.map(evt => ({ label: evt, value: evt }))
})

const showTextProp = computed(() => {
  const comp = firstSelectedComponent.value
  if (!comp) return false
  return !['divider', 'progress', 'slider', 'image'].includes(comp.type)
})

const textPropLabel = computed(() => {
  const comp = firstSelectedComponent.value
  if (!comp) return t('designer.textText')
  if (comp.type === 'card') return t('designer.textTitle')
  if (comp.type === 'tabs') return t('designer.textTabs')
  return t('designer.textText')
})

const showItemsProp = computed(() => {
  const comp = firstSelectedComponent.value
  if (!comp) return false
  return ['listbox', 'combobox', 'tabs'].includes(comp.type)
})

function propValue(key, def) {
  const comp = firstSelectedComponent.value
  if (!comp) return def
  return comp.props?.[key] ?? def
}

function coerceValue(schema, value) {
  if (!schema) return value
  switch (schema.type) {
    case 'number':
      if (value === '' || value === null || value === undefined) return schema.default ?? 0
      const n = Number(value)
      return Number.isNaN(n) ? (schema.default ?? 0) : n
    case 'bool':
      return Boolean(value)
    case 'text':
    case 'select':
      return value === null || value === undefined ? (schema.default ?? '') : String(value)
    default:
      return value
  }
}

function setProp(key, value) {
  const comp = firstSelectedComponent.value
  if (!comp) return
  if (!comp.props) comp.props = {}
  const schema = currentSchema.value.find(s => s.key === key)
  comp.props[key] = coerceValue(schema, value)
  pushHistory()
  emitChange()
}

function updateComponentProp(key, value) {
  const list = selectedList.value
  if (!list || list.length === 0) return
  pushHistory()
  for (const comp of list) {
    comp[key] = value
  }
  emitChange()
}

const formSurfaceStyle = computed(() => {
  // 效果强度 1-100：值越大，背景越透明，Mica/Acrylic 效果越明显
  const intensity = clampIntensity(form.value.opacity)
  const rounded = !!form.value.rounded
  const shadow = !!form.value.shadow
  const backdrop = form.value.backdrop || 'auto'
  // 数值兜底：即使 form 字段缺失也不会生成 'undefinedpx'
  const w = Number(form.value.width) || 538
  const h = Number(form.value.height) || 350
  const style = {
    width: w + 'px',
    height: h + TITLE_HEIGHT + 'px',
  }
  // 圆角
  if (rounded) {
    style.borderRadius = '8px'
  } else {
    style.borderRadius = '0'
  }
  // 阴影
  if (shadow) {
    style.boxShadow = '0 4px 20px rgba(0, 0, 0, 0.25)'
  } else {
    style.boxShadow = 'none'
  }
  // 背景效果模拟（在设计器中用 CSS 近似展示 Mica/Acrylic/Tabbed）
  // 效果强度只对 mica / acrylic / tabbed 生效；强度越大，背景越透明
  if (backdrop === 'mica') {
    // Mica：浅蓝色半透明，受效果强度影响
    const a = (intensity / 100).toFixed(2)
    style.background = `linear-gradient(135deg, rgba(220, 232, 246, ${a}), rgba(198, 215, 234, ${a}))`
  } else if (backdrop === 'acrylic') {
    // Acrylic：白色半透明 + 模糊，受效果强度影响
    const a = (intensity / 100).toFixed(2)
    style.background = `linear-gradient(135deg, rgba(255, 255, 255, ${a}), rgba(220, 220, 220, ${Math.max(0.1, intensity / 200).toFixed(2)}))`
    if (intensity > 30) {
      style.backdropFilter = 'blur(18px) saturate(140%)'
      style.webkitBackdropFilter = 'blur(18px) saturate(140%)'
    } else {
      style.backdropFilter = 'none'
    }
  } else if (backdrop === 'tabbed') {
    // Tabbed：灰白垂直渐变，受效果强度影响
    const a = (intensity / 100).toFixed(2)
    style.background = `linear-gradient(180deg, rgba(245, 245, 245, ${a}) 0%, rgba(232, 232, 232, ${a}) 100%)`
  }
  return style
})

function clampIntensity(value) {
  const n = Number(value)
  if (Number.isNaN(n)) return 100
  return Math.max(1, Math.min(100, n))
}

const formClientStyle = computed(() => {
  const w = Number(form.value.width) || 538
  const h = Number(form.value.height) || 350
  const style = {
    width: w + 'px',
    height: h + 'px',
    background: form.value.bgColor
  }
  // 网格线可视化：showGrid 开启且 gridSize > 1 时叠加网格
  if (showGrid.value && gridSize.value > 1) {
    const g = gridSize.value
    style.backgroundImage = (
      'linear-gradient(to right, rgba(128,128,128,0.18) 1px, transparent 1px),' +
      'linear-gradient(to bottom, rgba(128,128,128,0.18) 1px, transparent 1px)'
    )
    style.backgroundSize = g + 'px ' + g + 'px'
    style.backgroundPosition = '0 0'
  }
  return style
})

function componentStyle(comp) {
  return {
    left: comp.x + 'px',
    top: comp.y + 'px',
    width: comp.width + 'px',
    height: comp.height + 'px',
    fontSize: (comp.fontSize || 12) + 'px',
    color: comp.color || '#1f2329',
    background: comp.bgColor || 'transparent',
    opacity: comp.enabled === false ? 0.6 : 1
  }
}

function isSelected(id) {
  return selectedIds.value.has(id)
}

// ===== 组件分组（Group / Ungroup）=====
// 同组组件共享 groupId；点击组内任一组件自动选中整组，拖动时通过多选机制整体移动。
// 视觉上用虚线 outline 标识同组，颜色由 groupId 哈希决定。
// GROUP_COLORS 已抽离到 utils/colors.js（规约 §2 公共常量集中）
function hashGroupId(gid) {
  let h = 0
  for (let i = 0; i < gid.length; i++) h = (h * 31 + gid.charCodeAt(i)) | 0
  return Math.abs(h)
}
function groupStyle(comp) {
  if (!comp.groupId) return {}
  const idx = hashGroupId(comp.groupId) % GROUP_COLORS.length
  return { outline: '2px dashed ' + GROUP_COLORS[idx], outlineOffset: '1px' }
}
// 组合选中的组件：分配相同的新 groupId
function groupSelected() {
  if (selectedIds.value.size < 2) return
  const gid = 'g' + Date.now()
  pushHistory()
  for (const c of components.value) {
    if (selectedIds.value.has(c.id)) c.groupId = gid
  }
  emitChange()
}
// 取消组合：清除选中组件的 groupId
function ungroupSelected() {
  let changed = false
  pushHistory()
  for (const c of components.value) {
    if (selectedIds.value.has(c.id) && c.groupId) {
      delete c.groupId
      changed = true
    }
  }
  if (changed) emitChange()
}

function selectForm() {
  selectedType.value = 'form'
  selectedIds.value.clear()
  hideContextMenu()
}

function selectComponent(id, append = false) {
  selectedType.value = 'component'
  if (append) {
    if (selectedIds.value.has(id)) {
      selectedIds.value.delete(id)
    } else {
      selectedIds.value.add(id)
    }
  } else {
    selectedIds.value.clear()
    selectedIds.value.add(id)
  }
}

function onComponentClick(e, comp) {
  hideContextMenu()
  if (dragMoved) {
    e.stopPropagation()
    return
  }
  const append = e.ctrlKey || e.metaKey
  // 非追加点击且有 groupId：自动选中整组（同 groupId 的所有组件）
  if (!append && comp.groupId) {
    selectedIds.value.clear()
    for (const c of components.value) {
      if (c.groupId === comp.groupId) selectedIds.value.add(c.id)
    }
    selectedType.value = 'component'
    return
  }
  selectComponent(comp.id, append)
}

function onDragStart(e, type) {
  e.dataTransfer.setData('component-type', type)
}

// 模板拖拽：用独立 key 区分，drop 时按相对偏移批量创建组件
function onTemplateDragStart(e, tplIdx) {
  e.dataTransfer.setData('component-template', String(tplIdx))
}
// 模板右键菜单：仅自定义模板可删除
function onTemplateContextMenu(e, idx) {
  e.preventDefault()
  const tpl = allTemplates.value[idx]
  if (!tpl || !tpl.custom) return
  deleteCustomTemplate(idx)
}

// ===== 组件层级面板 =====
// components 数组顺序即 z-order（后者在上层）；层级面板倒序展示（最上层在最前）。
const layersList = computed(() => components.value.slice().reverse())

function labelForType(type) {
  return toolbox.value.find(t => t.type === type)?.label || type
}

function iconForType(type) {
  return toolbox.value.find(t => t.type === type)?.icon || null
}

// G9：获取外置组件的 SVG 图标字符串（层级列表用）
function iconSvgForType(type) {
  return toolbox.value.find(t => t.type === type)?.iconSvg || null
}

// G9：获取外置组件的 preview 配置（传给 ComponentPreview）
function getExternalPreview(type) {
  return toolbox.value.find(t => t.type === type)?.preview || null
}

function onLayerClick(e, comp) {
  hideContextMenu()
  selectComponent(comp.id, e.ctrlKey || e.metaKey || e.shiftKey)
}

let layerDragId = null

function onLayerDragStart(e, comp) {
  layerDragId = comp.id
  e.dataTransfer.effectAllowed = 'move'
  e.dataTransfer.setData('text/plain', String(comp.id))
}

function onLayerDragOver(e, comp) {
  if (layerDragId == null || layerDragId === comp.id) return
  e.preventDefault()
  e.dataTransfer.dropEffect = 'move'
}

function onLayerDrop(e, targetComp) {
  if (layerDragId == null) return
  e.preventDefault()
  const fromId = layerDragId
  const toId = targetComp.id
  layerDragId = null
  if (fromId === toId) return
  const arr = design.value.components
  const fromIdx = arr.findIndex(c => c.id === fromId)
  const toIdx = arr.findIndex(c => c.id === toId)
  if (fromIdx < 0 || toIdx < 0) return
  pushHistory()
  const [moved] = arr.splice(fromIdx, 1)
  arr.splice(toIdx, 0, moved)
  emitChange()
}

function createComponent(type, x, y) {
  const template = toolbox.value.find(t => t.type === type)
  if (!template) return null
  const comp = {
    id: idCounter++,
    type: template.type,
    name: `${template.type}${idCounter}`,
    text: template.text,
    x,
    y,
    width: template.width,
    height: template.height,
    visible: true,
    enabled: true,
    fontSize: 12,
    color: '#1f2329',
    bgColor: '',
    props: defaultProps(type)
  }
  if (['listbox', 'combobox', 'tabs'].includes(type)) {
    comp.items = template.text
  }
  return comp
}

function onDrop(e) {
  const client = e.currentTarget.getBoundingClientRect()
  const dropX = e.clientX - client.left
  const dropY = e.clientY - client.top

  // 优先检查模板拖入
  const tplIdxRaw = e.dataTransfer.getData('component-template')
  if (tplIdxRaw !== '') {
    const tpl = allTemplates.value[parseInt(tplIdxRaw, 10)]
    if (!tpl) return
    pushHistory()
    const createdIds = []
    for (const spec of tpl.components) {
      const comp = createComponent(spec.type, dropX + spec.dx, dropY + spec.dy)
      if (!comp) continue
      // 应用模板预设尺寸和文本（覆盖 toolbox 默认值）
      if (spec.width) comp.width = spec.width
      if (spec.height) comp.height = spec.height
      if (spec.text !== undefined) comp.text = spec.text
      if (['listbox', 'combobox', 'tabs'].includes(spec.type) && spec.text) {
        comp.items = spec.text
      }
      components.value.push(comp)
      createdIds.push(comp.id)
    }
    // 选中刚创建的所有组件，便于后续整体移动
    selectedIds.value = new Set(createdIds)
    selectedType.value = createdIds.length === 1 ? 'component' : 'multi'
    emitChange()
    return
  }

  // 单组件拖入
  const type = e.dataTransfer.getData('component-type')
  if (!type) return
  const comp = createComponent(type, dropX, dropY)
  if (!comp) return
  pushHistory()
  components.value.push(comp)
  selectComponent(comp.id)
  emitChange()
}

// 锁定/解锁选中组件：锁定后不可拖动和调整大小，防止误操作
function lockSelected() {
  for (const c of selectedList.value) {
    c.locked = true
  }
  pushHistory()
  emitChange()
}
function unlockSelected() {
  for (const c of selectedList.value) {
    c.locked = false
  }
  pushHistory()
  emitChange()
}
// 切换锁定状态：如果全部已锁定则解锁，否则全部锁定
function toggleLockSelected() {
  const allLocked = selectedList.value.length > 0 && selectedList.value.every(c => c.locked)
  for (const c of selectedList.value) {
    c.locked = !allLocked
  }
  pushHistory()
  emitChange()
}

function removeSelected() {
  pushHistory()
  design.value.components = components.value.filter(c => !selectedIds.value.has(c.id))
  selectedIds.value.clear()
  selectedType.value = 'form'
  emitChange()
}

// ===== Z-Order 层级调整 =====
// components.value 数组顺序即 z-order：数组末尾 = 最上层。
// layersList 用 reverse() 显示，所以层级面板第一行对应最上层组件。

// 置顶：把所有选中组件按原顺序移到数组末尾
function bringToFront() {
  if (selectedIds.value.size === 0) return
  pushHistory()
  const sel = components.value.filter(c => selectedIds.value.has(c.id))
  const rest = components.value.filter(c => !selectedIds.value.has(c.id))
  design.value.components = [...rest, ...sel]
  emitChange()
}

// 置底：把所有选中组件按原顺序移到数组开头
function sendToBack() {
  if (selectedIds.value.size === 0) return
  pushHistory()
  const sel = components.value.filter(c => selectedIds.value.has(c.id))
  const rest = components.value.filter(c => !selectedIds.value.has(c.id))
  design.value.components = [...sel, ...rest]
  emitChange()
}

// 前置：每个选中组件在数组中向后移一位（更靠上层）
function bringForward() {
  if (selectedIds.value.size === 0) return
  pushHistory()
  const arr = components.value.slice()
  // 从后往前遍历，避免交换后索引错乱
  for (let i = arr.length - 2; i >= 0; i--) {
    if (selectedIds.value.has(arr[i].id) && !selectedIds.value.has(arr[i + 1].id)) {
      const tmp = arr[i]
      arr[i] = arr[i + 1]
      arr[i + 1] = tmp
    }
  }
  design.value.components = arr
  emitChange()
}

// 后置：每个选中组件在数组中向前移一位（更靠下层）
function sendBackward() {
  if (selectedIds.value.size === 0) return
  pushHistory()
  const arr = components.value.slice()
  // 从前往后遍历
  for (let i = 1; i < arr.length; i++) {
    if (selectedIds.value.has(arr[i].id) && !selectedIds.value.has(arr[i - 1].id)) {
      const tmp = arr[i]
      arr[i] = arr[i - 1]
      arr[i - 1] = tmp
    }
  }
  design.value.components = arr
  emitChange()
}

// ===== 复制 / 粘贴 / 原地复制 =====
let clipboard = [] // 存储深拷贝的组件快照
let styleClipboard = null // 存储样式属性快照（复制样式/粘贴样式用）

// 样式属性白名单：复制样式时只复制这些字段，不复制位置/大小/名称/事件
const STYLE_KEYS = [
  'color', 'bgColor', 'fontSize', 'fontFamily', 'fontWeight',
  'borderColor', 'borderWidth', 'borderStyle', 'borderRadius',
  'opacity', 'textAlign', 'textDecoration', 'lineHeight',
  'padding', 'margin', 'shadow', 'enabled', 'visible'
]

function copySelected() {
  const sel = components.value.filter(c => selectedIds.value.has(c.id))
  if (sel.length === 0) return
  clipboard = sel.map(c => JSON.parse(JSON.stringify(c)))
}

function copyStyle() {
  const sel = components.value.filter(c => selectedIds.value.has(c.id))
  if (sel.length === 0) return
  // 取第一个选中组件的样式属性
  const src = sel[0]
  const style = {}
  for (const k of STYLE_KEYS) {
    if (src[k] !== undefined) style[k] = src[k]
  }
  styleClipboard = style
}

function pasteStyle() {
  if (!styleClipboard) return
  const sel = components.value.filter(c => selectedIds.value.has(c.id))
  if (sel.length === 0) return
  pushHistory()
  for (const c of sel) {
    for (const k of STYLE_KEYS) {
      if (styleClipboard[k] !== undefined) {
        c[k] = styleClipboard[k]
      }
    }
  }
  emitChange()
}

function paste() {
  if (clipboard.length === 0) return
  pushHistory()
  const newIds = []
  for (const snap of clipboard) {
    const comp = JSON.parse(JSON.stringify(snap))
    comp.id = idCounter++
    comp.x = (comp.x || 0) + 20
    comp.y = (comp.y || 0) + 20
    comp.name = `${comp.type}${idCounter}`
    components.value.push(comp)
    newIds.push(comp.id)
  }
  selectedIds.value = new Set(newIds)
  selectedType.value = 'component'
  emitChange()
}

function duplicateSelected() {
  const sel = components.value.filter(c => selectedIds.value.has(c.id))
  if (sel.length === 0) return
  pushHistory()
  const newIds = []
  for (const src of sel) {
    const comp = JSON.parse(JSON.stringify(src))
    comp.id = idCounter++
    comp.x = (comp.x || 0) + 20
    comp.y = (comp.y || 0) + 20
    comp.name = `${comp.type}${idCounter}`
    components.value.push(comp)
    newIds.push(comp.id)
  }
  selectedIds.value = new Set(newIds)
  selectedType.value = 'component'
  emitChange()
}

// ===== 对齐 / 分布 =====
// 以选中组件的边界框为基准对齐。
function alignComponents(mode) {
  const sel = components.value.filter(c => selectedIds.value.has(c.id))
  if (sel.length < 2) return
  pushHistory()
  const xs = sel.map(c => c.x || 0)
  const ys = sel.map(c => c.y || 0)
  const rights = sel.map(c => (c.x || 0) + (c.width || 0))
  const bottoms = sel.map(c => (c.y || 0) + (c.height || 0))
  const minX = Math.min(...xs)
  const maxX = Math.max(...rights)
  const minY = Math.min(...ys)
  const maxY = Math.max(...bottoms)
  for (const c of sel) {
    const w = c.width || 0
    const h = c.height || 0
    switch (mode) {
      case 'left': c.x = minX; break
      case 'right': c.x = maxX - w; break
      case 'hcenter': c.x = (minX + maxX) / 2 - w / 2; break
      case 'top': c.y = minY; break
      case 'bottom': c.y = maxY - h; break
      case 'vcenter': c.y = (minY + maxY) / 2 - h / 2; break
    }
  }
  emitChange()
}

// 等间距分布：首尾不动，中间组件均匀分布。
function distributeComponents(axis) {
  const sel = components.value.filter(c => selectedIds.value.has(c.id))
  if (sel.length < 3) return
  pushHistory()
  const sorted = sel.slice().sort((a, b) => {
    const av = axis === 'horizontal' ? (a.x || 0) : (a.y || 0)
    const bv = axis === 'horizontal' ? (b.x || 0) : (b.y || 0)
    return av - bv
  })
  const first = sorted[0]
  const last = sorted[sorted.length - 1]
  if (axis === 'horizontal') {
    const start = first.x || 0
    const end = (last.x || 0) + (last.width || 0)
    const totalSpan = end - start
    const sumSize = sorted.reduce((s, c) => s + (c.width || 0), 0)
    const gap = (totalSpan - sumSize) / (sorted.length - 1)
    let cursor = start + (first.width || 0) + gap
    for (let i = 1; i < sorted.length - 1; i++) {
      sorted[i].x = cursor
      cursor += (sorted[i].width || 0) + gap
    }
  } else {
    const start = first.y || 0
    const end = (last.y || 0) + (last.height || 0)
    const totalSpan = end - start
    const sumSize = sorted.reduce((s, c) => s + (c.height || 0), 0)
    const gap = (totalSpan - sumSize) / (sorted.length - 1)
    let cursor = start + (first.height || 0) + gap
    for (let i = 1; i < sorted.length - 1; i++) {
      sorted[i].y = cursor
      cursor += (sorted[i].height || 0) + gap
    }
  }
  emitChange()
}

function getCompEl(id) {
  return formRef.value?.querySelector(`[data-id="${id}"]`)
}

let dragState = null
let dragMoved = false
const DRAG_THRESHOLD = 2

// 对齐辅助线：拖动过程中，若被拖组件的边缘 / 中心与其它组件或客户区边缘/中心在阈值内对齐，
// 显示贯穿客户区的虚线，并把位置吸附到该参考线。
const alignGuides = ref({ x: null, y: null })

// 等距分布提示线：拖动时，若被拖组件处于两个相邻组件之间且左右（或上下）间距接近相等，
// 显示间距标签并吸附到精确等距。horizontal/left/right 描述水平方向上的左右间距；
// vertical/top/bottom 描述垂直方向上的上下间距。
const distribGuides = ref({ horizontal: null, vertical: null })

// 间距测量提示：拖动时显示 first 组件与最近邻居（左/右/上/下各一）的间距值
const measureGuides = ref({ horizontal: null, vertical: null })

// 拖动/resize 时的坐标尺寸提示
const dragHint = ref({ visible: false, text: '', left: 0, top: 0 })

// 等距分布检测：给定被拖组件 first 及当前选中集，查找在水平/垂直方向上夹住 first 的两个最近未选中组件，
// 若两侧间距接近相等（阈值内），返回吸附偏移与显示用的 guide 信息。
// - 水平：左侧组件的 right 与 first.left 构成 leftGap，first.right 与右侧组件的 left 构成 rightGap
// - 垂直：上方组件的 bottom 与 first.top 构成 topGap，first.bottom 与下方组件的 top 构成 bottomGap
// 仅当左右（或上下）两个邻居都存在且 Y（或 X）方向有重叠时才检测，避免误判。
const DISTRIB_THRESHOLD = 5 // 间距差阈值（像素）
function computeDistribGuides(first, selIds) {
  const fx = first.x || 0, fy = first.y || 0
  const fw = first.width || 0, fh = first.height || 0
  const fRight = fx + fw, fBottom = fy + fh
  // 候选：未选中、可见的组件
  const others = components.value.filter(c => !selIds.has(c.id) && c.visible !== false)
  const result = { snapDx: 0, snapDy: 0, guides: { horizontal: null, vertical: null } }

  // 水平方向：找 first 左侧最近（right 最大且 < fx）和右侧最近（left 最小且 > fRight）的组件，
  // 且要求它们与 first 在 Y 方向有重叠（方便对齐场景下使用）
  let leftNeighbor = null, rightNeighbor = null
  for (const c of others) {
    const cy0 = c.y || 0, cy1 = (c.y || 0) + (c.height || 0)
    // Y 方向重叠判定
    if (cy1 < fy || cy0 > fBottom) continue
    const cRight = (c.x || 0) + (c.width || 0)
    if (cRight <= fx) {
      // 在 first 左侧
      if (!leftNeighbor || cRight > ((leftNeighbor.x || 0) + (leftNeighbor.width || 0))) leftNeighbor = c
    } else if ((c.x || 0) >= fRight) {
      // 在 first 右侧
      if (!rightNeighbor || (c.x || 0) < (rightNeighbor.x || 0)) rightNeighbor = c
    }
  }
  if (leftNeighbor && rightNeighbor) {
    const leftGap = fx - ((leftNeighbor.x || 0) + (leftNeighbor.width || 0))
    const rightGap = (rightNeighbor.x || 0) - fRight
    if (leftGap > 0 && rightGap > 0 && Math.abs(leftGap - rightGap) <= DISTRIB_THRESHOLD) {
      // 吸附到精确等距：把 first 平移使 leftGap == rightGap == 平均值
      const avg = (leftGap + rightGap) / 2
      const newFx = (leftNeighbor.x || 0) + (leftNeighbor.width || 0) + avg
      const snapDx = newFx - fx
      if (Math.abs(snapDx) > 0) result.snapDx = snapDx
      // guide：水平方向的左右间距标签
      const lNeighbor_right = (leftNeighbor.x || 0) + (leftNeighbor.width || 0)
      const rNeighbor_left = rightNeighbor.x || 0
      const labelY = Math.max(fy, (leftNeighbor.y || 0), (rightNeighbor.y || 0))
      result.guides.horizontal = {
        leftStart: lNeighbor_right,
        leftEnd: newFx,
        rightStart: newFx + fw,
        rightEnd: rNeighbor_left,
        gap: Math.round(avg),
        labelY
      }
    }
  }

  // 垂直方向：找 first 上方最近和下方最近的组件（要求 X 方向有重叠）
  let topNeighbor = null, bottomNeighbor = null
  for (const c of others) {
    const cx0 = c.x || 0, cx1 = (c.x || 0) + (c.width || 0)
    if (cx1 < fx || cx0 > fRight) continue
    const cBottom = (c.y || 0) + (c.height || 0)
    if (cBottom <= fy) {
      if (!topNeighbor || cBottom > ((topNeighbor.y || 0) + (topNeighbor.height || 0))) topNeighbor = c
    } else if ((c.y || 0) >= fBottom) {
      if (!bottomNeighbor || (c.y || 0) < (bottomNeighbor.y || 0)) bottomNeighbor = c
    }
  }
  if (topNeighbor && bottomNeighbor) {
    const topGap = fy - ((topNeighbor.y || 0) + (topNeighbor.height || 0))
    const bottomGap = (bottomNeighbor.y || 0) - fBottom
    if (topGap > 0 && bottomGap > 0 && Math.abs(topGap - bottomGap) <= DISTRIB_THRESHOLD) {
      const avg = (topGap + bottomGap) / 2
      const newFy = (topNeighbor.y || 0) + (topNeighbor.height || 0) + avg
      const snapDy = newFy - fy
      if (Math.abs(snapDy) > 0) result.snapDy = snapDy
      const tNeighbor_bottom = (topNeighbor.y || 0) + (topNeighbor.height || 0)
      const bNeighbor_top = bottomNeighbor.y || 0
      const labelX = Math.max(fx, (topNeighbor.x || 0), (bottomNeighbor.x || 0))
      result.guides.vertical = {
        topStart: tNeighbor_bottom,
        topEnd: newFy,
        bottomStart: newFy + fh,
        bottomEnd: bNeighbor_top,
        gap: Math.round(avg),
        labelX
      }
    }
  }
  return result
}

// 间距测量：查找 first 在水平/垂直方向上最近的邻居（每侧一个），返回间距值与位置信息用于显示标签。
// 与 computeDistribGuides 不同，这里只显示间距值，不做吸附，且每侧独立查找（不需要两侧同时存在）。
// 仅当间距 <= MEASURE_MAX（50px）时才显示，避免远处组件也显示测量值造成干扰。
const MEASURE_MAX = 50
function computeMeasureGuides(first, selIds) {
  const fx = first.x || 0, fy = first.y || 0
  const fw = first.width || 0, fh = first.height || 0
  const fRight = fx + fw, fBottom = fy + fh
  const others = components.value.filter(c => !selIds.has(c.id) && c.visible !== false)
  const result = { horizontal: null, vertical: null }

  // 水平方向：找 first 左侧最近和右侧最近的组件（Y 方向有重叠）
  let leftN = null, rightN = null
  for (const c of others) {
    const cy0 = c.y || 0, cy1 = (c.y || 0) + (c.height || 0)
    if (cy1 < fy || cy0 > fBottom) continue
    const cRight = (c.x || 0) + (c.width || 0)
    if (cRight <= fx) {
      const gap = fx - cRight
      if (gap <= MEASURE_MAX && (!leftN || gap < leftN.gap)) {
        leftN = { gap, edge: cRight, compY: cy0, compH: cy1 - cy0 }
      }
    } else if ((c.x || 0) >= fRight) {
      const gap = (c.x || 0) - fRight
      if (gap <= MEASURE_MAX && (!rightN || gap < rightN.gap)) {
        rightN = { gap, edge: c.x || 0, compY: cy0, compH: cy1 - cy0 }
      }
    }
  }
  if (leftN || rightN) {
    result.horizontal = {
      leftGap: leftN ? leftN.gap : null,
      leftEdge: leftN ? leftN.edge : null,
      rightGap: rightN ? rightN.gap : null,
      rightEdge: rightN ? rightN.edge : null,
      firstLeft: fx,
      firstRight: fRight,
      // 标签 Y 坐标取 first 的垂直中点
      labelY: fy + Math.round(fh / 2)
    }
  }

  // 垂直方向：找 first 上方最近和下方最近的组件（X 方向有重叠）
  let topN = null, bottomN = null
  for (const c of others) {
    const cx0 = c.x || 0, cx1 = (c.x || 0) + (c.width || 0)
    if (cx1 < fx || cx0 > fRight) continue
    const cBottom = (c.y || 0) + (c.height || 0)
    if (cBottom <= fy) {
      const gap = fy - cBottom
      if (gap <= MEASURE_MAX && (!topN || gap < topN.gap)) {
        topN = { gap, edge: cBottom, compX: cx0, compW: cx1 - cx0 }
      }
    } else if ((c.y || 0) >= fBottom) {
      const gap = (c.y || 0) - fBottom
      if (gap <= MEASURE_MAX && (!bottomN || gap < bottomN.gap)) {
        bottomN = { gap, edge: c.y || 0, compX: cx0, compW: cx1 - cx0 }
      }
    }
  }
  if (topN || bottomN) {
    result.vertical = {
      topGap: topN ? topN.gap : null,
      topEdge: topN ? topN.edge : null,
      bottomGap: bottomN ? bottomN.gap : null,
      bottomEdge: bottomN ? bottomN.edge : null,
      firstTop: fy,
      firstBottom: fBottom,
      // 标签 X 坐标取 first 的水平中点
      labelX: fx + Math.round(fw / 2)
    }
  }
  return result
}

function startDragComponent(e, comp) {
  hideContextMenu()
  // 锁定的组件不可拖动
  if (comp.locked) return
  if (!isSelected(comp.id)) {
    selectComponent(comp.id, e.ctrlKey || e.metaKey)
  }

  const startPositions = {}
  for (const c of selectedList.value) {
    startPositions[c.id] = { x: c.x, y: c.y }
  }

  dragState = {
    startX: e.clientX,
    startY: e.clientY,
    startPositions,
    moving: false
  }
  dragMoved = false
  alignGuides.value = { x: null, y: null }
  distribGuides.value = { horizontal: null, vertical: null }
  measureGuides.value = { horizontal: null, vertical: null }

  const SNAP_THRESHOLD = 6 // 对齐吸附阈值（像素）

  // 收集「参考边缘」：其它未选中组件的 x / x+w / y / y+h / 中心x / 中心y，
  // 加上客户区的 0 / 宽 / 高 / 中心
  function getReferenceEdges() {
    const edges = { xs: [], ys: [] }
    const clientW = form.value.width
    const clientH = form.value.height
    // 客户区参考
    edges.xs.push({ x: 0, label: 'left' })
    edges.xs.push({ x: clientW, label: 'right' })
    edges.xs.push({ x: Math.round(clientW / 2), label: 'center' })
    edges.ys.push({ y: 0, label: 'top' })
    edges.ys.push({ y: clientH, label: 'bottom' })
    edges.ys.push({ y: Math.round(clientH / 2), label: 'center' })
    // 其它组件参考
    for (const c of components.value) {
      if (selectedIds.value.has(c.id)) continue
      if (c.visible === false) continue
      edges.xs.push({ x: c.x, label: 'left' })
      edges.xs.push({ x: c.x + c.width, label: 'right' })
      edges.xs.push({ x: c.x + Math.round(c.width / 2), label: 'center' })
      edges.ys.push({ y: c.y, label: 'top' })
      edges.ys.push({ y: c.y + c.height, label: 'bottom' })
      edges.ys.push({ y: c.y + Math.round(c.height / 2), label: 'center' })
    }
    return edges
  }

  const onMove = (ev) => {
    if (!dragState) return
    let dx = ev.clientX - dragState.startX
    let dy = ev.clientY - dragState.startY
    if (!dragState.moving) {
      if (Math.abs(dx) <= DRAG_THRESHOLD && Math.abs(dy) <= DRAG_THRESHOLD) return
    }
    dragState.moving = true
    dragMoved = true

    // 实时更新位置（而非只 transform），便于对齐辅助线计算
    const refs = getReferenceEdges()
    let snapDx = 0, snapDy = 0
    // 以第一个选中组件为基准进行吸附
    const first = selectedList.value[0]
    if (first) {
      const baseX = startPositions[first.id].x + dx
      const baseY = startPositions[first.id].y + dy
      const baseRight = baseX + first.width
      const baseBottom = baseY + first.height
      const baseCenterX = baseX + Math.round(first.width / 2)
      const baseCenterY = baseY + Math.round(first.height / 2)
      // 找最近的 X 吸附
      let bestX = null, bestXDist = SNAP_THRESHOLD + 1
      for (const e of refs.xs) {
        const candidates = [
          { target: e.x, source: baseX, kind: 'left' },
          { target: e.x, source: baseRight, kind: 'right' },
          { target: e.x, source: baseCenterX, kind: 'center' }
        ]
        for (const cand of candidates) {
          const d = Math.abs(cand.source - cand.target)
          if (d < bestXDist) {
            bestXDist = d
            bestX = { target: e.x, delta: cand.target - cand.source, kind: cand.kind }
          }
        }
      }
      // 找最近的 Y 吸附
      let bestY = null, bestYDist = SNAP_THRESHOLD + 1
      for (const e of refs.ys) {
        const candidates = [
          { target: e.y, source: baseY, kind: 'top' },
          { target: e.y, source: baseBottom, kind: 'bottom' },
          { target: e.y, source: baseCenterY, kind: 'center' }
        ]
        for (const cand of candidates) {
          const d = Math.abs(cand.source - cand.target)
          if (d < bestYDist) {
            bestYDist = d
            bestY = { target: e.y, delta: cand.target - cand.source, kind: cand.kind }
          }
        }
      }
      if (bestX && bestXDist <= SNAP_THRESHOLD) {
        snapDx = bestX.delta
        alignGuides.value.x = bestX.target
      } else {
        alignGuides.value.x = null
      }
      if (bestY && bestYDist <= SNAP_THRESHOLD) {
        snapDy = bestY.delta
        alignGuides.value.y = bestY.target
      } else {
        alignGuides.value.y = null
      }
    }
    dx += snapDx
    dy += snapDy
    for (const c of selectedList.value) {
      // 网格吸附：如果未触发对齐吸附（snapDx/snapDy=0），则吸附到网格
      const targetX = dragState.startPositions[c.id].x + dx
      const targetY = dragState.startPositions[c.id].y + dy
      c.x = snapEnabled.value && snapDx === 0 ? snapToGrid(targetX) : targetX
      c.y = snapEnabled.value && snapDy === 0 ? snapToGrid(targetY) : targetY
    }
    // 等距分布检测：以第一个选中组件为基准，找左右/上下最近的有 Y/X 重叠的未选中组件，
    // 若两侧间距接近相等（阈值内），吸附到精确等距并显示间距标签。
    if (first) {
      const distrib = computeDistribGuides(first, selectedIds.value)
      if (distrib.snapDx) {
        for (const c of selectedList.value) c.x = (dragState.startPositions[c.id].x + dx) + distrib.snapDx
      }
      if (distrib.snapDy) {
        for (const c of selectedList.value) c.y = (dragState.startPositions[c.id].y + dy) + distrib.snapDy
      }
      distribGuides.value = distrib.guides
      // 间距测量：独立于等距检测，仅显示间距值不做吸附
      measureGuides.value = computeMeasureGuides(first, selectedIds.value)
    }
    // 显示拖动坐标提示（复用已声明的 first 变量）
    if (first) {
      dragHint.value = {
        visible: true,
        text: Math.round(first.x || 0) + ', ' + Math.round(first.y || 0),
        left: (first.x || 0) + (first.width || 0) / 2,
        top: (first.y || 0) > 20 ? (first.y || 0) - 20 : (first.y || 0) + (first.height || 0) + 4
      }
    }
  }

  const onUp = (ev) => {
    if (!dragState) return
    const moved = dragState.moving
    alignGuides.value = { x: null, y: null }
    distribGuides.value = { horizontal: null, vertical: null }
    measureGuides.value = { horizontal: null, vertical: null }
    dragHint.value = { visible: false, text: '', left: 0, top: 0 }
    dragState = null
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
    setTimeout(() => { dragMoved = false }, 0)
    if (moved) { pushHistory(); emitChange() }
  }

  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

function startResizeComponent(e, comp, dir) {
  hideContextMenu()
  // 锁定的组件不可调整大小
  if (comp.locked) return
  selectComponent(comp.id)
  const startX = e.clientX
  const startY = e.clientY
  const startRect = { x: comp.x, y: comp.y, width: comp.width, height: comp.height }
  const minSize = 8

  const onMove = (ev) => {
    const dx = ev.clientX - startX
    const dy = ev.clientY - startY
    let { x, y, width, height } = startRect

    if (dir.includes('e')) width = Math.max(minSize, startRect.width + dx)
    if (dir.includes('s')) height = Math.max(minSize, startRect.height + dy)
    if (dir.includes('w')) {
      const newW = Math.max(minSize, startRect.width - dx)
      x = startRect.x + startRect.width - newW
      width = newW
    }
    if (dir.includes('n')) {
      const newH = Math.max(minSize, startRect.height - dy)
      y = startRect.y + startRect.height - newH
      height = newH
    }

    // 网格吸附
    if (snapEnabled.value) {
      x = snapToGrid(x)
      y = snapToGrid(y)
      width = snapToGrid(width)
      height = snapToGrid(height)
    }

    comp.x = x
    comp.y = y
    comp.width = width
    comp.height = height
    dragHint.value = {
      visible: true,
      text: Math.round(width) + ' × ' + Math.round(height),
      left: x + width / 2,
      top: y > 20 ? y - 20 : y + height + 4
    }
  }
  const onUp = () => {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
    // 仅在确实发生 resize 时入栈
    if (comp.x !== startRect.x || comp.y !== startRect.y ||
        comp.width !== startRect.width || comp.height !== startRect.height) {
      pushHistory()
    }
    dragHint.value = { visible: false, text: '', left: 0, top: 0 }
    emitChange()
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

let formResize = null
function startResizeForm(dir, e) {
  formResize = { dir, startX: e.clientX, startY: e.clientY, startW: form.value.width, startH: form.value.height }

  const onMove = (ev) => {
    if (!formResize) return
    const dx = ev.clientX - formResize.startX
    const dy = ev.clientY - formResize.startY
    if (dir.includes('e')) {
      const w = Math.max(120, formResize.startW + dx)
      form.value.width = snapEnabled.value ? snapToGrid(w) : w
    }
    if (dir.includes('s')) {
      const h = Math.max(80, formResize.startH + dy)
      form.value.height = snapEnabled.value ? snapToGrid(h) : h
    }
  }
  const onUp = () => {
    const moved = formResize && (form.value.width !== formResize.startW || form.value.height !== formResize.startH)
    formResize = null
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
    if (moved) pushHistory()
    emitChange()
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

function startDragForm() {
  // 标题栏拖动预留
}

const formRef = ref(null)
const clientRef = ref(null)

const marqueeVisible = ref(false)
const marquee = ref({ x: 0, y: 0, w: 0, h: 0 })

const marqueeStyle = computed(() => ({
  left: marquee.value.x + 'px',
  top: marquee.value.y + 'px',
  width: marquee.value.w + 'px',
  height: marquee.value.h + 'px'
}))

let marqueeState = null

function onClientMouseDown(e) {
  hideContextMenu()
  if (e.target.closest('.designer-component') || e.target.closest('.resize-handle')) return
  if (!clientRef.value.contains(e.target)) return

  if (!(e.ctrlKey || e.metaKey)) {
    selectedType.value = 'component'
    selectedIds.value.clear()
  }

  const rect = clientRef.value.getBoundingClientRect()
  const startX = e.clientX - rect.left
  const startY = e.clientY - rect.top
  marqueeState = { startX, startY }
  marquee.value = { x: startX, y: startY, w: 0, h: 0 }
  marqueeVisible.value = true

  const onMove = (ev) => {
    if (!marqueeState) return
    const cx = ev.clientX - rect.left
    const cy = ev.clientY - rect.top
    const x = Math.min(startX, cx)
    const y = Math.min(startY, cy)
    const w = Math.abs(cx - startX)
    const h = Math.abs(cy - startY)
    marquee.value = { x, y, w, h }
  }

  const onUp = () => {
    const m = marquee.value
    if (!(e.ctrlKey || e.metaKey)) {
      selectedIds.value.clear()
    }
    for (const c of components.value) {
      if (c.visible === false) continue
      if (
        c.x < m.x + m.w &&
        c.x + c.width > m.x &&
        c.y < m.y + m.h &&
        c.y + c.height > m.y
      ) {
        selectedIds.value.add(c.id)
      }
    }
    if (selectedIds.value.size > 0) selectedType.value = 'component'
    marqueeVisible.value = false
    marqueeState = null
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }

  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

const contextMenu = ref({ visible: false, x: 0, y: 0 })

const contextMenuOptions = computed(() => {
  // 多级分组菜单 — 减少视觉高度，避免超出 IDE
  const opts = []
  // 层级组
  opts.push({
    label: t('designer.ctxLayer'), key: 'z-group', children: [
      { label: t('designer.ctxZTop'), key: 'z-top' },
      { label: t('designer.ctxZForward'), key: 'z-forward' },
      { label: t('designer.ctxZBackward'), key: 'z-backward' },
      { label: t('designer.ctxZBottom'), key: 'z-bottom' }
    ]
  })
  // 编辑组
  const editChildren = [
    { label: t('designer.ctxToggleLock'), key: 'toggle-lock' },
    { label: t('designer.ctxDuplicate'), key: 'duplicate' },
    { label: t('designer.ctxCopyStyle'), key: 'copy-style' }
  ]
  if (styleClipboard) editChildren.push({ label: t('designer.ctxPasteStyle'), key: 'paste-style' })
  opts.push({ label: t('designer.ctxEdit'), key: 'edit-group', children: editChildren })
  // 排列组
  const alignChildren = [
    { label: t('designer.ctxCenterParent'), key: 'center' },
    { label: t('designer.ctxSizeToFit'), key: 'size-to-fit' },
    { label: t('designer.ctxSnapToGrid'), key: 'snap-to-grid' }
  ]
  if (selectedIds.value.size >= 2) {
    alignChildren.push({ label: t('designer.ctxEqualWidth'), key: 'equal-width' })
    alignChildren.push({ label: t('designer.ctxEqualHeight'), key: 'equal-height' })
    alignChildren.push({ label: t('designer.ctxEqualSize'), key: 'equal-size' })
  }
  opts.push({ label: t('designer.ctxArrange'), key: 'align-group', children: alignChildren })
  // 组合
  if (selectedIds.value.size >= 2 || selectedList.value.some(c => c.groupId)) {
    const grpChildren = []
    if (selectedIds.value.size >= 2) grpChildren.push({ label: t('designer.ctxGroup'), key: 'group' })
    if (selectedList.value.some(c => c.groupId)) grpChildren.push({ label: t('designer.ctxUngroup'), key: 'ungroup' })
    opts.push({ label: t('designer.ctxGroup'), key: 'grp-group', children: grpChildren })
  }
  // Tab 顺序
  if (tabOrderMode.value && selectedIds.value.size >= 2) {
    opts.push({ label: t('designer.ctxTabOrder'), key: 'tab-group', children: [
      { label: t('designer.ctxTabOrderBySel'), key: 'tab-order-by-selection' }
    ] })
  }
  // 模板
  if (selectedIds.value.size >= 1) {
    opts.push({ label: t('designer.ctxTemplate'), key: 'tpl-group', children: [
      { label: t('designer.ctxSaveAsTemplate'), key: 'save-as-template' }
    ] })
  }
  // 操作（一级，常用）
  opts.push({ type: 'divider', key: 'd2' })
  opts.push({ label: t('designer.ctxDelete'), key: 'delete' })
  opts.push({ label: t('designer.ctxSelectAll'), key: 'select-all' })
  opts.push({ label: t('designer.ctxDeselectAll'), key: 'deselect-all' })
  return opts
})

function showContextMenu(e) {
  contextMenu.value = { visible: true, x: e.clientX, y: e.clientY }
}

function hideContextMenu() {
  contextMenu.value.visible = false
}

function onContextMenu(e) {
  e.preventDefault()
  showContextMenu(e)
}

function onComponentContextMenu(e, comp) {
  e.preventDefault()
  if (!isSelected(comp.id)) selectComponent(comp.id)
  showContextMenu(e)
}

// 估算组件内容所需尺寸：根据组件类型与文本内容返回建议的宽高。
// 文本类组件（button/label/checkbox/radio/combobox）按文本长度计算宽度；
// 多行/复杂组件（textarea/listbox/tabs/card）使用 toolbox 默认尺寸；
// 非文本组件（image/divider/progress/slider/switch）保持当前尺寸或用默认。
function estimateContentSize(comp) {
  const type = comp.type
  const def = toolbox.value.find(t => t.type === type)
  const defW = def?.width || 80
  const defH = def?.height || 24
  // 文本类：按字符宽度估算（CJK 14px，ASCII 8px）
  const text = String(comp.text || def?.text || '')
  function textWidth(s) {
    let w = 0
    for (const ch of s) {
      // CJK 统一表意文字 + 全角标点
      const code = ch.codePointAt(0)
      if (code >= 0x4e00 && code <= 0x9fff) w += 14
      else if (code >= 0x3000 && code <= 0x303f) w += 14
      else if (code >= 0xff00 && code <= 0xffef) w += 14
      else w += 8
    }
    return w
  }
  switch (type) {
    case 'button':
    case 'label':
    case 'checkbox':
    case 'radio':
    case 'combobox': {
      // 宽 = 文本宽 + 内边距（图标+padding 约 24px），最小 40
      const w = Math.max(40, textWidth(text) + 24)
      return { width: Math.round(w), height: defH }
    }
    case 'edit': {
      // 单行编辑框：宽按文本或默认，高固定 24
      const w = text ? Math.max(60, textWidth(text) + 12) : defW
      return { width: Math.round(w), height: defH }
    }
    case 'textarea': {
      // 多行：按行数估算高度
      const lines = text.split('\n')
      const lineH = 16
      const h = Math.max(defH, lines.length * lineH + 8)
      // 宽度按最长行
      const maxLine = lines.reduce((m, l) => Math.max(m, l), '')
      const w = Math.max(defW, textWidth(maxLine) + 12)
      return { width: Math.round(w), height: Math.round(h) }
    }
    case 'listbox': {
      // 列表框：按选项数估算高度
      const items = String(comp.items || '').split('\n').filter(s => s.trim())
      const itemH = 20
      const h = Math.max(defH, items.length * itemH + 8)
      const w = Math.max(defW, textWidth(text) + 16)
      return { width: Math.round(w), height: Math.round(h) }
    }
    case 'tabs': {
      // 标签页：按标签数估算宽度
      const tabs = String(comp.items || text).split('\n').filter(s => s.trim())
      const w = Math.max(defW, tabs.reduce((s, t) => s + textWidth(t) + 24, 0))
      return { width: Math.round(w), height: defH }
    }
    case 'card': {
      // 卡片：按标题宽度
      const w = Math.max(defW, textWidth(text) + 32)
      return { width: Math.round(w), height: defH }
    }
    case 'divider': {
      // 分割线：保持宽度，高 2
      return { width: comp.width || defW, height: 2 }
    }
    default:
      // 其它（image/progress/slider/switch）：保持当前尺寸
      return { width: comp.width || defW, height: comp.height || defH }
  }
}

function onContextMenuSelect(key) {
  if (key === 'z-top') bringToFront()
  if (key === 'z-forward') bringForward()
  if (key === 'z-backward') sendBackward()
  if (key === 'z-bottom') sendToBack()
  if (key === 'group') groupSelected()
  if (key === 'ungroup') ungroupSelected()
  if (key === 'toggle-lock') toggleLockSelected()
  if (key === 'duplicate') duplicateSelected()
  if (key === 'copy-style') copyStyle()
  if (key === 'paste-style') pasteStyle()
  if (key === 'center') {
    pushHistory()
    for (const c of selectedList.value) {
      c.x = Math.round((form.value.width - (c.width || 0)) / 2)
      c.y = Math.round((form.value.height - (c.height || 0)) / 2)
    }
    emitChange()
  }
  if (key === 'equal-width') {
    const sel = selectedList.value
    if (sel.length >= 2) {
      pushHistory()
      const maxW = Math.max(...sel.map(c => c.width || 0))
      for (const c of sel) c.width = maxW
      emitChange()
    }
  }
  if (key === 'equal-height') {
    const sel = selectedList.value
    if (sel.length >= 2) {
      pushHistory()
      const maxH = Math.max(...sel.map(c => c.height || 0))
      for (const c of sel) c.height = maxH
      emitChange()
    }
  }
  if (key === 'equal-size') {
    const sel = selectedList.value
    if (sel.length >= 2) {
      pushHistory()
      const maxW = Math.max(...sel.map(c => c.width || 0))
      const maxH = Math.max(...sel.map(c => c.height || 0))
      for (const c of sel) { c.width = maxW; c.height = maxH }
      emitChange()
    }
  }
  if (key === 'size-to-fit') {
    const sel = selectedList.value
    if (sel.length >= 1) {
      pushHistory()
      for (const c of sel) {
        const fit = estimateContentSize(c)
        c.width = fit.width
        c.height = fit.height
      }
      emitChange()
    }
  }
  if (key === 'tab-order-by-selection') {
    // 按 selectedIds 的迭代顺序（Set 保持插入顺序）批量设置 tabOrder
    const ids = [...selectedIds.value]
    if (ids.length >= 2) {
      pushHistory()
      // 获取所有可见组件，按 tabOrder 排序
      const visible = components.value.filter(c => c.visible !== false)
      const sorted = visible.slice().sort((a, b) => (a.tabOrder || 0) - (b.tabOrder || 0))
      // 选中组件按选中顺序排列
      const selectedOrdered = ids.map(id => components.value.find(c => c.id === id)).filter(Boolean)
      // 未选中组件保持原顺序，选中组件按选中顺序插入到它们首次出现的位置
      const result = []
      let selectedInserted = false
      for (const c of sorted) {
        if (selectedIds.value.has(c.id)) {
          if (!selectedInserted) {
            result.push(...selectedOrdered)
            selectedInserted = true
          }
        } else {
          result.push(c)
        }
      }
      if (!selectedInserted) result.push(...selectedOrdered)
      // 重新编号 1..N
      result.forEach((c, i) => { c.tabOrder = i + 1 })
      emitChange()
    }
  }
  if (key === 'snap-to-grid') {
    const sel = selectedList.value
    if (sel.length >= 1 && gridSize.value > 1) {
      pushHistory()
      for (const c of sel) {
        c.x = snapToGrid(c.x || 0)
        c.y = snapToGrid(c.y || 0)
        if (c.width) c.width = snapToGrid(c.width)
        if (c.height) c.height = snapToGrid(c.height)
      }
      emitChange()
    }
  }
  if (key === 'save-as-template') {
    saveSelectionAsTemplate()
  }
  if (key === 'delete') removeSelected()
  if (key === 'select-all') {
    selectedIds.value.clear()
    for (const c of components.value) {
      if (c.visible !== false) selectedIds.value.add(c.id)
    }
    if (selectedIds.value.size > 0) selectedType.value = 'component'
  }
  if (key === 'deselect-all') {
    selectedIds.value.clear()
    selectedType.value = 'form'
  }
  hideContextMenu()
}

function openEvent(evt) {
  const comp = firstSelectedComponent.value
  if (!comp) return
  emit('open-event', { component: comp.name, event: evt })
}

function onKeyDown(e) {
  // 撤销/重做
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'z' || e.key === 'Z')) {
    e.preventDefault()
    undo()
    return
  }
  if ((e.ctrlKey || e.metaKey) && (e.shiftKey && (e.key === 'z' || e.key === 'Z') || e.key === 'y' || e.key === 'Y')) {
    e.preventDefault()
    redo()
    return
  }
  if (e.key === 'Delete' && selectedType.value === 'component' && selectedIds.value.size > 0) {
    e.preventDefault()
    removeSelected()
    return
  }
  // 复制 / 粘贴 / 原地复制（在输入控件中保留原生行为）
  const tag = (e.target?.tagName || '').toLowerCase()
  const inInput = tag === 'input' || tag === 'textarea' || tag === 'select' || e.target?.isContentEditable
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'c' || e.key === 'C') && !inInput && selectedIds.value.size > 0) {
    e.preventDefault()
    copySelected()
    return
  }
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'v' || e.key === 'V') && !inInput) {
    e.preventDefault()
    paste()
    return
  }
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === 'd' || e.key === 'D') && !inInput && selectedIds.value.size > 0) {
    e.preventDefault()
    duplicateSelected()
    return
  }
  // Ctrl+L：锁定/解锁选中组件
  if ((e.ctrlKey || e.metaKey) && (e.key === 'l' || e.key === 'L') && !inInput && selectedIds.value.size > 0) {
    e.preventDefault()
    toggleLockSelected()
    return
  }
  // Ctrl+G：切换网格吸附
  if ((e.ctrlKey || e.metaKey) && (e.key === 'g' || e.key === 'G') && !inInput) {
    e.preventDefault()
    snapEnabled.value = !snapEnabled.value
    return
  }
  // Tab / Shift+Tab 在组件间循环选中（z-order 顺序）
  if (e.key === 'Tab') {
    // 在输入框/文本域中不拦截 Tab，保留默认焦点切换
    const tag = (e.target?.tagName || '').toLowerCase()
    if (tag === 'input' || tag === 'textarea' || tag === 'select' || e.target?.isContentEditable) return
    const comps = components.value
    if (comps.length === 0) return
    e.preventDefault()
    const curId = selectedIds.value.size > 0 ? [...selectedIds.value][0] : null
    const idx = curId != null ? comps.findIndex(c => c.id === curId) : -1
    let nextIdx
    if (e.shiftKey) {
      // 上一个，到头循环到最后
      nextIdx = idx < 0 ? comps.length - 1 : (idx > 0 ? idx - 1 : comps.length - 1)
    } else {
      // 下一个，到尾循环到第一个
      nextIdx = idx < 0 ? 0 : (idx < comps.length - 1 ? idx + 1 : 0)
    }
    selectComponent(comps[nextIdx].id)
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeyDown)
})
</script>

<style scoped>
.window-designer {
  display: flex;
  width: 100%;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.panel-header {
  padding: 10px 12px;
  font-size: var(--ide-font-size);
  font-weight: 600;
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}
.designer-toolbox {
  width: 140px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  background: var(--bg-secondary);
  border-right: 1px solid var(--border-color);
}
.toolbox-list {
  flex: 1 1 0;
  min-height: 0;
  overflow: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.grid-config {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 6px 8px;
  border-top: 1px solid var(--border-color);
  font-size: var(--ide-font-size-sm);
  flex-shrink: 0;
}
.grid-toggle {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  user-select: none;
}
.grid-select {
  width: 64px;
  height: 22px;
  padding: 0 4px;
  font-size: var(--ide-font-size-xs);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--bg-tertiary);
  color: var(--text-primary);
  cursor: pointer;
}
.layer-list {
  flex: 1 1 0;
  min-height: 0;
  overflow: auto;
  padding: 6px 8px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.layer-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 8px;
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: var(--ide-font-size-sm);
  cursor: pointer;
  user-select: none;
  transition: background 0.15s, color 0.15s;
  border: 1px solid transparent;
}
.layer-item:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}
.layer-item.selected {
  background: color-mix(in srgb, var(--accent-color) 18%, transparent);
  border-color: color-mix(in srgb, var(--accent-color) 45%, transparent);
  color: var(--text-primary);
}
.layer-item.hidden {
  opacity: 0.55;
}
.layer-item[draggable="true"] {
  cursor: grab;
}
.layer-item[draggable="true"]:active {
  cursor: grabbing;
}
.layer-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.layer-flag {
  font-size: 10px;
  padding: 0 4px;
  border-radius: 4px;
  background: var(--bg-tertiary);
  color: var(--text-tertiary);
}
.layer-flag.locked {
  background: var(--color-warning);
  color: #fff;
}
.layer-empty {
  color: var(--text-tertiary);
  font-size: var(--ide-font-size-sm);
  text-align: center;
  padding: 16px 8px;
}
.toolbox-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: var(--ide-font-size);
  cursor: grab;
  transition: all 0.2s;
}
.toolbox-item:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}
.toolbox-item:active {
  cursor: grabbing;
}
/* G9：外置组件 SVG 图标（限制尺寸，与 n-icon 视觉一致） */
.toolbox-icon-svg,
.layer-icon-svg {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}
.toolbox-icon-svg :deep(svg),
.layer-icon-svg :deep(svg) {
  width: 16px;
  height: 16px;
  fill: currentColor;
}
/* 模板 header：操作按钮（标题在顶部标签栏） */
.template-header {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 4px 8px;
  flex-shrink: 0;
}
.template-actions {
  display: flex;
  gap: 2px;
}
.tpl-action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--text-tertiary);
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.tpl-action-btn:hover {
  background: var(--bg-hover);
  color: var(--accent-color);
}
.tpl-action-btn:active {
  transform: scale(0.92);
}
/* 模板项：左侧 accent 竖条区分 */
.template-item {
  position: relative;
}
.template-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 2px;
  height: 60%;
  background: var(--accent-color);
  border-radius: 1px;
  opacity: 0.6;
}
.template-item:hover::before {
  opacity: 1;
}
/* 自定义模板：用不同颜色（警告色）竖条区分 */
.custom-template::before {
  background: var(--color-warning);
}
.custom-template {
  background: color-mix(in srgb, var(--color-warning) 6%, transparent);
}
.custom-template:hover {
  background: color-mix(in srgb, var(--color-warning) 12%, transparent);
}
.designer-canvas {
  flex: 1;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: auto;
  background:
    radial-gradient(circle at 1px 1px, var(--border-color) 1px, transparent 0);
  background-size: 16px 16px;
}
.designer-hint {
  position: absolute;
  top: 8px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  font-size: var(--ide-font-size-xs);
  color: var(--text-secondary);
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 999px;
  z-index: 50;
  pointer-events: none;
  user-select: none;
  max-width: calc(100% - 32px);
  white-space: nowrap;
}
.designer-hint span {
  overflow: hidden;
  text-overflow: ellipsis;
}
.align-toolbar {
  position: absolute;
  top: 13px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 4px 6px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  z-index: 50;
  box-shadow: var(--shadow-1);
}
.align-btn {
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--ide-font-size-lg);
  border: 1px solid transparent;
  border-radius: 5px;
  background: transparent;
  color: var(--text-primary);
  cursor: pointer;
  transition: all 0.15s ease;
  padding: 0;
}
.align-btn:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--accent-color) 45%, transparent);
  background: color-mix(in srgb, var(--accent-color) 12%, transparent);
}
.align-btn:active:not(:disabled) {
  transform: translateY(1px);
}
.align-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.align-sep {
  width: 1px;
  height: 18px;
  background: var(--border-color);
  margin: 0 3px;
}
.form-surface {
  position: relative;
  flex-shrink: 0;
  background: #ffffff;
  border: 1px solid #a0a0a0;
  border-radius: 6px 6px 0 0;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.25);
  overflow: visible;
}
.form-surface.selected {
  border-color: var(--accent-color);
}
.form-titlebar {
  height: 32px;
  background: linear-gradient(180deg, #ffffff 0%, #e5e5e5 100%);
  border-bottom: 1px solid #d0d0d0;
  display: flex;
  align-items: center;
  padding: 0 4px 0 10px;
  user-select: none;
  cursor: default;
  gap: 6px;
}
.form-titlebar.no-controls {
  padding-right: 10px;
}
.form-title-icon {
  flex-shrink: 0;
  width: 14px;
  height: 14px;
  object-fit: contain;
}
.form-title {
  font-size: var(--ide-font-size-sm);
  color: #1f2329;
  font-family: var(--ide-font);
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.form-titlebar-controls {
  display: flex;
  align-items: center;
  height: 100%;
  flex-shrink: 0;
}
.ctrl-btn {
  width: 30px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: #1f2329;
  cursor: default;
  border-radius: 3px;
  transition: background 0.12s;
}
.ctrl-btn:hover {
  background: rgba(0, 0, 0, 0.08);
}
.ctrl-btn.ctrl-close:hover {
  background: #e81123;
  color: #fff;
}
.form-client {
  position: relative;
}
.form-resize {
  position: absolute;
  z-index: 20;
}
.form-resize-e {
  right: -4px;
  top: 30px;
  bottom: 0;
  width: 8px;
  cursor: e-resize;
}
.form-resize-s {
  left: 0;
  right: 0;
  bottom: -4px;
  height: 8px;
  cursor: s-resize;
}
.form-resize-se {
  right: -4px;
  bottom: -4px;
  width: 12px;
  height: 12px;
  cursor: se-resize;
  background: linear-gradient(135deg, transparent 50%, #a0a0a0 50%);
}
.designer-component {
  position: absolute;
  display: flex;
  align-items: stretch;
  justify-content: stretch;
  border: 1px solid transparent;
  box-sizing: content-box;
  will-change: transform;
}
.designer-component.selected {
  border-color: var(--accent-color);
  z-index: 5;
}
.designer-component.locked {
  border-style: dashed;
  cursor: default;
}
.designer-component.locked::after {
  content: '\01F512';
  position: absolute;
  top: 2px;
  right: 2px;
  font-size: var(--ide-font-size-sm);
  opacity: 0.7;
  pointer-events: none;
}
.resize-handle {
  position: absolute;
  width: 6px;
  height: 6px;
  background: #fff;
  border: 1px solid var(--accent-color);
  z-index: 10;
}
.resize-handle.nw { top: -4px; left: -4px; cursor: nw-resize; }
.resize-handle.n { top: -4px; left: 50%; transform: translateX(-50%); cursor: n-resize; }
.resize-handle.ne { top: -4px; right: -4px; cursor: ne-resize; }
.resize-handle.w { top: 50%; left: -4px; transform: translateY(-50%); cursor: w-resize; }
.resize-handle.e { top: 50%; right: -4px; transform: translateY(-50%); cursor: e-resize; }
.resize-handle.sw { bottom: -4px; left: -4px; cursor: sw-resize; }
.resize-handle.s { bottom: -4px; left: 50%; transform: translateX(-50%); cursor: s-resize; }
.resize-handle.se { bottom: -4px; right: -4px; cursor: se-resize; }
.marquee {
  position: absolute;
  border: 1px dashed var(--accent-color);
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  pointer-events: none;
  z-index: 100;
}
/* 对齐辅助线：拖动时贯穿客户区的虚线 */
.align-guide {
  position: absolute;
  pointer-events: none;
  z-index: 99;
  background: var(--accent-color);
}
.align-guide-v {
  /* 垂直线：1px 宽，从顶到底 */
  width: 1px;
  top: 0;
  bottom: 0;
  opacity: 0.7;
}
.align-guide-h {
  /* 水平线：1px 高，从左到右 */
  height: 1px;
  left: 0;
  right: 0;
  opacity: 0.7;
}
.drag-hint {
  position: absolute;
  transform: translateX(-50%);
  background: var(--accent-color);
  color: #fff;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: var(--ide-font-size-xs);
  pointer-events: none;
  z-index: 1000;
  white-space: nowrap;
  box-shadow: var(--shadow-1);
}
.selection-bbox {
  position: absolute;
  border: 1px dashed var(--accent-color);
  pointer-events: none;
  z-index: 50;
  opacity: 0.6;
}
.bbox-size {
  position: absolute;
  bottom: -18px;
  right: 0;
  background: var(--accent-color);
  color: #fff;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  white-space: nowrap;
}
/* 等距分布提示线：用绿色区分于普通对齐辅助线 */
.distrib-guide {
  position: absolute;
  pointer-events: none;
  z-index: 60;
  background: var(--color-success);
  opacity: 0.55;
}
.distrib-h-left, .distrib-h-right {
  height: 2px;
  transform: translateY(-1px);
}
.distrib-v-top, .distrib-v-bottom {
  width: 2px;
  transform: translateX(-1px);
}
.distrib-label {
  position: absolute;
  transform: translate(-50%, -50%);
  background: var(--color-success);
  color: #fff;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  pointer-events: none;
  z-index: 61;
  white-space: nowrap;
  box-shadow: var(--shadow-1);
}
/* 间距测量标签：警告色，区别于等距分布（绿）和对齐辅助线（强调色） */
.measure-label {
  position: absolute;
  transform: translate(-50%, -50%);
  background: var(--color-warning);
  color: #fff;
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 10px;
  pointer-events: none;
  z-index: 59;
  white-space: nowrap;
  box-shadow: var(--shadow-1);
}
.designer-props {
  width: 240px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--bg-secondary);
  border-left: 1px solid var(--border-color);
}
.props-form {
  flex: 1;
  overflow: auto;
  padding: 8px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px 8px;
  align-content: start;
}
.prop-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.prop-row > :deep(.n-input),
.prop-row > :deep(.n-input-number),
.prop-row > :deep(.n-select),
.prop-row > :deep(.n-color-picker) {
  flex: 1;
  min-width: 0;
}
/* 长文本/textarea/input-group 等需要占满一行 */
.prop-row-full {
  grid-column: span 2;
}
.prop-label {
  font-size: var(--ide-font-size-xs);
  color: var(--text-secondary);
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-shrink: 0;
  min-width: 28px;
  white-space: nowrap;
}
.prop-label-wide {
  min-width: 48px;
}
.prop-value {
  font-size: var(--ide-font-size-sm);
  color: var(--text-primary);
  display: flex;
  align-items: center;
}
.prop-section {
  grid-column: span 2;
  font-size: var(--ide-font-size-xs);
  font-weight: 600;
  color: var(--text-primary);
  padding: 4px 0 2px 2px;
  border-bottom: 1px solid var(--border-color);
  margin-top: 2px;
}
/* 颜色选择器：只显示颜色块，隐藏色号文本 */
.color-block-only :deep(.n-color-picker-trigger) {
  width: 100% !important;
  min-height: 28px !important;
  padding: 0 !important;
  font-size: 0 !important;
  border-radius: var(--radius-md, 6px) !important;
}
.color-block-only :deep(.n-color-picker-trigger__value),
.color-block-only :deep(.n-color-picker-trigger__placeholder),
.color-block-only :deep(.n-color-picker-trigger > span),
.color-block-only :deep(.n-color-picker-trigger > div) {
  display: none !important;
}
.props-empty {
  grid-column: span 2;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 80px;
}
.row-checks {
  grid-column: span 2;
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  justify-content: flex-start;
  padding-left: 4px;
}
.row-checks-3 {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px 8px;
}
.row-checks-3 :deep(.n-checkbox) {
  margin-right: 0;
}
.events-row {
  grid-column: span 2;
}
.events-row .prop-label {
  align-self: center;
}
.section-title {
  grid-column: span 2;
  display: contents;
}
.section-title .prop-label {
  justify-content: flex-start;
  padding-left: 4px;
  font-weight: 600;
  color: var(--text-primary);
  margin-top: 4px;
}

/* Tab 顺序编辑模式 */
.tab-reset-btn {
  cursor: pointer;
  background: var(--bg-tertiary);
  color: var(--text-secondary);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  padding: 0 6px;
  font-size: var(--ide-font-size-sm);
  height: 22px;
}
.tab-reset-btn:hover {
  background: var(--accent-color);
  color: #fff;
}
.designer-component.tab-order-mode {
  cursor: pointer;
}
.designer-component.tab-order-mode .resize-handle {
  display: none;
}
/* 锁定组件标识：右上角警告色小锁 */
.lock-badge {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 14px;
  height: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-warning);
  border-radius: 3px;
  color: #fff;
  pointer-events: none;
  z-index: 5;
}
.tab-order-badge {
  position: absolute;
  top: 2px;
  left: 2px;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  border-radius: 9px;
  background: var(--accent-color);
  color: #fff;
  font-size: var(--ide-font-size-xs);
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 10;
  box-shadow: var(--shadow-1);
}
.tab-order-badge:hover {
  background: color-mix(in srgb, var(--accent-color) 70%, #000);
}
.tab-order-badge[draggable="true"] {
  cursor: grab;
}
.tab-order-badge[draggable="true"]:active {
  cursor: grabbing;
}
</style>
