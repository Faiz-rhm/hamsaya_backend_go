package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/observability"
	ws "github.com/hamsaya/backend/pkg/websocket"
	"go.uber.org/zap"
)

// ChatService handles chat operations
type ChatService struct {
	conversationRepo    repositories.ConversationRepository
	messageRepo         repositories.MessageRepository
	userRepo            repositories.UserRepository
	businessRepo        repositories.BusinessRepository
	relationshipsRepo   repositories.RelationshipsRepository
	notificationService *NotificationService
	wsHub               *ws.Hub
	logger              *zap.Logger
}

// NewChatService creates a new chat service
func NewChatService(
	conversationRepo repositories.ConversationRepository,
	messageRepo repositories.MessageRepository,
	userRepo repositories.UserRepository,
	businessRepo repositories.BusinessRepository,
	relationshipsRepo repositories.RelationshipsRepository,
	notificationService *NotificationService,
	wsHub *ws.Hub,
	logger *zap.Logger,
) *ChatService {
	return &ChatService{
		conversationRepo:    conversationRepo,
		messageRepo:         messageRepo,
		userRepo:            userRepo,
		businessRepo:        businessRepo,
		relationshipsRepo:   relationshipsRepo,
		notificationService: notificationService,
		wsHub:               wsHub,
		logger:              logger,
	}
}

// SendMessage sends a message to another user
func (s *ChatService) SendMessage(ctx context.Context, senderID string, req *models.SendMessageRequest) (*models.MessageResponse, error) {
	// Validate message type — accept TEXT, IMAGE, FILE, LOCATION.
	switch req.MessageType {
	case models.MessageTypeText, models.MessageTypeImage, models.MessageTypeFile, models.MessageTypeLocation, models.MessageTypeVoice:
		// valid
	default:
		return nil, utils.NewBadRequestError("message_type must be one of: TEXT IMAGE FILE LOCATION VOICE", nil)
	}

	// Validate message content
	if req.MessageType == models.MessageTypeText && (req.Content == nil || *req.Content == "") {
		return nil, utils.NewBadRequestError("Content is required for text messages", nil)
	}

	// Reject self-messaging — would violate ordered_participants CHECK constraint
	// (participant1_id < participant2_id) and is meaningless UX-wise.
	if senderID == req.RecipientID {
		return nil, utils.NewBadRequestError("Cannot send a message to yourself", nil)
	}

	// Block check: if either side blocked the other, refuse send. Apple UGC
	// compliance + general user safety. Two IsBlocked calls cover both
	// directions (sender→recipient and recipient→sender) using the existing
	// relationships repo — no schema or new method needed.
	if s.relationshipsRepo != nil {
		if blocked, _ := s.relationshipsRepo.IsBlocked(ctx, senderID, req.RecipientID); blocked {
			return nil, utils.NewBadRequestError("Unable to send message", nil)
		}
		if blocked, _ := s.relationshipsRepo.IsBlocked(ctx, req.RecipientID, senderID); blocked {
			return nil, utils.NewBadRequestError("Unable to send message", nil)
		}
	}

	// Get or create conversation (optionally scoped to a business)
	conversation, err := s.conversationRepo.GetOrCreate(ctx, senderID, req.RecipientID, req.BusinessID)
	if err != nil {
		s.logger.Error("Failed to get or create conversation",
			zap.Error(err),
			zap.String("sender_id", senderID),
			zap.String("recipient_id", req.RecipientID),
		)
		return nil, utils.NewInternalError("Failed to create conversation", err)
	}

	// Create message
	messageID := uuid.New().String()
	message := &models.Message{
		ID:               messageID,
		ConversationID:   conversation.ID,
		SenderID:         senderID,
		Content:          req.Content,
		MessageType:      req.MessageType,
		ProductID:        req.ProductID,
		ReplyToMessageID: req.ReplyToMessageID,
		CreatedAt:        time.Now(),
	}

	if err := s.messageRepo.Create(ctx, message); err != nil {
		s.logger.Error("Failed to create message",
			zap.Error(err),
			zap.String("conversation_id", conversation.ID),
		)
		return nil, utils.NewInternalError("Failed to send message", err)
	}

	observability.RecordMessageCreated(ctx)

	// Update conversation's last_message_at
	if err := s.conversationRepo.UpdateLastMessageAt(ctx, conversation.ID); err != nil {
		s.logger.Warn("Failed to update last_message_at",
			zap.Error(err),
			zap.String("conversation_id", conversation.ID),
		)
	}

	s.logger.Info("Message sent",
		zap.String("message_id", messageID),
		zap.String("sender_id", senderID),
		zap.String("recipient_id", req.RecipientID),
	)

	// Send real-time notification to recipient via WebSocket. Pass the
	// conversation so the persisted notification can be stamped with
	// business_id when the chat is business-scoped.
	go s.notifyMessageSent(message, req.RecipientID, conversation)

	// Get enriched message response
	return s.enrichMessage(ctx, message, senderID)
}

