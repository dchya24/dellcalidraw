import { wsService } from './websocket';
import type {
  RoomSettings,
  RoomMember,
  RoomInvitation,
  RoomSettingsPayload,
  RoomMembersListPayload,
  InvitationsListPayload,
  MemberRoleUpdatedPayload,
  MemberRemovedPayload,
  PermissionDeniedPayload,
  PasswordRequiredPayload,
  InvitationAcceptedPayload,
} from '../types/roomPermissions';

class RoomPermissionsService {
  private settingsListeners: Set<(settings: RoomSettings) => void> = new Set();
  private membersListeners: Set<(members: RoomMember[]) => void> = new Set();
  private invitationsListeners: Set<(invitations: RoomInvitation[]) => void> = new Set();
  private permissionDeniedListeners: Set<(payload: PermissionDeniedPayload) => void> = new Set();
  private passwordRequiredListeners: Set<(roomId: string) => void> = new Set();
  private memberRoleUpdatedListeners: Set<(payload: MemberRoleUpdatedPayload) => void> = new Set();
  private memberRemovedListeners: Set<(payload: MemberRemovedPayload) => void> = new Set();
  private invitationAcceptedListeners: Set<(payload: InvitationAcceptedPayload) => void> = new Set();

  private initialized = false;

  constructor() {
    this.setupMessageHandlers();
  }

  private setupMessageHandlers(): void {
    if (this.initialized) return;
    this.initialized = true;

    // Room settings
    wsService.on('room_settings', (payload: RoomSettingsPayload) => {
      const settings: RoomSettings = {
        roomId: payload.roomId,
        hasPassword: payload.hasPassword,
        isPublic: payload.isPublic,
        allowAnonymous: payload.allowAnonymous,
        isOwner: payload.isOwner,
        userRole: (payload.userRole as RoomSettings['userRole']) || '',
      };
      this.settingsListeners.forEach(listener => listener(settings));
    });

    wsService.on('room_settings_updated', (payload: RoomSettingsPayload) => {
      const settings: RoomSettings = {
        roomId: payload.roomId,
        hasPassword: payload.hasPassword,
        isPublic: payload.isPublic,
        allowAnonymous: payload.allowAnonymous,
        isOwner: payload.isOwner,
        userRole: (payload.userRole as RoomSettings['userRole']) || '',
      };
      this.settingsListeners.forEach(listener => listener(settings));
    });

    // Room members
    wsService.on('room_members', (payload: RoomMembersListPayload) => {
      this.membersListeners.forEach(listener => listener(payload.members));
    });

    wsService.on('member_role_updated', (payload: MemberRoleUpdatedPayload) => {
      this.memberRoleUpdatedListeners.forEach(listener => listener(payload));
    });

    wsService.on('member_removed', (payload: MemberRemovedPayload) => {
      this.memberRemovedListeners.forEach(listener => listener(payload));
    });

    // Invitations
    wsService.on('invitations_list', (payload: InvitationsListPayload) => {
      this.invitationsListeners.forEach(listener => listener(payload.invitations));
    });

    wsService.on('invitation_created', (payload: RoomInvitation) => {
      // Refresh invitations list
      this.invitationsListeners.forEach(listener => listener([payload]));
    });

    wsService.on('invitation_accepted', (payload: InvitationAcceptedPayload) => {
      this.invitationAcceptedListeners.forEach(listener => listener(payload));
    });

    // Permission denied
    wsService.on('permission_denied', (payload: PermissionDeniedPayload) => {
      this.permissionDeniedListeners.forEach(listener => listener(payload));
    });

    // Password required
    wsService.on('password_required', (payload: PasswordRequiredPayload) => {
      this.passwordRequiredListeners.forEach(listener => listener(payload.roomId));
    });
  }

  // Get room settings
  getRoomSettings(roomId: string): void {
    wsService.send('get_room_settings', { roomId });
  }

  // Set room password (empty string to remove)
  setRoomPassword(roomId: string, password: string): void {
    wsService.send('set_room_password', { roomId, password });
  }

  // Update room settings
  updateRoomSettings(roomId: string, settings: { isPublic?: boolean; allowAnonymous?: boolean }): void {
    wsService.send('update_room_settings', { roomId, ...settings });
  }

  // Get room members
  getRoomMembers(roomId: string): void {
    wsService.send('get_room_members', { roomId });
  }

  // Update member role
  updateMemberRole(roomId: string, userId: string, role: 'editor' | 'viewer'): void {
    wsService.send('update_member_role', { roomId, userId, role });
  }

  // Remove member
  removeMember(roomId: string, userId: string): void {
    wsService.send('remove_member', { roomId, userId });
  }

  // Create invitation
  createInvitation(roomId: string, role: 'editor' | 'viewer', email?: string): void {
    wsService.send('create_invitation', { roomId, role, email });
  }

  // Accept invitation
  acceptInvitation(token: string): void {
    wsService.send('accept_invitation', { token });
  }

  // Get invitations
  getInvitations(roomId: string): void {
    wsService.send('get_invitations', { roomId });
  }

  // Delete invitation
  deleteInvitation(roomId: string, invitationId: string): void {
    wsService.send('delete_invitation', { roomId, invitationId });
  }

  // Join room with password
  joinRoomWithPassword(roomId: string, username: string, password: string): void {
    wsService.send('join_room', { roomId, username, password });
  }

  // Event listeners
  onSettingsChange(callback: (settings: RoomSettings) => void): () => void {
    this.settingsListeners.add(callback);
    return () => this.settingsListeners.delete(callback);
  }

  onMembersChange(callback: (members: RoomMember[]) => void): () => void {
    this.membersListeners.add(callback);
    return () => this.membersListeners.delete(callback);
  }

  onInvitationsChange(callback: (invitations: RoomInvitation[]) => void): () => void {
    this.invitationsListeners.add(callback);
    return () => this.invitationsListeners.delete(callback);
  }

  onPermissionDenied(callback: (payload: PermissionDeniedPayload) => void): () => void {
    this.permissionDeniedListeners.add(callback);
    return () => this.permissionDeniedListeners.delete(callback);
  }

  onPasswordRequired(callback: (roomId: string) => void): () => void {
    this.passwordRequiredListeners.add(callback);
    return () => this.passwordRequiredListeners.delete(callback);
  }

  onMemberRoleUpdated(callback: (payload: MemberRoleUpdatedPayload) => void): () => void {
    this.memberRoleUpdatedListeners.add(callback);
    return () => this.memberRoleUpdatedListeners.delete(callback);
  }

  onMemberRemoved(callback: (payload: MemberRemovedPayload) => void): () => void {
    this.memberRemovedListeners.add(callback);
    return () => this.memberRemovedListeners.delete(callback);
  }

  onInvitationAccepted(callback: (payload: InvitationAcceptedPayload) => void): () => void {
    this.invitationAcceptedListeners.add(callback);
    return () => this.invitationAcceptedListeners.delete(callback);
  }
}

// Singleton instance
export const roomPermissionsService = new RoomPermissionsService();
