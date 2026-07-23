<template>
  <AppLayout selected-key="/mail">
    <section
      ref="workspaceEl"
      class="mail-workspace"
      :style="{
        '--folder-pane-width': `${folderPaneWidth}px`,
        '--list-pane-width': `${listPaneWidth}px`,
      }"
    >
      <aside class="mail-folders">
        <div class="folder-heading account-heading">邮箱账号</div>
        <button
          class="folder-item account-filter-item"
          :class="{ active: !filters.accountId }"
          @click="selectAccount()"
        >
          <mail-outlined class="folder-icon" />
          <span>全部账号</span>
        </button>
        <button
          v-for="account in visibleAccounts"
          :key="account.id"
          class="folder-item account-filter-item"
          :class="{ active: filters.accountId === account.id }"
          :title="`${account.displayName || account.email} <${account.email}>`"
          @click="selectAccount(account.id)"
        >
          <mail-outlined class="folder-icon" />
          <span class="account-filter-label">
            <strong>{{ account.displayName || account.email }}</strong>
            <small>{{ account.email }}</small>
          </span>
        </button>
        <div v-if="visibleAccounts.length === 0" class="folder-empty account-empty">
          <mail-outlined />
          <span>{{ accounts.length === 0 ? '暂无邮箱账号' : '暂无启用邮箱账号' }}</span>
        </div>

        <div class="folder-heading mailbox-heading">邮箱</div>
        <button
          v-for="folder in systemFolders"
          :key="folder.key"
          class="folder-item"
          :class="{ active: activeFolderKey === folder.key }"
          @click="selectSystemFolder(folder.key)"
        >
          <component :is="folder.icon" class="folder-icon" />
          <span>{{ folder.label }}</span>
        </button>

        <div class="folder-section-title">
          <span>文件夹</span>
          <a-button type="link" size="small" @click="openFolderCreate">新增</a-button>
        </div>
        <div v-if="folders.length === 0" class="folder-empty">
          <folder-open-outlined />
          <span>暂无文件夹</span>
        </div>
        <button
          v-for="folder in folders"
          :key="folder.id"
          class="folder-item"
          :class="{ active: activeFolderKey === `folder:${folder.id}` }"
          @click="selectLocalFolder(folder.id)"
        >
          <span class="folder-dot" :style="{ background: folder.color || '#64748b' }"></span>
          <span>{{ folder.name }}</span>
          <a-tooltip title="编辑文件夹">
            <a-button class="folder-action" type="text" size="small" aria-label="编辑文件夹" @click.stop="openFolderEdit(folder)">
              <template #icon><edit-outlined /></template>
            </a-button>
          </a-tooltip>
          <a-tooltip title="删除文件夹">
            <a-button class="folder-action" type="text" size="small" danger aria-label="删除文件夹" @click.stop="deleteFolder(folder)">
              <template #icon><delete-outlined /></template>
            </a-button>
          </a-tooltip>
        </button>
      </aside>

      <div class="mail-resizer" title="拖拽调整文件夹栏宽度" @mousedown="startResize('folders', $event)"></div>

      <main class="mail-list-pane">
        <div class="mail-list-header">
          <div>
            <h2 class="mail-page-title">{{ activeFolderLabel }}</h2>
            <p class="mail-count">{{ mailCountText }}</p>
          </div>
          <a-space>
            <a-button type="primary" @click="openCompose">
              <template #icon><send-outlined /></template>
              写邮件
            </a-button>
            <a-button @click="refreshAll">
              <template #icon><reload-outlined /></template>
              刷新
            </a-button>
          </a-space>
        </div>

        <div class="mail-filter-bar">
          <div class="mail-search-box">
            <a-select v-model:value="filters.searchField" class="search-field-select" @change="onFilterChanged">
              <a-select-option value="all">全部</a-select-option>
              <a-select-option value="from">发件人</a-select-option>
              <a-select-option value="subject">主题</a-select-option>
              <a-select-option value="body">正文</a-select-option>
            </a-select>
            <a-input
              v-model:value="filters.keyword"
              allow-clear
              class="search-keyword-input"
              :placeholder="searchPlaceholder"
              @change="onFilterChanged"
              @press-enter="loadMessages"
            />
            <a-button class="search-submit-button" aria-label="搜索" @click="loadMessages">
              <template #icon><search-outlined /></template>
            </a-button>
          </div>
          <div class="filter-summary-row">
            <a-button class="advanced-filter-toggle" type="text" size="small" @click="advancedFiltersOpen = !advancedFiltersOpen">
              <template #icon><filter-outlined /></template>
              筛选
              <span v-if="activeAdvancedFilterCount" class="filter-count">{{ activeAdvancedFilterCount }}</span>
              <down-outlined :class="{ open: advancedFiltersOpen }" />
            </a-button>
            <a-space v-if="activeAdvancedFilterCount" class="filter-chips" size="small" wrap>
              <span v-if="dateRange" class="filter-chip">{{ dateRangeLabel }}</span>
              <span v-if="filters.readState !== 'all'" class="filter-chip">{{ filters.readState === 'read' ? '已读' : '未读' }}</span>
              <span v-if="filters.hasAttachments" class="filter-chip">有附件</span>
              <span v-if="filters.starred" class="filter-chip">星标</span>
              <a-button type="link" size="small" @click="clearAdvancedFilters">清空</a-button>
            </a-space>
          </div>
          <div v-if="advancedFiltersOpen" class="advanced-filter-panel">
            <a-range-picker
              v-model:value="dateRange"
              class="date-filter"
              :placeholder="['开始日期', '结束日期']"
              @change="onDateChanged"
            />
            <a-select v-model:value="filters.readState" class="state-filter" @change="onFilterChanged">
              <a-select-option value="all">全部状态</a-select-option>
              <a-select-option value="unread">未读</a-select-option>
              <a-select-option value="read">已读</a-select-option>
            </a-select>
            <a-checkbox v-model:checked="filters.hasAttachments" @change="onFilterChanged">有附件</a-checkbox>
            <a-checkbox v-model:checked="filters.starred" @change="onFilterChanged">星标</a-checkbox>
          </div>
        </div>

        <div class="batch-toolbar" :class="{ active: selectedMessageIds.length > 0 }">
          <a-checkbox
            :checked="pageAllSelected"
            :indeterminate="pageSomeSelected"
            @change="toggleSelectPage"
          />
          <span class="batch-count">已选 {{ selectedMessageIds.length }} 封</span>
          <template v-if="selectedMessageIds.length">
            <a-button size="small" :loading="batching" @click="runBatchAction('mark_read')">已读</a-button>
            <a-button size="small" :loading="batching" @click="runBatchAction('mark_unread')">未读</a-button>
            <a-select
              v-model:value="batchMoveFolderId"
              class="batch-folder-select"
              size="small"
              placeholder="移动到"
            >
              <a-select-option v-for="folder in folders" :key="folder.id" :value="folder.id">{{ folder.name }}</a-select-option>
            </a-select>
            <a-button size="small" :disabled="!batchMoveFolderId" :loading="batching" @click="runBatchAction('move_folder')">移动</a-button>
            <a-dropdown :trigger="['click']">
              <a-button size="small">
                更多
                <down-outlined />
              </a-button>
              <template #overlay>
                <a-menu @click="handleBatchMenuClick">
                  <a-menu-item key="star">加星标</a-menu-item>
                  <a-menu-item key="unstar">取消星标</a-menu-item>
                  <a-menu-item v-if="activeSystemFolder !== 'spam'" key="mark_spam" danger>标记垃圾邮件</a-menu-item>
                  <a-menu-item v-else key="unmark_spam">移出垃圾邮件</a-menu-item>
                  <a-menu-divider />
                  <a-menu-item v-if="activeSystemFolder === 'trash'" key="restore">恢复</a-menu-item>
                  <a-menu-item v-else key="delete" danger>删除</a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </template>
        </div>

        <a-spin :spinning="loading">
          <div v-if="loading && !hasLoadedMessages" class="mail-list-skeleton">
            <a-skeleton active :paragraph="{ rows: 8 }" />
          </div>
          <div v-else-if="messages.length === 0" class="mail-list-empty">
            <a-empty description="没有符合条件的邮件" />
          </div>
          <div v-else class="mail-list">
            <div
              v-for="item in messages"
              :key="item.id"
              class="mail-list-item"
              :class="{ active: selectedMessageId === item.id, unread: !item.isRead, deleted: !!item.deletedAt }"
              role="button"
              tabindex="0"
              @click="openDetail(item.id)"
              @keydown.enter="openDetail(item.id)"
              @keydown.space.prevent="openDetail(item.id)"
            >
              <a-checkbox
                class="mail-select-checkbox"
                :checked="selectedMessageSet.has(item.id)"
                @click.stop
                @change="toggleSelectMessage(item.id)"
              />
              <div class="mail-item-avatar" aria-hidden="true">
                {{ senderInitial(item) }}
              </div>
              <span v-if="!item.isRead" class="mail-unread-dot" aria-hidden="true"></span>
              <div class="mail-item-content">
                <div class="mail-item-top">
                  <strong>{{ displayAddressName(parseContactAddress(item.from || '')) }}</strong>
                  <span>{{ formatShortTime(item.sentAt || item.receivedAt) }}</span>
                </div>
                <div class="mail-item-subject">
                  <star-filled v-if="item.starred" class="mail-star active" />
                  <star-outlined v-else class="mail-star" />
                  <paper-clip-outlined v-if="item.hasAttachments" />
                  <span>{{ item.subject || '无主题' }}</span>
                </div>
                <div class="mail-item-meta-row">
                  <span class="mail-item-meta">{{ mailPreview(item) }}</span>
                  <span v-if="item.isSpam" class="mail-state-chip danger">垃圾</span>
                  <span v-if="item.deletedAt" class="mail-state-chip muted">已删除</span>
                  <span v-if="!item.isRead" class="mail-state-chip accent">未读</span>
                </div>
              </div>
            </div>
          </div>
        </a-spin>

        <a-pagination
          v-if="total > pageSize"
          v-model:current="page"
          :page-size="pageSize"
          :total="total"
          size="small"
          class="mail-pagination"
          @change="loadMessages"
        />
      </main>

      <div class="mail-resizer" title="拖拽调整邮件列表宽度" @mousedown="startResize('list', $event)"></div>

      <section class="mail-reader-pane">
        <a-skeleton v-if="detailLoading" active />
        <div v-else-if="detail" class="mail-reader">
          <div class="reader-header">
            <div class="reader-title-row">
              <h3 class="mail-subject">{{ detail.subject || '无主题' }}</h3>
              <a-space class="reader-actions" size="small" wrap>
                <a-button size="small" @click="openReply('reply')">
                  <template #icon><rollback-outlined /></template>
                  回复
                </a-button>
                <a-button size="small" @click="openReply('replyAll')">
                  <template #icon><retweet-outlined /></template>
                  回复全部
                </a-button>
                <a-button size="small" @click="openReply('forward')">
                  <template #icon><forward-outlined /></template>
                  转发
                </a-button>
              </a-space>
            </div>
            <div class="reader-time">{{ formatTime(detail.sentAt || detail.receivedAt) }}</div>
            <div class="reader-address-row">
              <span class="reader-address-label">发件人</span>
              <div class="reader-contact-list">
                <a-popover trigger="click" placement="bottomLeft">
                  <template #content>
                    <div class="contact-popover">
                      <div class="contact-popover-header">
                        <strong>{{ displayAddressName(detailFromAddress) }}</strong>
                        <a-tooltip title="编辑联系人">
                          <a-button
                            class="contact-popover-edit"
                            type="text"
                            size="small"
                            aria-label="编辑联系人"
                            @click.stop="editAddressContact(detailFromAddress)"
                          >
                            <template #icon><edit-outlined /></template>
                          </a-button>
                        </a-tooltip>
                      </div>
                      <span>{{ contactEmail(detailFromAddress) || detailFromAddress.raw }}</span>
                      <span v-if="contactInfo(detailFromAddress)?.phone">电话：{{ contactInfo(detailFromAddress)?.phone }}</span>
                      <span v-if="contactInfo(detailFromAddress)?.company">公司：{{ contactInfo(detailFromAddress)?.company }}</span>
                      <span v-if="contactInfo(detailFromAddress)?.notes">备注：{{ contactInfo(detailFromAddress)?.notes }}</span>
                    </div>
                  </template>
                  <button class="reader-contact-chip" type="button">
                    <span class="reader-contact-name">{{ displayAddressName(detailFromAddress) }}</span>
                  </button>
                </a-popover>
              </div>
            </div>
            <div class="reader-address-row">
              <span class="reader-address-label">收件人</span>
              <div class="reader-contact-list">
                <span v-if="!detailToAddresses.length" class="reader-address-empty">-</span>
                <a-popover v-for="(address, index) in detailToAddresses" :key="`${address.raw}-${index}`" trigger="click" placement="bottomLeft">
                  <template #content>
                    <div class="contact-popover">
                      <div class="contact-popover-header">
                        <strong>{{ displayAddressName(address) }}</strong>
                        <a-tooltip title="编辑联系人">
                          <a-button
                            class="contact-popover-edit"
                            type="text"
                            size="small"
                            aria-label="编辑联系人"
                            @click.stop="editAddressContact(address)"
                          >
                            <template #icon><edit-outlined /></template>
                          </a-button>
                        </a-tooltip>
                      </div>
                      <span>{{ contactEmail(address) || address.raw }}</span>
                      <span v-if="contactInfo(address)?.phone">电话：{{ contactInfo(address)?.phone }}</span>
                      <span v-if="contactInfo(address)?.company">公司：{{ contactInfo(address)?.company }}</span>
                      <span v-if="contactInfo(address)?.notes">备注：{{ contactInfo(address)?.notes }}</span>
                    </div>
                  </template>
                  <button class="reader-contact-chip" type="button">
                    <span class="reader-contact-name">{{ displayAddressName(address) }}</span>
                  </button>
                </a-popover>
              </div>
            </div>
            <div v-if="detailCcAddresses.length" class="reader-address-row">
              <span class="reader-address-label">抄送</span>
              <div class="reader-contact-list">
                <a-popover v-for="(address, index) in detailCcAddresses" :key="`${address.raw}-${index}`" trigger="click" placement="bottomLeft">
                  <template #content>
                    <div class="contact-popover">
                      <div class="contact-popover-header">
                        <strong>{{ displayAddressName(address) }}</strong>
                        <a-tooltip title="编辑联系人">
                          <a-button
                            class="contact-popover-edit"
                            type="text"
                            size="small"
                            aria-label="编辑联系人"
                            @click.stop="editAddressContact(address)"
                          >
                            <template #icon><edit-outlined /></template>
                          </a-button>
                        </a-tooltip>
                      </div>
                      <span>{{ contactEmail(address) || address.raw }}</span>
                      <span v-if="contactInfo(address)?.phone">电话：{{ contactInfo(address)?.phone }}</span>
                      <span v-if="contactInfo(address)?.company">公司：{{ contactInfo(address)?.company }}</span>
                      <span v-if="contactInfo(address)?.notes">备注：{{ contactInfo(address)?.notes }}</span>
                    </div>
                  </template>
                  <button class="reader-contact-chip" type="button">
                    <span class="reader-contact-name">{{ displayAddressName(address) }}</span>
                  </button>
                </a-popover>
              </div>
            </div>
          </div>
          <div v-if="detail.htmlBody" class="mail-body" v-html="detail.htmlBody"></div>
          <pre v-else class="mail-text-body">{{ detail.textBody || '没有正文内容' }}</pre>
          <section v-if="normalAttachments.length" class="attachments-panel">
            <h4 class="attachments-title">附件</h4>
            <a-list :data-source="normalAttachments" size="small">
              <template #renderItem="{ item }">
                <a-list-item>
                  <template #actions>
                    <a-button type="link" size="small" @click="downloadAttachment(item)">下载</a-button>
                  </template>
                  <a-list-item-meta>
                    <template #title>{{ item.filename }}</template>
                    <template #description>{{ attachmentDescription(item) }}</template>
                  </a-list-item-meta>
                </a-list-item>
              </template>
            </a-list>
          </section>
        </div>
        <div v-else class="reader-empty">
          <mail-outlined />
          <p>选择一封邮件开始阅读</p>
        </div>
      </section>

      <a-modal
        v-model:open="folderModalOpen"
        :title="folderModalTitle"
        :ok-text="folderModalOkText"
        cancel-text="取消"
        @ok="saveFolder"
      >
        <a-form layout="vertical">
          <a-form-item label="名称">
            <a-input v-model:value="folderForm.name" placeholder="例如：安全通知" />
          </a-form-item>
          <a-form-item label="颜色">
            <div class="folder-color-picker">
              <button
                v-for="color in folderColorOptions"
                :key="color"
                class="folder-color-swatch"
                :class="{ selected: folderForm.color === color }"
                :style="{ '--swatch-color': color }"
                type="button"
                :aria-label="`选择颜色 ${color}`"
                @click="folderForm.color = color"
              >
                <check-outlined v-if="folderForm.color === color" />
              </button>
            </div>
          </a-form-item>
        </a-form>
      </a-modal>

      <a-modal
        v-model:open="composeOpen"
        :title="composeDrawerTitle"
        :width="1040"
        :destroy-on-close="false"
        :mask-closable="false"
        @cancel="closeCompose"
        :footer="null"
        class="compose-modal"
      >
        <a-spin :spinning="composeLoading" tip="正在准备邮件...">
          <a-form layout="vertical" :model="composeForm" class="compose-form">
            <a-form-item label="发件账号">
              <a-select v-model:value="composeForm.accountId" placeholder="选择发件邮箱" @change="onComposeAccountChanged">
                <a-select-option v-for="account in visibleAccounts" :key="account.id" :value="account.id">
                  {{ account.displayName }} &lt;{{ account.email }}&gt;
                </a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="收件人">
              <a-select
                v-model:value="composeForm.to"
                mode="tags"
                :options="contactOptions"
                placeholder="输入邮箱后回车"
                :token-separators="[',', ';', '，', '；']"
              />
            </a-form-item>
            <div class="compose-address-grid">
              <a-form-item label="抄送">
                <a-select
                  v-model:value="composeForm.cc"
                  mode="tags"
                  :options="contactOptions"
                  placeholder="可选"
                  :token-separators="[',', ';', '，', '；']"
                />
              </a-form-item>
              <a-form-item label="密送">
                <a-select
                  v-model:value="composeForm.bcc"
                  mode="tags"
                  :options="contactOptions"
                  placeholder="可选"
                  :token-separators="[',', ';', '，', '；']"
                />
              </a-form-item>
            </div>
            <a-form-item label="主题">
              <a-input v-model:value="composeForm.subject" placeholder="邮件主题" />
            </a-form-item>
            <a-form-item label="正文" class="compose-body-item">
              <div class="compose-editor">
                <div class="compose-toolbar">
                  <input
                    ref="composeAttachmentInput"
                    class="compose-file-input"
                    hidden
                    type="file"
                    multiple
                    @change="onComposeFilesSelected"
                  />
                  <input
                    ref="composeImageInput"
                    class="compose-file-input"
                    hidden
                    type="file"
                    accept="image/*"
                    multiple
                    @change="onComposeImagesSelected"
                  />
                  <div class="compose-toolbar-group" @mousedown="saveComposeSelection">
                    <a-tooltip title="添加附件">
                      <a-button type="text" class="compose-tool-button" aria-label="添加附件" @mousedown.prevent @click="chooseComposeFiles">
                        <template #icon><paper-clip-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="插入图片">
                      <a-button type="text" class="compose-tool-button" aria-label="插入图片" @mousedown.prevent @click="chooseComposeImages">
                        <template #icon><file-image-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="插入签名">
                      <a-button type="text" class="compose-tool-button" aria-label="插入签名" @mousedown.prevent @click="insertComposeSignature">
                        <template #icon><edit-outlined /></template>
                      </a-button>
                    </a-tooltip>
                  </div>
                  <span class="compose-toolbar-divider"></span>
                  <div class="compose-toolbar-group compose-toolbar-selects" @mousedown="saveComposeSelection">
                    <a-select
                      v-model:value="composeFontFamily"
                      class="compose-font-select"
                      size="small"
                      @change="applyComposeFontFamily"
                    >
                      <template #suffixIcon><font-size-outlined /></template>
                      <a-select-option v-for="font in composeFontFamilies" :key="font.value" :value="font.value">
                        {{ font.label }}
                      </a-select-option>
                    </a-select>
                    <a-select
                      v-model:value="composeFontSize"
                      class="compose-size-select"
                      size="small"
                      @change="applyComposeFontSize"
                    >
                      <a-select-option v-for="size in composeFontSizes" :key="size.value" :value="size.value">
                        {{ size.label }}
                      </a-select-option>
                    </a-select>
                  </div>
                  <span class="compose-toolbar-divider"></span>
                  <div class="compose-toolbar-group" @mousedown="saveComposeSelection">
                    <a-tooltip title="加粗">
                      <a-button type="text" class="compose-tool-button" aria-label="加粗" @mousedown.prevent @click="runComposeCommand('bold')">
                        <template #icon><bold-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="斜体">
                      <a-button type="text" class="compose-tool-button" aria-label="斜体" @mousedown.prevent @click="runComposeCommand('italic')">
                        <template #icon><italic-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="下划线">
                      <a-button type="text" class="compose-tool-button" aria-label="下划线" @mousedown.prevent @click="runComposeCommand('underline')">
                        <template #icon><underline-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="删除线">
                      <a-button type="text" class="compose-tool-button" aria-label="删除线" @mousedown.prevent @click="runComposeCommand('strikeThrough')">
                        <template #icon><strikethrough-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-popover trigger="click" placement="bottomLeft">
                      <template #content>
                        <div class="compose-color-grid" @mousedown.prevent>
                          <button
                            v-for="color in composeTextColors"
                            :key="color"
                            class="compose-color-swatch"
                            :class="{ selected: composeTextColor === color }"
                            :style="{ '--compose-swatch-color': color }"
                            type="button"
                            :aria-label="`文字颜色 ${color}`"
                            @click="applyComposeTextColor(color)"
                          >
                            <check-outlined v-if="composeTextColor === color" />
                          </button>
                        </div>
                      </template>
                      <a-button type="text" class="compose-tool-button compose-color-button" aria-label="字体颜色" @mousedown.prevent>
                        <template #icon><bg-colors-outlined /></template>
                        <span class="compose-color-indicator" :style="{ background: composeTextColor }"></span>
                      </a-button>
                    </a-popover>
                    <a-popover trigger="click" placement="bottomLeft">
                      <template #content>
                        <div class="compose-color-panel" @mousedown.prevent>
                          <button class="compose-color-clear" type="button" @click="clearComposeBackgroundColor">
                            无背景
                          </button>
                          <div class="compose-color-grid">
                            <button
                              v-for="color in composeBackgroundColors"
                              :key="color"
                              class="compose-color-swatch"
                              :class="{ selected: composeBackgroundColor === color }"
                              :style="{ '--compose-swatch-color': color }"
                              type="button"
                              :aria-label="`背景颜色 ${color}`"
                              @click="applyComposeBackgroundColor(color)"
                            >
                              <check-outlined v-if="composeBackgroundColor === color" />
                            </button>
                          </div>
                        </div>
                      </template>
                      <a-button type="text" class="compose-tool-button compose-color-button" aria-label="背景颜色" @mousedown.prevent>
                        <span class="compose-bg-label">A</span>
                        <span class="compose-color-indicator" :style="{ background: composeBackgroundColor || 'transparent' }"></span>
                      </a-button>
                    </a-popover>
                  </div>
                  <span class="compose-toolbar-divider"></span>
                  <div class="compose-toolbar-group" @mousedown="saveComposeSelection">
                    <a-tooltip title="项目列表">
                      <a-button type="text" class="compose-tool-button" aria-label="项目列表" @mousedown.prevent @click="runComposeCommand('insertUnorderedList')">
                        <template #icon><unordered-list-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="编号列表">
                      <a-button type="text" class="compose-tool-button" aria-label="编号列表" @mousedown.prevent @click="runComposeCommand('insertOrderedList')">
                        <template #icon><ordered-list-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="左对齐">
                      <a-button type="text" class="compose-tool-button" aria-label="左对齐" @mousedown.prevent @click="runComposeCommand('justifyLeft')">
                        <template #icon><align-left-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="居中">
                      <a-button type="text" class="compose-tool-button" aria-label="居中" @mousedown.prevent @click="runComposeCommand('justifyCenter')">
                        <template #icon><align-center-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="右对齐">
                      <a-button type="text" class="compose-tool-button" aria-label="右对齐" @mousedown.prevent @click="runComposeCommand('justifyRight')">
                        <template #icon><align-right-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="插入链接">
                      <a-button type="text" class="compose-tool-button" aria-label="插入链接" @mousedown.prevent @click="insertComposeLink">
                        <template #icon><link-outlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="清除格式">
                      <a-button type="text" class="compose-tool-button" aria-label="清除格式" @mousedown.prevent @click="runComposeCommand('removeFormat')">
                        <template #icon><clear-outlined /></template>
                      </a-button>
                    </a-tooltip>
                  </div>
                </div>
                <div
                  ref="composeEditor"
                  class="compose-editor-body"
                  contenteditable="true"
                  data-placeholder="输入邮件正文"
                  @input="onComposeEditorInput"
                  @focus="saveComposeSelection"
                  @keyup="saveComposeSelection"
                  @mouseup="saveComposeSelection"
                  @paste="onComposeEditorPaste"
                  @blur="onComposeEditorInput"
                ></div>
              </div>
            </a-form-item>
            <div v-if="composeForwardAttachments.length" class="compose-forward-box">
              <div class="compose-forward-title">转发附件</div>
              <a-checkbox-group v-model:value="selectedForwardAttachmentIds" class="compose-forward-list">
                <a-checkbox
                  v-for="item in composeForwardAttachments"
                  :key="item.id"
                  :value="item.id"
                >
                  {{ item.filename }} · {{ formatSize(item.size) }}
                </a-checkbox>
              </a-checkbox-group>
            </div>
            <div v-if="composeForm.attachments.length" class="compose-attachments">
              <div v-for="(file, index) in composeForm.attachments" :key="`${file.name}-${file.size}-${index}`" class="compose-attachment-item">
                <paper-clip-outlined />
                <span>{{ file.name }}</span>
                <small>{{ formatSize(file.size) }}</small>
                <a-button type="text" size="small" aria-label="移除附件" @click="removeComposeAttachment(index)">
                  移除
                </a-button>
              </div>
            </div>
            <div class="compose-footer">
              <a-button @click="closeCompose">取消</a-button>
              <a-button type="primary" :loading="sending" @click="sendMail">
                <template #icon><send-outlined /></template>
                发送
              </a-button>
            </div>
          </a-form>
        </a-spin>
      </a-modal>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, markRaw, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import {
  AlignCenterOutlined,
  AlignLeftOutlined,
  AlignRightOutlined,
  BgColorsOutlined,
  BoldOutlined,
  CheckOutlined,
  ClearOutlined,
  DeleteOutlined,
  DownOutlined,
  EditOutlined,
  FilterOutlined,
  FolderOpenOutlined,
  FileImageOutlined,
  FontSizeOutlined,
  InboxOutlined,
  ItalicOutlined,
  LinkOutlined,
  MailOutlined,
  OrderedListOutlined,
  PaperClipOutlined,
  ForwardOutlined,
  ReloadOutlined,
  SearchOutlined,
  SendOutlined,
  RollbackOutlined,
  RetweetOutlined,
  StopOutlined,
  StarFilled,
  StarOutlined,
  StrikethroughOutlined,
  UnderlineOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons-vue';
import { Modal, message } from 'ant-design-vue';
import type { Dayjs } from 'dayjs';
import { useRoute, useRouter } from 'vue-router';
import {
  mailAccountApi,
  contactApi,
  isCanceledRequest,
  mailFolderApi,
  messageApi,
  type Contact,
  type ComposeMode,
  type ComposeForwardAttachment,
  type ComposeContext,
  type MailAccount,
  type MailAttachment,
  type MailFolder,
  type MailMessage,
  type MailMessageDetail,
} from '../api/client';
import AppLayout from '../components/AppLayout.vue';

type SystemFolderKey = 'inbox' | 'sent' | 'all' | 'starred' | 'spam' | 'trash' | 'attachments';
type ResizePane = 'folders' | 'list';
type SearchField = 'all' | 'from' | 'subject' | 'body';
type ContactAddress = {
  raw: string;
  name: string;
  email: string;
};

const systemFolders = [
  { key: 'inbox' as const, label: '收件箱', icon: markRaw(InboxOutlined) },
  { key: 'sent' as const, label: '发件箱', icon: markRaw(SendOutlined) },
  { key: 'all' as const, label: '全部邮件', icon: markRaw(MailOutlined) },
  { key: 'starred' as const, label: '星标邮件', icon: markRaw(StarOutlined) },
  { key: 'spam' as const, label: '垃圾邮件', icon: markRaw(StopOutlined) },
  { key: 'trash' as const, label: '回收站', icon: markRaw(DeleteOutlined) },
  { key: 'attachments' as const, label: '有附件', icon: markRaw(PaperClipOutlined) },
];
const folderColorOptions = ['#1f66d1', '#0f9f6e', '#d97706', '#dc2626', '#7c3aed', '#0891b2', '#64748b', '#be185d'];
const router = useRouter();
const route = useRoute();

const loading = ref(false);
const detailLoading = ref(false);
let detailRequestController: AbortController | null = null;
const accounts = ref<MailAccount[]>([]);
const folders = ref<MailFolder[]>([]);
const messages = ref<MailMessage[]>([]);
const contacts = ref<Contact[]>([]);
const detail = ref<MailMessageDetail | null>(null);
const selectedMessageId = ref<string | null>(null);
const activeSystemFolder = ref<SystemFolderKey>('inbox');
const activeLocalFolderId = ref<string | null>(null);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const hasLoadedMessages = ref(false);
const dateRange = ref<[Dayjs, Dayjs] | null>(null);
const advancedFiltersOpen = ref(false);
const folderModalOpen = ref(false);
const editingFolderId = ref<string | null>(null);
const composeOpen = ref(false);
const composeMode = ref<ComposeMode>('new');
const composeSourceMessageId = ref('');
const composeLoading = ref(false);
const sending = ref(false);
const batching = ref(false);
const composeEditor = ref<HTMLElement | null>(null);
const composeAttachmentInput = ref<HTMLInputElement | null>(null);
const composeImageInput = ref<HTMLInputElement | null>(null);
const workspaceEl = ref<HTMLElement | null>(null);
const composeForwardAttachments = ref<ComposeForwardAttachment[]>([]);
const selectedForwardAttachmentIds = ref<string[]>([]);
const selectedMessageSet = ref(new Set<string>());
const batchMoveFolderId = ref<string | undefined>();
const folderPaneWidth = ref(210);
const listPaneWidth = ref(430);
const resizeConstraints = {
  minFolder: 150,
  maxFolder: 300,
  minList: 300,
  maxList: 680,
  minReader: 320,
  resizers: 12,
};
let composeSignatureInserted = false;
let composeContextRequestId = 0;
let resizeState: {
  pane: ResizePane;
  startX: number;
  startFolderWidth: number;
  startListWidth: number;
} | null = null;
const folderForm = reactive({
  name: '',
  color: '#1f66d1',
  sortOrder: 10,
});
const composeForm = reactive({
  accountId: '',
  to: [] as string[],
  cc: [] as string[],
  bcc: [] as string[],
  subject: '',
  textBody: '',
  htmlBody: '',
  attachments: [] as File[],
});
const composeFontFamilies = [
  { label: '系统默认', value: 'system-ui' },
  { label: '微软雅黑', value: 'Microsoft YaHei' },
  { label: '苹方', value: 'PingFang SC' },
  { label: '宋体', value: 'SimSun' },
  { label: '黑体', value: 'SimHei' },
  { label: 'Georgia', value: 'Georgia' },
  { label: 'Arial', value: 'Arial' },
  { label: 'Courier New', value: 'Courier New' },
];
const composeFontSizes = [
  { label: '12', value: '12px' },
  { label: '14', value: '14px' },
  { label: '16', value: '16px' },
  { label: '18', value: '18px' },
  { label: '24', value: '24px' },
  { label: '32', value: '32px' },
];
const composeTextColors = ['#1f2937', '#111827', '#b91c1c', '#d97706', '#ca8a04', '#047857', '#0369a1', '#7c3aed', '#be185d', '#ffffff'];
const composeBackgroundColors = ['#ffffff', '#fef3c7', '#fde68a', '#d1fae5', '#dbeafe', '#ede9fe', '#fee2e2', '#f3f4f6'];
const composeFontFamily = ref('system-ui');
const composeFontSize = ref('14px');
const composeTextColor = ref('#1f2937');
const composeBackgroundColor = ref('');
let composeSavedRange: Range | null = null;
const filters = reactive({
  keyword: '',
  searchField: 'all' as SearchField,
  accountId: undefined as string | undefined,
  hasAttachments: false,
  readState: 'all' as 'all' | 'read' | 'unread',
  starred: false,
});

const normalAttachments = computed(() => (detail.value?.attachments || []).filter((item) => !item.inline));
const visibleAccounts = computed(() => accounts.value.filter((account) => account.enabled));
const selectedComposeAccount = computed(() => visibleAccounts.value.find((account) => account.id === composeForm.accountId));
const detailFromAddress = computed(() => parseContactAddress(detail.value?.from || ''));
const detailToAddresses = computed(() => parseContactAddresses(detail.value?.to || []));
const detailCcAddresses = computed(() => parseContactAddresses(detail.value?.cc || []));
const contactByEmail = computed(() => {
  const map = new Map<string, Contact>();
  for (const contact of contacts.value) {
    map.set(contact.email.toLowerCase(), contact);
  }
  return map;
});
const contactOptions = computed(() => contacts.value.map((contact) => ({
  value: contact.displayName || contact.nickname
    ? `${contact.displayName || contact.nickname} <${contact.email}>`
    : contact.email,
  label: `${contact.name} <${contact.email}>`,
})));
const searchPlaceholder = computed(() => {
  const placeholders: Record<SearchField, string> = {
    all: '搜索主题、发件人、正文',
    from: '搜索发件人',
    subject: '搜索主题',
    body: '搜索正文',
  };
  return placeholders[filters.searchField];
});
const activeAdvancedFilterCount = computed(() => {
  let count = 0;
  if (dateRange.value) count += 1;
  if (filters.readState !== 'all') count += 1;
  if (filters.hasAttachments) count += 1;
  if (filters.starred) count += 1;
  return count;
});
const dateRangeLabel = computed(() => {
  if (!dateRange.value) {
    return '';
  }
  const [start, end] = dateRange.value;
  return `${start.format('YYYY-MM-DD')} → ${end.format('YYYY-MM-DD')}`;
});
const activeFolderKey = computed(() => activeLocalFolderId.value ? `folder:${activeLocalFolderId.value}` : activeSystemFolder.value);
const activeFolderLabel = computed(() => {
  if (activeLocalFolderId.value) {
    return folders.value.find((item) => item.id === activeLocalFolderId.value)?.name || '文件夹';
  }
  return systemFolders.find((item) => item.key === activeSystemFolder.value)?.label || '邮件';
});
const mailCountText = computed(() => (hasLoadedMessages.value ? `${total.value} 封邮件` : '加载中...'));
const folderModalTitle = computed(() => (editingFolderId.value ? '编辑文件夹' : '新增文件夹'));
const folderModalOkText = computed(() => (editingFolderId.value ? '保存' : '创建'));
const selectedMessageIds = computed(() => Array.from(selectedMessageSet.value));
const pageSelectableIds = computed(() => messages.value.map((item) => item.id));
const pageSelectedCount = computed(() => pageSelectableIds.value.filter((id) => selectedMessageSet.value.has(id)).length);
const pageAllSelected = computed(() => pageSelectableIds.value.length > 0 && pageSelectedCount.value === pageSelectableIds.value.length);
const pageSomeSelected = computed(() => pageSelectedCount.value > 0 && !pageAllSelected.value);
const composeDrawerTitle = computed(() => {
  const titles: Record<ComposeMode, string> = {
    new: '写邮件',
    reply: '回复邮件',
    replyAll: '回复全部',
    forward: '转发邮件',
  };
  return titles[composeMode.value];
});

onMounted(() => {
  fitPaneWidthsToViewport();
  window.addEventListener('resize', fitPaneWidthsToViewport);
  document.addEventListener('selectionchange', saveComposeSelection);
  void refreshAll();
});
onBeforeUnmount(() => {
  stopResize();
  detailRequestController?.abort();
  window.removeEventListener('resize', fitPaneWidthsToViewport);
  document.removeEventListener('selectionchange', saveComposeSelection);
});
watch(() => route.query.messageId, () => {
  void applyRouteMessageSelection();
});

async function refreshAll() {
  await Promise.all([loadAccounts(), loadFolders(), loadContacts(), loadMessages()]);
  await applyRouteMessageSelection();
}

async function loadAccounts() {
  try {
    accounts.value = (await mailAccountApi.list()).items;
    if (filters.accountId && !visibleAccounts.value.some((account) => account.id === filters.accountId)) {
      filters.accountId = undefined;
      page.value = 1;
      if (hasLoadedMessages.value) {
        void loadMessages();
      }
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取邮箱账号失败');
  }
}

async function loadFolders() {
  try {
    folders.value = (await mailFolderApi.list()).items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取文件夹失败');
  }
}

async function loadContacts() {
  try {
    contacts.value = (await contactApi.list({ pageSize: 1000 })).items;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取联系人失败');
  }
}

async function loadMessages() {
  loading.value = true;
  try {
    const data = await messageApi.list({
      page: page.value,
      pageSize: pageSize.value,
      accountId: filters.accountId,
      folderId: activeLocalFolderId.value || undefined,
      systemFolder: activeLocalFolderId.value ? undefined : activeSystemFolder.value,
      keyword: keywordQuery(),
      from: fieldQuery('from'),
      subject: fieldQuery('subject'),
      body: fieldQuery('body'),
      dateFrom: dateRange.value?.[0]?.format('YYYY-MM-DD'),
      dateTo: dateRange.value?.[1]?.format('YYYY-MM-DD'),
      hasAttachments: filters.hasAttachments || undefined,
      isRead: filters.readState === 'all' ? undefined : filters.readState === 'read',
      starred: filters.starred || undefined,
    });
    messages.value = data.items;
    total.value = data.total;
    hasLoadedMessages.value = true;
    pruneSelectedMessages();
    if (!messages.value.some((item) => item.id === selectedMessageId.value)) {
      selectedMessageId.value = null;
      detail.value = null;
    }
    if (!selectedMessageId.value && messages.value.length > 0) {
      void openDetail(messages.value[0].id);
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取邮件失败');
  } finally {
    loading.value = false;
  }
}

async function openDetail(id: string) {
  detailRequestController?.abort();
  const controller = new AbortController();
  detailRequestController = controller;
  selectedMessageId.value = id;
  detailLoading.value = true;
  detail.value = null;
  try {
    const nextDetail = await messageApi.detail(id, { signal: controller.signal });
    if (detailRequestController !== controller || selectedMessageId.value !== id) {
      return;
    }
    detail.value = nextDetail;
    messages.value = messages.value.map((item) => (
      item.id === id ? { ...item, isRead: true } : item
    ));
  } catch (error) {
    if (isCanceledRequest(error)) {
      return;
    }
    message.error(error instanceof Error ? error.message : '获取邮件详情失败');
  } finally {
    if (detailRequestController === controller) {
      detailRequestController = null;
      detailLoading.value = false;
    }
  }
}

async function applyRouteMessageSelection() {
  const routeMessageId = typeof route.query.messageId === 'string' ? route.query.messageId : '';
  if (!routeMessageId || route.path !== '/mail') {
    return
  }
  const inCurrentList = messages.value.some((item) => item.id === routeMessageId)
  if (inCurrentList) {
    if (selectedMessageId.value !== routeMessageId) {
      await openDetail(routeMessageId)
    }
  } else {
    await openDetail(routeMessageId)
  }
  await router.replace({ path: '/mail', query: {} })
}

function selectSystemFolder(key: SystemFolderKey) {
  activeSystemFolder.value = key;
  activeLocalFolderId.value = null;
  page.value = 1;
  void loadMessages();
}

function toggleSelectMessage(id: string) {
  const next = new Set(selectedMessageSet.value);
  if (next.has(id)) {
    next.delete(id);
  } else {
    next.add(id);
  }
  selectedMessageSet.value = next;
}

function toggleSelectPage() {
  const next = new Set(selectedMessageSet.value);
  if (pageAllSelected.value) {
    for (const id of pageSelectableIds.value) {
      next.delete(id);
    }
  } else {
    for (const id of pageSelectableIds.value) {
      next.add(id);
    }
  }
  selectedMessageSet.value = next;
}

function handleBatchMenuClick(info: { key: string }) {
  if (!selectedMessageIds.value.length) {
    return;
  }
  void runBatchAction(info.key);
}

function pruneSelectedMessages() {
  const visible = new Set(messages.value.map((item) => item.id));
  selectedMessageSet.value = new Set(Array.from(selectedMessageSet.value).filter((id) => visible.has(id)));
}

async function runBatchAction(action: string) {
  if (selectedMessageIds.value.length === 0) {
    return;
  }
  if (action === 'move_folder' && !batchMoveFolderId.value) {
    message.warning('请选择目标文件夹');
    return;
  }
  batching.value = true;
  try {
    const preview = await messageApi.batchPreview({ messageIds: selectedMessageIds.value });
    const result = await messageApi.batchAction({
      messageIds: selectedMessageIds.value,
      action,
      folderId: action === 'move_folder' ? batchMoveFolderId.value : undefined,
    });
    message.success(`已处理 ${result.changedCount} 封邮件，共匹配 ${preview.total} 封`);
    selectedMessageSet.value = new Set();
    await loadMessages();
    if (selectedMessageId.value) {
      await openDetail(selectedMessageId.value);
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '批量操作失败');
  } finally {
    batching.value = false;
  }
}

function selectLocalFolder(id: string) {
  activeLocalFolderId.value = id;
  page.value = 1;
  void loadMessages();
}

function selectAccount(accountId?: string) {
  if (accountId && !visibleAccounts.value.some((account) => account.id === accountId)) {
    filters.accountId = undefined;
    message.warning('该邮箱账号已停用，已切换到全部启用账号');
    return;
  }
  filters.accountId = accountId;
  page.value = 1;
  void loadMessages();
}

function openFolderCreate() {
  editingFolderId.value = null;
  folderForm.name = '';
  folderForm.color = '#1f66d1';
  folderForm.sortOrder = folders.value.length * 10 + 10;
  folderModalOpen.value = true;
}

function openFolderEdit(folder: MailFolder) {
  editingFolderId.value = folder.id;
  folderForm.name = folder.name;
  folderForm.color = folder.color || '#1f66d1';
  folderForm.sortOrder = folder.sortOrder;
  folderModalOpen.value = true;
}

function openCompose() {
  startCompose('new');
  finishComposeOpen();
  composeLoading.value = false;
}

function startCompose(mode: ComposeMode) {
  const enabledAccounts = visibleAccounts.value;
  if (enabledAccounts.length === 0) {
    message.warning(accounts.value.length === 0 ? '请先新增邮箱账号' : '请先启用一个邮箱账号');
    return false;
  }
  resetCompose();
  composeMode.value = mode;
  const filteredAccount = enabledAccounts.find((account) => account.id === filters.accountId);
  composeForm.accountId = filteredAccount?.id
    || enabledAccounts.find((account) => account.smtpConfigured)?.id
    || enabledAccounts[0].id;
  composeOpen.value = true;
  composeLoading.value = true;
  return true;
}

function finishComposeOpen() {
  window.setTimeout(() => {
    resetComposeEditor();
    if (composeForm.htmlBody || composeForm.textBody) {
      syncComposeEditorContent();
      return;
    }
    insertComposeSignatureIfEmpty();
  });
}

function applyComposeContext(context: Partial<ComposeContext> | undefined) {
  composeSourceMessageId.value = context?.sourceMessageId || '';
  if (context) {
    composeForm.to = [...(context.to || [])];
    composeForm.cc = [...(context.cc || [])];
    composeForm.bcc = [...(context.bcc || [])];
    composeForm.subject = context.subject || '';
    composeForm.textBody = context.textBody || '';
    composeForm.htmlBody = context.htmlBody || '';
    composeForwardAttachments.value = context.forwardAttachments || [];
    selectedForwardAttachmentIds.value = composeForwardAttachments.value.filter((item) => item.selected).map((item) => item.id);
  } else {
    composeForwardAttachments.value = [];
    selectedForwardAttachmentIds.value = [];
  }
  composeLoading.value = false;
  finishComposeOpen();
}

async function openReply(mode: Exclude<ComposeMode, 'new'>) {
  if (!detail.value) {
    return;
  }
  const requestId = ++composeContextRequestId;
  try {
    if (!startCompose(mode)) {
      return;
    }
    const context = await messageApi.composeContext(detail.value.id, mode);
    if (requestId !== composeContextRequestId || !composeOpen.value) {
      return;
    }
    applyComposeContext(context);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '获取写信上下文失败');
    closeCompose();
  }
}

async function sendMail() {
  if (!composeForm.accountId) {
    message.warning('请选择发件账号');
    return;
  }
  const to = normalizeComposeAddresses(composeForm.to);
  const cc = normalizeComposeAddresses(composeForm.cc);
  const bcc = normalizeComposeAddresses(composeForm.bcc);
  if (!to.length && !cc.length && !bcc.length) {
    message.warning('请填写至少一个收件人');
    return;
  }
  syncComposeEditorContent();
  if (!composeForm.subject.trim() && !hasComposeBodyContent() && composeForm.attachments.length === 0) {
    message.warning('主题和正文不能同时为空');
    return;
  }
  sending.value = true;
  try {
    const sent = await messageApi.send({
      accountId: composeForm.accountId,
      to,
      cc,
      bcc,
      subject: composeForm.subject.trim(),
      textBody: composeForm.textBody,
      htmlBody: composeForm.htmlBody,
      composeMode: composeMode.value,
      sourceMessageId: composeSourceMessageId.value || undefined,
      forwardAttachmentIds: composeMode.value === 'forward' ? selectedForwardAttachmentIds.value : undefined,
      attachments: composeForm.attachments,
    });
    message.success('邮件已发送');
    closeCompose();
    resetCompose();
    composeMode.value = 'new';
    composeSourceMessageId.value = '';
    composeForwardAttachments.value = [];
    selectedForwardAttachmentIds.value = [];
    activeSystemFolder.value = 'sent';
    activeLocalFolderId.value = null;
    page.value = 1;
    await Promise.all([loadContacts(), loadMessages()]);
    await openDetail(sent.id);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '发送失败');
  } finally {
    sending.value = false;
  }
}

function resetCompose() {
  Object.assign(composeForm, {
    accountId: filters.accountId || '',
    to: [],
    cc: [],
    bcc: [],
    subject: '',
    textBody: '',
    htmlBody: '',
    attachments: [],
  });
  composeSignatureInserted = false;
  resetComposeEditor();
  composeForwardAttachments.value = [];
  selectedForwardAttachmentIds.value = [];
  composeLoading.value = false;
  if (composeAttachmentInput.value) {
    composeAttachmentInput.value.value = '';
  }
  if (composeImageInput.value) {
    composeImageInput.value.value = '';
  }
  composeFontFamily.value = 'system-ui';
  composeFontSize.value = '14px';
  composeTextColor.value = '#1f2937';
  composeBackgroundColor.value = '';
  composeSavedRange = null;
}

function closeCompose() {
  composeContextRequestId += 1;
  composeLoading.value = false;
  composeOpen.value = false;
}

function onComposeAccountChanged() {
  insertComposeSignatureIfEmpty();
}

function onComposeEditorInput() {
  syncComposeEditorContent();
}

function syncComposeEditorContent() {
  const editor = composeEditor.value;
  if (!editor) {
    return;
  }
  composeForm.htmlBody = editor.innerHTML.trim();
  composeForm.textBody = editor.innerText.trim();
}

function resetComposeEditor() {
  if (composeEditor.value) {
    composeEditor.value.innerHTML = composeForm.htmlBody;
  }
}

function insertComposeSignatureIfEmpty() {
  if (composeSignatureInserted || !selectedComposeAccount.value?.signatureHtml || !composeEditor.value) {
    return;
  }
  syncComposeEditorContent();
  if (composeForm.textBody || composeForm.htmlBody.replace(/<br\s*\/?>|&nbsp;/gi, '').trim()) {
    return;
  }
  composeEditor.value.innerHTML = `<br><br>${selectedComposeAccount.value.signatureHtml}`;
  composeSignatureInserted = true;
  syncComposeEditorContent();
}

function insertComposeSignature() {
  if (!selectedComposeAccount.value?.signatureHtml) {
    message.info('当前发件账号还没有维护签名模板');
    return;
  }
  insertHTMLAtCursor(`<br>${selectedComposeAccount.value.signatureHtml}`);
  composeSignatureInserted = true;
  syncComposeEditorContent();
}

function runComposeCommand(command: string, value?: string) {
  restoreComposeSelection();
  document.execCommand('styleWithCSS', false, 'true');
  const applied = document.execCommand(command, false, value);
  if (!applied && command === 'hiliteColor') {
    document.execCommand('backColor', false, value);
  }
  syncComposeEditorContent();
  saveComposeSelection();
}

function applyComposeFontFamily(value: string) {
  composeFontFamily.value = value;
  applyComposeInlineStyle({ 'font-family': value });
}

function applyComposeFontSize(value: string) {
  composeFontSize.value = value;
  applyComposeInlineStyle({ 'font-size': value });
}

function applyComposeTextColor(color: string) {
  composeTextColor.value = color;
  applyComposeInlineStyle({ color });
}

function applyComposeBackgroundColor(color: string) {
  composeBackgroundColor.value = color;
  applyComposeInlineStyle({ 'background-color': color });
}

function clearComposeBackgroundColor() {
  composeBackgroundColor.value = '';
  clearComposeInlineStyle('background-color');
}

function applyComposeInlineStyle(styles: Record<string, string>) {
  restoreComposeSelection();
  const editor = composeEditor.value;
  const selection = window.getSelection();
  if (!editor || !selection || selection.rangeCount === 0) {
    return;
  }
  const range = selection.getRangeAt(0);
  if (!editor.contains(range.commonAncestorContainer)) {
    return;
  }

  const styleText = Object.entries(styles)
    .map(([name, value]) => `${name}:${value}`)
    .join(';');
  if (range.collapsed) {
    insertHTMLAtCursor(`<span style="${styleText}">\u200b</span>`);
    return;
  }

  const span = document.createElement('span');
  for (const [name, value] of Object.entries(styles)) {
    span.style.setProperty(name, value);
  }
  const contents = range.extractContents();
  span.appendChild(contents);
  range.insertNode(span);

  const nextRange = document.createRange();
  nextRange.selectNodeContents(span);
  selection.removeAllRanges();
  selection.addRange(nextRange);
  composeSavedRange = nextRange.cloneRange();
  syncComposeEditorContent();
}

function clearComposeInlineStyle(styleName: string) {
  restoreComposeSelection();
  const editor = composeEditor.value;
  const selection = window.getSelection();
  if (!editor || !selection || selection.rangeCount === 0) {
    return;
  }
  const range = selection.getRangeAt(0);
  if (!editor.contains(range.commonAncestorContainer)) {
    return;
  }

  if (range.collapsed) {
    document.execCommand('hiliteColor', false, 'transparent');
    syncComposeEditorContent();
    saveComposeSelection();
    return;
  }

  const root = (range.commonAncestorContainer.nodeType === Node.ELEMENT_NODE
    ? range.commonAncestorContainer
    : range.commonAncestorContainer.parentElement) as Element | null;
  const elements = root ? [root, ...Array.from(root.querySelectorAll('*'))] : [];
  for (const element of elements) {
    if (!range.intersectsNode(element) || !(element instanceof HTMLElement)) {
      continue;
    }
    element.style.removeProperty(styleName);
    if (styleName === 'background-color') {
      element.style.removeProperty('background');
    }
    if (!element.getAttribute('style')?.trim()) {
      element.removeAttribute('style');
    }
  }
  syncComposeEditorContent();
  saveComposeSelection();
}

function insertComposeLink() {
  saveComposeSelection();
  const value = window.prompt('链接地址');
  if (!value?.trim()) {
    return;
  }
  runComposeCommand('createLink', value.trim());
}

function insertHTMLAtCursor(html: string) {
  restoreComposeSelection();
  document.execCommand('insertHTML', false, html);
  syncComposeEditorContent();
  saveComposeSelection();
}

function chooseComposeFiles() {
  composeAttachmentInput.value?.click();
}

function chooseComposeImages() {
  saveComposeSelection();
  composeImageInput.value?.click();
}

function onComposeFilesSelected(event: Event) {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files || []);
  const existingKeys = new Set(composeForm.attachments.map((file) => `${file.name}:${file.size}:${file.lastModified}`));
  for (const file of files) {
    const key = `${file.name}:${file.size}:${file.lastModified}`;
    if (!existingKeys.has(key)) {
      composeForm.attachments.push(file);
      existingKeys.add(key);
    }
  }
  input.value = '';
}

