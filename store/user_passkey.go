package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// UserPasskey is a WebAuthn credential registered by a user.
type UserPasskey struct {
	ID           int32
	UserID       int32
	CredentialID string // base64url (raw, no padding) credential ID
	Name         string
	Credential   string // JSON-encoded webauthn.Credential
	CreatedTs    int64
	LastUsedTs   int64
}

// FindUserPasskey filters passkeys.
type FindUserPasskey struct {
	ID           *int32
	UserID       *int32
	CredentialID *string
}

// rebind converts '?' placeholders to the driver's syntax.
func (s *Store) rebind(query string) string {
	if s.profile == nil || s.profile.Driver != "postgres" {
		return query
	}
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (s *Store) CreateUserPasskey(ctx context.Context, p *UserPasskey) (*UserPasskey, error) {
	db := s.driver.GetDB()
	p.CreatedTs = time.Now().Unix()
	if s.profile != nil && s.profile.Driver == "postgres" {
		q := s.rebind("INSERT INTO user_passkey (user_id, credential_id, name, credential, created_ts) VALUES (?, ?, ?, ?, ?) RETURNING id")
		if err := db.QueryRowContext(ctx, q, p.UserID, p.CredentialID, p.Name, p.Credential, p.CreatedTs).Scan(&p.ID); err != nil {
			return nil, err
		}
		return p, nil
	}
	res, err := db.ExecContext(ctx, "INSERT INTO user_passkey (user_id, credential_id, name, credential, created_ts) VALUES (?, ?, ?, ?, ?)", p.UserID, p.CredentialID, p.Name, p.Credential, p.CreatedTs)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	p.ID = int32(id)
	return p, nil
}

func (s *Store) ListUserPasskeys(ctx context.Context, find *FindUserPasskey) ([]*UserPasskey, error) {
	where, args := []string{"1 = 1"}, []any{}
	if find.ID != nil {
		where, args = append(where, "id = ?"), append(args, *find.ID)
	}
	if find.UserID != nil {
		where, args = append(where, "user_id = ?"), append(args, *find.UserID)
	}
	if find.CredentialID != nil {
		where, args = append(where, "credential_id = ?"), append(args, *find.CredentialID)
	}
	q := s.rebind("SELECT id, user_id, credential_id, name, credential, created_ts, last_used_ts FROM user_passkey WHERE " + strings.Join(where, " AND ") + " ORDER BY id ASC")
	rows, err := s.driver.GetDB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*UserPasskey
	for rows.Next() {
		p := &UserPasskey{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.CredentialID, &p.Name, &p.Credential, &p.CreatedTs, &p.LastUsedTs); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *Store) GetUserPasskey(ctx context.Context, find *FindUserPasskey) (*UserPasskey, error) {
	list, err := s.ListUserPasskeys(ctx, find)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return list[0], nil
}

// UpdateUserPasskeyUsage stores the refreshed credential (sign count, flags) after a login.
func (s *Store) UpdateUserPasskeyUsage(ctx context.Context, id int32, credential string) error {
	_, err := s.driver.GetDB().ExecContext(ctx, s.rebind("UPDATE user_passkey SET credential = ?, last_used_ts = ? WHERE id = ?"), credential, time.Now().Unix(), id)
	return err
}

// DeleteUserPasskey removes a passkey owned by userID.
func (s *Store) DeleteUserPasskey(ctx context.Context, userID, id int32) (bool, error) {
	res, err := s.driver.GetDB().ExecContext(ctx, s.rebind("DELETE FROM user_passkey WHERE id = ? AND user_id = ?"), id, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

