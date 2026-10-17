import { apiClient, requestEnvelope } from '../../api/client';
import type { ContactGroup, ContactGroupList, ContactGroupPayload, GroupMember } from './types';

export const contactGroupApi = {
  list(params?: { keyword?: string; page?: number; pageSize?: number }, signal?: AbortSignal) {
    return requestEnvelope<ContactGroupList>(apiClient.get('/contact-groups', { params, signal }));
  },
  detail(id: string, signal?: AbortSignal) {
    return requestEnvelope<{ group: ContactGroup; members: GroupMember[] }>(apiClient.get(`/contact-groups/${id}`, { signal }));
  },
  create(payload: ContactGroupPayload) {
    return requestEnvelope<ContactGroup>(apiClient.post('/contact-groups', payload));
  },
  update(id: string, payload: ContactGroupPayload) {
    return requestEnvelope<ContactGroup>(apiClient.put(`/contact-groups/${id}`, payload));
  },
  remove(id: string) {
    return requestEnvelope<Record<string, never>>(apiClient.delete(`/contact-groups/${id}`));
  },
  changeMembers(id: string, contactIds: string[], action: 'add' | 'remove') {
    return requestEnvelope<Record<string, never>>(apiClient.post(`/contact-groups/${id}/members`, { contactIds, action }));
  },
  resolve(groupIds: string[], signal?: AbortSignal) {
    return requestEnvelope<{ items: GroupMember[] }>(apiClient.post('/contact-groups/resolve', { groupIds }, { signal }));
  },
};
