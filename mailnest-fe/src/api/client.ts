import axios, { AxiosError } from 'axios';

export interface Envelope<T> {
  success: boolean;
  message: string;
  httpCode: number;
  data: T;
}

export interface User {
  id: string;
  username: string;
  email: string;
  nickname: string | null;
  avatarUrl: string | null;
  bio: string | null;
  uiTheme: string;
  isAdmin: boolean;
  enabled: boolean;
}

export interface AuthData {
  user: User;
  token: string;
}

export interface CaptchaData {
  id: string;
  imageData: string;
  expireSeconds: number;
}

export interface CaptchaPayload {
  captchaId: string;
  captchaAnswer: string;
}

export interface ChangePasswordPayload {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
}

export interface UpdateProfilePayload {
  nickname: string;
  bio: string;
  uiTheme: string;
}

export interface AdminUserSummary {
  id: string;
  username: string;
  email: string;
  nickname: string | null;
  isAdmin: boolean;
  enabled: boolean;
  mailAccountCount: number;
  messageCount: number;
  attachmentCount: number;
  attachmentBytes: number;
  contactCount: number;
  folderCount: number;
  ruleCount: number;
  lastMessageAt: string | null;
  lastSyncAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface AdminUserListData {
  items: AdminUserSummary[];
}

export interface MailAccount {
  id: string;
  provider: string;
  authType: string;
  displayName: string;
  email: string;
  imapHost: string;
  imapPort: number;
  imapTls: boolean;
  imapUsername: string;
  smtpHost: string;
  smtpPort: number;
  smtpTls: boolean;
  smtpStartTls: boolean;
  smtpUsername: string;
  smtpConfigured: boolean;
  sentFolder: string;
  signatureHtml: string;
  pollIntervalMinutes: number;
  enabled: boolean;
  lastSyncAt: string | null;
  lastSyncStatus: string | null;
  lastSyncError: string | null;
  fullSyncStatus: 'idle' | 'running' | 'success' | 'failed' | 'cancelled';
  fullSyncTotal: number;
  fullSyncProcessed: number;
  fullSyncNewCount: number;
  fullSyncStartedAt: string | null;
  fullSyncFinishedAt: string | null;
  fullSyncError: string | null;
  cleanupEnabled: boolean;
  cleanupRetentionDays: number;
}

export interface MailAccountListData {
  items: MailAccount[];
}

export interface SyncResult {
  jobId: string;
  newMessageCount: number;
  warnings?: string[];
}

export interface MailAccountFolder {
  name: string;
  delimiter: string;
  attributes: string[];
  sentCandidate: boolean;
}

export interface MailAccountFoldersData {
  items: MailAccountFolder[];
}

export interface FullSyncStatusData {
  fullSyncStatus: 'idle' | 'running' | 'success' | 'failed' | 'cancelled';
  fullSyncTotal: number;
  fullSyncProcessed: number;
  fullSyncNewCount: number;
  fullSyncStartedAt: string | null;
  fullSyncFinishedAt: string | null;
  fullSyncError: string | null;
  cleanupEnabled: boolean;
  cleanupRetentionDays: number;
}

export interface MailMessage {
  id: string;
  accountId: string;
  threadId: string | null;
  localFolderId: string | null;
  subject: string | null;
  from: string | null;
  to: string[];
  sentAt: string | null;
  receivedAt: string | null;
  hasAttachments: boolean;
  isRead: boolean;
  starred: boolean;
  isSpam: boolean;
  spamAt: string | null;
  deletedAt: string | null;
}

export interface MailThread {
  id: string;
  accountId: string;
  rootMessageId: string | null;
  subject: string;
  messageCount: number;
  unreadCount: number;
  hasAttachments: boolean;
  lastMessageAt: string | null;
  participants: string[];
  latestMessage: MailMessage;
}

export interface MailThreadDetail extends Omit<MailThread, 'participants' | 'latestMessage'> {
  messages: MailMessage[];
}

export interface MailThreadListData {
  items: MailThread[];
  page: number;
  pageSize: number;
  total: number;
}

export interface MailAttachment {
  id: string;
  messageId: string;
  filename: string;
  contentType: string | null;
  contentId: string | null;
  inline: boolean;
  size: number;
  downloadUrl: string;
}

export interface MailMessageDetail extends MailMessage {
  cc: string[];
  folder: string;
  messageId: string | null;
  textBody: string;
  htmlBody: string;
  attachments: MailAttachment[];
}

export type ComposeMode = 'new' | 'reply' | 'replyAll' | 'forward';

export interface ComposeForwardAttachment {
  id: string;
  filename: string;
  contentType: string;
  size: number;
  selected: boolean;
}

export interface ComposeContext {
  mode: ComposeMode;
  sourceMessageId: string;
  accountId: string;
  to: string[];
  cc: string[];
  bcc: string[];
  subject: string;
  textBody: string;
  htmlBody: string;
  forwardAttachments: ComposeForwardAttachment[];
}

export interface MailDraft {
  id: string;
  accountId: string;
  composeMode: ComposeMode;
  sourceMessageId: string | null;
  to: string[];
  cc: string[];
  bcc: string[];
  subject: string;
  textBody: string;
  htmlBody: string;
  forwardAttachmentIds: string[];
  localAttachmentNames: string[];
  createdAt: string;
  updatedAt: string;
}

export interface MailDraftListData {
  items: MailDraft[];
  page: number;
  pageSize: number;
  total: number;
}

export interface MessageListData {
  items: MailMessage[];
  page: number;
  pageSize: number;
  total: number;
}

export interface MailFolder {
  id: string;
  name: string;
  color: string | null;
  sortOrder: number;
  ruleCount: number;
}

export interface MailFolderListData {
  items: MailFolder[];
}

export interface Contact {
  id: string;
  email: string;
  displayName: string | null;
  nickname: string | null;
  name: string;
  phone: string | null;
  company: string | null;
  notes: string | null;
  source: 'manual' | 'auto' | string;
  firstSeenAt: string | null;
  lastSeenAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface ContactListData {
  items: Contact[];
  page: number;
  pageSize: number;
  total: number;
}

export interface ContactPayload {
  email: string;
  displayName: string;
  nickname: string;
  phone: string;
  company: string;
  notes: string;
}

export interface MailRuleCondition {
  id?: string;
  field: string;
  operator: string;
  value: string;
}

export interface MailRule {
  id: string;
  name: string;
  enabled: boolean;
  matchMode: 'all' | 'any';
  priority: number;
  stopOnMatch: boolean;
  actionType: 'move_folder' | 'mark_read' | 'star' | 'mark_spam' | string;
  targetFolderId: string | null;
  sortOrder: number;
  conditions: MailRuleCondition[];
  hitCount: number;
  lastHitAt: string | null;
  lastResult: string | null;
}

export interface MailRuleListData {
  items: MailRule[];
}

export interface MailRuleLog {
  id: string;
  ruleId: string | null;
  ruleName: string;
  messageId: string;
  messageSubject: string | null;
  matched: boolean;
  actionType: string;
  targetFolderId: string | null;
  triggerType: string;
  conditionSnapshot: MailRuleCondition[];
  resultStatus: 'applied' | 'skipped' | 'failed' | string;
  resultMessage: string;
  createdAt: string;
}

export interface MailRuleLogListData {
  items: MailRuleLog[];
  page: number;
  pageSize: number;
  total: number;
}

export interface MicrosoftOAuthStartData {
  state: string;
  authUrl: string;
}

export interface CreateMailAccountPayload {
  displayName: string;
  email: string;
  imapHost: string;
  imapPort: number;
  imapTls: boolean;
  imapUsername: string;
  imapPassword: string;
  smtpHost: string;
  smtpPort: number;
  smtpTls: boolean;
  smtpStartTls: boolean;
  smtpUsername: string;
  smtpPassword: string;
  smtpUseImapPassword: boolean;
  sentFolder: string;
  signatureHtml: string;
  pollIntervalMinutes: number;
  enabled: boolean;
  cleanupEnabled: boolean;
  cleanupRetentionDays: number;
}

export type UpdateMailAccountPayload = CreateMailAccountPayload;

export interface SendMessagePayload {
  draftId?: string;
  accountId: string;
  to: string[];
  cc: string[];
  bcc: string[];
  subject: string;
  textBody: string;
  htmlBody: string;
  composeMode?: ComposeMode;
  sourceMessageId?: string;
  forwardAttachmentIds?: string[];
  attachments?: File[];
}

export interface SaveDraftPayload {
  accountId: string;
  to: string[];
  cc: string[];
  bcc: string[];
  subject: string;
  textBody: string;
  htmlBody: string;
  composeMode?: ComposeMode;
  sourceMessageId?: string;
  forwardAttachmentIds?: string[];
  localAttachmentNames?: string[];
}

export interface MailRulePayload {
  name: string;
  enabled: boolean;
  matchMode: 'all' | 'any';
  priority: number;
  stopOnMatch: boolean;
  actionType: string;
  targetFolderId: string | null;
  sortOrder: number;
  conditions: MailRuleCondition[];
}

export interface MessageBatchActionResult {
  matchedCount: number;
  changedCount: number;
  skippedCount: number;
}

export interface MessageBatchPreview {
  total: number;
  readCount: number;
  unreadCount: number;
  starredCount: number;
  spamCount: number;
  deletedCount: number;
  folderCounts: Array<{ folderId: string; name: string; count: number }>;
}

export interface AttachmentCenterItem extends MailAttachment {
  accountId: string;
  folderId: string | null;
  messageSubject: string | null;
  messageFrom: string | null;
  messageTime: string | null;
}

export interface AttachmentCenterListData {
  items: AttachmentCenterItem[];
  page: number;
  pageSize: number;
  total: number;
}

export interface SyncJob {
  id: string;
  accountId: string;
  triggerType: string;
  status: string;
  startedAt: string | null;
  finishedAt: string | null;
  newMessageCount: number;
  errorMessage: string | null;
}

export interface SyncJobListData {
  items: SyncJob[];
  page: number;
  pageSize: number;
  total: number;
}

export interface SyncJobEvent {
  id: string;
  jobId: string;
  level: 'info' | 'warn' | 'error' | string;
  phase: string;
  message: string;
  detail: Record<string, unknown> | null;
  createdAt: string;
}

export interface SyncJobEventListData {
  items: SyncJobEvent[];
  page: number;
  pageSize: number;
  total: number;
}

export interface RulePreviewData {
  matchedCount: number;
  samples: MailMessage[];
}

export const tokenStorageKey = 'mailnest.token';

export const apiClient = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
});

apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem(tokenStorageKey);
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

export function isCanceledRequest(error: unknown): boolean {
  return error instanceof Error && (
    error.name === 'CanceledError' ||
    error.message === 'canceled' ||
    error.message === '请求已取消'
  );
}

export async function requestEnvelope<T>(request: Promise<{ data: Envelope<T> }>): Promise<T> {
  try {
    const response = await request;
    const envelope = response.data;
    if (!envelope.success) {
      throw new Error(envelope.message || '请求失败');
    }
    return envelope.data;
  } catch (error) {
    if (error instanceof AxiosError) {
      if (error.code === 'ERR_CANCELED') {
        const canceledError = new Error('请求已取消');
        canceledError.name = 'CanceledError';
        throw canceledError;
      }
      const envelope = error.response?.data as Envelope<unknown> | undefined;
      if (error.code === 'ECONNABORTED') {
        throw new Error('请求超时，请稍后重试');
      }
      throw new Error(envelope?.message || error.message || '请求失败');
    }
    throw error;
  }
}

export const authApi = {
  captcha() {
    return requestEnvelope<CaptchaData>(apiClient.get('/auth/captcha'));
  },
  register(payload: { username: string; email: string; password: string } & CaptchaPayload) {
    return requestEnvelope<AuthData>(apiClient.post('/auth/register', payload));
  },
  login(payload: { account: string; password: string } & CaptchaPayload) {
    return requestEnvelope<AuthData>(apiClient.post('/auth/login', payload));
  },
  me() {
    return requestEnvelope<User>(apiClient.get('/auth/me'));
  },
  changePassword(payload: ChangePasswordPayload) {
    return requestEnvelope<Record<string, never>>(apiClient.post('/auth/change-password', payload));
  },
  logout() {
    return requestEnvelope<Record<string, never>>(apiClient.post('/auth/logout'));
  },
};

