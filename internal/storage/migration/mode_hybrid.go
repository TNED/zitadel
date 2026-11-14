//go:build storage_dynamodb || storage_firestore

package migration

// DefaultIsPureRelational is false when NoSQL build tags are active.
const DefaultIsPureRelational = false
