package inst

import (
	"context"
	"fmt"
	"github.com/Davincible/goinsta"
	"github.com/rendizi/stay-connected-inst/internal/redis"
	"github.com/rendizi/stay-connected-inst/pkg/logger"
	"log"
)

func Login(login string, password string) (*goinsta.Instagram, error) {
	var err error
	var instaCookies string
	var insta *goinsta.Instagram

	instaCookies, err = redis.GetCookies(context.Background(), login)
	if err != nil {
		insta = goinsta.New(login, password)
		err = insta.Login()
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to login to instagram: %w", err))
			log.Fatalf("failed to login to Instagram: %w", err)
			return nil, fmt.Errorf("failed to login to Instagram: %w", err)
		}
		logger.Info("Logged in successfully")
		return insta, nil
	}

	insta, err = goinsta.ImportFromBase64String(instaCookies)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Instagram cookies: %w", err)
	}

	err = insta.OpenApp()
	if err != nil {
		insta = goinsta.New(login, password)
		err = insta.Login()
		if err != nil {
			log.Fatalf("failed to re-login to Instagram: %w", err)
			return nil, fmt.Errorf("failed to re-login to Instagram: %w", err)
		}
	}

	return insta, nil
}

func EntryContainsDate(entry, date string) bool {
	return len(entry) > 10 && entry[len(entry)-10:] == date
}