export const adminApi = {
  users() {
    return requestEnvelope<AdminUserListData>(apiClient.get('/admin/users'));
  },
  updateUserEnabled(id: string, enabled: boolean) {
    return requestEnvelope<AdminUserSummary>(apiClient.put(`/admin/users/${id}/enabled`, { enabled }));
  },
};

export const profileApi = {
  get() {
    return requestEnvelope<User>(apiClient.get('/profile'));
  },
  update(payload: UpdateProfilePayload) {
    return requestEnvelope<User>(apiClient.put('/profile', payload));
  },
  uploadAvatar(file: File) {
    const form = new FormData();
    form.append('avatar', file);
    return requestEnvelope<User>(apiClient.post('/profile/avatar', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }));
  },
};

export const mailAccountApi = {
  list() {
    return requestEnvelope<MailAccountListData>(apiClient.get('/mail-accounts'));
  },
  create(payload: CreateMailAccountPayload) {
    return requestEnvelope<MailAccount>(apiClient.post('/mail-accounts', payload));
  },
  update(id: string, payload: UpdateMailAccountPayload) {
    return requestEnvelope<MailAccount>(apiClient.put(`/mail-accounts/${id}`, payload));
  },
  remove(id: string) {
    return requestEnvelope<Record<string, never>>(apiClient.delete(`/mail-accounts/${id}`));
  },
  testConnection(id: string) {
    return requestEnvelope<Record<string, never>>(apiClient.post(`/mail-accounts/${id}/test-connection`));
  },
  folders(id: string) {
    return requestEnvelope<MailAccountFoldersData>(apiClient.get(`/mail-accounts/${id}/folders`));
  },
  sync(id: string) {
    return requestEnvelope<SyncResult>(apiClient.post(`/mail-accounts/${id}/sync`));
  },
  startFullSync(id: string) {
    return requestEnvelope<FullSyncStatusData>(apiClient.post(`/mail-accounts/${id}/full-sync/start`));
  },
  stopFullSync(id: string) {
    return requestEnvelope<FullSyncStatusData>(apiClient.post(`/mail-accounts/${id}/full-sync/stop`));
  },
  syncStatus(id: string) {
    return requestEnvelope<FullSyncStatusData>(apiClient.get(`/mail-accounts/${id}/sync-status`));
  },
};