// GetConversations retrieves all conversations for a user. businessID nil =
// personal chats only; non-nil = chats scoped to that business.
func (s *ChatService) GetConversations(ctx context.Context, userID string, limit, offset int, businessID *string) ([]*models.ConversationResponse, error) {
	filter := &models.GetConversationsFilter{
		UserID:     userID,
		BusinessID: businessID,
		Limit:      limit,
		Offset:     offset,
	}

	conversations, err := s.conversationRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list conversations",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return nil, utils.NewInternalError("Failed to get conversations", err)
	}

	// Enrich conversations
	var enrichedConversations []*models.ConversationResponse
	for _, conversation := range conversations {
		enriched, err := s.enrichConversation(ctx, conversation, userID)
		if err != nil {
			s.logger.Warn("Failed to enrich conversation",
				zap.Error(err),
				zap.String("conversation_id", conversation.ID),
			)
			continue
		}
		enrichedConversations = append(enrichedConversations, enriched)
	}

	return enrichedConversations, nil
}

// GetMessages retrieves messages in a conversation
func (s *ChatService) GetMessages(ctx context.Context, userID, conversationID string, limit, offset int) ([]*models.MessageResponse, error) {
	// Check if user is participant
	isParticipant, err := s.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		s.logger.Error("Failed to check participant",
			zap.Error(err),
			zap.String("conversation_id", conversationID),
		)
		return nil, utils.NewInternalError("Failed to verify access", err)
	}

	if !isParticipant {
		return nil, utils.NewForbiddenError("You don't have access to this conversation", nil)
	}

	// Get messages
	filter := &models.GetMessagesFilter{
		ConversationID: conversationID,
		ViewerID:       userID,
		Limit:          limit,
		Offset:         offset,
	}

	messages, err := s.messageRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list messages",
			zap.Error(err),
			zap.String("conversation_id", conversationID),
		)
		return nil, utils.NewInternalError("Failed to get messages", err)
	}

	// Enrich messages
	var enrichedMessages []*models.MessageResponse
	for _, message := range messages {
		enriched, err := s.enrichMessage(ctx, message, userID)
		if err != nil {
			s.logger.Warn("Failed to enrich message",
				zap.Error(err),
				zap.String("message_id", message.ID),
			)
			continue
		}
		enrichedMessages = append(enrichedMessages, enriched)
	}

	return enrichedMessages, nil
}

// MarkConversationAsRead marks all unread messages in a conversation as read
func (s *ChatService) MarkConversationAsRead(ctx context.Context, userID, conversationID string) error {
	// Check if user is participant
	isParticipant, err := s.conversationRepo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return utils.NewInternalError("Failed to verify access", err)
	}

	if !isParticipant {
		return utils.NewForbiddenError("You don't have access to this conversation", nil)
	}

	// Mark as read
	if err := s.messageRepo.MarkConversationAsRead(ctx, conversationID, userID); err != nil {
		s.logger.Error("Failed to mark conversation as read",
			zap.Error(err),
			zap.String("conversation_id", conversationID),
		)
		return utils.NewInternalError("Failed to mark as read", err)
	}

	s.logger.Info("Conversation marked as read",
		zap.String("conversation_id", conversationID),
		zap.String("user_id", userID),
	)

	// Also clear the MESSAGE notifications for this conversation from the bell
	// badge, so the user doesn't have to open the notification screen to mark
	// them read. Best-effort — never fail the read flow on this.
	if s.notificationService != nil {
		s.notificationService.MarkConversationRead(ctx, userID, conversationID)
	}

	return nil
}

