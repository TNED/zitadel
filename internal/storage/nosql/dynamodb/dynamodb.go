//go:build storage_all || storage_dynamodb

package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	aws_cfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/zitadel/zitadel/internal/storage"
	"github.com/zitadel/zitadel/internal/storage/nosql"
)

func init() {
	nosql.RegisterProvider("dynamodb", &Provider{})
}

type Provider struct{}

func (p *Provider) Name() string {
	return "dynamodb"
}

func (p *Provider) NewSessionRepository(ctx context.Context, cfg nosql.Config) (storage.SessionRepository, error) {
	client, err := initClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	tableName := cfg.TableName
	if tableName == "" {
		tableName = "zitadel_sessions"
	}
	return &SessionRepo{client: client, table: tableName}, nil
}

func (p *Provider) NewAuditRepository(ctx context.Context, cfg nosql.Config) (storage.AuditRepository, error) {
	client, err := initClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	tableName := "zitadel_audit_logs"
	if cfg.TableName != "" {
		tableName = cfg.TableName + "_audit"
	}
	return &AuditRepo{client: client, table: tableName}, nil
}

func initClient(ctx context.Context, cfg nosql.Config) (*dynamodb.Client, error) {
	opts := []func(*aws_cfg.LoadOptions) error{}
	if cfg.Region != "" {
		opts = append(opts, aws_cfg.WithRegion(cfg.Region))
	}
	awscfg, err := aws_cfg.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws load config: %w", err)
	}

	clientOpts := []func(*dynamodb.Options){}
	if cfg.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}
	return dynamodb.NewFromConfig(awscfg, clientOpts...), nil
}

// SessionRepo backed by DynamoDB with native TTL
type SessionRepo struct {
	client *dynamodb.Client
	table  string
}

type ddbSessionItem struct {
	PK        string                 `dynamodbav:"pk"` // instance_id#session_id
	ID        string                 `dynamodbav:"id"`
	InstanceID string                `dynamodbav:"instance_id"`
	UserID    string                 `dynamodbav:"user_id"`
	UserAgent string                 `dynamodbav:"user_agent"`
	IPAddress string                 `dynamodbav:"ip_address"`
	Data      map[string]interface{} `dynamodbav:"data"`
	CreatedAt int64                  `dynamodbav:"created_at"`
	ExpiresAt int64                  `dynamodbav:"expires_at"` // Native DynamoDB TTL attribute
}

func (r *SessionRepo) CreateSession(ctx context.Context, s *storage.Session) error {
	item := ddbSessionItem{
		PK:         fmt.Sprintf("%s#%s", s.InstanceID, s.ID),
		ID:         s.ID,
		InstanceID: s.InstanceID,
		UserID:     s.UserID,
		UserAgent:  s.UserAgent,
		IPAddress:  s.IPAddress,
		Data:       s.Data,
		CreatedAt:  s.CreatedAt.Unix(),
		ExpiresAt:  s.ExpiresAt.Unix(), // native TTL
	}
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.table),
		Item:      av,
	})
	return err
}

func (r *SessionRepo) GetSession(ctx context.Context, instanceID, sessionID string) (*storage.Session, error) {
	pk := fmt.Sprintf("%s#%s", instanceID, sessionID)
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Item) == 0 {
		return nil, storage.ErrNotFound
	}
	var item ddbSessionItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, err
	}

	// Check expiration in case DynamoDB TTL hasn't reaped it yet
	if item.ExpiresAt > 0 && time.Now().Unix() > item.ExpiresAt {
		return nil, storage.ErrNotFound
	}

	return &storage.Session{
		ID:         item.ID,
		InstanceID: item.InstanceID,
		UserID:     item.UserID,
		UserAgent:  item.UserAgent,
		IPAddress:  item.IPAddress,
		Data:       item.Data,
		CreatedAt:  time.Unix(item.CreatedAt, 0),
		ExpiresAt:  time.Unix(item.ExpiresAt, 0),
	}, nil
}

func (r *SessionRepo) UpdateSession(ctx context.Context, s *storage.Session) error {
	return r.CreateSession(ctx, s)
}

func (r *SessionRepo) DeleteSession(ctx context.Context, instanceID, sessionID string) error {
	pk := fmt.Sprintf("%s#%s", instanceID, sessionID)
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	return err
}

// AuditRepo backed by DynamoDB append-only logs
type AuditRepo struct {
	client *dynamodb.Client
	table  string
}

type ddbAuditItem struct {
	PK           string                 `dynamodbav:"pk"` // instance_id
	SK           string                 `dynamodbav:"sk"` // timestamp#id
	ID           string                 `dynamodbav:"id"`
	InstanceID   string                 `dynamodbav:"instance_id"`
	ActorID      string                 `dynamodbav:"actor_id"`
	Action       string                 `dynamodbav:"action"`
	ResourceType string                 `dynamodbav:"resource_type"`
	ResourceID   string                 `dynamodbav:"resource_id"`
	Status       string                 `dynamodbav:"status"`
	Details      map[string]interface{} `dynamodbav:"details"`
	Timestamp    int64                  `dynamodbav:"timestamp"`
}

func (r *AuditRepo) Log(ctx context.Context, a *storage.AuditEntry) error {
	item := ddbAuditItem{
		PK:           a.InstanceID,
		SK:           fmt.Sprintf("%d#%s", a.Timestamp.UnixNano(), a.ID),
		ID:           a.ID,
		InstanceID:   a.InstanceID,
		ActorID:      a.ActorID,
		Action:       a.Action,
		ResourceType: a.ResourceType,
		ResourceID:   a.ResourceID,
		Status:       a.Status,
		Details:      a.Details,
		Timestamp:    a.Timestamp.Unix(),
	}
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.table),
		Item:      av,
	})
	return err
}

func (r *AuditRepo) Query(ctx context.Context, filter storage.AuditFilter) ([]*storage.AuditEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		KeyConditionExpression: aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: filter.InstanceID},
		},
		ScanIndexForward: aws.Bool(false), // newest first
		Limit:            aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, err
	}
	var entries []*storage.AuditEntry
	for _, itemMap := range out.Items {
		var item ddbAuditItem
		if err := attributevalue.UnmarshalMap(itemMap, &item); err != nil {
			continue
		}
		entries = append(entries, &storage.AuditEntry{
			ID:           item.ID,
			InstanceID:   item.InstanceID,
			ActorID:      item.ActorID,
			Action:       item.Action,
			ResourceType: item.ResourceType,
			ResourceID:   item.ResourceID,
			Status:       item.Status,
			Details:      item.Details,
			Timestamp:    time.Unix(item.Timestamp, 0),
		})
	}
	return entries, nil
}