export const oauthApi = {
  startMicrosoft() {
    return requestEnvelope<MicrosoftOAuthStartData>(apiClient.post('/oauth/microsoft/start'));
  },
  completeMicrosoft(payload: { code: string; state: string }) {
    return requestEnvelope<MailAccount>(apiClient.post('/oauth/microsoft/complete', payload));
  },
};

export const messageApi = {
  list(params?: {
    accountId?: string;
    folderId?: string;
    systemFolder?: string;
    keyword?: string;
    from?: string;
    subject?: string;
    body?: string;
    dateFrom?: string;
    dateTo?: string;
    hasAttachments?: boolean;
    isRead?: boolean;
    starred?: boolean;
    page?: number;
    pageSize?: number;
  }) {
    return requestEnvelope<MessageListData>(apiClient.get('/messages', { params }));
  },
  detail(id: string, options?: { signal?: AbortSignal }) {
    return requestEnvelope<MailMessageDetail>(apiClient.get(`/messages/${id}`, {
      signal: options?.signal,
      timeout: 60000,
    }));
  },
  composeContext(id: string, mode: Exclude<ComposeMode, 'new'>) {
    return requestEnvelope<ComposeContext>(apiClient.get(`/messages/${id}/compose-context`, { params: { mode } }));
  },
  ruleLogs(id: string) {
    return requestEnvelope<MailRuleLogListData>(apiClient.get(`/messages/${id}/rule-logs`));
  },
  batchAction(payload: { messageIds: string[]; action: string; folderId?: string }) {
    return requestEnvelope<MessageBatchActionResult>(apiClient.post('/messages/batch-actions', payload));
  },
  batchPreview(payload: { messageIds: string[] }) {
    return requestEnvelope<MessageBatchPreview>(apiClient.post('/messages/batch-preview', payload));
  },
  send(payload: SendMessagePayload) {
    const form = new FormData();
    if (payload.draftId) {
      form.append('draftId', payload.draftId);
    }
    form.append('accountId', payload.accountId);
    form.append('to', JSON.stringify(payload.to));
    form.append('cc', JSON.stringify(payload.cc));
    form.append('bcc', JSON.stringify(payload.bcc));
    form.append('subject', payload.subject);
    form.append('textBody', payload.textBody);
    form.append('htmlBody', payload.htmlBody);
    form.append('composeMode', payload.composeMode || 'new');
    if (payload.sourceMessageId) {
      form.append('sourceMessageId', payload.sourceMessageId);
    }
    if (payload.forwardAttachmentIds?.length) {
      form.append('forwardAttachmentIds', JSON.stringify(payload.forwardAttachmentIds));
    }
    for (const file of payload.attachments || []) {
      form.append('attachments', file, file.name);
    }
    return requestEnvelope<MailMessage>(apiClient.post('/messages/send', form, {
      timeout: 60000,
    }));
  },
  async downloadAttachment(attachment: MailAttachment) {
    const response = await apiClient.get<Blob>(attachment.downloadUrl.replace(/^\/api\/v1/, ''), {
      responseType: 'blob',
    });
    return response.data;
  },
};

export const threadApi = {
  list(params?: {
    accountId?: string;
    folderId?: string;
    systemFolder?: string;
    keyword?: string;
    from?: string;
    subject?: string;
    body?: string;
    dateFrom?: string;
    dateTo?: string;
    hasAttachments?: boolean;
    isRead?: boolean;
    starred?: boolean;
    page?: number;
    pageSize?: number;
  }) {
    return requestEnvelope<MailThreadListData>(apiClient.get('/threads', { params }));
  },
  detail(id: string) {
    return requestEnvelope<MailThreadDetail>(apiClient.get(`/threads/${id}`));
  },
  rebuild(payload: { scope: 'empty' | 'all'; accountId?: string }) {
    return requestEnvelope<{ processedCount: number; threadCount: number }>(apiClient.post('/threads/rebuild', payload));
  },
};

