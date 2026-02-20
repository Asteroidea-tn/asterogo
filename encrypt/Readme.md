# 🔐 Encryption Package

A simple, secure, and reusable encryption package for Go applications with automatic struct field encryption/decryption using tags.

## Features

✅ AES-256-GCM encryption (industry standard)  
✅ Automatic encryption/decryption with struct tags  
✅ Bun ORM integration with hooks  
✅ Base64 encoding for database storage  
✅ Thread-safe  
✅ Easy to use  

## Installation
```bash
go get github.com/yourusername/encryption
```

## Quick Start

### 1. Setup
```go
package main

import (
    "log"
    "github.com/yourusername/encryption"
)

func main() {
    // Option 1: From environment variable
    config, err := encryption.NewConfigFromEnv("ENCRYPTION_KEY")
    if err != nil {
        log.Fatal(err)
    }

    // Option 2: From your config struct
    config, err := encryption.NewConfig(yourConfig.EncryptionKey)
    if err != nil {
        log.Fatal(err)
    }

    // Create encryption service
    encryptor, err := encryption.NewService(config)
    if err != nil {
        log.Fatal(err)
    }
}
```

### 2. Environment Setup

**.env** or **.env.prod**
```
ENCRYPTION_KEY=your-32-character-secret-key!!
```

**Important:** Key must be exactly 16, 24, or 32 bytes for AES-128, AES-192, or AES-256.

### 3. Define Your Model
```go
type User struct {
    bun.BaseModel `bun:"table:users,alias:u"`

    ID        int64     `bun:"id,pk,autoincrement" json:"id"`
    Name      string    `bun:"name" json:"name"`
    Email     string    `bun:"email" json:"email" encrypt:"true"`
    Phone     string    `bun:"phone" json:"phone" encrypt:"true"`
    Address   string    `bun:"address" json:"address" encrypt:"true"`
    CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

// Global encryption service
var encryptionService *encryption.Service

// BeforeInsert - encrypts before INSERT
func (u *User) BeforeInsert(ctx context.Context, query *bun.InsertQuery) error {
    return encryptionService.EncryptStruct(u)
}

// BeforeUpdate - encrypts before UPDATE
func (u *User) BeforeUpdate(ctx context.Context, query *bun.UpdateQuery) error {
    return encryptionService.EncryptStruct(u)
}

// AfterSelect - decrypts after SELECT
func (u *User) AfterSelect(ctx context.Context) error {
    return encryptionService.DecryptStruct(u)
}
```

### 4. Use in Your Application
```go
// INSERT
user := &User{
    Name:    "John Doe",
    Email:   "john@example.com",   // Auto-encrypted
    Phone:   "+1234567890",         // Auto-encrypted
    Address: "123 Main St",         // Auto-encrypted
}
_, err := db.NewInsert().Model(user).Exec(ctx)

// SELECT
user := new(User)
err := db.NewSelect().Model(user).Where("id = ?", 1).Scan(ctx)
// user.Email, user.Phone, user.Address are auto-decrypted!

// UPDATE
user.Email = "newemail@example.com"
_, err := db.NewUpdate().Model(user).WherePK().Exec(ctx)

// LIST
var users []*User
err := db.NewSelect().Model(&users).Scan(ctx)
// All users are auto-decrypted!
```

## API Reference

### Service Methods
```go
// Basic encryption/decryption
func (s *Service) Encrypt(plaintext string) (string, error)
func (s *Service) Decrypt(ciphertext string) (string, error)

// Byte slice encryption/decryption
func (s *Service) EncryptBytes(plaintext []byte) ([]byte, error)
func (s *Service) DecryptBytes(ciphertext []byte) ([]byte, error)

// Struct-based encryption/decryption (tag-based)
func (s *Service) EncryptStruct(v interface{}) error
func (s *Service) DecryptStruct(v interface{}) error

// Field-based encryption/decryption (by name)
func (s *Service) EncryptFields(v interface{}, fieldNames ...string) error
func (s *Service) DecryptFields(v interface{}, fieldNames ...string) error
```

## Usage Methods

### Method 1: Automatic with Tags (Recommended)

Add `encrypt:"true"` tag to fields:
```go
type User struct {
    Email string `encrypt:"true"`
    Phone string `encrypt:"true"`
}

// Bun hooks handle everything automatically
```

### Method 2: Manual Struct Encryption
```go
user := &User{Email: "test@example.com"}

// Encrypt all tagged fields
encryptor.EncryptStruct(user)

// Decrypt all tagged fields
encryptor.DecryptStruct(user)
```

### Method 3: Field-Specific Encryption
```go
user := &User{Email: "test@example.com", Phone: "+123"}

// Encrypt specific fields
encryptor.EncryptFields(user, "Email", "Phone")

// Decrypt specific fields
encryptor.DecryptFields(user, "Email", "Phone")
```

### Method 4: Direct String Encryption
```go
encrypted, err := encryptor.Encrypt("sensitive data")
decrypted, err := encryptor.Decrypt(encrypted)
```

## Security Best Practices

1. **Never hardcode encryption keys** - use environment variables or secret managers
2. **Use 32-byte keys** for AES-256 (strongest)
3. **Rotate keys periodically** (implement key versioning if needed)
4. **Keep keys separate from database** - app layer encryption is more secure
5. **Use HTTPS** - encryption at rest doesn't protect data in transit
6. **Limit access** - not all fields need encryption

## Database Storage

Encrypted data is stored as **base64-encoded strings**, so use `TEXT` or `VARCHAR` columns:
```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255),
    email TEXT,        -- Stores encrypted data
    phone TEXT,        -- Stores encrypted data
    address TEXT,      -- Stores encrypted data
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Error Handling
```go
encrypted, err := encryptor.Encrypt("data")
if err != nil {
    switch err {
    case encryption.ErrMissingKey:
        // Handle missing key
    case encryption.ErrInvalidKeyLength:
        // Handle invalid key length
    case encryption.ErrEncryptionFailed:
        // Handle encryption failure
    }
}
```

## Examples

See the `examples/` directory for:
- `basic_example.go` - Basic usage without ORM
- `bun_example.go` - Full Bun integration
- `repository_example.go` - Repository pattern

