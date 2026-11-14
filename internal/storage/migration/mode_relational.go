//go:build !storage_dynamodb && !storage_firestore

package migration

// DefaultIsPureRelational is true when no NoSQL build tags are active.
const DefaultIsPureRelational = true