export const draftApi = {
  list(params?: { page?: number; pageSize?: number }) {
    return requestEnvelope<MailDraftListData>(apiClient.get('/drafts', { params }));
  },
  detail(id: string) {
    return requestEnvelope<MailDraft>(apiClient.get(`/drafts/${id}`));
  },
  create(payload: SaveDraftPayload) {
    return requestEnvelope<MailDraft>(apiClient.post('/drafts', payload));
  },
  update(id: string, payload: SaveDraftPayload) {
    return requestEnvelope<MailDraft>(apiClient.put(`/drafts/${id}`, payload));
  },
  remove(id: string) {
    return requestEnvelope<Record<string, never>>(apiClient.delete(`/drafts/${id}`));
  },
  send(id: string) {
    return requestEnvelope<MailMessage>(apiClient.post(`/drafts/${id}/send`));
  },
};

export const attachmentApi = {
  list(params?: {
    keyword?: string;
    contentType?: string;
    accountId?: string;
    folderId?: string;
    inline?: boolean;
    dateFrom?: string;
    dateTo?: string;
    page?: number;
    pageSize?: number;
  }) {
    return requestEnvelope<AttachmentCenterListData>(apiClient.get('/attachments', { params }));
  },
};

export const syncJobApi = {
  list(params?: { accountId?: string; page?: number; pageSize?: number }) {
    return requestEnvelope<SyncJobListData>(apiClient.get('/sync-jobs', { params }));
  },
  detail(id: string) {
    return requestEnvelope<SyncJob>(apiClient.get(`/sync-jobs/${id}`));
  },
  events(id: string, params?: { level?: string; page?: number; pageSize?: number }) {
    return requestEnvelope<SyncJobEventListData>(apiClient.get(`/sync-jobs/${id}/events`, { params }));
  },
};

export const mailFolderApi = {
  list() {
    return requestEnvelope<MailFolderListData>(apiClient.get('/mail-folders'));
  },
  create(payload: { name: string; color?: string; sortOrder?: number }) {
    return requestEnvelope<MailFolder>(apiClient.post('/mail-folders', payload));
  },
  update(id: string, payload: { name: string; color?: string; sortOrder?: number }) {
    return requestEnvelope<MailFolder>(apiClient.put(`/mail-folders/${id}`, payload));
  },
  remove(id: string) {
    return requestEnvelope<Record<string, never>>(apiClient.delete(`/mail-folders/${id}`));
  },
};

export const contactApi = {
  list(params?: { keyword?: string; page?: number; pageSize?: number }) {
    return requestEnvelope<ContactListData>(apiClient.get('/contacts', { params }));
  },
  create(payload: ContactPayload) {
    return requestEnvelope<Contact>(apiClient.post('/contacts', payload));
  },
  update(id: string, payload: ContactPayload) {
    return requestEnvelope<Contact>(apiClient.put(`/contacts/${id}`, payload));
  },
  remove(id: string) {
    return requestEnvelope<Record<string, never>>(apiClient.delete(`/contacts/${id}`));
  },
};

export const mailRuleApi = {
  list() {
    return requestEnvelope<MailRuleListData>(apiClient.get('/mail-rules'));
  },
  create(payload: MailRulePayload) {
    return requestEnvelope<MailRule>(apiClient.post('/mail-rules', payload));
  },
  update(id: string, payload: MailRulePayload) {
    return requestEnvelope<MailRule>(apiClient.put(`/mail-rules/${id}`, payload));
  },
  remove(id: string) {
    return requestEnvelope<Record<string, never>>(apiClient.delete(`/mail-rules/${id}`));
  },
  apply(payload: { scope: 'unfiled' | 'all' | 'filtered' }) {
    return requestEnvelope<{ appliedCount: number }>(apiClient.post('/mail-rules/apply', payload));
  },
  preview(payload: MailRulePayload & { limit?: number }) {
    return requestEnvelope<RulePreviewData>(apiClient.post('/mail-rules/preview', payload));
  },
};

export const ruleLogApi = {
  list(params?: { messageId?: string; ruleId?: string; resultStatus?: string; triggerType?: string; page?: number; pageSize?: number }) {
    return requestEnvelope<MailRuleLogListData>(apiClient.get('/rule-logs', { params }));
  },
};
