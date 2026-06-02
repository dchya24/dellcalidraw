package websocket

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Role constants
const (
	RoleOwner  = "owner"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

// handleJoinRoomWithPassword handles join_room with optional password
func (h *Hub) handleJoinRoomWithPassword(conn *Connection, payload map[string]interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var joinMsg JoinRoomWithPasswordPayload
	if err := json.Unmarshal(data, &joinMsg); err != nil {
		h.sendError(conn, "Invalid join room payload", "invalid_payload")
		return
	}

	// Check if room requires password
	if h.dbClient != nil {
		roomDBID, err := h.dbClient.GetRoomByKey(joinMsg.RoomID)
		if err == nil && roomDBID != "" {
			hasPassword, err := h.dbClient.HasRoomPassword(roomDBID)
			if err == nil && hasPassword {
				// Verify password
				valid, err := h.dbClient.VerifyRoomPassword(roomDBID, joinMsg.Password)
				if err != nil || !valid {
					h.sendMessage(conn, "password_required", PasswordRequiredPayload{
						RoomID: joinMsg.RoomID,
					})
					return
				}
			}

			// Check if room allows anonymous users
			settings, err := h.dbClient.GetRoomSettings(roomDBID)
			if err == nil && settings != nil {
				if !settings.AllowAnonymous && conn.AuthUserID == "" {
					h.sendError(conn, "This room requires authentication", "auth_required")
					return
				}
			}
		}
	}

	// Delegate to regular join handler with the parsed payload
	h.handleJoinRoom(conn, payload)
}

// handleGetRoomSettings handles get_room_settings messages
func (h *Hub) handleGetRoomSettings(conn *Connection, payload map[string]interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var msg struct {
		RoomID string `json:"roomId"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	if h.dbClient == nil {
		// Return default settings if no database
		h.sendMessage(conn, "room_settings", RoomSettingsPayload{
			RoomID:         msg.RoomID,
			HasPassword:    false,
			IsPublic:       true,
			AllowAnonymous: true,
			IsOwner:        false,
			UserRole:       "",
		})
		return
	}

	roomDBID, err := h.dbClient.GetRoomByKey(msg.RoomID)
	if err != nil || roomDBID == "" {
		h.sendError(conn, "Room not found", "room_not_found")
		return
	}

	settings, err := h.dbClient.GetRoomSettings(roomDBID)
	if err != nil {
		h.sendError(conn, "Failed to get room settings", "internal_error")
		return
	}

	// Determine user's role
	var userRole string
	var isOwner bool
	if conn.AuthUserID != "" {
		role, err := h.dbClient.GetUserRole(roomDBID, conn.AuthUserID)
		if err == nil {
			userRole = role
			isOwner = role == RoleOwner
		}
	}

	h.sendMessage(conn, "room_settings", RoomSettingsPayload{
		RoomID:         msg.RoomID,
		HasPassword:    settings.HasPassword,
		IsPublic:       settings.IsPublic,
		AllowAnonymous: settings.AllowAnonymous,
		IsOwner:        isOwner,
		UserRole:       userRole,
	})
}

// handleSetRoomPassword handles set_room_password messages
func (h *Hub) handleSetRoomPassword(conn *Connection, payload map[string]interface{}) {
	if conn.AuthUserID == "" {
		h.sendError(conn, "Authentication required", "auth_required")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var msg SetRoomPasswordPayload
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	if h.dbClient == nil {
		h.sendError(conn, "Database not available", "db_unavailable")
		return
	}

	roomDBID, err := h.dbClient.GetOrCreateRoom(msg.RoomID)
	if err != nil {
		h.sendError(conn, "Failed to get room", "internal_error")
		return
	}

	// Check if user is owner
	canManage, err := h.dbClient.CanUserPerformAction(roomDBID, conn.AuthUserID, "change_settings")
	if err != nil || !canManage {
		// If no owner yet, claim ownership
		ownerID, _ := h.dbClient.GetRoomOwner(roomDBID)
		if ownerID == nil {
			// First authenticated user becomes owner
			if err := h.dbClient.SetRoomOwner(roomDBID, conn.AuthUserID); err != nil {
				h.sendError(conn, "Failed to set room owner", "internal_error")
				return
			}
		} else {
			h.sendMessage(conn, "permission_denied", PermissionDeniedPayload{
				Action:  "set_password",
				Message: "Only the room owner can set a password",
			})
			return
		}
	}

	// Set or remove password
	if msg.Password == "" {
		err = h.dbClient.RemoveRoomPassword(roomDBID)
	} else {
		err = h.dbClient.SetRoomPassword(roomDBID, msg.Password)
	}

	if err != nil {
		h.sendError(conn, "Failed to set password", "internal_error")
		return
	}

	// Broadcast settings update to room
	h.broadcastRoomSettingsUpdate(msg.RoomID, roomDBID)

	slog.Info("Room password updated", "roomID", msg.RoomID, "hasPassword", msg.Password != "")
}

// handleUpdateRoomSettings handles update_room_settings messages
func (h *Hub) handleUpdateRoomSettings(conn *Connection, payload map[string]interface{}) {
	if conn.AuthUserID == "" {
		h.sendError(conn, "Authentication required", "auth_required")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var msg UpdateRoomSettingsPayload
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	if h.dbClient == nil {
		h.sendError(conn, "Database not available", "db_unavailable")
		return
	}

	roomDBID, err := h.dbClient.GetOrCreateRoom(msg.RoomID)
	if err != nil {
		h.sendError(conn, "Failed to get room", "internal_error")
		return
	}

	// Check permission
	canManage, err := h.dbClient.CanUserPerformAction(roomDBID, conn.AuthUserID, "change_settings")
	if err != nil || !canManage {
		h.sendMessage(conn, "permission_denied", PermissionDeniedPayload{
			Action:  "update_settings",
			Message: "Only the room owner can change settings",
		})
		return
	}

	// Get current settings
	settings, err := h.dbClient.GetRoomSettings(roomDBID)
	if err != nil {
		h.sendError(conn, "Failed to get room settings", "internal_error")
		return
	}

	// Apply updates
	isPublic := settings.IsPublic
	allowAnonymous := settings.AllowAnonymous

	if msg.IsPublic != nil {
		isPublic = *msg.IsPublic
	}
	if msg.AllowAnonymous != nil {
		allowAnonymous = *msg.AllowAnonymous
	}

	if err := h.dbClient.UpdateRoomSettings(roomDBID, isPublic, allowAnonymous); err != nil {
		h.sendError(conn, "Failed to update settings", "internal_error")
		return
	}

	// Broadcast settings update
	h.broadcastRoomSettingsUpdate(msg.RoomID, roomDBID)

	slog.Info("Room settings updated", "roomID", msg.RoomID, "isPublic", isPublic, "allowAnonymous", allowAnonymous)
}

// handleGetRoomMembers handles get_room_members messages
func (h *Hub) handleGetRoomMembers(conn *Connection, payload map[string]interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var msg GetRoomMembersPayload
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	if h.dbClient == nil {
		h.sendMessage(conn, "room_members", RoomMembersListPayload{
			RoomID:  msg.RoomID,
			Members: []RoomMemberPayload{},
		})
		return
	}

	roomDBID, err := h.dbClient.GetRoomByKey(msg.RoomID)
	if err != nil || roomDBID == "" {
		h.sendError(conn, "Room not found", "room_not_found")
		return
	}

	members, err := h.dbClient.GetRoomMembers(roomDBID)
	if err != nil {
		h.sendError(conn, "Failed to get members", "internal_error")
		return
	}

	// Also include owner
	ownerID, _ := h.dbClient.GetRoomOwner(roomDBID)

	memberPayloads := make([]RoomMemberPayload, 0, len(members)+1)

	// Add owner first if exists
	if ownerID != nil {
		user, err := h.dbClient.GetUserByID(*ownerID)
		if err == nil && user != nil {
			memberPayloads = append(memberPayloads, RoomMemberPayload{
				UserID:   user.ID,
				Username: user.Username,
				Email:    user.Email,
				Role:     RoleOwner,
			})
		}
	}

	// Add other members
	for _, m := range members {
		// Skip if already added as owner
		if ownerID != nil && m.UserID == *ownerID {
			continue
		}
		memberPayloads = append(memberPayloads, RoomMemberPayload{
			UserID:   m.UserID,
			Username: m.Username,
			Email:    m.Email,
			Role:     m.Role,
		})
	}

	h.sendMessage(conn, "room_members", RoomMembersListPayload{
		RoomID:  msg.RoomID,
		Members: memberPayloads,
	})
}

// handleUpdateMemberRole handles update_member_role messages
func (h *Hub) handleUpdateMemberRole(conn *Connection, payload map[string]interface{}) {
	if conn.AuthUserID == "" {
		h.sendError(conn, "Authentication required", "auth_required")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var msg UpdateMemberRolePayload
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	// Validate role
	if msg.Role != RoleEditor && msg.Role != RoleViewer {
		h.sendError(conn, "Invalid role. Must be 'editor' or 'viewer'", "invalid_role")
		return
	}

	if h.dbClient == nil {
		h.sendError(conn, "Database not available", "db_unavailable")
		return
	}

	roomDBID, err := h.dbClient.GetRoomByKey(msg.RoomID)
	if err != nil || roomDBID == "" {
		h.sendError(conn, "Room not found", "room_not_found")
		return
	}

	// Check permission
	canManage, err := h.dbClient.CanUserPerformAction(roomDBID, conn.AuthUserID, "manage_members")
	if err != nil || !canManage {
		h.sendMessage(conn, "permission_denied", PermissionDeniedPayload{
			Action:  "update_member_role",
			Message: "Only the room owner can manage members",
		})
		return
	}

	// Cannot change owner's role
	ownerID, _ := h.dbClient.GetRoomOwner(roomDBID)
	if ownerID != nil && *ownerID == msg.UserID {
		h.sendError(conn, "Cannot change owner's role", "invalid_operation")
		return
	}

	// Update role
	if err := h.dbClient.UpdateRoomMemberRole(roomDBID, msg.UserID, msg.Role); err != nil {
		// If member doesn't exist, add them
		if err.Error() == "member not found" {
			if err := h.dbClient.AddRoomMember(roomDBID, msg.UserID, msg.Role, &conn.AuthUserID); err != nil {
				h.sendError(conn, "Failed to add member", "internal_error")
				return
			}
		} else {
			h.sendError(conn, "Failed to update role", "internal_error")
			return
		}
	}

	// Get username for broadcast
	user, _ := h.dbClient.GetUserByID(msg.UserID)
	username := ""
	if user != nil {
		username = user.Username
	}

	// Broadcast role update
	h.broadcastToRoom(msg.RoomID, "member_role_updated", MemberRoleUpdatedPayload{
		RoomID:    msg.RoomID,
		UserID:    msg.UserID,
		Username:  username,
		Role:      msg.Role,
		UpdatedBy: conn.AuthUserID,
	}, "")

	slog.Info("Member role updated", "roomID", msg.RoomID, "userID", msg.UserID, "role", msg.Role)
}

// handleRemoveMember handles remove_member messages
func (h *Hub) handleRemoveMember(conn *Connection, payload map[string]interface{}) {
	if conn.AuthUserID == "" {
		h.sendError(conn, "Authentication required", "auth_required")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var msg RemoveMemberPayload
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	if h.dbClient == nil {
		h.sendError(conn, "Database not available", "db_unavailable")
		return
	}

	roomDBID, err := h.dbClient.GetRoomByKey(msg.RoomID)
	if err != nil || roomDBID == "" {
		h.sendError(conn, "Room not found", "room_not_found")
		return
	}

	// Check permission (owner can remove anyone, users can remove themselves)
	canManage, _ := h.dbClient.CanUserPerformAction(roomDBID, conn.AuthUserID, "manage_members")
	if !canManage && conn.AuthUserID != msg.UserID {
		h.sendMessage(conn, "permission_denied", PermissionDeniedPayload{
			Action:  "remove_member",
			Message: "You can only remove yourself or be the room owner",
		})
		return
	}

	// Cannot remove owner
	ownerID, _ := h.dbClient.GetRoomOwner(roomDBID)
	if ownerID != nil && *ownerID == msg.UserID {
		h.sendError(conn, "Cannot remove room owner", "invalid_operation")
		return
	}

	// Remove member
	if err := h.dbClient.RemoveRoomMember(roomDBID, msg.UserID); err != nil {
		h.sendError(conn, "Failed to remove member", "internal_error")
		return
	}

	// Broadcast removal
	h.broadcastToRoom(msg.RoomID, "member_removed", MemberRemovedPayload{
		RoomID:    msg.RoomID,
		UserID:    msg.UserID,
		RemovedBy: conn.AuthUserID,
	}, "")

	slog.Info("Member removed", "roomID", msg.RoomID, "userID", msg.UserID, "removedBy", conn.AuthUserID)
}

// handleCreateInvitation handles create_invitation messages
func (h *Hub) handleCreateInvitation(conn *Connection, payload map[string]interface{}) {
	if conn.AuthUserID == "" {
		h.sendError(conn, "Authentication required", "auth_required")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var msg CreateInvitationPayload
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	// Validate role
	if msg.Role != RoleEditor && msg.Role != RoleViewer {
		h.sendError(conn, "Invalid role. Must be 'editor' or 'viewer'", "invalid_role")
		return
	}

	if h.dbClient == nil {
		h.sendError(conn, "Database not available", "db_unavailable")
		return
	}

	roomDBID, err := h.dbClient.GetOrCreateRoom(msg.RoomID)
	if err != nil {
		h.sendError(conn, "Failed to get room", "internal_error")
		return
	}

	// Check permission
	canManage, err := h.dbClient.CanUserPerformAction(roomDBID, conn.AuthUserID, "manage_members")
	if err != nil || !canManage {
		// If no owner, claim ownership
		ownerID, _ := h.dbClient.GetRoomOwner(roomDBID)
		if ownerID == nil {
			if err := h.dbClient.SetRoomOwner(roomDBID, conn.AuthUserID); err != nil {
				h.sendError(conn, "Failed to set room owner", "internal_error")
				return
			}
		} else {
			h.sendMessage(conn, "permission_denied", PermissionDeniedPayload{
				Action:  "create_invitation",
				Message: "Only the room owner can create invitations",
			})
			return
		}
	}

	// Create invitation (expires in 7 days)
	invitation, err := h.dbClient.CreateRoomInvitation(
		roomDBID,
		msg.Email,
		msg.Role,
		&conn.AuthUserID,
		7*24*time.Hour,
	)
	if err != nil {
		h.sendError(conn, "Failed to create invitation", "internal_error")
		return
	}

	// Generate invite URL
	inviteURL := fmt.Sprintf("http://localhost:3000?invite=%s", invitation.Token)

	// Handle ExpiresAt which could be time.Time or *time.Time
	var expiresAtStr string
	switch v := invitation.ExpiresAt.(type) {
	case time.Time:
		expiresAtStr = v.Format(time.RFC3339)
	case *time.Time:
		if v != nil {
			expiresAtStr = v.Format(time.RFC3339)
		}
	}

	h.sendMessage(conn, "invitation_created", InvitationPayload{
		ID:        invitation.ID,
		RoomID:    msg.RoomID,
		Email:     invitation.Email,
		Role:      invitation.Role,
		Token:     invitation.Token,
		InviteURL: inviteURL,
		ExpiresAt: expiresAtStr,
	})

	slog.Info("Invitation created", "roomID", msg.RoomID, "email", msg.Email, "role", msg.Role)
}

// handleAcceptInvitation handles accept_invitation messages
func (h *Hub) handleAcceptInvitation(conn *Connection, payload map[string]interface{}) {
	if conn.AuthUserID == "" {
		h.sendError(conn, "Authentication required to accept invitation", "auth_required")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var msg AcceptInvitationPayload
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	if h.dbClient == nil {
		h.sendError(conn, "Database not available", "db_unavailable")
		return
	}

	// Get invitation
	invitation, err := h.dbClient.GetRoomInvitationByToken(msg.Token)
	if err != nil {
		h.sendError(conn, "Failed to get invitation", "internal_error")
		return
	}
	if invitation == nil {
		h.sendError(conn, "Invitation not found", "invitation_not_found")
		return
	}

	// Check if expired
	var isExpired bool
	switch v := invitation.ExpiresAt.(type) {
	case time.Time:
		isExpired = time.Now().After(v)
	case *time.Time:
		if v != nil {
			isExpired = time.Now().After(*v)
		}
	}
	if isExpired {
		h.sendError(conn, "Invitation has expired", "invitation_expired")
		return
	}

	// Check if already used
	if invitation.UsedAt != nil {
		h.sendError(conn, "Invitation has already been used", "invitation_used")
		return
	}

	// Add user as member
	if err := h.dbClient.AddRoomMember(invitation.RoomID, conn.AuthUserID, invitation.Role, invitation.InvitedBy); err != nil {
		h.sendError(conn, "Failed to add member", "internal_error")
		return
	}

	// Mark invitation as used
	if err := h.dbClient.UseRoomInvitation(msg.Token); err != nil {
		slog.Warn("Failed to mark invitation as used", "error", err)
	}

	h.sendMessage(conn, "invitation_accepted", map[string]interface{}{
		"roomId": invitation.RoomID,
		"role":   invitation.Role,
	})

	slog.Info("Invitation accepted", "userID", conn.AuthUserID, "role", invitation.Role)
}

// handleGetInvitations handles get_invitations messages
func (h *Hub) handleGetInvitations(conn *Connection, payload map[string]interface{}) {
	if conn.AuthUserID == "" {
		h.sendError(conn, "Authentication required", "auth_required")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var msg GetInvitationsPayload
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	if h.dbClient == nil {
		h.sendMessage(conn, "invitations_list", InvitationsListPayload{
			RoomID:      msg.RoomID,
			Invitations: []InvitationPayload{},
		})
		return
	}

	roomDBID, err := h.dbClient.GetRoomByKey(msg.RoomID)
	if err != nil || roomDBID == "" {
		h.sendError(conn, "Room not found", "room_not_found")
		return
	}

	// Check permission
	canManage, _ := h.dbClient.CanUserPerformAction(roomDBID, conn.AuthUserID, "manage_members")
	if !canManage {
		h.sendMessage(conn, "permission_denied", PermissionDeniedPayload{
			Action:  "get_invitations",
			Message: "Only the room owner can view invitations",
		})
		return
	}

	invitations, err := h.dbClient.GetRoomInvitations(roomDBID)
	if err != nil {
		h.sendError(conn, "Failed to get invitations", "internal_error")
		return
	}

	invitationPayloads := make([]InvitationPayload, len(invitations))
	for i, inv := range invitations {
		inviteURL := fmt.Sprintf("http://localhost:3000?invite=%s", inv.Token)

		// Handle ExpiresAt which could be time.Time or *time.Time
		var expiresAtStr string
		switch v := inv.ExpiresAt.(type) {
		case time.Time:
			expiresAtStr = v.Format(time.RFC3339)
		case *time.Time:
			if v != nil {
				expiresAtStr = v.Format(time.RFC3339)
			}
		}

		invitationPayloads[i] = InvitationPayload{
			ID:        inv.ID,
			RoomID:    msg.RoomID,
			Email:     inv.Email,
			Role:      inv.Role,
			Token:     inv.Token,
			InviteURL: inviteURL,
			ExpiresAt: expiresAtStr,
		}
	}

	h.sendMessage(conn, "invitations_list", InvitationsListPayload{
		RoomID:      msg.RoomID,
		Invitations: invitationPayloads,
	})
}

// handleDeleteInvitation handles delete_invitation messages
func (h *Hub) handleDeleteInvitation(conn *Connection, payload map[string]interface{}) {
	if conn.AuthUserID == "" {
		h.sendError(conn, "Authentication required", "auth_required")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	var msg DeleteInvitationPayload
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(conn, "Invalid payload", "invalid_payload")
		return
	}

	if h.dbClient == nil {
		h.sendError(conn, "Database not available", "db_unavailable")
		return
	}

	roomDBID, err := h.dbClient.GetRoomByKey(msg.RoomID)
	if err != nil || roomDBID == "" {
		h.sendError(conn, "Room not found", "room_not_found")
		return
	}

	// Check permission
	canManage, _ := h.dbClient.CanUserPerformAction(roomDBID, conn.AuthUserID, "manage_members")
	if !canManage {
		h.sendMessage(conn, "permission_denied", PermissionDeniedPayload{
			Action:  "delete_invitation",
			Message: "Only the room owner can delete invitations",
		})
		return
	}

	if err := h.dbClient.DeleteRoomInvitation(msg.InvitationID); err != nil {
		h.sendError(conn, "Failed to delete invitation", "internal_error")
		return
	}

	h.sendMessage(conn, "invitation_deleted", map[string]string{
		"invitationId": msg.InvitationID,
	})

	slog.Info("Invitation deleted", "invitationID", msg.InvitationID)
}

// broadcastRoomSettingsUpdate broadcasts updated room settings to all participants
func (h *Hub) broadcastRoomSettingsUpdate(roomKey, roomDBID string) {
	if h.dbClient == nil {
		return
	}

	settings, err := h.dbClient.GetRoomSettings(roomDBID)
	if err != nil {
		return
	}

	// Broadcast to all connections in the room
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conn := range h.connections {
		if conn.RoomID == roomKey {
			var userRole string
			var isOwner bool
			if conn.AuthUserID != "" {
				role, err := h.dbClient.GetUserRole(roomDBID, conn.AuthUserID)
				if err == nil {
					userRole = role
					isOwner = role == RoleOwner
				}
			}

			h.sendMessage(conn, "room_settings_updated", RoomSettingsPayload{
				RoomID:         roomKey,
				HasPassword:    settings.HasPassword,
				IsPublic:       settings.IsPublic,
				AllowAnonymous: settings.AllowAnonymous,
				IsOwner:        isOwner,
				UserRole:       userRole,
			})
		}
	}
}

// checkEditPermission checks if a user can edit elements in a room
func (h *Hub) checkEditPermission(conn *Connection) bool {
	if h.dbClient == nil {
		return true // No database, allow all
	}

	if conn.RoomID == "" {
		return false
	}

	roomDBID, err := h.dbClient.GetRoomByKey(conn.RoomID)
	if err != nil || roomDBID == "" {
		return true // Room not in DB yet, allow
	}

	canEdit, err := h.dbClient.CanUserPerformAction(roomDBID, conn.AuthUserID, "edit")
	if err != nil {
		return true // Error checking, allow by default
	}

	return canEdit
}
