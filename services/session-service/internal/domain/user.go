package domain

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the domain layer
type User struct {
	ID        string    `gorm:"primaryKey"`
	Email     string    `gorm:"uniqueIndex;not null"`
	Username  string    `gorm:"uniqueIndex;not null"`
	FullName  string    `gorm:"not null"`
	Password  string    `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewUser(email, username, fullName, password string) (*User, error) {
	if email == "" || username == "" || fullName == "" || password == "" {
		return nil, errors.New("all fields are required")
	}

	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters long")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &User{
		ID:        generateID(),
		Email:     email,
		Username:  username,
		FullName:  fullName,
		Password:  string(hashedPassword),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

func (u *User) UpdatePassword(newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	u.Password = string(hashedPassword)
	u.UpdatedAt = time.Now()
	return nil
}

func (u *User) UpdateProfile(email, username, fullName string) error {
	if email == "" || username == "" || fullName == "" {
		return errors.New("all fields are required")
	}

	u.Email = email
	u.Username = username
	u.FullName = fullName
	u.UpdatedAt = time.Now()
	return nil
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
