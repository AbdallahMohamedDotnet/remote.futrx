package googleoauth

import (
	"context"
	"encoding/json"
	"fmt"

	serviceauth "github.com/Kings-Of-The-Web/remote.futrx.dev/internal/service/auth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Client struct {
	oauth *oauth2.Config
}

func New(clientID, clientSecret, redirectURL string) *Client {
	return &Client{
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

func (c *Client) AuthCodeURL(state string) string {
	return c.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (c *Client) ExchangeUser(ctx context.Context, code string) (serviceauth.User, error) {
	token, err := c.oauth.Exchange(ctx, code)
	if err != nil {
		return serviceauth.User{}, fmt.Errorf("token exchange failed: %w", err)
	}

	client := c.oauth.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return serviceauth.User{}, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return serviceauth.User{}, fmt.Errorf("userinfo non-200: %s", resp.Status)
	}

	var userInfo struct {
		Sub     string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return serviceauth.User{}, fmt.Errorf("userinfo parse: %w", err)
	}
	return serviceauth.User{
		Email:   userInfo.Email,
		Sub:     userInfo.Sub,
		Name:    userInfo.Name,
		Picture: userInfo.Picture,
	}, nil
}
