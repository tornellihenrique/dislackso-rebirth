// Package livekit implements the media provider with a LiveKit server:
// participant tokens, the room API (Twirp) and webhook verification.
package livekit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/livekit/protocol/auth"
	lkproto "github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/webhook"
	"github.com/twitchtv/twirp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// adminTokenTTL covers one API call.
const adminTokenTTL = time.Minute

// maxWebhookBytes bounds a webhook body; the verifier reads it whole.
const maxWebhookBytes = 1 << 20

type Config struct {
	// PublicURL is where clients connect, such as "/lk" behind the hub's proxy.
	PublicURL string
	// APIURL reaches the server's HTTP API, such as "http://127.0.0.1:7880".
	APIURL    string
	APIKey    string
	APISecret string
	// Ready reports whether the server is up; nil means always.
	Ready      func() bool
	HTTPClient *http.Client
}

type Provider struct {
	c     Config
	rooms lkproto.RoomService
	keys  auth.KeyProvider
}

var _ ports.MediaProvider = (*Provider)(nil)

func New(c Config) *Provider {
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Provider{
		c:     c,
		rooms: lkproto.NewRoomServiceProtobufClient(strings.TrimRight(c.APIURL, "/"), client),
		keys:  auth.NewSimpleKeyProvider(c.APIKey, c.APISecret),
	}
}

func (p *Provider) URL() string { return p.c.PublicURL }

func (p *Provider) Available() bool { return p.c.Ready == nil || p.c.Ready() }

var sources = map[domain.MediaSource]lkproto.TrackSource{
	domain.SourceMicrophone:       lkproto.TrackSource_MICROPHONE,
	domain.SourceCamera:           lkproto.TrackSource_CAMERA,
	domain.SourceScreenShare:      lkproto.TrackSource_SCREEN_SHARE,
	domain.SourceScreenShareAudio: lkproto.TrackSource_SCREEN_SHARE_AUDIO,
}

func toSources(ss []domain.MediaSource) []lkproto.TrackSource {
	out := make([]lkproto.TrackSource, 0, len(ss))
	for _, s := range ss {
		out = append(out, sources[s])
	}
	return out
}

func fromSource(s lkproto.TrackSource) domain.MediaSource {
	for d, l := range sources {
		if l == s {
			return d
		}
	}
	return ""
}

func (p *Provider) JoinToken(j ports.MediaJoin) (string, error) {
	at := auth.NewAccessToken(p.c.APIKey, p.c.APISecret).
		SetIdentity(j.Identity).
		SetMetadata(j.Metadata).
		SetValidFor(j.TTL).
		SetVideoGrant(&auth.VideoGrant{RoomJoin: true, Room: j.Room})
	g := at.GetGrants().Video
	// An empty source list would allow every source, so a fully muted
	// participant gets no publish grant at all.
	g.SetCanPublish(len(j.Permissions.Publish) > 0)
	g.SetCanPublishSources(toSources(j.Permissions.Publish))
	g.SetCanSubscribe(j.Permissions.Subscribe)
	g.SetCanPublishData(true)
	g.SetCanUpdateOwnMetadata(false)
	token, err := at.ToJWT()
	if err != nil {
		return "", fmt.Errorf("sign media token: %w", err)
	}
	return token, nil
}