async function onComposeImagesSelected(event: Event) {
  const input = event.target as HTMLInputElement;
  await insertComposeImageFiles(Array.from(input.files || []));
  input.value = '';
}

async function onComposeEditorPaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files || []).filter((file) => file.type.startsWith('image/'));
  if (files.length === 0) {
    window.setTimeout(() => {
      syncComposeEditorContent();
      saveComposeSelection();
    });
    return;
  }
  event.preventDefault();
  await insertComposeImageFiles(files);
}

async function insertComposeImageFiles(files: File[]) {
  const imageFiles = files.filter((file) => file.type.startsWith('image/'));
  if (imageFiles.length === 0) {
    message.warning('请选择图片文件');
    return;
  }
  const maxInlineImageSize = 3 * 1024 * 1024;
  let insertedCount = 0;
  for (const file of imageFiles) {
    if (file.size > maxInlineImageSize) {
      message.warning(`${file.name} 超过 3MB，建议作为附件发送`);
      continue;
    }
    const dataUrl = await readFileAsDataURL(file);
    insertHTMLAtCursor(buildComposeImageHTML(dataUrl, file.name));
    insertedCount += 1;
  }
  if (insertedCount > 0) {
    message.success(`已插入 ${insertedCount} 张图片`);
  }
}

function readFileAsDataURL(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ''));
    reader.onerror = () => reject(new Error('读取图片失败'));
    reader.readAsDataURL(file);
  });
}

