package brotherband

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

const componentListRequests = "usecase.brotherband.list_requests"

// ListRequests returns the user's pending brotherband requests, in
// either or both directions.
type ListRequests struct {
	requests brotherband.RequestRepository
}

// NewListRequests wires the pending-requests list.
func NewListRequests(requests brotherband.RequestRepository) *ListRequests {
	return &ListRequests{requests: requests}
}

const (
	DirectionReceived = "received"
	DirectionSent     = "sent"
	DirectionAll      = "all"
)

func (uc *ListRequests) Execute(ctx context.Context, in ListRequestsInput) (ListRequestsOutput, error) {
	log := logging.FromContext(ctx).With(logging.Component(componentListRequests), logging.UserID(in.UserID))

	if err := ctx.Err(); err != nil {
		return ListRequestsOutput{}, fmt.Errorf("list_requests: context cancelled: %w", err)
	}

	out := ListRequestsOutput{}
	wantReceived := in.Direction == "" || in.Direction == DirectionAll || in.Direction == DirectionReceived
	wantSent := in.Direction == "" || in.Direction == DirectionAll || in.Direction == DirectionSent

	if wantReceived {
		received, err := uc.requests.ListReceived(ctx, in.UserID)
		if err != nil {
			log.LogAttrs(ctx, slog.LevelError, "list_requests: received query failed",
				slog.String(logging.AttrError, err.Error()),
			)
			return ListRequestsOutput{}, fmt.Errorf("list_requests: received: %w", err)
		}
		out.Received = make([]RequestView, 0, len(received))
		for _, r := range received {
			out.Received = append(out.Received, RequestView{
				ID:                r.ID(),
				RequesterID:       r.RequesterID(),
				RecipientID:       r.RecipientID(),
				RequesterUsername: r.RequesterUsername,
				CreatedAt:         r.CreatedAt(),
			})
		}
	}
	if wantSent {
		sent, err := uc.requests.ListSent(ctx, in.UserID)
		if err != nil {
			log.LogAttrs(ctx, slog.LevelError, "list_requests: sent query failed",
				slog.String(logging.AttrError, err.Error()),
			)
			return ListRequestsOutput{}, fmt.Errorf("list_requests: sent: %w", err)
		}
		out.Sent = make([]RequestView, 0, len(sent))
		for _, r := range sent {
			out.Sent = append(out.Sent, RequestView{
				ID:                r.ID(),
				RequesterID:       r.RequesterID(),
				RecipientID:       r.RecipientID(),
				RecipientUsername: r.RecipientUsername,
				CreatedAt:         r.CreatedAt(),
			})
		}
	}
	log.LogAttrs(ctx, slog.LevelDebug, "brotherband requests listed",
		slog.Int("received", len(out.Received)),
		slog.Int("sent", len(out.Sent)),
	)
	return out, nil
}
