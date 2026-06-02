// Room Permission Types (Phase 11)

export type RoomRole = 'owner' | 'editor' | 'viewer';

export interface RoomSettings {
  roomId: string;
  hasPassword: boolean;
  isPublic: boolean;
  allowAnonymous: boolean;
  isOwner: boolean;
  userRole: RoomRole | '';
}

export interface RoomMember {
  userId: string;
  username: string;
  email?: string;
  role: RoomRole;
}

export interface RoomInvitation {
  id: string;
  roomId: string;
  email?: string;
  role: RoomRole;
  token: string;
  inviteUrl: string;
  expiresAt: string;
}

// WebSocket Payloads

export interface SetRoomPasswordPayload {
  roomId: string;
  password: string;
}

export interface UpdateRoomSettingsPayload {
  roomId: string;
  isPublic?: boolean;
  allowAnonymous?: boolean;
}

export interface UpdateMemberRolePayload {
  roomId: string;
  userId: string;
  role: 'editor' | 'viewer';
}

export interface RemoveMemberPayload {
  roomId: string;
  userId: string;
}

export interface CreateInvitationPayload {
  roomId: string;
  email?: string;
  role: 'editor' | 'viewer';
}

export interface AcceptInvitationPayload {
  token: string;
}

export interface DeleteInvitationPayload {
  roomId: string;
  invitationId: string;
}

// Server Response Payloads

export interface RoomSettingsPayload {
  roomId: string;
  hasPassword: boolean;
  isPublic: boolean;
  allowAnonymous: boolean;
  isOwner: boolean;
  userRole: string;
}

export interface RoomMembersListPayload {
  roomId: string;
  members: RoomMember[];
}

export interface InvitationsListPayload {
  roomId: string;
  invitations: RoomInvitation[];
}

export interface MemberRoleUpdatedPayload {
  roomId: string;
  userId: string;
  username: string;
  role: string;
  updatedBy: string;
}

export interface MemberRemovedPayload {
  roomId: string;
  userId: string;
  removedBy: string;
}

export interface PermissionDeniedPayload {
  action: string;
  message: string;
}

export interface PasswordRequiredPayload {
  roomId: string;
}

export interface InvitationAcceptedPayload {
  roomId: string;
  role: string;
}