function buildComposeImageHTML(src: string, filename: string) {
  const alt = escapeHTML(filename || '正文图片');
  return `<p><img src="${src}" alt="${alt}" style="max-width:100%;height:auto;border-radius:6px;"></p>`;
}

function escapeHTML(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function hasComposeBodyContent() {
  if (composeForm.textBody.trim()) {
    return true;
  }
  const html = composeForm.htmlBody.trim();
  return /<img\b|<table\b|<hr\b/i.test(html) || html.replace(/<[^>]+>/g, '').replace(/&nbsp;/g, '').trim().length > 0;
}

function saveComposeSelection() {
  const editor = composeEditor.value;
  const selection = window.getSelection();
  if (!editor || !selection || selection.rangeCount === 0) {
    return;
  }
  const range = selection.getRangeAt(0);
  if (!editor.contains(range.commonAncestorContainer)) {
    return;
  }
  composeSavedRange = range.cloneRange();
}

function restoreComposeSelection() {
  const editor = composeEditor.value;
  if (!editor) {
    return;
  }
  editor.focus();
  if (!composeSavedRange || !editor.contains(composeSavedRange.commonAncestorContainer)) {
    return;
  }
  const selection = window.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(composeSavedRange);
}

function removeComposeAttachment(index: number) {
  composeForm.attachments.splice(index, 1);
}

function normalizeComposeAddresses(values: string[]) {
  return values
    .map((value) => value.trim())
    .filter(Boolean);
}

function startResize(pane: ResizePane, event: MouseEvent) {
  resizeState = {
    pane,
    startX: event.clientX,
    startFolderWidth: folderPaneWidth.value,
    startListWidth: listPaneWidth.value,
  };
  document.body.classList.add('mail-resizing');
  window.addEventListener('mousemove', onResizeMove);
  window.addEventListener('mouseup', stopResize);
  event.preventDefault();
}

function onResizeMove(event: MouseEvent) {
  if (!resizeState) {
    return;
  }
  const delta = event.clientX - resizeState.startX;
  const availableWidth = workspaceAvailableWidth();
  if (resizeState.pane === 'folders') {
    const maxFolderWidth = Math.min(
      resizeConstraints.maxFolder,
      availableWidth - listPaneWidth.value - resizeConstraints.minReader - resizeConstraints.resizers,
    );
    folderPaneWidth.value = clamp(resizeState.startFolderWidth + delta, resizeConstraints.minFolder, maxFolderWidth);
    fitPaneWidthsToViewport();
    return;
  }
  const maxListWidth = Math.min(
    resizeConstraints.maxList,
    availableWidth - folderPaneWidth.value - resizeConstraints.minReader - resizeConstraints.resizers,
  );
  listPaneWidth.value = clamp(resizeState.startListWidth + delta, resizeConstraints.minList, maxListWidth);
}

function stopResize() {
  if (!resizeState) {
    return;
  }
  resizeState = null;
  document.body.classList.remove('mail-resizing');
  window.removeEventListener('mousemove', onResizeMove);
  window.removeEventListener('mouseup', stopResize);
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), Math.max(min, max));
}

