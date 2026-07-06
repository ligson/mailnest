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
}

export interface AuthData {
  user: User;
  token: string;
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
  pollIntervalMinutes: number;
  enabled: boolean;
  lastSyncAt: string | null;
  lastSyncStatus: string | null;
  lastSyncError: string | null;
}

export interface MailAccountListData {
  items: MailAccount[];
}

export interface SyncResult {
  jobId: string;
  newMessageCount: number;
}

export interface MailMessage {
  id: string;
  accountId: string;
  localFolderId: string | null;
  subject: string | null;
  from: string | null;
  to: string[];
  sentAt: string | null;
  receivedAt: string | null;
  hasAttachments: boolean;
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
}

export interface MailFolderListData {
  items: MailFolder[];
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
  targetFolderId: string;
  sortOrder: number;
  conditions: MailRuleCondition[];
}

export interface MailRuleListData {
  items: MailRule[];
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
  pollIntervalMinutes: number;
  enabled: boolean;
}

export type UpdateMailAccountPayload = CreateMailAccountPayload;

export interface MailRulePayload {
  name: string;
  enabled: boolean;
  matchMode: 'all' | 'any';
  targetFolderId: string;
  sortOrder: number;
  conditions: MailRuleCondition[];
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
      const envelope = error.response?.data as Envelope<unknown> | undefined;
      throw new Error(envelope?.message || error.message || '请求失败');
    }
    throw error;
  }
}

export const authApi = {
  register(payload: { username: string; email: string; password: string }) {
    return requestEnvelope<AuthData>(apiClient.post('/auth/register', payload));
  },
  login(payload: { account: string; password: string }) {
    return requestEnvelope<AuthData>(apiClient.post('/auth/login', payload));
  },
  me() {
    return requestEnvelope<User>(apiClient.get('/auth/me'));
  },
  logout() {
    return requestEnvelope<Record<string, never>>(apiClient.post('/auth/logout'));
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
  sync(id: string) {
    return requestEnvelope<SyncResult>(apiClient.post(`/mail-accounts/${id}/sync`));
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
    dateFrom?: string;
    dateTo?: string;
    hasAttachments?: boolean;
    page?: number;
    pageSize?: number;
  }) {
    return requestEnvelope<MessageListData>(apiClient.get('/messages', { params }));
  },
  detail(id: string) {
    return requestEnvelope<MailMessageDetail>(apiClient.get(`/messages/${id}`));
  },
  async downloadAttachment(attachment: MailAttachment) {
    const response = await apiClient.get<Blob>(attachment.downloadUrl.replace(/^\/api\/v1/, ''), {
      responseType: 'blob',
    });
    return response.data;
  },
};

export const mailFolderApi = {
  list() {
    return requestEnvelope<MailFolderListData>(apiClient.get('/mail-folders'));
  },
  create(payload: { name: string; color?: string; sortOrder?: number }) {
    return requestEnvelope<MailFolder>(apiClient.post('/mail-folders', payload));
  },
  remove(id: string) {
    return requestEnvelope<Record<string, never>>(apiClient.delete(`/mail-folders/${id}`));
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
  apply(payload: { scope: 'unfiled' | 'all' }) {
    return requestEnvelope<{ appliedCount: number }>(apiClient.post('/mail-rules/apply', payload));
  },
};