// DeleteMessage soft-deletes a message for everyone in the conversation
// (sender-only). Broadcasts a WS "message_deleted" frame to the other
// participant so their UI removes the bubble in real time.
func (s *ChatService) DeleteMessage(ctx context.Context, userID, messageID string) error {
	message, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return utils.NewNotFoundError("Message not found", err)
	}

	if message.SenderID != userID {
		return utils.NewForbiddenError("You can only delete your own messages", nil)
	}

	if err := s.messageRepo.Delete(ctx, messageID); err != nil {
		s.logger.Error("Failed to delete message",
			zap.Error(err),
			zap.String("message_id", messageID),
		)
		return utils.NewInternalError("Failed to delete message", err)
	}

	s.logger.Info("Message deleted",
		zap.String("message_id", messageID),
		zap.String("user_id", userID),
	)

	// Broadcast to the other participant so their open chat removes the
	// bubble without waiting for a refresh. Done in a goroutine so the
	// HTTP request returns immediately even if the WS hub is slow.
	if s.wsHub != nil {
		go s.broadcastMessageDeleted(message)
	}

	return nil
}

// DeleteMessageForMe removes the message for the requesting user only.
// Other participants continue to see it. Any participant (sender OR
// recipient) may call this on a message they can currently see.
func (s *ChatService) DeleteMessageForMe(ctx context.Context, userID, messageID string) error {
	message, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return utils.NewNotFoundError("Message not found", err)
	}

	// Authorize: user must be a participant of the conversation. The simplest
	// path is checking they sent the message OR are the other participant of
	// the conversation — reuse the existing IsParticipant repo method.
	isParticipant, perr := s.conversationRepo.IsParticipant(ctx, message.ConversationID, userID)
	if perr != nil {
		return utils.NewInternalError("Failed to verify access", perr)
	}
	if !isParticipant {
		return utils.NewForbiddenError("You don't have access to this message", nil)
	}

	if err := s.messageRepo.DeleteForUser(ctx, messageID, userID); err != nil {
		s.logger.Error("Failed to delete message for user",
			zap.Error(err),
			zap.String("message_id", messageID),
			zap.String("user_id", userID),
		)
		return utils.NewInternalError("Failed to delete message", err)
	}

	s.logger.Info("Message hidden for user",
		zap.String("message_id", messageID),
		zap.String("user_id", userID),
	)
	return nil
}

// broadcastMessageDeleted notifies the other conversation participant that a
// message was removed-for-everyone. Looks up the conversation so business
// scope can be stamped on the WS payload (mirrors the new-message frame).
func (s *ChatService) broadcastMessageDeleted(message *models.Message) {
	ctx := context.Background()
	convo, cerr := s.conversationRepo.GetByID(ctx, message.ConversationID)
	if cerr != nil {
		s.logger.Debug("delete broadcast: failed to load conversation",
			zap.Error(cerr),
			zap.String("conversation_id", message.ConversationID),
		)
		return
	}

	// Find the other participant id (everyone except the sender).
	other, oerr := s.conversationRepo.GetOtherParticipantID(ctx, convo.ID, message.SenderID)
	if oerr != nil || other == "" {
		return
	}

	var businessID *string
	if convo.BusinessID != nil && *convo.BusinessID != "" {
		businessID = convo.BusinessID
	}

	frame := models.WSMessage{
		Type: "message_deleted",
		Payload: models.WSMessageDeletedPayload{
			ConversationID: message.ConversationID,
			MessageID:      message.ID,
			BusinessID:     businessID,
		},
	}
	if err := s.wsHub.SendToUser(other, frame); err != nil {
		s.logger.Debug("Failed to send WS message_deleted",
			zap.Error(err),
			zap.String("recipient_id", other),
		)
	}
}

// EditMessage replaces the text of a message the user sent. Only the sender
// may edit, and only TEXT messages. Returns the enriched updated message and
// broadcasts a `message_edited` frame to the other participant.
func (s *ChatService) EditMessage(ctx context.Context, userID, messageID, content string) (*models.MessageResponse, error) {
	message, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, utils.NewNotFoundError("Message not found", err)
	}

	if message.SenderID != userID {
		return nil, utils.NewForbiddenError("You can only edit your own messages", nil)
	}

	if message.MessageType != models.MessageTypeText {
		return nil, utils.NewBadRequestError("Only text messages can be edited", nil)
	}

	updated, err := s.messageRepo.UpdateContent(ctx, messageID, content)
	if err != nil {
		s.logger.Error("Failed to edit message",
			zap.Error(err),
			zap.String("message_id", messageID),
		)
		return nil, utils.NewInternalError("Failed to edit message", err)
	}

	s.logger.Info("Message edited",
		zap.String("message_id", messageID),
		zap.String("user_id", userID),
	)

	if s.wsHub != nil {
		go s.broadcastMessageEdited(updated)
	}

	return s.enrichMessage(ctx, updated, userID)
}

