//go:build storage_all || storage_firestore

package firestore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/zitadel/zitadel/internal/storage"
	"github.com/zitadel/zitadel/internal/storage/nosql"
)

func init() {
	nosql.RegisterProvider("firestore", &Provider{})
	nosql.RegisterProvider("firebase", &Provider{})
}

type Provider struct{}

func (p *Provider) Name() string {
	return "firestore"
}

func (p *Provider) NewSessionRepository(ctx context.Context, cfg nosql.Config) (storage.SessionRepository, error) {
	client, err := firestore.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("firestore new client: %w", err)
	}
	return &SessionRepo{client: client}, nil
}

func (p *Provider) NewAuditRepository(ctx context.Context, cfg nosql.Config) (storage.AuditRepository, error) {
	client, err := firestore.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("firestore new client: %w", err)
	}
	return &AuditRepo{client: client}, nil
}

type SessionRepo struct {
	client *firestore.Client
}

type firestoreSessionDoc struct {
	ID         string                 `firestore:"id"`
	InstanceID string                 `firestore:"instance_id"`
	UserID     string                 `firestore:"user_id"`
	UserAgent  string                 `firestore:"user_agent"`
	IPAddress  string                 `firestore:"ip_address"`
	Data       map[string]interface{} `firestore:"data"`
	CreatedAt  time.Time              `firestore:"created_at"`
	ExpiresAt  time.Time              `firestore:"expires_at"` // Native GCP TTL policy field
}

func (r *SessionRepo) CreateSession(ctx context.Context, s *storage.Session) error {
	docRef := r.client.Collection("zitadel_sessions").Doc(fmt.Sprintf("%s_%s", s.InstanceID, s.ID))
	_, err := docRef.Set(ctx, firestoreSessionDoc{
		ID:         s.ID,
		InstanceID: s.InstanceID,
		UserID:     s.UserID,
		UserAgent:  s.UserAgent,
		IPAddress:  s.IPAddress,
		Data:       s.Data,
		CreatedAt:  s.CreatedAt,
		ExpiresAt:  s.ExpiresAt,
	})
	return err
}

func (r *SessionRepo) GetSession(ctx context.Context, instanceID, sessionID string) (*storage.Session, error) {
	docRef := r.client.Collection("zitadel_sessions").Doc(fmt.Sprintf("%s_%s", instanceID, sessionID))
	doc, err := docRef.Get(ctx)
	if err != nil {
		return nil, storage.ErrNotFound
	}
	var d firestoreSessionDoc
	if err := doc.DataTo(&d); err != nil {
		return nil, err
	}
	if !d.ExpiresAt.IsZero() && time.Now().After(d.ExpiresAt) {
		return nil, storage.ErrNotFound
	}
	return &storage.Session{
		ID:         d.ID,
		InstanceID: d.InstanceID,
		UserID:     d.UserID,
		UserAgent:  d.UserAgent,
		IPAddress:  d.IPAddress,
		Data:       d.Data,
		CreatedAt:  d.CreatedAt,
		ExpiresAt:  d.ExpiresAt,
	}, nil
}

func (r *SessionRepo) UpdateSession(ctx context.Context, s *storage.Session) error {
	return r.CreateSession(ctx, s)
}

func (r *SessionRepo) DeleteSession(ctx context.Context, instanceID, sessionID string) error {
	docRef := r.client.Collection("zitadel_sessions").Doc(fmt.Sprintf("%s_%s", instanceID, sessionID))
	_, err := docRef.Delete(ctx)
	return err
}

type AuditRepo struct {
	client *firestore.Client
}

type firestoreAuditDoc struct {
	ID           string                 `firestore:"id"`
	InstanceID   string                 `firestore:"instance_id"`
	ActorID      string                 `firestore:"actor_id"`
	Action       string                 `firestore:"action"`
	ResourceType string                 `firestore:"resource_type"`
	ResourceID   string                 `firestore:"resource_id"`
	Status       string                 `firestore:"status"`
	Details      map[string]interface{} `firestore:"details"`
	Timestamp    time.Time              `firestore:"timestamp"`
}

func (r *AuditRepo) Log(ctx context.Context, a *storage.AuditEntry) error {
	docRef := r.client.Collection("zitadel_audit_logs").Doc(fmt.Sprintf("%s_%s", a.InstanceID, a.ID))
	_, err := docRef.Set(ctx, firestoreAuditDoc{
		ID:           a.ID,
		InstanceID:   a.InstanceID,
		ActorID:      a.ActorID,
		Action:       a.Action,
		ResourceType: a.ResourceType,
		ResourceID:   a.ResourceID,
		Status:       a.Status,
		Details:      a.Details,
		Timestamp:    a.Timestamp,
	})
	return err
}

func (r *AuditRepo) Query(ctx context.Context, filter storage.AuditFilter) ([]*storage.AuditEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	q := r.client.Collection("zitadel_audit_logs").
		Where("instance_id", "==", filter.InstanceID).
		OrderBy("timestamp", firestore.Desc).
		Limit(limit)

	iter := q.Documents(ctx)
	defer iter.Stop()

	var entries []*storage.AuditEntry
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var d firestoreAuditDoc
		if err := doc.DataTo(&d); err != nil {
			continue
		}
		entries = append(entries, &storage.AuditEntry{
			ID:           d.ID,
			InstanceID:   d.InstanceID,
			ActorID:      d.ActorID,
			Action:       d.Action,
			ResourceType: d.ResourceType,
			ResourceID:   d.ResourceID,
			Status:       d.Status,
			Details:      d.Details,
			Timestamp:    d.Timestamp,
		})
	}
	return entries, nil
}