function workspaceAvailableWidth() {
  return workspaceEl.value?.parentElement?.clientWidth || Math.max(0, window.innerWidth - 280);
}

function fitPaneWidthsToViewport() {
  const availableWidth = workspaceAvailableWidth();
  if (availableWidth <= 920) {
    return;
  }
  const maxFolderWidth = Math.min(
    resizeConstraints.maxFolder,
    availableWidth - resizeConstraints.minList - resizeConstraints.minReader - resizeConstraints.resizers,
  );
  folderPaneWidth.value = clamp(folderPaneWidth.value, resizeConstraints.minFolder, maxFolderWidth);

  const maxListWidth = Math.min(
    resizeConstraints.maxList,
    availableWidth - folderPaneWidth.value - resizeConstraints.minReader - resizeConstraints.resizers,
  );
  listPaneWidth.value = clamp(listPaneWidth.value, resizeConstraints.minList, maxListWidth);
}

async function saveFolder() {
  if (!folderForm.name.trim()) {
    message.warning('请输入文件夹名称');
    return;
  }
  try {
    const payload = {
      name: folderForm.name.trim(),
      color: folderForm.color.trim(),
      sortOrder: folderForm.sortOrder,
    };
    if (editingFolderId.value) {
      await mailFolderApi.update(editingFolderId.value, payload);
      message.success('文件夹已保存');
    } else {
      await mailFolderApi.create(payload);
      message.success('文件夹已创建');
    }
    folderModalOpen.value = false;
    editingFolderId.value = null;
    await loadFolders();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存文件夹失败');
  }
}

