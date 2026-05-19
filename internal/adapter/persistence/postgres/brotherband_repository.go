package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// BrotherbandRequestRepository implements brotherband.RequestRepository.
type BrotherbandRequestRepository struct{ db DBTX }

// NewBrotherbandRequestRepository accepts any DBTX (pool in prod, tx in tests).
func NewBrotherbandRequestRepository(db DBTX) *BrotherbandRequestRepository {
	return &BrotherbandRequestRepository{db: db}
}

var _ brotherband.RequestRepository = (*BrotherbandRequestRepository)(nil)

func (r *BrotherbandRequestRepository) Save(ctx context.Context, req brotherband.Request) (brotherband.Request, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO brotherband_requests (requester_id, recipient_id)
		VALUES ($1, $2)
		RETURNING id, requester_id, recipient_id, created_at`,
		req.RequesterID().UUID(), req.RecipientID().UUID(),
	)
	var (
		id, requesterID, recipientID uuid.UUID
		createdAt                    time.Time
	)
	if err := row.Scan(&id, &requesterID, &recipientID, &createdAt); err != nil {
		if pgErrorMatches(err, pgUniqueViolation) {
			return brotherband.Request{}, brotherband.ErrRequestExists
		}
		if pgErrorMatches(err, pgForeignKeyViolation) {
			return brotherband.Request{}, brotherband.ErrRequestNotFound
		}
		return brotherband.Request{}, fmt.Errorf("postgres: save request: %w", err)
	}
	return brotherband.RehydrateRequest(
		shared.MustParseID(id.String()),
		shared.MustParseID(requesterID.String()),
		shared.MustParseID(recipientID.String()),
		createdAt,
	), nil
}

func (r *BrotherbandRequestRepository) FindByID(ctx context.Context, id shared.ID) (brotherband.Request, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, requester_id, recipient_id, created_at
		FROM brotherband_requests WHERE id = $1`, id.UUID())
	var (
		rowID, requesterID, recipientID uuid.UUID
		createdAt                       time.Time
	)
	if err := row.Scan(&rowID, &requesterID, &recipientID, &createdAt); err != nil {
		if isNoRows(err) {
			return brotherband.Request{}, brotherband.ErrRequestNotFound
		}
		return brotherband.Request{}, fmt.Errorf("postgres: find request: %w", err)
	}
	return brotherband.RehydrateRequest(
		shared.MustParseID(rowID.String()),
		shared.MustParseID(requesterID.String()),
		shared.MustParseID(recipientID.String()),
		createdAt,
	), nil
}

func (r *BrotherbandRequestRepository) Delete(ctx context.Context, id shared.ID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM brotherband_requests WHERE id = $1`, id.UUID())
	if err != nil {
		return fmt.Errorf("postgres: delete request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return brotherband.ErrRequestNotFound
	}
	return nil
}

func (r *BrotherbandRequestRepository) ListReceived(ctx context.Context, userID shared.ID) ([]brotherband.ReceivedRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.id, r.requester_id, r.recipient_id, r.created_at, u.username
		FROM brotherband_requests r
		JOIN users u ON u.id = r.requester_id
		WHERE r.recipient_id = $1
		ORDER BY r.created_at DESC`, userID.UUID())
	if err != nil {
		return nil, fmt.Errorf("postgres: list received: %w", err)
	}
	defer rows.Close()

	out := []brotherband.ReceivedRequest{}
	for rows.Next() {
		var (
			id, requesterID, recipientID uuid.UUID
			createdAt                    time.Time
			username                     string
		)
		if err := rows.Scan(&id, &requesterID, &recipientID, &createdAt, &username); err != nil {
			return nil, err
		}
		out = append(out, brotherband.ReceivedRequest{
			Request: brotherband.RehydrateRequest(
				shared.MustParseID(id.String()),
				shared.MustParseID(requesterID.String()),
				shared.MustParseID(recipientID.String()),
				createdAt,
			),
			RequesterUsername: username,
		})
	}
	return out, rows.Err()
}

