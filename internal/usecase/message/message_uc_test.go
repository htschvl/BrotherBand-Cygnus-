package message_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	domainbb "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	domainmedia "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	domainmsg "github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/clock"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fakes"
	usecasemsg "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/message"
)

var fixedClock = clock.Fixed{At: time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)}

func capCtx(t *testing.T) (context.Context, *logging.Capture) {
	t.Helper()
	c := logging.NewCapture(slog.LevelDebug)
	return logging.WithLogger(context.Background(), c.Logger()), c
}

func brothers(t *testing.T) (*fakes.BrotherhoodRepo, shared.ID, shared.ID) {
	t.Helper()
	bonds := fakes.NewBrotherhoodRepo()
	a, b := shared.NewID(), shared.NewID()
	bond, err := domainbb.NewBrotherhood(a, b, fixedClock.Now())
	if err != nil {
		t.Fatalf("bond: %v", err)
	}
	if err := bonds.Save(context.Background(), bond); err != nil {
		t.Fatalf("save bond: %v", err)
	}
	return bonds, a, b
}

// ─── SendMessage ─────────────────────────────────────────────────────

func TestSendMessage_WhenBrothers_CreatesConversationAndLogs(t *testing.T) {
	t.Parallel()
	bonds, a, b := brothers(t)
	convs := fakes.NewConversationRepo()
	msgs := fakes.NewMessageRepo()
	uc := usecasemsg.NewSendMessage(convs, msgs, bonds, fixedClock, fakes.NewImageStore())
	ctx, c := capCtx(t)

	out, err := uc.Execute(ctx, usecasemsg.SendInput{SenderID: a, BrotherID: b, Body: "hey brother"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Body != "hey brother" {
		t.Fatalf("wrong body: %q", out.Body)
	}
	if _, ok := c.FindByMessage("conversation created on first message"); !ok {
		t.Fatal("first message must log conversation creation")
	}
	if _, ok := c.FindByMessage("message sent"); !ok {
		t.Fatal("expected INFO 'message sent'")
	}

	// Second message must reuse the conversation (no second 'created' line).
	c.Reset()
	if _, err := uc.Execute(ctx, usecasemsg.SendInput{SenderID: b, BrotherID: a, Body: "hi back"}); err != nil {
		t.Fatalf("second send: %v", err)
	}
	if _, ok := c.FindByMessage("conversation created on first message"); ok {
		t.Fatal("second message must NOT create a new conversation")
	}
}

func TestSendMessage_WhenNotBrothers_Forbidden(t *testing.T) {
	t.Parallel()
	uc := usecasemsg.NewSendMessage(
		fakes.NewConversationRepo(), fakes.NewMessageRepo(),
		fakes.NewBrotherhoodRepo(), fixedClock, fakes.NewImageStore(),
	)
	_, err := uc.Execute(context.Background(), usecasemsg.SendInput{
		SenderID: shared.NewID(), BrotherID: shared.NewID(), Body: "intrusion",
	})
	if !errors.Is(err, domainbb.ErrNotABrother) {
		t.Fatalf("expected ErrNotABrother, got %v", err)
	}
}

func TestSendMessage_EmptyBody_Rejected(t *testing.T) {
	t.Parallel()
	bonds, a, b := brothers(t)
	uc := usecasemsg.NewSendMessage(
		fakes.NewConversationRepo(), fakes.NewMessageRepo(), bonds, fixedClock, fakes.NewImageStore(),
	)
	_, err := uc.Execute(context.Background(), usecasemsg.SendInput{SenderID: a, BrotherID: b, Body: "   "})
	if !errors.Is(err, domainmsg.ErrInvalidBody) {
		t.Fatalf("expected ErrInvalidBody, got %v", err)
	}
}

func TestSendMessage_ConversationCreateFails_WrappedAndLoggedError(t *testing.T) {
	t.Parallel()
	bonds, a, b := brothers(t)
	uc := usecasemsg.NewSendMessage(
		fakes.FailingConversationRepo{}, fakes.NewMessageRepo(), bonds, fixedClock, fakes.NewImageStore(),
	)
	ctx, c := capCtx(t)
	_, err := uc.Execute(ctx, usecasemsg.SendInput{SenderID: a, BrotherID: b, Body: "hello"})
	if !errors.Is(err, fakes.ErrInjected) {
		t.Fatalf("expected wrapped injected error, got %v", err)
	}
	if _, ok := c.FindByMessage("send_message: conversation lookup failed"); !ok {
		t.Fatal("conversation failure must log at ERROR")
	}
}

// ─── ListMessages ────────────────────────────────────────────────────

func TestListMessages_NoConversationYet_ReturnsEmptyPage(t *testing.T) {
	t.Parallel()
	bonds, a, b := brothers(t)
	uc := usecasemsg.NewListMessages(
		fakes.NewConversationRepo(), fakes.NewMessageRepo(), bonds, fakes.NewImageStore(),
	)
	out, err := uc.Execute(context.Background(), usecasemsg.ListInput{ActorID: a, BrotherID: b})
	if err != nil {
		t.Fatalf("brothers with no messages must NOT error: %v", err)
	}
	if len(out.Items) != 0 || out.NextCursor != "" {
		t.Fatalf("expected an empty page, got %#v", out)
	}
}

func TestListMessages_Pagination_NewestFirstAndCursor(t *testing.T) {
	t.Parallel()
	bonds, a, b := brothers(t)
	convs := fakes.NewConversationRepo()
	msgs := fakes.NewMessageRepo()
	send := usecasemsg.NewSendMessage(convs, msgs, bonds, fixedClock, fakes.NewImageStore())

	// Three messages, strictly increasing timestamps via the clock.
	for i, body := range []string{"first", "second", "third"} {
		send2 := usecasemsg.NewSendMessage(convs, msgs, bonds,
			clock.Fixed{At: fixedClock.At.Add(time.Duration(i) * time.Minute)}, fakes.NewImageStore())
		if _, err := send2.Execute(context.Background(), usecasemsg.SendInput{SenderID: a, BrotherID: b, Body: body}); err != nil {
			t.Fatalf("send %q: %v", body, err)
		}
	}
	_ = send

	list := usecasemsg.NewListMessages(convs, msgs, bonds, fakes.NewImageStore())
	page1, err := list.Execute(context.Background(), usecasemsg.ListInput{ActorID: a, BrotherID: b, Limit: 2})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1.Items) != 2 {
		t.Fatalf("expected 2 items in page1, got %d", len(page1.Items))
	}
	if page1.Items[0].Body != "third" {
		t.Fatalf("newest-first ordering broken: %q", page1.Items[0].Body)
	}
	if page1.NextCursor == "" {
		t.Fatal("expected a next cursor when a full page is returned")
	}
	page2, err := list.Execute(context.Background(), usecasemsg.ListInput{
		ActorID: a, BrotherID: b, Limit: 2, Cursor: page1.NextCursor,
	})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2.Items) != 1 || page2.Items[0].Body != "first" {
		t.Fatalf("cursor pagination broken: %#v", page2.Items)
	}
}