function deleteFolder(folder: MailFolder) {
  if (folder.ruleCount > 0) {
    message.warning('请先调整或删除关联规则');
    return;
  }
  Modal.confirm({
    title: `删除文件夹「${folder.name}」？`,
    content: '邮件不会被删除，只会移出该本地文件夹。',
    okText: '删除',
    okType: 'danger',
    cancelText: '取消',
    async onOk() {
      await mailFolderApi.remove(folder.id);
      if (activeLocalFolderId.value === folder.id) {
        activeLocalFolderId.value = null;
        activeSystemFolder.value = 'inbox';
      }
      await loadFolders();
      await loadMessages();
      message.success('文件夹已删除');
    },
  });
}

function onFilterChanged() {
  page.value = 1;
  void loadMessages();
}

function clearAdvancedFilters() {
  dateRange.value = null;
  filters.readState = 'all';
  filters.hasAttachments = false;
  filters.starred = false;
  page.value = 1;
  void loadMessages();
}

function keywordQuery() {
  const keyword = filters.keyword.trim();
  if (!keyword) {
    return undefined;
  }
  return filters.searchField === 'all' ? keyword : undefined;
}

function fieldQuery(field: Exclude<SearchField, 'all'>) {
  const keyword = filters.keyword.trim();
  if (!keyword) {
    return undefined;
  }
  return filters.searchField === field ? keyword : undefined;
}