// broadcastMessageEdited notifies the other conversation participant that a
// message's text changed so their open chat updates the bubble in real time.
func (s *ChatService) broadcastMessageEdited(message *models.Message) {
	ctx := context.Background()
	convo, cerr := s.conversationRepo.GetByID(ctx, message.ConversationID)
	if cerr != nil {
		s.logger.Debug("edit broadcast: failed to load conversation",
			zap.Error(cerr),
			zap.String("conversation_id", message.ConversationID),
		)
		return
	}

	other, oerr := s.conversationRepo.GetOtherParticipantID(ctx, convo.ID, message.SenderID)
	if oerr != nil || other == "" {
		return
	}

	var businessID *string
	if convo.BusinessID != nil && *convo.BusinessID != "" {
		businessID = convo.BusinessID
	}

	editedAt := message.CreatedAt
	if message.EditedAt != nil {
		editedAt = *message.EditedAt
	}

	frame := models.WSMessage{
		Type: "message_edited",
		Payload: models.WSMessageEditedPayload{
			ConversationID: message.ConversationID,
			MessageID:      message.ID,
			Content:        message.Content,
			EditedAt:       editedAt,
			BusinessID:     businessID,
		},
	}
	if err := s.wsHub.SendToUser(other, frame); err != nil {
		s.logger.Debug("Failed to send WS message_edited",
			zap.Error(err),
			zap.String("recipient_id", other),
		)
	}
}

// ReactToMessage toggles an emoji reaction by the user on a message. add=true
// adds the reaction, add=false removes it. Authorizes the user as a participant
// and broadcasts the change to the other participant over WebSocket.
func (s *ChatService) ReactToMessage(ctx context.Context, userID, messageID, emoji string, add bool) error {
	message, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return utils.NewNotFoundError("Message not found", err)
	}

	isParticipant, perr := s.conversationRepo.IsParticipant(ctx, message.ConversationID, userID)
	if perr != nil {
		return utils.NewInternalError("Failed to verify access", perr)
	}
	if !isParticipant {
		return utils.NewForbiddenError("You don't have access to this message", nil)
	}

	if add {
		if err := s.messageRepo.AddReaction(ctx, messageID, userID, emoji); err != nil {
			return utils.NewInternalError("Failed to add reaction", err)
		}
	} else {
		if err := s.messageRepo.RemoveReaction(ctx, messageID, userID, emoji); err != nil {
			return utils.NewInternalError("Failed to remove reaction", err)
		}
	}

	go s.broadcastReaction(message, userID, emoji, add)
	return nil
}

// broadcastReaction pushes a reaction add/remove to the other participant.
func (s *ChatService) broadcastReaction(message *models.Message, userID, emoji string, added bool) {
	if s.wsHub == nil {
		return
	}
	other, oerr := s.conversationRepo.GetOtherParticipantID(context.Background(), message.ConversationID, userID)
	if oerr != nil || other == "" {
		return
	}
	frame := models.WSMessage{
		Type: "message_reaction",
		Payload: models.WSReactionPayload{
			ConversationID: message.ConversationID,
			MessageID:      message.ID,
			UserID:         userID,
			Emoji:          emoji,
			Added:          added,
		},
	}
	if err := s.wsHub.SendToUser(other, frame); err != nil {
		s.logger.Debug("Failed to send WS message_reaction", zap.Error(err), zap.String("recipient_id", other))
	}
}

