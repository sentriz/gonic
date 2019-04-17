package utilities

import "fmt"

func ValidateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("please enter the username")
	}
	return nil
}

func ValidatePasswords(pOne, pTwo string) error {
	if pOne == "" || pTwo == "" {
		return fmt.Errorf("please enter the password twice")
	}
	if !(pOne == pTwo) {
		return fmt.Errorf("the two passwords entered were not the same")
	}
	return nil
}

func ValidateAPIKey(apiKey, secret string) error {
	if apiKey == "" || secret == "" {
		return fmt.Errorf("please enter both the api key and secret")
	}
	return nil
}