function onDateChanged() {
  onFilterChanged();
}

async function downloadAttachment(attachment: MailAttachment) {
  try {
    const blob = await messageApi.downloadAttachment(attachment);
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = attachment.filename || 'attachment';
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '下载附件失败');
  }
}

function attachmentDescription(attachment: MailAttachment) {
  const parts: string[] = [];
  if (attachment.contentType) {
    parts.push(attachment.contentType);
  }
  parts.push(formatSize(attachment.size));
  return parts.join(' · ');
}

function formatSize(value: number) {
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function formatShortTime(value: string | null) {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}

function formatTime(value: string | null) {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleString();
}

function senderInitial(messageItem: MailMessage) {
  const name = displayAddressName(parseContactAddress(messageItem.from || ''));
  return (name.trim().slice(0, 1) || '?').toUpperCase();
}

function mailPreview(messageItem: MailMessage) {
  const recipients = addressSummary(messageItem.to);
  if (!recipients) {
    return '无收件人';
  }
  return `收件人 ${recipients}`;
}

function parseContactAddresses(values: string[]) {
  return values.map(parseContactAddress).filter((item) => item.name || item.email);
}

function addressSummary(values: string[]) {
  return parseContactAddresses(values).map(displayAddressName).join(', ');
}

function parseContactAddress(value: string): ContactAddress {
  const raw = value.trim();
  if (!raw) {
    return { raw: '-', name: '-', email: '' };
  }

  const matched = raw.match(/^(.*?)\s*<([^<>]+)>$/);
  if (!matched) {
    if (looksLikeEmail(raw)) {
      const emailName = raw.split('@')[0] || raw;
      return { raw, name: emailName, email: raw };
    }
    return { raw, name: raw, email: '' };
  }

  const email = matched[2].trim();
  const displayName = matched[1].trim().replace(/^"|"$/g, '');
  const fallbackName = email.split('@')[0] || email;
  const name = displayName && displayName.toLowerCase() !== email.toLowerCase() ? displayName : fallbackName;
  return { raw, name, email };
}

function displayAddressName(address: ContactAddress) {
  const contact = contactInfo(address);
  return contact?.nickname || contact?.displayName || address.name || address.email || '未知联系人';
}

function contactEmail(address: ContactAddress) {
  return address.email || (looksLikeEmail(address.raw) ? address.raw : '');
}

function contactInfo(address: ContactAddress) {
  const email = contactEmail(address).toLowerCase();
  return email ? contactByEmail.value.get(email) : undefined;
}

async function editAddressContact(address: ContactAddress) {
  const email = contactEmail(address).trim();
  if (!email) {
    message.warning('这个联系人没有可编辑的邮箱地址');
    return;
  }
  const displayName = address.name && !looksLikeEmail(address.name) ? address.name : '';
  await router.push({
    path: '/contacts',
    query: {
      email,
      ...(displayName ? { displayName } : {}),
    },
  });
}

function looksLikeEmail(value: string) {
  return /^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/.test(value.trim());
}
</script>

<style scoped>
.mail-workspace {
  display: grid;
  grid-template-columns: minmax(150px, var(--folder-pane-width, 210px)) 6px minmax(300px, var(--list-pane-width, 430px)) 6px minmax(0, 1fr);
  width: 100%;
  min-width: 0;
  height: 100%;
  min-height: 0;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--surface-bg);
  box-shadow: var(--shadow-soft);
}

