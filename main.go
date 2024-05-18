package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	irc "github.com/fluffle/goirc/client"
	anthropic "github.com/liushuangls/go-anthropic/v2"
)

const maxTokens = 100
const maxIRCMessageLength = 420
const maxContextMessages = 20
const shortAnswerHint = " (limit answer to 200 characters)"

var anthropicClient *anthropic.Client
var contextMessagesPerChannel = make(map[string][]*ContextMessage)

type Config struct {
	AnthropicKey string   `json:"anthropic_api_key"`
	SystemPrompt string   `json:"system_prompt"`
	IrcServer    string   `json:"irc_server"`
	IrcPort      int      `json:"irc_port"`
	IrcNick      string   `json:"irc_nick"`
	IrcPassword  string   `json:"irc_password"`
	IrcChannels  []string `json:"irc_channels"`
}

type ContextMessage struct {
	Timestamp int64
	Role      string
	Content   string
	Response  *ContextMessage // a user message's response points to the assistant's answer
}

func NewContextMessage(role string, content string) *ContextMessage {
	return &ContextMessage{
		Timestamp: time.Now().Unix(),
		Role:      role,
		Content:   content,
	}
}

func main() {
	// Define the command-line flag for the configuration file path
	configFile := flag.String("c", "", "path to the configuration file")
	flag.Parse()

	// Check if the -c flag is provided
	if *configFile == "" {
		log.Println("Error: -c flag is required.")
		flag.Usage()
		os.Exit(1)
	}

	config, done := readConfig(configFile)
	if done {
		return
	}

	// Or, create a config and fiddle with it first:
	cfg := irc.NewConfig(config.IrcNick, config.IrcNick, config.IrcNick)
	cfg.SSL = true
	cfg.SSLConfig = &tls.Config{ServerName: config.IrcServer}
	cfg.Server = fmt.Sprintf("%s:%d", config.IrcServer, config.IrcPort)
	cfg.NewNick = func(n string) string { return n + "_" }

	// Create the Anthropic client with the API key from the configuration
	anthropicClient = anthropic.NewClient(config.AnthropicKey)

	ircClient := irc.Client(cfg)
	ircClient.HandleFunc(irc.CONNECTED, handleConnected(cfg, config))
	ircClient.HandleFunc(irc.NOTICE, handleNotice(config))
	ircClient.HandleFunc(irc.PRIVMSG, handlePrivMsg(config))

	// And a signal on disconnect
	quit := make(chan bool)
	ircClient.HandleFunc(irc.DISCONNECTED, func(conn *irc.Conn, line *irc.Line) { quit <- true })

	// Tell irc client to connect.
	if err := ircClient.Connect(); err != nil {
		log.Printf("Connection error: %s\n", err.Error())
	}

	// Wait for disconnect
	<-quit
}

// reads the configuration file
func readConfig(configFile *string) (Config, bool) {
	// Read the configuration file
	file, err := os.Open(*configFile)
	if err != nil {
		log.Printf("Error opening config file: %v\n", err)
		return Config{}, true
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Failed to close file: %v", err)
		}
	}(file)

	// Parse the JSON configuration
	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		log.Printf("Error parsing config file: %v\n", err)
		return Config{}, true
	}
	return config, false
}

// handles CONNECTED events
func handleConnected(cfg *irc.Config, config Config) func(conn *irc.Conn, line *irc.Line) {
	return func(conn *irc.Conn, line *irc.Line) {
		log.Printf("Connected to %s, identify to NickServ...\n", cfg.Server)
		conn.Privmsg("NickServ", "IDENTIFY "+config.IrcPassword)
	}
}

// handles NOTICE events
func handleNotice(config Config) func(conn *irc.Conn, line *irc.Line) {
	return func(conn *irc.Conn, line *irc.Line) {
		if line.Nick == "NickServ" {
			log.Printf("NickServ: %s\n", line.Text())
			if strings.Contains(line.Text(), "You are now identified") {
				log.Printf("Identified, joining channels...\n")
				for _, channel := range config.IrcChannels {
					conn.Join(channel)
				}
			}
		}
	}
}

