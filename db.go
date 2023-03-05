package main

import (
	"github.com/rs/zerolog/log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	dbFile = "chats.db"
	DB     *gorm.DB
)

type Message struct {
	gorm.Model
	ID      uint   `gorm:"primaryKey" json:"id"`
	ChatID  string `json:"chatId,omitempty"`  // telegrams conversation id
	Role    string `json:"role,omitempty"`    // chatgpt role
	Content string `json:"content,omitempty"` // message content

	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// ConnectDB
func ConnectDB() error {
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{
		Logger: logger.Default,
	})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&Message{})

	DB = db
	log.Debug().Msg("database migrated")
	return nil
}

// FindMessages finds the prevous users conversations from the telegrams conversation id
func FindMessages(chatId string) ([]Message, error) {
	var messages []Message

	err := DB.Where(&Message{
		ChatID: chatId,
	}).Find(&messages).Error

	if err != nil {
		return nil, err
	}
	return messages, nil
}

// CreateMessage creates a new chat
func CreateMessage(msg Message) (*Message, error) {
	if err := DB.Create(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}
