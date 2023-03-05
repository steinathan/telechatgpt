package main

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"
	"github.com/rakyll/openai-go"
	"github.com/rakyll/openai-go/chat"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	retainHistory bool
	messages      = map[string][]*chat.Message{}
	promptName    = "prompt.txt"
)

func main() {
	// setup logger
	log.Logger = log.With().Caller().Logger()
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Environment files
	err := godotenv.Load()
	if err != nil {
		log.Debug().Msg(err.Error())
	}

	retainHistory = os.Getenv("RETAIN_HISTORY") == "true"

	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	if err := ConnectDB(); err != nil {
		log.Fatal().Msg(err.Error())
	}

	// start server
	StartServer()
}

// StartServer starts the telegram server
func StartServer() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(os.Getenv("TELEGRAM_API_KEY"), opts...)
	if err != nil {
		panic(err)
	}

	log.Debug().Msg("Telegram bot started!")
	b.Start(ctx)
}

// SendToChatGPT send a message to chatgpt
func SendToChatGPT(chatId, textMsg string) []*chat.Choice {
	var (
		ctx = context.Background()
		s   = openai.NewSession(os.Getenv("OPENAI_TOKEN"))

		// current chatgpt messages
		gptMsgs = make([]*chat.Message, 0)
	)

	// check if the user has a previous conversation
	prevMessages, err := FindMessages(chatId)
	if err != nil {
		log.Err(err)
	}

	// add system prompt if user is initially starting out the conversation
	if len(messages) == 0 {
		// create & add the systems prompt first
		prmptB, _ := os.ReadFile(promptName)
		log.Debug().Msg("fetching sys prompt..")
		gptMsgs = append(gptMsgs, &chat.Message{
			Role:    "user", // "system"
			Content: string(prmptB),
		})

		// add this current message
		gptMsgs = append(gptMsgs, &chat.Message{
			Role:    "user",
			Content: textMsg})
	} else {

		// add the whole users conversation and send to chatgpt
		// if retainHistory {
		// }
	}

	// update map of users outgoing message & current users prompt
	// only when we want to retain history
	if retainHistory {
		// send all messages associated with this user  to retain conversation history
	} else {
		// only send the current prompt with the systemPrompt
		messages[chatId] = []*chat.Message{
			&sysPrompt,
			{
				Role:    "user",
				Content: prompt,
			},
		}

	}

	// process request
	client := chat.NewClient(s, "gpt-3.5-turbo-0301")
	resp, err := client.CreateCompletion(ctx, &chat.CreateCompletionParams{
		Messages: gptMsgs,
	})
	if err != nil {
		log.Error().Msgf("Failed to complete: %v", err)
		return nil
	}

	// When done, loop and persit messages
	for _, gptMsg := range gptMsgs {
		newMsg := Message{
			ChatID:  chatId,
			Role:    gptMsg.Role,
			Content: gptMsg.Content,

			// metrics for this single chat session
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		_, err := CreateMessage(newMsg)
		if err != nil {
			log.Error().Msgf("unable to add system prompt: %v", err)
			return nil
		}
	}
	log.Info().
		Int("TotalTokens", resp.Usage.TotalTokens).
		Int("CompletionTokens", resp.Usage.CompletionTokens).
		Int("PromptTokens", resp.Usage.PromptTokens).
		Msg("usage")

	return resp.Choices
}

// handler
func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	outgoingMsg := update.Message.Text
	chatId := update.Message.Chat.ID
	log.Debug().Msg(outgoingMsg)

	// convert number to string
	chatIdStr := strconv.Itoa(int(chatId))
	chatResp := SendToChatGPT(chatIdStr, outgoingMsg)
	if chatResp == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   "unable to get chat response from server",
		})
		return
	}

	for _, choice := range chatResp {
		incomingMsg := choice.Message
		log.Printf("role=%q, content=%q", incomingMsg.Role, incomingMsg.Content)

		// update messages of this users request reply
		if retainHistory {
			messages[chatIdStr] = append(messages[chatIdStr], &chat.Message{
				Role:    incomingMsg.Role,
				Content: incomingMsg.Content,
			})

		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   incomingMsg.Content,
		})
	}

	// save the conversation for backup purposes
	msgBytes, _ := json.Marshal(messages)
	if err := os.WriteFile("conversations.json", msgBytes, 0644); err != nil {
		log.Err(err)
	}
}