// handles PRIVMSG events
func handlePrivMsg(config Config) func(conn *irc.Conn, line *irc.Line) {
	return func(conn *irc.Conn, line *irc.Line) {
		log.Printf("PRIVMSG %s: %s\n", line.Target(), line.Text())
		// if the string starts with the bot's nick and a colon
		if strings.HasPrefix(line.Text(), conn.Me().Nick+":") {
			// remove the bot's nick and the colon
			text := strings.TrimPrefix(line.Text(), conn.Me().Nick+":")
			// remove leading and trailing whitespace
			text = strings.TrimSpace(text)
			// send the message to Anthropic
			log.Printf("Anthropic: %s\n", text)

			response, err := respond(config, line.Target(), text)

			if err != nil {
				log.Printf("Error responding to Anthropic: %v\n", err)
				conn.Privmsg(line.Target(), sanitizeResponse(fmt.Sprintf("Claude had a brainfart: %v", err)))
			} else {
				conn.Privmsg(line.Target(), response)
			}
		}
	}
}

// responds to a user message using the Anthropic API
func respond(config Config, channel, text string) (string, error) {

	// Get the context messages for the current channel
	contextMessages, ok := contextMessagesPerChannel[channel]
	if !ok {
		contextMessages = []*ContextMessage{}
	}

	// Get the current timestamp
	currentTimestamp := time.Now().Unix()

	// Remove messages older than two hours
	for i := 0; i < len(contextMessages); i++ {
		if currentTimestamp-contextMessages[i].Timestamp > 2*60*60 {
			// Remove the message at index i
			contextMessages = append(contextMessages[:i], contextMessages[i+1:]...)
			i-- // Adjust the index to account for the removed message
		}
	}

	// Add the user's message to the context
	userMessage := NewContextMessage("user", text+shortAnswerHint)
	contextMessages = append(contextMessages, userMessage)

	// Limit the context messages
	if len(contextMessages) > maxContextMessages {
		// remove the first two messages (user query and assistant response)
		contextMessages = contextMessages[2:]
	}

	// Update the context messages for the channel
	contextMessagesPerChannel[channel] = contextMessages

	// Prepare the messages for the Anthropic API request
	var messages []anthropic.Message
	for _, msg := range contextMessages {
		messages = append(messages, anthropic.Message{
			Role: msg.Role,
			Content: []anthropic.MessageContent{
				{
					Type: anthropic.MessagesContentTypeText,
					Text: &msg.Content,
				},
			},
		})
		if msg.Response != nil {
			messages = append(messages, anthropic.Message{
				Role: msg.Response.Role,
				Content: []anthropic.MessageContent{
					{
						Type: anthropic.MessagesContentTypeText,
						Text: &msg.Response.Content,
					},
				},
			})
		}
	}

	resp, err := anthropicClient.CreateMessages(
		context.Background(),
		anthropic.MessagesRequest{
			Model:     anthropic.ModelClaude3Haiku20240307,
			Messages:  messages,
			MaxTokens: maxTokens,
			System:    config.SystemPrompt,
		})
	if err != nil {
		log.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}
	log.Printf("Anthropic response: %s\n", *resp.Content[0].Text)

	// Add the assistant's response to the context
	saneResponse := sanitizeResponse(*resp.Content[0].Text)
	userMessage.Response = NewContextMessage("assistant", saneResponse)

	return saneResponse, nil
}

// sanitizeResponse removes excessive whitespace and limits the length of the response
func sanitizeResponse(content string) string {
	// Replace multiple whitespace characters with a single space
	content = strings.Join(strings.Fields(content), " ")

	// Trim leading and trailing whitespace
	content = strings.TrimSpace(content)

	// Limit the response length if it exceeds maxIRCMessageLength
	if len(content) > maxIRCMessageLength {
		content = content[:maxIRCMessageLength]
	}

	return content
}