func TestListMessages_BadCursor_Rejected(t *testing.T) {
	t.Parallel()
	bonds, a, b := brothers(t)
	convs := fakes.NewConversationRepo()
	msgs := fakes.NewMessageRepo()
	send := usecasemsg.NewSendMessage(convs, msgs, bonds, fixedClock, fakes.NewImageStore())
	if _, err := send.Execute(context.Background(), usecasemsg.SendInput{SenderID: a, BrotherID: b, Body: "x"}); err != nil {
		t.Fatalf("seed message: %v", err)
	}
	list := usecasemsg.NewListMessages(convs, msgs, bonds, fakes.NewImageStore())
	_, err := list.Execute(context.Background(), usecasemsg.ListInput{
		ActorID: a, BrotherID: b, Cursor: "!!!not-base64!!!",
	})
	if !errors.Is(err, domainmsg.ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

// ─── AttachMedia ─────────────────────────────────────────────────────

func TestAttachMedia_OnlySenderMayAttach(t *testing.T) {
	t.Parallel()
	msgs := fakes.NewMessageRepo()
	sender := shared.NewID()
	conv := shared.NewID()
	body, _ := domainmsg.NewBody("with photo")
	msg := domainmsg.New(conv, sender, body, fixedClock.Now())
	if _, err := msgs.Save(context.Background(), msg); err != nil {
		t.Fatalf("seed message: %v", err)
	}
	uc := usecasemsg.NewAttachMedia(msgs, fakes.NewImageStore(), fixedClock)

	intruder := shared.NewID()
	_, err := uc.Execute(context.Background(), usecasemsg.AttachInput{
		ActorID:   intruder,
		MessageID: msg.ID(),
		MediaKey:  "pending/" + intruder.String() + "/x.webp",
	})
	if !errors.Is(err, domainmsg.ErrNotParticipant) {
		t.Fatalf("non-sender must be rejected with ErrNotParticipant, got %v", err)
	}
}

func TestAttachMedia_HappyPath_PromotesAndRecords(t *testing.T) {
	t.Parallel()
	msgs := fakes.NewMessageRepo()
	store := fakes.NewImageStore()
	sender := shared.NewID()
	conv := shared.NewID()
	body, _ := domainmsg.NewBody("with photo")
	msg := domainmsg.New(conv, sender, body, fixedClock.Now())
	if _, err := msgs.Save(context.Background(), msg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	uc := usecasemsg.NewAttachMedia(msgs, store, fixedClock)
	ctx, c := capCtx(t)

	pendingKey := "pending/" + sender.String() + "/photo.jpg"
	out, err := uc.Execute(ctx, usecasemsg.AttachInput{ActorID: sender, MessageID: msg.ID(), MediaKey: pendingKey})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Attachments) != 1 || out.Attachments[0].ContentType != string(domainmedia.JPEG) {
		t.Fatalf("attachment not recorded correctly: %#v", out.Attachments)
	}
	if _, ok := store.Promoted[pendingKey]; !ok {
		t.Fatal("media must have been promoted out of pending/")
	}
	if _, ok := c.FindByMessage("attachment recorded"); !ok {
		t.Fatal("expected INFO 'attachment recorded'")
	}
}

func TestAttachMedia_NonOwnedKey_Rejected(t *testing.T) {
	t.Parallel()
	msgs := fakes.NewMessageRepo()
	sender := shared.NewID()
	body, _ := domainmsg.NewBody("x")
	msg := domainmsg.New(shared.NewID(), sender, body, fixedClock.Now())
	if _, err := msgs.Save(context.Background(), msg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	uc := usecasemsg.NewAttachMedia(msgs, fakes.NewImageStore(), fixedClock)
	_, err := uc.Execute(context.Background(), usecasemsg.AttachInput{
		ActorID:   sender,
		MessageID: msg.ID(),
		MediaKey:  "pending/" + shared.NewID().String() + "/notyours.webp",
	})
	if !errors.Is(err, domainmedia.ErrPromotionFailed) {
		t.Fatalf("expected ErrPromotionFailed for a non-owned key, got %v", err)
	}
}

// ─── ListConversations ───────────────────────────────────────────────

func TestListConversations_ReturnsRowPerBrother(t *testing.T) {
	t.Parallel()
	bonds := fakes.NewBrotherhoodRepo()
	me := shared.NewID()
	brotherID := shared.NewID()
	bonds.SetUserLookup(func(id shared.ID) domainbb.Brother {
		return domainbb.Brother{ID: id, Username: "bob", Status: "around"}
	})
	bond, _ := domainbb.NewBrotherhood(me, brotherID, fixedClock.Now())
	_ = bonds.Save(context.Background(), bond)

	uc := usecasemsg.NewListConversations(bonds, fakes.NewConversationRepo(), fakes.NewImageStore())
	out, err := uc.Execute(context.Background(), me)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].BrotherUsername != "bob" {
		t.Fatalf("expected one conversation row for the brother, got %#v", out)
	}
}