// enrichConversation enriches a conversation with participant and last message info
func (s *ChatService) enrichConversation(ctx context.Context, conversation *models.Conversation, viewerID string) (*models.ConversationResponse, error) {
	response := &models.ConversationResponse{
		ID:            conversation.ID,
		LastMessageAt: conversation.LastMessageAt,
		CreatedAt:     conversation.CreatedAt,
	}

	// Attach business reference when this conversation is business-scoped.
	if conversation.BusinessID != nil && *conversation.BusinessID != "" && s.businessRepo != nil {
		biz, berr := s.businessRepo.GetByID(ctx, *conversation.BusinessID)
		if berr == nil && biz != nil {
			response.Business = &models.ConversationBizRef{
				ID:     biz.ID,
				Name:   biz.Name,
				Avatar: biz.Avatar,
			}
		}
	}

	// Get other participant ID
	otherParticipantID, err := s.conversationRepo.GetOtherParticipantID(ctx, conversation.ID, viewerID)
	if err != nil {
		return nil, err
	}

	// Get other participant's profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, otherParticipantID)
	if err == nil {
		firstName := ""
		if profile.FirstName != nil {
			firstName = *profile.FirstName
		}
		lastName := ""
		if profile.LastName != nil {
			lastName = *profile.LastName
		}
		avatarColor := profile.AvatarColor
		if avatarColor == nil || *avatarColor == "" {
			c := models.DefaultAvatarColorForProfile(profile.ID)
			avatarColor = &c
		}
		response.OtherParticipant = &models.UserInfo{
			UserID:      otherParticipantID,
			FirstName:   firstName,
			LastName:    lastName,
			FullName:    profile.FullName(),
			Avatar:      profile.Avatar,
			AvatarColor: avatarColor,
		}
	}

	// Get last message visible to this viewer (per-user deletes excluded).
	lastMessage, err := s.messageRepo.GetLastMessage(ctx, conversation.ID, viewerID)
	if err == nil && lastMessage != nil {
		response.LastMessage = &models.MessageInfo{
			ID:          lastMessage.ID,
			Content:     lastMessage.Content,
			MessageType: lastMessage.MessageType,
			SenderID:    lastMessage.SenderID,
			CreatedAt:   lastMessage.CreatedAt,
		}
	}

	// Get unread count
	unreadCount, err := s.messageRepo.GetUnreadCount(ctx, conversation.ID, viewerID)
	if err == nil {
		response.UnreadCount = unreadCount
	}

	return response, nil
}

// enrichMessage enriches a message with sender info
func (s *ChatService) enrichMessage(ctx context.Context, message *models.Message, viewerID string) (*models.MessageResponse, error) {
	response := &models.MessageResponse{
		ID:             message.ID,
		ConversationID: message.ConversationID,
		Content:        message.Content,
		MessageType:    message.MessageType,
		ProductID:      message.ProductID,
		IsRead:         message.ReadAt != nil,
		CreatedAt:      message.CreatedAt,
		EditedAt:       message.EditedAt,
	}

	// Quoted message preview (reply target).
	if message.ReplyToMessageID != nil && *message.ReplyToMessageID != "" {
		if replied, rErr := s.messageRepo.GetByID(ctx, *message.ReplyToMessageID); rErr == nil && replied != nil {
			response.ReplyTo = &models.MessageReplyPreview{
				ID:          replied.ID,
				SenderID:    replied.SenderID,
				Content:     replied.Content,
				MessageType: replied.MessageType,
			}
		}
	}

	// Emoji reactions, aggregated, with the viewer's own reactions flagged.
	if reactions, rErr := s.messageRepo.GetReactions(ctx, []string{message.ID}, viewerID); rErr == nil {
		response.Reactions = reactions[message.ID]
	}

	// Get sender's profile
	profile, err := s.userRepo.GetProfileByUserID(ctx, message.SenderID)
	if err == nil {
		firstName := ""
		if profile.FirstName != nil {
			firstName = *profile.FirstName
		}
		lastName := ""
		if profile.LastName != nil {
			lastName = *profile.LastName
		}
		avatarColor := profile.AvatarColor
		if avatarColor == nil || *avatarColor == "" {
			c := models.DefaultAvatarColorForProfile(profile.ID)
			avatarColor = &c
		}
		response.Sender = &models.UserInfo{
			UserID:      message.SenderID,
			FirstName:   firstName,
			LastName:    lastName,
			FullName:    profile.FullName(),
			Avatar:      profile.Avatar,
			AvatarColor: avatarColor,
		}
	}

	return response, nil
}

