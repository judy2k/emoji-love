package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env"
	"github.com/dghubble/oauth1"
	"github.com/judy2k/go-twitter/twitter"
)

type config struct {
	ConsumerKey    string `env:"CONSUMER_KEY,required"`
	ConsumerSecret string `env:"CONSUMER_SECRET,required"`
	Token          string `env:"TOKEN,required"`
	TokenSecret    string `env:"TOKEN_SECRET,required"`
}

type emojiLove struct {
	httpClient    *http.Client
	twitterClient *twitter.Client
}

func containsEmoji(text string) bool {
	for _, r := range text {
		if (0x2190 <= r && r <= 0x21FF) ||
			(0x2600 <= r && r <= 0x26ff) ||
			(0x2700 <= r && r <= 0x27BF) ||
			(0x3000 <= r && r <= 0x303F) ||
			(0x1f300 <= r && r <= 0x1f64f) ||
			(0x1F680 <= r && r <= 0x1F6FF) {
			return true
		}
	}
	return false
}

func newApp(consumerKey, consumerSecret, token, tokenSecret string) *emojiLove {
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	t := oauth1.NewToken(token, tokenSecret)

	httpClient := config.Client(oauth1.NoContext, t)
	twitterClient := twitter.NewClient(httpClient)

	return &emojiLove{
		httpClient:    httpClient,
		twitterClient: twitterClient,
	}
}

func (app *emojiLove) follow(userIDs []string) (*twitter.Stream, error) {
	params := &twitter.StreamFilterParams{
		Follow:        userIDs,
		StallWarnings: twitter.Bool(true),
	}

	return app.twitterClient.Streams.Filter(params)
}

func (app *emojiLove) lookupUserID(username string) (string, error) {
	users, _, err := app.twitterClient.Users.Lookup(&twitter.UserLookupParams{
		ScreenName: []string{username},
	})
	if err != nil {
		return "", err
	}
	user := users[0]
	return user.IDStr, nil
}

func (app *emojiLove) handleChan(m chan interface{}) {
	for message := range m {
		switch message := message.(type) {
		case error:
			fmt.Fprintf(os.Stderr, "Received an error: %s", message)
		case *twitter.Tweet:
			if containsEmoji(message.Text) {
				fmt.Printf("\U0001F603  Tweet %d contained emoji - love it!\n", message.ID)
				app.twitterClient.Favorites.Create(&twitter.FavoriteCreateParams{ID: message.ID})
			} else {
				fmt.Printf("\U0001F61E  Tweet %d did not contain any emoji.\n", message.ID)
			}
		default:
			fmt.Printf("Unsupported message type: %t", message)
		}
	}
}

func main() {
	username := flag.String("username", "", "The username to monitor")
	flag.Parse()

	cfg := config{}
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Printf("%+v\n", err)
	}

	app := newApp(
		cfg.ConsumerKey,
		cfg.ConsumerSecret,
		cfg.Token,
		cfg.TokenSecret)

	userID, err := app.lookupUserID(*username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can not look up user ID: %s: %v", *username, err)
		os.Exit(-1)
	}

	stream, err := app.follow([]string{userID})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot follow %s! %v", userID, err)
		os.Exit(-1)
	}

	go app.handleChan(stream.Messages)

	// Wait for SIGINT and SIGTERM (HIT CTRL-C)
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch)

	stream.Stop()
}
