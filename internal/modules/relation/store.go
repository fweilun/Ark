// README: Relation store — PostgreSQL-backed persistence for the friendships table.
package relation

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RelationStore defines the persistence operations required by the relation Service.
type RelationStore interface {
	CreateRequest(ctx context.Context, from, to UserID) error
	CancelRequest(ctx context.Context, from, to UserID) error
	UpdateStatus(ctx context.Context, from, to UserID, status FriendshipStatus) error
	GetFriendship(ctx context.Context, uid1, uid2 UserID) (*Friendship, error)
	HasActiveRelation(ctx context.Context, uid1, uid2 UserID) (bool, error)
	RemoveFriend(ctx context.Context, uid1, uid2 UserID) error
	ListReceived(ctx context.Context, userID UserID) ([]FriendRequest, error)
	ListSent(ctx context.Context, userID UserID) ([]FriendRequest, error)
	ListFriends(ctx context.Context, userID UserID) ([]Friend, error)
	FindByPhone(ctx context.Context, phone string) (*User, error)
	SearchUsers(ctx context.Context, query string) ([]User, error)
}

// Store is the PostgreSQL implementation of RelationStore.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// CreateRequest inserts a pending friend request row.
func (s *Store) CreateRequest(ctx context.Context, from, to UserID) error {
	now := time.Now()
	_, err := s.db.Exec(ctx, `
		INSERT INTO friendships (user_id, friend_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)`,
		string(from), string(to), StatusPending, now,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return err
	}
	return nil
}

// CancelRequest deletes a pending outgoing request from → to.
func (s *Store) CancelRequest(ctx context.Context, from, to UserID) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM friendships
		WHERE user_id = $1 AND friend_id = $2 AND status = $3`,
		string(from), string(to), StatusPending,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus changes the status of the request from → to (must currently be pending).
func (s *Store) UpdateStatus(ctx context.Context, from, to UserID, status FriendshipStatus) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE friendships SET status = $1, updated_at = NOW()
		WHERE user_id = $2 AND friend_id = $3 AND status = $4`,
		status, string(from), string(to), StatusPending,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetFriendship returns the accepted friendship row between uid1 and uid2.
func (s *Store) GetFriendship(ctx context.Context, uid1, uid2 UserID) (*Friendship, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, user_id, friend_id, status, group_id, remark, created_at, updated_at
		FROM friendships
		WHERE ((user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1))
		  AND status = $3
		LIMIT 1`,
		string(uid1), string(uid2), StatusAccepted,
	)
	var f Friendship
	var groupID sql.NullInt32
	var remark sql.NullString
	err := row.Scan(&f.ID, &f.UserID, &f.FriendID, &f.Status, &groupID, &remark, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if groupID.Valid {
		v := int(groupID.Int32)
		f.GroupID = &v
	}
	if remark.Valid {
		f.Remark = &remark.String
	}
	return &f, nil
}

// HasActiveRelation reports whether a pending or accepted friendship row exists
// between uid1 and uid2 in either direction.
func (s *Store) HasActiveRelation(ctx context.Context, uid1, uid2 UserID) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM friendships
			WHERE ((user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1))
			  AND status IN ($3, $4)
		)`,
		string(uid1), string(uid2), StatusPending, StatusAccepted,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// RemoveFriend deletes the accepted friendship record between uid1 and uid2.
func (s *Store) RemoveFriend(ctx context.Context, uid1, uid2 UserID) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM friendships
		WHERE ((user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1))
		  AND status = $3`,
		string(uid1), string(uid2), StatusAccepted,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListReceived returns pending friend requests where userID is the recipient.
func (s *Store) ListReceived(ctx context.Context, userID UserID) ([]FriendRequest, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id, friend_id, status, created_at
		FROM friendships
		WHERE friend_id = $1 AND status = $2
		ORDER BY created_at DESC`,
		string(userID), StatusPending,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []FriendRequest
	for rows.Next() {
		var r FriendRequest
		if err := rows.Scan(&r.FromUserID, &r.ToUserID, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		reqs = append(reqs, r)
	}
	return reqs, rows.Err()
}

// ListSent returns pending friend requests sent by userID.
func (s *Store) ListSent(ctx context.Context, userID UserID) ([]FriendRequest, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id, friend_id, status, created_at
		FROM friendships
		WHERE user_id = $1 AND status = $2
		ORDER BY created_at DESC`,
		string(userID), StatusPending,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []FriendRequest
	for rows.Next() {
		var r FriendRequest
		if err := rows.Scan(&r.FromUserID, &r.ToUserID, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		reqs = append(reqs, r)
	}
	return reqs, rows.Err()
}

// ListFriends returns accepted friends with their user details.
func (s *Store) ListFriends(ctx context.Context, userID UserID) ([]Friend, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.user_id, u.name, u.phone, f.remark, f.group_id, f.updated_at
		FROM friendships f
		JOIN users u ON u.user_id = CASE
			WHEN f.user_id = $1 THEN f.friend_id
			ELSE f.user_id
		END
		WHERE (f.user_id = $1 OR f.friend_id = $1) AND f.status = $2
		ORDER BY u.name`,
		string(userID), StatusAccepted,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var friends []Friend
	for rows.Next() {
		var fr Friend
		var remark sql.NullString
		var groupID sql.NullInt32
		if err := rows.Scan(&fr.UserID, &fr.Name, &fr.Phone, &remark, &groupID, &fr.Since); err != nil {
			return nil, err
		}
		if remark.Valid {
			fr.Remark = &remark.String
		}
		if groupID.Valid {
			v := int(groupID.Int32)
			fr.GroupID = &v
		}
		friends = append(friends, fr)
	}
	return friends, rows.Err()
}

// FindByPhone looks up a user by their phone number.
func (s *Store) FindByPhone(ctx context.Context, phone string) (*User, error) {
	row := s.db.QueryRow(ctx, `
		SELECT user_id, name, phone FROM users WHERE phone = $1`, phone,
	)
	var u User
	err := row.Scan(&u.UserID, &u.Name, &u.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// SearchUsers returns up to 20 users whose name or phone matches the query string.
func (s *Store) SearchUsers(ctx context.Context, query string) ([]User, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id, name, phone FROM users
		WHERE name ILIKE '%' || $1 || '%' OR phone LIKE '%' || $1 || '%'
		ORDER BY name
		LIMIT 20`,
		query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.UserID, &u.Name, &u.Phone); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
