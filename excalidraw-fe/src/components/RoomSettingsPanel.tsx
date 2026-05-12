import React, { useState, useEffect } from 'react';
import { Settings, Lock, Unlock, Users, Globe, UserX, Copy, Trash2, Plus, X, Shield, Eye, Edit3 } from 'lucide-react';
import { roomPermissionsService } from '../services/roomPermissionsService';
import type { RoomSettings, RoomMember, RoomInvitation, PermissionDeniedPayload } from '../types/roomPermissions';
import { useThemeStore } from '../store/useThemeStore';

interface RoomSettingsPanelProps {
  roomId: string;
  isOpen: boolean;
  onClose: () => void;
}

export const RoomSettingsPanel: React.FC<RoomSettingsPanelProps> = ({ roomId, isOpen, onClose }) => {
  const { theme } = useThemeStore();
  const isDark = theme === 'dark';

  const [activeTab, setActiveTab] = useState<'settings' | 'members' | 'invitations'>('settings');
  const [settings, setSettings] = useState<RoomSettings | null>(null);
  const [members, setMembers] = useState<RoomMember[]>([]);
  const [invitations, setInvitations] = useState<RoomInvitation[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Password form
  const [showPasswordForm, setShowPasswordForm] = useState(false);
  const [newPassword, setNewPassword] = useState('');

  // Invitation form
  const [showInviteForm, setShowInviteForm] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteRole, setInviteRole] = useState<'editor' | 'viewer'>('editor');

  useEffect(() => {
    if (!isOpen || !roomId) return;

    // Subscribe to events
    const unsubSettings = roomPermissionsService.onSettingsChange(setSettings);
    const unsubMembers = roomPermissionsService.onMembersChange(setMembers);
    const unsubInvitations = roomPermissionsService.onInvitationsChange(setInvitations);
    const unsubPermissionDenied = roomPermissionsService.onPermissionDenied((payload: PermissionDeniedPayload) => {
      setError(payload.message);
      setTimeout(() => setError(null), 5000);
    });

    // Fetch initial data
    roomPermissionsService.getRoomSettings(roomId);
    roomPermissionsService.getRoomMembers(roomId);
    roomPermissionsService.getInvitations(roomId);

    return () => {
      unsubSettings();
      unsubMembers();
      unsubInvitations();
      unsubPermissionDenied();
    };
  }, [isOpen, roomId]);

  const handleSetPassword = () => {
    if (!roomId) return;
    setLoading(true);
    roomPermissionsService.setRoomPassword(roomId, newPassword);
    setNewPassword('');
    setShowPasswordForm(false);
    setLoading(false);
  };

  const handleRemovePassword = () => {
    if (!roomId) return;
    roomPermissionsService.setRoomPassword(roomId, '');
  };

  const handleTogglePublic = () => {
    if (!roomId || !settings) return;
    roomPermissionsService.updateRoomSettings(roomId, { isPublic: !settings.isPublic });
  };

  const handleToggleAnonymous = () => {
    if (!roomId || !settings) return;
    roomPermissionsService.updateRoomSettings(roomId, { allowAnonymous: !settings.allowAnonymous });
  };

  const handleUpdateMemberRole = (userId: string, role: 'editor' | 'viewer') => {
    if (!roomId) return;
    roomPermissionsService.updateMemberRole(roomId, userId, role);
  };

  const handleRemoveMember = (userId: string) => {
    if (!roomId) return;
    if (confirm('Are you sure you want to remove this member?')) {
      roomPermissionsService.removeMember(roomId, userId);
    }
  };

  const handleCreateInvitation = () => {
    if (!roomId) return;
    roomPermissionsService.createInvitation(roomId, inviteRole, inviteEmail || undefined);
    setInviteEmail('');
    setShowInviteForm(false);
  };

  const handleCopyInviteLink = (url: string) => {
    navigator.clipboard.writeText(url);
  };

  const handleDeleteInvitation = (invitationId: string) => {
    if (!roomId) return;
    roomPermissionsService.deleteInvitation(roomId, invitationId);
  };

  const getRoleIcon = (role: string) => {
    switch (role) {
      case 'owner': return <Shield className="w-4 h-4 text-yellow-500" />;
      case 'editor': return <Edit3 className="w-4 h-4 text-blue-500" />;
      case 'viewer': return <Eye className="w-4 h-4 text-gray-500" />;
      default: return null;
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className={`w-full max-w-lg rounded-lg shadow-xl ${isDark ? 'bg-gray-800 text-white' : 'bg-white text-gray-900'}`}>
        {/* Header */}
        <div className={`flex items-center justify-between p-4 border-b ${isDark ? 'border-gray-700' : 'border-gray-200'}`}>
          <div className="flex items-center gap-2">
            <Settings className="w-5 h-5" />
            <h2 className="text-lg font-semibold">Room Settings</h2>
          </div>
          <button onClick={onClose} className={`p-1 rounded hover:${isDark ? 'bg-gray-700' : 'bg-gray-100'}`}>
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Error message */}
        {error && (
          <div className="mx-4 mt-4 p-3 bg-red-100 text-red-700 rounded-lg text-sm">
            {error}
          </div>
        )}

        {/* Tabs */}
        <div className={`flex border-b ${isDark ? 'border-gray-700' : 'border-gray-200'}`}>
          {(['settings', 'members', 'invitations'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`flex-1 px-4 py-3 text-sm font-medium capitalize transition-colors
                ${activeTab === tab
                  ? `${isDark ? 'border-b-2 border-blue-500 text-blue-400' : 'border-b-2 border-blue-600 text-blue-600'}`
                  : `${isDark ? 'text-gray-400 hover:text-gray-200' : 'text-gray-500 hover:text-gray-700'}`
                }`}
            >
              {tab}
            </button>
          ))}
        </div>

        {/* Content */}
        <div className="p-4 max-h-96 overflow-y-auto">
          {/* Settings Tab */}
          {activeTab === 'settings' && settings && (
            <div className="space-y-4">
              {/* Your Role */}
              <div className={`p-3 rounded-lg ${isDark ? 'bg-gray-700' : 'bg-gray-100'}`}>
                <div className="flex items-center gap-2">
                  {getRoleIcon(settings.userRole || 'viewer')}
                  <span className="text-sm">
                    Your role: <strong className="capitalize">{settings.userRole || 'Guest'}</strong>
                  </span>
                </div>
              </div>

              {/* Password Protection */}
              <div className={`p-4 rounded-lg border ${isDark ? 'border-gray-700' : 'border-gray-200'}`}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    {settings.hasPassword ? <Lock className="w-5 h-5 text-green-500" /> : <Unlock className="w-5 h-5 text-gray-400" />}
                    <div>
                      <p className="font-medium">Password Protection</p>
                      <p className={`text-sm ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>
                        {settings.hasPassword ? 'Room is password protected' : 'Anyone with the link can join'}
                      </p>
                    </div>
                  </div>
                  {settings.isOwner && (
                    <button
                      onClick={() => settings.hasPassword ? handleRemovePassword() : setShowPasswordForm(true)}
                      className={`px-3 py-1.5 text-sm rounded ${
                        settings.hasPassword
                          ? 'bg-red-100 text-red-600 hover:bg-red-200'
                          : 'bg-blue-100 text-blue-600 hover:bg-blue-200'
                      }`}
                    >
                      {settings.hasPassword ? 'Remove' : 'Set Password'}
                    </button>
                  )}
                </div>

                {showPasswordForm && (
                  <div className="mt-3 flex gap-2">
                    <input
                      type="password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      placeholder="Enter new password"
                      className={`flex-1 px-3 py-2 rounded border ${isDark ? 'bg-gray-700 border-gray-600' : 'bg-white border-gray-300'}`}
                    />
                    <button
                      onClick={handleSetPassword}
                      disabled={!newPassword || loading}
                      className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
                    >
                      Save
                    </button>
                    <button
                      onClick={() => setShowPasswordForm(false)}
                      className={`px-4 py-2 rounded ${isDark ? 'bg-gray-700 hover:bg-gray-600' : 'bg-gray-200 hover:bg-gray-300'}`}
                    >
                      Cancel
                    </button>
                  </div>
                )}
              </div>

              {/* Public Room */}
              <div className={`p-4 rounded-lg border ${isDark ? 'border-gray-700' : 'border-gray-200'}`}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Globe className={`w-5 h-5 ${settings.isPublic ? 'text-green-500' : 'text-gray-400'}`} />
                    <div>
                      <p className="font-medium">Public Room</p>
                      <p className={`text-sm ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>
                        {settings.isPublic ? 'Room is visible to everyone' : 'Room is private'}
                      </p>
                    </div>
                  </div>
                  {settings.isOwner && (
                    <button
                      onClick={handleTogglePublic}
                      className={`relative w-12 h-6 rounded-full transition-colors ${
                        settings.isPublic ? 'bg-green-500' : isDark ? 'bg-gray-600' : 'bg-gray-300'
                      }`}
                    >
                      <span className={`absolute top-1 w-4 h-4 bg-white rounded-full transition-transform ${
                        settings.isPublic ? 'left-7' : 'left-1'
                      }`} />
                    </button>
                  )}
                </div>
              </div>

              {/* Allow Anonymous */}
              <div className={`p-4 rounded-lg border ${isDark ? 'border-gray-700' : 'border-gray-200'}`}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Users className={`w-5 h-5 ${settings.allowAnonymous ? 'text-green-500' : 'text-gray-400'}`} />
                    <div>
                      <p className="font-medium">Allow Anonymous Users</p>
                      <p className={`text-sm ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>
                        {settings.allowAnonymous ? 'Guests can join without signing in' : 'Users must sign in to join'}
                      </p>
                    </div>
                  </div>
                  {settings.isOwner && (
                    <button
                      onClick={handleToggleAnonymous}
                      className={`relative w-12 h-6 rounded-full transition-colors ${
                        settings.allowAnonymous ? 'bg-green-500' : isDark ? 'bg-gray-600' : 'bg-gray-300'
                      }`}
                    >
                      <span className={`absolute top-1 w-4 h-4 bg-white rounded-full transition-transform ${
                        settings.allowAnonymous ? 'left-7' : 'left-1'
                      }`} />
                    </button>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* Members Tab */}
          {activeTab === 'members' && (
            <div className="space-y-3">
              {members.length === 0 ? (
                <p className={`text-center py-8 ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>
                  No members yet
                </p>
              ) : (
                members.map((member) => (
                  <div
                    key={member.userId}
                    className={`flex items-center justify-between p-3 rounded-lg ${isDark ? 'bg-gray-700' : 'bg-gray-100'}`}
                  >
                    <div className="flex items-center gap-3">
                      <div className={`w-8 h-8 rounded-full flex items-center justify-center ${isDark ? 'bg-gray-600' : 'bg-gray-300'}`}>
                        {member.username.charAt(0).toUpperCase()}
                      </div>
                      <div>
                        <p className="font-medium">{member.username}</p>
                        {member.email && (
                          <p className={`text-xs ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>{member.email}</p>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      {getRoleIcon(member.role)}
                      {settings?.isOwner && member.role !== 'owner' && (
                        <>
                          <select
                            value={member.role}
                            onChange={(e) => handleUpdateMemberRole(member.userId, e.target.value as 'editor' | 'viewer')}
                            className={`text-sm px-2 py-1 rounded border ${isDark ? 'bg-gray-600 border-gray-500' : 'bg-white border-gray-300'}`}
                          >
                            <option value="editor">Editor</option>
                            <option value="viewer">Viewer</option>
                          </select>
                          <button
                            onClick={() => handleRemoveMember(member.userId)}
                            className="p-1 text-red-500 hover:bg-red-100 rounded"
                          >
                            <UserX className="w-4 h-4" />
                          </button>
                        </>
                      )}
                      {member.role === 'owner' && (
                        <span className="text-xs px-2 py-1 bg-yellow-100 text-yellow-700 rounded">Owner</span>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>
          )}

          {/* Invitations Tab */}
          {activeTab === 'invitations' && (
            <div className="space-y-3">
              {settings?.isOwner && (
                <div className="mb-4">
                  {!showInviteForm ? (
                    <button
                      onClick={() => setShowInviteForm(true)}
                      className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
                    >
                      <Plus className="w-4 h-4" />
                      Create Invitation
                    </button>
                  ) : (
                    <div className={`p-4 rounded-lg border ${isDark ? 'border-gray-700' : 'border-gray-200'}`}>
                      <div className="space-y-3">
                        <input
                          type="email"
                          value={inviteEmail}
                          onChange={(e) => setInviteEmail(e.target.value)}
                          placeholder="Email (optional)"
                          className={`w-full px-3 py-2 rounded border ${isDark ? 'bg-gray-700 border-gray-600' : 'bg-white border-gray-300'}`}
                        />
                        <select
                          value={inviteRole}
                          onChange={(e) => setInviteRole(e.target.value as 'editor' | 'viewer')}
                          className={`w-full px-3 py-2 rounded border ${isDark ? 'bg-gray-700 border-gray-600' : 'bg-white border-gray-300'}`}
                        >
                          <option value="editor">Editor - Can edit</option>
                          <option value="viewer">Viewer - View only</option>
                        </select>
                        <div className="flex gap-2">
                          <button
                            onClick={handleCreateInvitation}
                            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
                          >
                            Create
                          </button>
                          <button
                            onClick={() => setShowInviteForm(false)}
                            className={`px-4 py-2 rounded ${isDark ? 'bg-gray-700 hover:bg-gray-600' : 'bg-gray-200 hover:bg-gray-300'}`}
                          >
                            Cancel
                          </button>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              )}

              {invitations.length === 0 ? (
                <p className={`text-center py-8 ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>
                  No pending invitations
                </p>
              ) : (
                invitations.map((invitation) => (
                  <div
                    key={invitation.id}
                    className={`p-3 rounded-lg border ${isDark ? 'border-gray-700' : 'border-gray-200'}`}
                  >
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-2">
                        {getRoleIcon(invitation.role)}
                        <span className="capitalize">{invitation.role}</span>
                      </div>
                      <span className={`text-xs ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>
                        Expires: {new Date(invitation.expiresAt).toLocaleDateString()}
                      </span>
                    </div>
                    {invitation.email && (
                      <p className={`text-sm mb-2 ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>
                        For: {invitation.email}
                      </p>
                    )}
                    <div className="flex items-center gap-2">
                      <input
                        type="text"
                        value={invitation.inviteUrl}
                        readOnly
                        className={`flex-1 text-xs px-2 py-1 rounded border ${isDark ? 'bg-gray-700 border-gray-600' : 'bg-gray-100 border-gray-300'}`}
                      />
                      <button
                        onClick={() => handleCopyInviteLink(invitation.inviteUrl)}
                        className={`p-1.5 rounded ${isDark ? 'hover:bg-gray-700' : 'hover:bg-gray-200'}`}
                        title="Copy link"
                      >
                        <Copy className="w-4 h-4" />
                      </button>
                      {settings?.isOwner && (
                        <button
                          onClick={() => handleDeleteInvitation(invitation.id)}
                          className="p-1.5 text-red-500 hover:bg-red-100 rounded"
                          title="Delete invitation"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default RoomSettingsPanel;
