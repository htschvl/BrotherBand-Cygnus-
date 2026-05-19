package brotherband

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
)

const componentCut = "usecase.brotherband.cut"

// CutBrotherband ends a confirmed brotherhood. Either side may cut.
// The repository handles the canonical (low, high) ordering so the
// use case is order-agnostic.
type CutBrotherband struct {
	brotherhood brotherband.BrotherhoodRepository
}

// NewCutBrotherband wires the use case against the brotherhood
// repository.
func NewCutBrotherband(brotherhood brotherband.BrotherhoodRepository) *CutBrotherband {
	return &CutBrotherband{brotherhood: brotherhood}
}

func (uc *CutBrotherband) Execute(ctx context.Context, in CutInput) error {
	log := logging.FromContext(ctx).With(
		logging.Component(componentCut),
		logging.UserID(in.UserID),
		logging.TargetUserID(in.BrotherID),
	)

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cut: context cancelled: %w", err)
	}

	exists, err := uc.brotherhood.Exists(ctx, in.UserID, in.BrotherID)
	if err != nil {
		log.LogAttrs(ctx, slog.LevelError, "cut: probe failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return fmt.Errorf("cut: probe: %w", err)
	}
	if !exists {
		log.LogAttrs(ctx, slog.LevelInfo, "cut rejected: no brotherhood between these users")
		return brotherband.ErrNotABrother
	}
	if err := uc.brotherhood.Delete(ctx, in.UserID, in.BrotherID); err != nil {
		log.LogAttrs(ctx, slog.LevelError, "cut: delete failed",
			slog.String(logging.AttrError, err.Error()),
		)
		return fmt.Errorf("cut: %w", err)
	}
	log.LogAttrs(ctx, slog.LevelInfo, "brotherhood cut",
		slog.String(logging.AttrEvent, "brotherband.cut"),
	)
	return nil
}