// admin authorizes one room API call; an empty room is for listing rooms.
func (p *Provider) admin(ctx context.Context, room string) (context.Context, error) {
	grant := &auth.VideoGrant{RoomList: true}
	if room != "" {
		grant = &auth.VideoGrant{RoomAdmin: true, Room: room}
	}
	token, err := auth.NewAccessToken(p.c.APIKey, p.c.APISecret).SetVideoGrant(grant).SetValidFor(adminTokenTTL).ToJWT()
	if err != nil {
		return nil, fmt.Errorf("sign media admin token: %w", err)
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	return twirp.WithHTTPRequestHeaders(ctx, h)
}

func mapError(err error, op string) error {
	var te twirp.Error
	if errors.As(err, &te) && te.Code() == twirp.NotFound {
		return fmt.Errorf("%s: %w", op, domain.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", op, err)
}

func (p *Provider) RemoveParticipant(ctx context.Context, room, identity string) error {
	ctx, err := p.admin(ctx, room)
	if err != nil {
		return err
	}
	if _, err := p.rooms.RemoveParticipant(ctx, &lkproto.RoomParticipantIdentity{Room: room, Identity: identity}); err != nil {
		return mapError(err, "remove participant")
	}
	return nil
}

func (p *Provider) SetPermissions(ctx context.Context, room, identity string, perm domain.VoicePermissions) error {
	ctx, err := p.admin(ctx, room)
	if err != nil {
		return err
	}
	// The permission replaces the old one whole, so every field is set.
	_, err = p.rooms.UpdateParticipant(ctx, &lkproto.UpdateParticipantRequest{
		Room:     room,
		Identity: identity,
		Permission: &lkproto.ParticipantPermission{
			CanSubscribe:      perm.Subscribe,
			CanPublish:        len(perm.Publish) > 0,
			CanPublishData:    true,
			CanPublishSources: toSources(perm.Publish),
		},
	})
	if err != nil {
		return mapError(err, "update participant permissions")
	}
	return nil
}

func (p *Provider) ListRooms(ctx context.Context) ([]string, error) {
	ctx, err := p.admin(ctx, "")
	if err != nil {
		return nil, err
	}
	res, err := p.rooms.ListRooms(ctx, &lkproto.ListRoomsRequest{})
	if err != nil {
		return nil, mapError(err, "list rooms")
	}
	names := make([]string, len(res.GetRooms()))
	for i, r := range res.GetRooms() {
		names[i] = r.GetName()
	}
	return names, nil
}

func (p *Provider) ListParticipants(ctx context.Context, room string) ([]ports.MediaParticipant, error) {
	ctx, err := p.admin(ctx, room)
	if err != nil {
		return nil, err
	}
	res, err := p.rooms.ListParticipants(ctx, &lkproto.ListParticipantsRequest{Room: room})
	if err != nil {
		return nil, mapError(err, "list participants")
	}
	out := make([]ports.MediaParticipant, len(res.GetParticipants()))
	for i, pi := range res.GetParticipants() {
		out[i] = participant(pi)
	}
	return out, nil
}

func participant(pi *lkproto.ParticipantInfo) ports.MediaParticipant {
	p := ports.MediaParticipant{
		SID:      pi.GetSid(),
		Identity: pi.GetIdentity(),
		Metadata: pi.GetMetadata(),
		Active:   pi.GetState() == lkproto.ParticipantInfo_ACTIVE,
	}
	for _, t := range pi.GetTracks() {
		if s := fromSource(t.GetSource()); s != "" {
			p.Sources = append(p.Sources, s)
		}
	}
	return p
}

var webhookKinds = map[string]ports.MediaEventKind{
	webhook.EventParticipantJoined:            ports.MediaParticipantJoined,
	webhook.EventParticipantLeft:              ports.MediaParticipantLeft,
	webhook.EventParticipantConnectionAborted: ports.MediaParticipantLeft,
	webhook.EventTrackPublished:               ports.MediaTrackPublished,
	webhook.EventTrackUnpublished:             ports.MediaTrackUnpublished,
}

// ParseWebhook verifies a webhook request signed with the API secret and
// maps it to a media event. ok is false for events the hub does not use.
func (p *Provider) ParseWebhook(w http.ResponseWriter, r *http.Request) (ev ports.MediaEvent, ok bool, err error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBytes)
	e, err := webhook.ReceiveWebhookEvent(r, p.keys)
	if err != nil {
		return ports.MediaEvent{}, false, fmt.Errorf("verify media webhook: %w", err)
	}
	kind, known := webhookKinds[e.GetEvent()]
	if !known || e.GetParticipant() == nil {
		return ports.MediaEvent{}, false, nil
	}
	ev = ports.MediaEvent{Kind: kind, Room: e.GetRoom().GetName(), Participant: participant(e.GetParticipant())}
	if t := e.GetTrack(); t != nil {
		ev.Source = fromSource(t.GetSource())
	}
	return ev, true, nil
}
