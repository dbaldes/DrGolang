# IRC Bot with Anthropic API

This is a simple IRC bot written in Go that uses the Anthropic API to answer messages directed at it. The bot connects to an IRC server, joins specified channels, and responds to messages that start with its nickname followed by a colon.

## Features

- Connects to an IRC server using SSL/TLS
- Identifies with NickServ using the provided password
- Joins multiple IRC channels specified in the configuration file
- Listens for messages directed at the bot (starting with the bot's nickname followed by a colon)
- Sends the message content to the Anthropic API for processing
- Responds with the generated answer from the Anthropic API
- Maintains a context of recent messages per channel to provide contextual responses
- Limits the response length to fit within IRC message size limits
- Removes old context messages to keep the context relevant and prevent excessive token usage

## Learning Go

I created this project as a way to learn and practice Go programming. It serves as a practical example of using various Go concepts and libraries, such as:

- Parsing JSON configuration files
- Handling command-line flags
- Connecting to an IRC server using the `goirc` library
- Making HTTP requests to the Anthropic API using the `go-anthropic` library
- Structuring a Go project with multiple files and packages
- Managing data structures and pointers
- Implementing error handling and logging

Throughout the development process, I aimed to follow Go best practices and conventions to write idiomatic and efficient code.

## Getting Started

To run the IRC bot locally, follow these steps:

1. Clone the repository:

   ```
   git clone https://github.com/your-username/irc-bot.git
   ```

2. Install the required dependencies:

   ```
   go get github.com/fluffle/goirc/client
   go get github.com/liushuangls/go-anthropic/v2
   ```

3. Create a configuration file (`config.json`) with the necessary settings:

   ```json
   {
     "anthropic_api_key": "your-anthropic-api-key",
     "system_prompt": "your-system-prompt",
     "irc_server": "irc.example.com",
     "irc_port": 6697,
     "irc_nick": "your-bot-nickname",
     "irc_password": "your-nickserv-password",
     "irc_channels": [
       "#channel1",
       "#channel2"
     ]
   }
   ```

4. Build and run the bot:

   ```
   go build -o irc-bot
   ./irc-bot -c config.json
   ```

The bot will connect to the specified IRC server, identify with NickServ, join the configured channels, and start responding to messages.

## Contributing

Contributions to this project are welcome! If you find any issues or have suggestions for improvements, please feel free to open an issue or submit a pull request.

## License

This project is licensed under the [MIT License](LICENSE).