.mail-resizer {
  position: relative;
  z-index: 2;
  background: var(--border-subtle);
  cursor: col-resize;
}

.mail-resizer::after {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 2px;
  width: 2px;
  background: transparent;
  content: '';
}

.mail-resizer:hover::after {
  background: var(--accent);
}

:global(.mail-resizing) {
  cursor: col-resize;
  user-select: none;
}

.mail-folders {
  min-width: 0;
  min-height: 0;
  padding: 16px 10px;
  overflow: auto;
  background: var(--surface-muted);
}

.folder-heading,
.folder-section-title {
  padding: 8px 10px;
  color: var(--muted-color);
  font-size: 12px;
  font-weight: 700;
}

.folder-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14px;
}

.mailbox-heading {
  margin-top: 14px;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
}

.folder-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  padding: 7px 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-color);
  cursor: pointer;
  text-align: left;
}

.folder-item > span:nth-child(2) {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-filter-item {
  align-items: flex-start;
}

.account-filter-label {
  display: grid;
  gap: 2px;
}

.account-filter-label strong,
.account-filter-label small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-filter-label strong {
  font-size: 13px;
  font-weight: 600;
}

.account-filter-label small {
  color: var(--muted-color);
  font-size: 11px;
}

.account-filter-item.active .account-filter-label small {
  color: currentColor;
  opacity: 0.72;
}

.account-empty {
  padding-top: 8px;
  padding-bottom: 8px;
}

.folder-action {
  opacity: 0;
  padding: 0;
  flex: none;
}

.folder-item:hover .folder-action,
.folder-item:focus-within .folder-action {
  opacity: 1;
}

.folder-item:hover,
.folder-item.active {
  background: var(--accent-soft);
  color: var(--accent-strong);
}

.folder-icon {
  font-size: 15px;
}

.folder-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex: none;
}

.folder-empty {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px;
  color: var(--muted-weak);
  font-size: 13px;
}

.folder-empty .anticon {
  color: var(--muted-weak);
  font-size: 15px;
}

.folder-color-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.folder-color-swatch {
  display: inline-flex;
  width: 34px;
  height: 34px;
  align-items: center;
  justify-content: center;
  border: 2px solid transparent;
  border-radius: 8px;
  background: var(--swatch-color);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.36);
  color: #ffffff;
  cursor: pointer;
}

.folder-color-swatch:hover,
.folder-color-swatch.selected {
  border-color: var(--heading-color);
}

.folder-color-swatch .anticon {
  font-size: 16px;
  filter: drop-shadow(0 1px 1px rgba(31, 35, 41, 0.35));
}

.mail-list-pane {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  background: linear-gradient(180deg, var(--surface-bg), var(--surface-muted));
}

.mail-list-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  flex: none;
  padding: 18px 18px 12px;
}

.mail-list-header > div {
  min-width: 0;
}

.mail-list-header :deep(.ant-space) {
  flex: none;
}

.mail-page-title {
  margin: 0;
  color: var(--heading-color);
  font-size: 20px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mail-count {
  margin: 4px 0 0;
  color: var(--muted-color);
}

.mail-filter-bar {
  display: grid;
  gap: 8px;
  min-width: 0;
  flex: none;
  padding: 0 18px 10px;
  overflow: hidden;
}

.mail-search-box {
  display: grid;
  grid-template-columns: 92px minmax(0, 1fr) 42px;
  min-width: 0;
  width: 100%;
}

.search-field-select {
  min-width: 0;
}

.mail-search-box :deep(.ant-select),
.mail-search-box :deep(.ant-input-affix-wrapper),
.mail-search-box :deep(.ant-input) {
  min-width: 0;
}

.mail-search-box :deep(.ant-select-selector) {
  height: 38px !important;
  border-start-end-radius: 0 !important;
  border-end-end-radius: 0 !important;
}

.mail-search-box :deep(.ant-select-selection-item) {
  line-height: 36px !important;
}

.search-keyword-input {
  height: 38px;
  border-radius: 0;
  margin-left: -1px;
}

.search-keyword-input:hover,
.search-keyword-input:focus {
  position: relative;
  z-index: 1;
}

.search-submit-button {
  width: 42px;
  height: 38px;
  border-start-start-radius: 0;
  border-end-start-radius: 0;
  margin-left: -1px;
  color: var(--muted-color);
}

.search-submit-button:hover,
.search-submit-button:focus {
  position: relative;
  z-index: 1;
  color: var(--accent);
}

.filter-summary-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  min-width: 0;
  min-height: 28px;
}

.advanced-filter-toggle {
  display: inline-flex;
  height: 28px;
  align-items: center;
  gap: 5px;
  padding: 0 8px;
  color: var(--muted-color);
  font-size: 12px;
}

.advanced-filter-toggle:hover {
  color: var(--accent);
  background: var(--accent-tint);
}

.advanced-filter-toggle .anticon {
  font-size: 12px;
}

.advanced-filter-toggle .anticon:last-child {
  transition: transform 0.18s ease;
}

.advanced-filter-toggle .anticon:last-child.open {
  transform: rotate(180deg);
}

.filter-count {
  display: inline-flex;
  min-width: 18px;
  height: 18px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent-strong);
  font-size: 11px;
  font-weight: 700;
}

.filter-chips {
  min-width: 0;
  overflow: hidden;
}

.filter-chip {
  display: inline-flex;
  max-width: 100%;
  height: 24px;
  align-items: center;
  padding: 0 8px;
  border: 1px solid var(--border-subtle);
  border-radius: 999px;
  background: var(--surface-muted);
  color: var(--muted-color);
  font-size: 12px;
  white-space: nowrap;
}

.advanced-filter-panel {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  min-width: 0;
  padding: 9px 10px;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  background: var(--surface-muted);
}

.date-filter {
  min-width: 0;
  flex: 1 1 230px;
}

.state-filter {
  min-width: 120px;
  flex: 0 1 140px;
}

.advanced-filter-panel :deep(.ant-select-selector),
.advanced-filter-panel :deep(.ant-picker),
.advanced-filter-panel :deep(.ant-checkbox-wrapper) {
  min-height: 32px;
}

.advanced-filter-panel :deep(.ant-picker) {
  width: 100%;
}

.advanced-filter-panel :deep(.ant-checkbox-wrapper) {
  display: flex;
  align-items: center;
  white-space: nowrap;
  font-size: 13px;
}

.batch-toolbar {
  display: flex;
  min-height: 34px;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  flex: none;
  padding: 0 18px 10px;
  color: var(--muted-color);
}

.batch-toolbar.active {
  padding-bottom: 12px;
}

.batch-count {
  color: var(--muted-color);
  font-size: 12px;
  white-space: nowrap;
}

.batch-folder-select {
  width: 128px;
}

@media (max-width: 1280px) {
  .mail-search-box {
    grid-template-columns: 88px minmax(0, 1fr) 42px;
  }
}

@media (max-width: 1080px) {
  .advanced-filter-panel > * {
    flex-basis: 100%;
  }

  .reader-title-row {
    display: grid;
  }

  .reader-actions {
    padding-top: 0;
  }
}

.mail-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  border-top: 1px solid var(--border-subtle);
  background: var(--surface-bg);
}

.mail-list-empty {
  flex: 1;
  min-height: 0;
  padding: 48px 12px;
  overflow: auto;
}

.mail-list-skeleton {
  flex: 1;
  min-height: 0;
  padding: 16px 18px 0;
  overflow: hidden;
}

.mail-list-pane :deep(.ant-spin-nested-loading),
.mail-list-pane :deep(.ant-spin-container) {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
}

.mail-list-item {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  column-gap: 11px;
  position: relative;
  width: 100%;
  min-width: 0;
  padding: 13px 16px 13px 46px;
  border: 0;
  border-left: 3px solid transparent;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-bg);
  color: var(--text-color);
  cursor: pointer;
  text-align: left;
}

.mail-select-checkbox {
  position: absolute;
  top: 14px;
  left: 14px;
}

.mail-item-avatar {
  display: inline-flex;
  width: 36px;
  height: 36px;
  align-items: center;
  justify-content: center;
  border: 1px solid color-mix(in srgb, var(--accent) 18%, var(--border-color));
  border-radius: 8px;
  background: var(--accent-tint);
  color: var(--accent-strong);
  font-size: 14px;
  font-weight: 800;
}

.mail-unread-dot {
  position: absolute;
  top: 25px;
  left: 34px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 0 2px var(--surface-bg);
}

.mail-item-content {
  display: grid;
  min-width: 0;
  gap: 5px;
}

.mail-list-item:hover {
  background: var(--accent-tint);
}

.mail-list-item.active {
  border-left-color: var(--accent);
  background: var(--accent-soft);
}

.mail-list-item.unread .mail-item-top strong,
.mail-list-item.unread .mail-item-subject {
  color: var(--heading-color);
}

.mail-item-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  font-size: 13px;
}

.mail-item-top strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mail-item-top span,
.mail-item-meta {
  color: var(--muted-color);
  font-size: 12px;
}