func (r *BrotherbandRequestRepository) ListSent(ctx context.Context, userID shared.ID) ([]brotherband.SentRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.id, r.requester_id, r.recipient_id, r.created_at, u.username
		FROM brotherband_requests r
		JOIN users u ON u.id = r.recipient_id
		WHERE r.requester_id = $1
		ORDER BY r.created_at DESC`, userID.UUID())
	if err != nil {
		return nil, fmt.Errorf("postgres: list sent: %w", err)
	}
	defer rows.Close()

	out := []brotherband.SentRequest{}
	for rows.Next() {
		var (
			id, requesterID, recipientID uuid.UUID
			createdAt                    time.Time
			username                     string
		)
		if err := rows.Scan(&id, &requesterID, &recipientID, &createdAt, &username); err != nil {
			return nil, err
		}
		out = append(out, brotherband.SentRequest{
			Request: brotherband.RehydrateRequest(
				shared.MustParseID(id.String()),
				shared.MustParseID(requesterID.String()),
				shared.MustParseID(recipientID.String()),
				createdAt,
			),
			RecipientUsername: username,
		})
	}
	return out, rows.Err()
}

// BrotherhoodRepository implements brotherband.BrotherhoodRepository.
// Pairs are canonicalised to (low, high) so the relationship has
// exactly one row regardless of caller order.
type BrotherhoodRepository struct{ db DBTX }

// NewBrotherhoodRepository accepts any DBTX (pool in prod, tx in tests).
func NewBrotherhoodRepository(db DBTX) *BrotherhoodRepository {
	return &BrotherhoodRepository{db: db}
}

var _ brotherband.BrotherhoodRepository = (*BrotherhoodRepository)(nil)

func (r *BrotherhoodRepository) Save(ctx context.Context, b brotherband.Brotherhood) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO brotherhoods (user_low_id, user_high_id)
		VALUES (LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid))
		ON CONFLICT DO NOTHING`,
		b.UserA().UUID(), b.UserB().UUID(),
	)
	if err != nil {
		return fmt.Errorf("postgres: save brotherhood: %w", err)
	}
	return nil
}

func (r *BrotherhoodRepository) Delete(ctx context.Context, a, b shared.ID) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM brotherhoods
		WHERE user_low_id = LEAST($1::uuid, $2::uuid)
		  AND user_high_id = GREATEST($1::uuid, $2::uuid)`,
		a.UUID(), b.UUID(),
	)
	if err != nil {
		return fmt.Errorf("postgres: delete brotherhood: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return brotherband.ErrNotABrother
	}
	return nil
}

func (r *BrotherhoodRepository) Exists(ctx context.Context, a, b shared.ID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM brotherhoods
		  WHERE user_low_id = LEAST($1::uuid, $2::uuid)
		    AND user_high_id = GREATEST($1::uuid, $2::uuid)
		)`, a.UUID(), b.UUID()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres: brotherhood exists: %w", err)
	}
	return exists, nil
}

func (r *BrotherhoodRepository) ListBrothers(ctx context.Context, userID shared.ID) ([]brotherband.Brother, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.username, u.status, u.favorites, u.avatar_key, u.registered_at,
		       b.became_brothers_at
		FROM brotherhoods b
		JOIN users u ON u.id = CASE
		    WHEN b.user_low_id = $1 THEN b.user_high_id
		    ELSE b.user_low_id
		END
		WHERE b.user_low_id = $1 OR b.user_high_id = $1
		ORDER BY b.became_brothers_at DESC`, userID.UUID())
	if err != nil {
		return nil, fmt.Errorf("postgres: list brothers: %w", err)
	}
	defer rows.Close()

	out := []brotherband.Brother{}
	for rows.Next() {
		var (
			id                             uuid.UUID
			username, status               string
			favorites                      []string
			avatarKey                      *string
			registeredAt, becameBrothersAt time.Time
		)
		if err := rows.Scan(&id, &username, &status, &favorites, &avatarKey, &registeredAt, &becameBrothersAt); err != nil {
			return nil, err
		}
		out = append(out, brotherband.Brother{
			ID:               shared.MustParseID(id.String()),
			Username:         username,
			Status:           status,
			Favorites:        favorites,
			AvatarKey:        avatarKey,
			RegisteredAt:     registeredAt,
			BecameBrothersAt: becameBrothersAt,
		})
	}
	return out, rows.Err()
}
