import { apiClient, requestEnvelope } from '../../api/client';

export type CompanyBatchMode = 'fill_empty' | 'set' | 'clear';
export type GroupBatchMode = 'add' | 'remove';
export interface ContactBatchPayload {
  contactIds: string[];
  company?: { mode: CompanyBatchMode; value: string };
  groups?: { mode: GroupBatchMode; groupIds: string[] };
}
export interface ContactBatchResult {
  selectedCount: number;
  companyUpdatedCount: number;
  groupMembershipChangedCount: number;
}
export function batchUpdateContacts(payload: ContactBatchPayload) {
  return requestEnvelope<ContactBatchResult>(apiClient.patch('/contacts/batch', payload));
}
