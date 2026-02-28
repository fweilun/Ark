// README: Relation route registration — mounts all relation endpoints onto the given router group.
package relation

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the relation endpoints onto the provided authenticated router group.
//
//	POST   /api/relations/requests
//	POST   /api/relations/requests/by-phone
//	GET    /api/relations/search
//	GET    /api/relations/requests/received
//	GET    /api/relations/requests/sent
//	DELETE /api/relations/requests/:friend_id
//	POST   /api/relations/requests/:friend_id/accept
//	POST   /api/relations/requests/:friend_id/reject
//	GET    /api/relations/friends
//	DELETE /api/relations/friends/:friend_id
//	GET    /api/relations/friends/:friend_id/is
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rel := rg.Group("/api/relations")

	// friend requests
	rel.POST("/requests", h.SendRequest)
	rel.POST("/requests/by-phone", h.SendRequestByPhone)
	rel.GET("/requests/received", h.ListReceived)
	rel.GET("/requests/sent", h.ListSent)
	rel.DELETE("/requests/:friend_id", h.CancelRequest)
	rel.POST("/requests/:friend_id/accept", h.AcceptRequest)
	rel.POST("/requests/:friend_id/reject", h.RejectRequest)

	// user search
	rel.GET("/search", h.SearchUsers)

	// friends
	rel.GET("/friends", h.ListFriends)
	rel.DELETE("/friends/:friend_id", h.RemoveFriend)
	rel.GET("/friends/:friend_id/is", h.IsFriend)
}