// notifyMessageSent sends a WebSocket notification to the recipient and
// triggers a persisted notification + FCM push so the user sees it when offline.
// [conversation] is optional — when supplied and BusinessID is set, the
// persisted notification gets `data.business_id` so the business-scoped
// unread-count and notification list pick it up.
func (s *ChatService) notifyMessageSent(message *models.Message, recipientID string, conversation *models.Conversation) {
	// Real-time WebSocket frame for foreground app
	if s.wsHub != nil {
		var businessID *string
		if conversation != nil && conversation.BusinessID != nil && *conversation.BusinessID != "" {
			businessID = conversation.BusinessID
		}
		wsMessage := models.WSMessage{
			Type: "message",
			Payload: models.WSMessagePayload{
				ConversationID: message.ConversationID,
				MessageID:      message.ID,
				SenderID:       message.SenderID,
				BusinessID:     businessID,
				Content:        message.Content,
				MessageType:    message.MessageType,
				CreatedAt:      message.CreatedAt,
			},
		}
		if err := s.wsHub.SendToUser(recipientID, wsMessage); err != nil {
			s.logger.Debug("Failed to send WebSocket notification",
				zap.Error(err),
				zap.String("recipient_id", recipientID),
			)
		}
	}

	// Persisted notification + FCM push (for background/closed-app delivery).
	// Skip when the recipient currently has the conversation open in the
	// foreground — the WS frame above already updated their UI, a push
	// notification would be redundant and noisy.
	if s.notificationService == nil {
		return
	}
	if s.wsHub != nil && s.wsHub.IsUserActiveInConversation(recipientID, message.ConversationID) {
		s.logger.Debug("Skipping push: recipient actively viewing conversation",
			zap.String("recipient_id", recipientID),
			zap.String("conversation_id", message.ConversationID),
		)
		return
	}

	ctx := context.Background()
	senderProfile, err := s.userRepo.GetProfileByUserID(ctx, message.SenderID)
	senderName := "New message"
	if err == nil && senderProfile != nil {
		fn := senderProfile.FullName()
		if fn != "" {
			senderName = fn
		}
	}

	preview := "Sent a message"
	switch message.MessageType {
	case models.MessageTypeImage:
		preview = "📷 Photo"
	case models.MessageTypeLocation:
		preview = "📍 Location"
	case models.MessageTypeFile:
		preview = "📎 File"
	case models.MessageTypeVoice:
		preview = "🎤 Voice message"
	default:
		if message.Content != nil && *message.Content != "" {
			c := *message.Content
			if len(c) > 80 {
				c = c[:80] + "…"
			}
			preview = c
		}
	}

	data := map[string]interface{}{
		"actor_id":        message.SenderID,
		"actor_name":      senderName, // used by NotificationCard for display name
		"conversation_id": message.ConversationID,
		"message_id":      message.ID,
		"recipient_name":  senderName,
	}
	if senderProfile != nil && senderProfile.Avatar != nil {
		data["actor_avatar"] = senderProfile.Avatar.URL
		data["recipient_avatar"] = senderProfile.Avatar.URL
	}
	if senderProfile != nil && senderProfile.AvatarColor != nil {
		data["actor_avatar_color"] = *senderProfile.AvatarColor
	}
	// Tag with business_id when the conversation is business-scoped so the
	// business notification page + dashboard badge filter sees it. Falls
	// back to looking the conversation up via the message id when the
	// caller didn't pass one.
	convo := conversation
	if convo == nil && s.conversationRepo != nil {
		if c, err := s.conversationRepo.GetByID(ctx, message.ConversationID); err == nil {
			convo = c
		}
	}
	if convo != nil && convo.BusinessID != nil && *convo.BusinessID != "" {
		data["business_id"] = *convo.BusinessID

		// When the SENDER is the business owner replying inside the
		// business-scoped chat, surface the business identity (name + logo)
		// instead of the owner's personal profile so the buyer sees
		// "Hamsaya Bakery" rather than "John Doe" in their notification.
		if s.businessRepo != nil {
			biz, berr := s.businessRepo.GetByID(ctx, *convo.BusinessID)
			if berr == nil && biz != nil && biz.UserID == message.SenderID {
				if biz.Name != "" {
					senderName = biz.Name
					data["actor_name"] = biz.Name
					data["recipient_name"] = biz.Name
				}
				if biz.Avatar != nil {
					data["actor_avatar"] = biz.Avatar.URL
					data["recipient_avatar"] = biz.Avatar.URL
				}
				if biz.AvatarColor != nil {
					data["actor_avatar_color"] = *biz.AvatarColor
				}
				data["business_name"] = biz.Name
			}
		}
	}

	title := senderName
	_, nerr := s.notificationService.CreateNotification(ctx, &models.CreateNotificationRequest{
		UserID:  recipientID,
		Type:    models.NotificationTypeMessage,
		Title:   &title,
		Message: &preview,
		Data:    data,
	})
	if nerr != nil {
		s.logger.Warn("Failed to create chat notification",
			zap.Error(nerr),
			zap.String("recipient_id", recipientID),
		)
	}
}
