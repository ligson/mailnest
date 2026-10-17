export interface ContactGroup {
  id: string;
  name: string;
  description: string;
  memberCount: number;
  revision: number;
  createdAt: string;
  updatedAt: string;
}

export interface GroupMember {
  id: string;
  email: string;
  name: string;
  nickname: string;
  displayName: string;
}

export interface ContactGroupPayload {
  name: string;
  description: string;
  contactIds: string[];
  revision?: number;
}

export interface ContactGroupList {
  items: ContactGroup[];
  total: number;
  page: number;
  pageSize: number;
}