.mail-item-subject {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  font-weight: 700;
  line-height: 1.4;
}

.mail-item-subject > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mail-star,
.mail-item-subject > .anticon {
  flex: none;
}

.mail-star.active {
  color: #f59e0b;
}

.mail-item-meta-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.mail-item-meta {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mail-state-chip {
  display: inline-flex;
  height: 20px;
  align-items: center;
  padding: 0 7px;
  border-radius: 999px;
  flex: none;
  font-size: 11px;
  font-weight: 700;
}

.mail-state-chip.accent {
  background: var(--accent-soft);
  color: var(--accent-strong);
}

.mail-state-chip.danger {
  background: #fee2e2;
  color: #b91c1c;
}

.mail-state-chip.muted {
  background: var(--border-subtle);
  color: var(--muted-color);
}

.mail-pagination {
  flex: none;
  margin: 12px 16px 16px;
  align-self: flex-end;
}

.mail-reader-pane {
  min-width: 0;
  min-height: 0;
  padding: 26px 30px;
  overflow: auto;
  background: var(--surface-bg);
}

.mail-reader {
  min-width: 0;
  max-width: 880px;
}

.reader-header {
  padding-bottom: 18px;
  border-bottom: 1px solid var(--border-subtle);
  margin-bottom: 18px;
}

.reader-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.reader-actions {
  flex: none;
  padding-top: 1px;
}

.mail-subject {
  min-width: 0;
  margin: 0 0 10px;
  color: var(--heading-color);
  font-size: 22px;
  font-weight: 700;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.reader-time {
  margin-bottom: 12px;
  color: var(--muted-color);
  font-size: 13px;
}

.reader-address-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-top: 8px;
  color: var(--muted-color);
  font-size: 13px;
  line-height: 1.7;
}

.reader-address-label {
  width: 46px;
  flex: none;
  color: var(--muted-weak);
  font-weight: 600;
  text-align: right;
}

.reader-contact-list {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-wrap: wrap;
  gap: 6px;
}

.reader-contact-chip {
  display: inline-flex;
  max-width: 100%;
  align-items: baseline;
  gap: 5px;
  padding: 2px 8px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--surface-muted);
  color: var(--text-color);
  cursor: pointer;
  font: inherit;
  line-height: 1.55;
}

.reader-contact-chip:hover {
  border-color: var(--accent);
  background: var(--accent-soft);
}

.reader-contact-name,
.reader-contact-email {
  min-width: 0;
  overflow-wrap: anywhere;
}

.reader-contact-name {
  font-weight: 600;
}

.reader-contact-email {
  color: var(--muted-color);
  font-size: 12px;
}

.reader-address-empty {
  color: var(--muted-weak);
}

.contact-popover {
  display: grid;
  max-width: 280px;
  min-width: 190px;
  gap: 6px;
  color: var(--text-color);
  font-size: 13px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.contact-popover-header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 26px;
  align-items: center;
  gap: 10px;
}

.contact-popover strong {
  min-width: 0;
  color: var(--heading-color);
  overflow-wrap: anywhere;
}

.contact-popover-edit {
  width: 26px;
  height: 26px;
  color: var(--muted-color);
}

.contact-popover-edit:hover {
  color: var(--accent);
  background: var(--accent-soft);
}

.mail-body {
  max-width: 100%;
  overflow-x: auto;
  color: var(--text-color);
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.mail-body :deep(img) {
  max-width: 100%;
  height: auto;
}

.mail-body :deep(table) {
  max-width: 100%;
}

.mail-text-body {
  margin: 0;
  color: var(--text-color);
  font-family: inherit;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.attachments-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 22px;
  padding-top: 18px;
  border-top: 1px solid var(--border-subtle);
}

.attachments-title {
  margin: 0;
  color: var(--heading-color);
  font-size: 15px;
  font-weight: 700;
}

.reader-empty {
  display: grid;
  min-height: 55vh;
  place-items: center;
  align-content: center;
  gap: 12px;
  color: var(--muted-weak);
}

.reader-empty .anticon {
  font-size: 42px;
}

.compose-modal {
  max-width: calc(100vw - 32px);
}

.compose-modal :deep(.ant-modal-content) {
  max-height: calc(100vh - 32px);
  display: flex;
  flex-direction: column;
  border-radius: 12px;
  overflow: hidden;
}

.compose-modal :deep(.ant-modal-header) {
  flex: none;
  padding: 18px 24px 14px;
  border-bottom: 1px solid var(--border-subtle);
  margin-bottom: 0;
  background: var(--surface-bg);
}

.compose-modal :deep(.ant-modal-title) {
  color: var(--heading-color);
  font-size: 18px;
  font-weight: 800;
}

.compose-modal :deep(.ant-modal-body) {
  max-height: calc(100vh - 116px);
  padding: 0;
  overflow-x: hidden;
  overflow-y: auto;
}

.compose-modal :deep(.ant-select-selection-overflow) {
  align-items: center;
}

.compose-form {
  display: grid;
  min-width: 0;
  gap: 12px;
  padding: 16px 24px 0;
}

.compose-form :deep(.ant-form-item) {
  margin-bottom: 0;
  min-width: 0;
}

.compose-form :deep(.ant-form-item-control),
.compose-form :deep(.ant-form-item-control-input),
.compose-form :deep(.ant-form-item-control-input-content),
.compose-form :deep(.ant-select),
.compose-form :deep(.ant-input),
.compose-form :deep(.ant-input-affix-wrapper) {
  min-width: 0;
  width: 100%;
}

.compose-address-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 12px;
  min-width: 0;
}

.compose-address-grid :deep(.ant-form-item) {
  margin-bottom: 0;
}

.compose-footer {
  position: sticky;
  bottom: 0;
  z-index: 2;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin: 12px -24px 0;
  padding: 14px 24px;
  border-top: 1px solid var(--border-subtle);
  background: color-mix(in srgb, var(--surface-bg) 94%, transparent);
  backdrop-filter: blur(10px);
}

.compose-editor {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--surface-bg);
  overflow: hidden;
  box-shadow: inset 0 1px 0 color-mix(in srgb, #ffffff 72%, transparent);
}

.compose-toolbar {
  display: flex;
  min-height: 48px;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 6px;
  padding: 7px 9px;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-muted);
  overflow-x: hidden;
  row-gap: 7px;
}

.compose-toolbar-group {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  flex-wrap: wrap;
  gap: 2px;
}

.compose-toolbar-selects {
  gap: 6px;
}

.compose-font-select {
  width: 126px;
}

.compose-size-select {
  width: 72px;
}

.compose-font-select :deep(.ant-select-selector),
.compose-size-select :deep(.ant-select-selector) {
  height: 32px !important;
}

.compose-font-select :deep(.ant-select-selection-item),
.compose-size-select :deep(.ant-select-selection-item) {
  line-height: 30px !important;
}

.compose-file-input {
  display: none;
}

.compose-tool-button {
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  color: var(--muted-color);
}

.compose-tool-button:hover,
.compose-tool-button:focus {
  color: var(--accent);
  background: var(--accent-soft);
}

.compose-toolbar-divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  margin: 5px 2px 0;
  background: var(--border-color);
}

.compose-color-button {
  position: relative;
}

.compose-color-indicator {
  position: absolute;
  right: 8px;
  bottom: 5px;
  left: 8px;
  height: 3px;
  border: 1px solid rgba(31, 41, 55, 0.12);
  border-radius: 999px;
}

.compose-bg-label {
  font-size: 14px;
  font-weight: 800;
  line-height: 1;
}

.compose-color-panel {
  display: grid;
  gap: 8px;
}

.compose-color-clear {
  height: 30px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--surface-bg);
  color: var(--text-color);
  cursor: pointer;
  font-size: 12px;
}

.compose-color-clear:hover {
  border-color: var(--accent);
  color: var(--accent);
  background: var(--accent-tint);
}

.compose-color-grid {
  display: grid;
  grid-template-columns: repeat(5, 26px);
  gap: 7px;
  padding: 2px;
}

.compose-color-swatch {
  display: inline-flex;
  width: 26px;
  height: 26px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--compose-swatch-color);
  color: #ffffff;
  cursor: pointer;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.28);
}

.compose-color-swatch:hover,
.compose-color-swatch.selected {
  border-color: var(--accent);
  outline: 2px solid var(--accent-soft);
}

.compose-color-swatch[style*="#ffffff"],
.compose-color-swatch[style*="255, 255, 255"] {
  color: var(--heading-color);
}

.compose-editor-body {
  min-height: 360px;
  max-height: 54vh;
  padding: 14px 16px;
  background: var(--surface-bg);
  color: var(--text-color);
  line-height: 1.7;
  outline: none;
  overflow-y: auto;
  overflow-x: hidden;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
  word-break: break-word;
}

.compose-editor-body:empty::before {
  color: var(--muted-weak);
  content: attr(data-placeholder);
}

.compose-editor-body :deep(img) {
  max-width: 100%;
  height: auto;
  vertical-align: middle;
}

.compose-attachments {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 8px;
  margin-top: 10px;
}

.compose-forward-box {
  display: grid;
  gap: 8px;
  margin-top: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  background: var(--surface-muted);
}

.compose-forward-title {
  color: var(--muted-color);
  font-size: 12px;
  font-weight: 700;
}

.compose-forward-list {
  display: grid;
  gap: 6px;
}

.compose-forward-list :deep(.ant-checkbox-wrapper) {
  min-width: 0;
  margin-inline-start: 0;
  overflow-wrap: anywhere;
}

.compose-attachment-item {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 8px;
  padding: 9px 10px;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  background: var(--surface-bg);
  color: var(--text-color);
  font-size: 13px;
}

.compose-attachment-item span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.compose-attachment-item small {
  color: var(--muted-color);
}

@media (max-width: 1180px) {
  .mail-workspace {
    grid-template-columns: minmax(150px, var(--folder-pane-width, 190px)) 6px minmax(300px, var(--list-pane-width, 340px)) 6px minmax(0, 1fr);
  }

  .mail-folders {
    padding: 14px 8px;
  }

  .mail-list-header,
  .mail-filter-bar,
  .batch-toolbar {
    padding-right: 12px;
    padding-left: 12px;
  }

  .mail-reader-pane {
    padding: 18px 18px;
  }
}

@media (max-width: 920px) {
  .mail-workspace {
    grid-template-columns: 1fr;
  }

  .mail-folders,
  .mail-list-pane {
    border-right: 0;
    border-bottom: 1px solid var(--border-subtle);
  }

  .mail-resizer {
    display: none;
  }
}
</style>
