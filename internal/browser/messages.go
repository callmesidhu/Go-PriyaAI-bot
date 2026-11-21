package browser

// IncomingMessage contains only the text and public conversation identifiers needed for a reply.
// Authentication material must never cross the browser-worker boundary.
type IncomingMessage struct{ ID, SenderID, Username, Text string }
type OutgoingMessage struct{ ConversationID, Text string }
