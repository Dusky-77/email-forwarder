package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GetClient returns a ready to use gmail service for one account
// first run will print a url and ask you to paste back a code, after that
// the token gets cached to disk and this just works silently
func GetClient(ctx context.Context, credentialsFile, tokenFile string) (*gmailapi.Service, error) {
	credBytes, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("reading credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(credBytes, gmailapi.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}

	token, err := tokenFromFile(tokenFile)
	if err != nil {
		token, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		saveToken(tokenFile, token)
	}

	client := config.Client(ctx, token)

	return gmailapi.NewService(ctx, option.WithHTTPClient(client))
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Println("go to this url in your browser and authorize access:")
	fmt.Println(authURL)
	fmt.Print("paste the authorization code here: ")

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return nil, fmt.Errorf("reading auth code: %w", err)
	}

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("exchanging auth code: %w", err)
	}

	return token, nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

func saveToken(file string, token *oauth2.Token) {
	log.Printf("saving new token to %s", file)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Printf("warning: could not save token: %v", err)
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
